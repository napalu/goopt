package goopt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/input"
	"github.com/napalu/goopt/v2/internal/messages"

	"github.com/napalu/goopt/v2/completion"
	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/internal/parse"
	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/types/orderedmap"
	"github.com/napalu/goopt/v2/types/queue"
	"github.com/napalu/goopt/v2/validation"
	"golang.org/x/text/language"
)

func (p *Parser) parseFlag(state parse.State, currentCommandPath string) bool {
	stripped := strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc)
	flag := p.flagOrShortFlag(stripped, currentCommandPath)
	flagInfo, found := p.acceptedFlags.Get(flag)

	if !found {
		flagInfo, found = p.acceptedFlags.Get(stripped)
		if found {
			flag = stripped
		}
	}

	// If not found, try translation lookup
	if !found {
		if canonical, ok := p.translationRegistry.GetCanonicalFlagName(stripped, p.GetLanguage()); ok {
			// The canonical name might already include command context (e.g., "flag@command")
			// Try it as-is first
			flag = canonical
			flagInfo, found = p.acceptedFlags.Get(flag)

			// If not found and we have a command path, try building the full path
			if !found && currentCommandPath != "" {
				commandParts := strings.Split(currentCommandPath, " ")
				flag = buildPathFlag(canonical, commandParts...)
				flagInfo, found = p.acceptedFlags.Get(flag)
			}
		}
	}

	if found {
		p.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
		return true
	} else {
		// Don't add error here - return false to indicate not processed
		return false
	}
}

func (p *Parser) parsePosixFlag(state parse.State, currentCommandPath string) bool {
	flag := p.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc))
	flagInfo, found := p.getFlagInCommandPath(flag, currentCommandPath)
	if !found {
		// two-pass process to account for flag values directly adjacent to a flag (e.g. `-f1` instead of `-f 1`)
		p.normalizePosixArgs(state, flag, currentCommandPath)
		flag = p.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc))
		flagInfo, found = p.getFlagInCommandPath(flag, currentCommandPath)
	}

	// If not found, try translation lookup
	if !found {
		stripped := strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc)
		if canonical, ok := p.translationRegistry.GetCanonicalFlagName(stripped, p.GetLanguage()); ok {
			flagInfo, found = p.getFlagInCommandPath(canonical, currentCommandPath)
		}
	}

	if found {
		p.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
		return true
	} else {
		// Don't add error here - return false to indicate not processed
		return false
	}
}

func (p *Parser) normalizePosixArgs(state parse.State, currentArg string, commandPath string) {
	newArgs := make([]string, 0, state.Len())
	statePos := state.Pos()
	if statePos > 0 {
		newArgs = append(newArgs, state.Args()[:statePos]...)
	}

	value := ""
	for i := 0; i < len(currentArg); i++ {
		cf := p.flagOrShortFlag(currentArg[i:i+1], commandPath)
		if _, found := p.acceptedFlags.Get(cf); found {
			if len(value) > 0 {
				newArgs = append(newArgs, value)
				value = ""
			}
			newArgs = append(newArgs, "-"+cf)
		} else {
			v := splitPathFlag(cf)
			value += v[0]
		}
	}

	if len(value) > 0 {
		newArgs = append(newArgs, value)
	}

	if state.Len() > statePos+1 {
		newArgs = append(newArgs, state.Args()[statePos+1:]...)
	}

	state.ReplaceArgs(newArgs...)
}

func (p *Parser) processFlagArg(state parse.State, argument *Argument, currentArg string, currentCommandPath ...string) {
	lookup := buildPathFlag(currentArg, currentCommandPath...)

	if isNestedSlicePath(currentArg) {
		if err := p.validateSlicePath(lookup); err != nil {
			p.addError(errs.WrapOnce(err, errs.ErrProcessingField, lookup))
			return
		}
	}

	switch argument.TypeOf {
	case types.Standalone:
		if argument.Secure.IsSecure {
			p.queueSecureArgument(lookup, argument)
		} else {
			boolVal := "true"
			if state.Pos()+1 < state.Len() {
				nextArg := state.Peek()
				if !p.isCommand(nextArg) && !p.isFlag(nextArg) {
					if _, err := strconv.ParseBool(nextArg); err == nil {
						boolVal = nextArg
						state.Skip()
					}
				}
			}

			// Run validators on standalone flag value
			if len(argument.Validators) > 0 {
				for _, validator := range argument.Validators {
					if err := validator(boolVal); err != nil {
						p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, lookup))
						return
					}
				}
			}

			p.registerFlagValue(lookup, boolVal, currentArg)
			p.options[lookup] = boolVal
			err := p.setBoundVariable(boolVal, lookup)
			if err != nil {
				p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, lookup))
			}
		}
	case types.Single, types.Chained, types.File:
		p.processFlag(argument, state, lookup)
	}
}

func (p *Parser) registerCommandRecursive(cmd *Command) {
	// Add the current command to the map
	cmd.topLevel = strings.Count(cmd.path, " ") == 0

	// Check if command already exists and merge properties
	if existing, found := p.registeredCommands.Get(cmd.path); found {
		// Merge properties - prefer existing non-empty values
		if existing.NameKey != "" && cmd.NameKey == "" {
			cmd.NameKey = existing.NameKey
		}
		if existing.Description != "" && cmd.Description == "" {
			cmd.Description = existing.Description
		}
		if existing.DescriptionKey != "" && cmd.DescriptionKey == "" {
			cmd.DescriptionKey = existing.DescriptionKey
		}
		// Also preserve other important fields
		if existing.Callback != nil && cmd.Callback == nil {
			cmd.Callback = existing.Callback
		}
	}

	p.registeredCommands.Set(cmd.path, cmd)

	// Register command translations if NameKey is provided
	if cmd.NameKey != "" {
		p.registerCommandTranslations(cmd)
	}

	// Recursively register all subcommands
	for i := range cmd.Subcommands {
		subCmd := &cmd.Subcommands[i]
		p.registerCommandRecursive(subCmd)
	}

}

func (p *Parser) validateCommand(cmdArg *Command, level, maxDepth int) (bool, error) {
	if level > maxDepth {
		return false, errs.ErrRecursionDepthExceeded.WithArgs(maxDepth)
	}

	var commandType string
	if level > 0 {
		commandType = "sub-command"
	} else {
		commandType = "command"
	}
	if cmdArg.Name == "" {
		return false, errs.ErrMissingPropertyOnLevel.WithArgs("Name", commandType, level, cmdArg)
	}

	if level == 0 {
		cmdArg.path = cmdArg.Name
	}

	for i := 0; i < len(cmdArg.Subcommands); i++ {
		cmdArg.Subcommands[i].path = cmdArg.path + " " + cmdArg.Subcommands[i].Name
		if ok, err := p.validateCommand(&cmdArg.Subcommands[i], level+1, maxDepth); err != nil {
			return ok, err
		}
	}

	return true, nil
}

func (p *Parser) ensureInit() {
	if p.options == nil {
		p.options = map[string]string{}
	}
	if p.acceptedFlags == nil {
		p.acceptedFlags = orderedmap.NewOrderedMap[string, *FlagInfo]()
	}
	if p.lookup == nil {
		p.lookup = map[string]string{}
	}
	if p.errors == nil {
		p.errors = []error{}
	}
	if p.bind == nil {
		p.bind = make(map[string]interface{}, 1)
	}
	if p.customBind == nil {
		p.customBind = map[string]ValueSetFunc{}
	}
	if p.registeredCommands == nil {
		p.registeredCommands = orderedmap.NewOrderedMap[string, *Command]()
	}
	if p.commandOptions == nil {
		p.commandOptions = orderedmap.NewOrderedMap[string, bool]()
	}
	if p.positionalArgs == nil {
		p.positionalArgs = []PositionalArgument{}
	}
	if p.rawArgs == nil {
		p.rawArgs = map[string]string{}
	}
	if p.repeatedFlags == nil {
		p.repeatedFlags = map[string]bool{}
	}
	if p.callbackQueue == nil {
		p.callbackQueue = queue.New[*Command]()
	}
	if p.callbackResults == nil {
		p.callbackResults = map[string]error{}
	}
	if p.secureArguments == nil {
		p.secureArguments = orderedmap.NewOrderedMap[string, *types.Secure]()
	}
	if p.stderr == nil {
		p.stderr = os.Stderr
	}
	if p.stdout == nil {
		p.stdout = os.Stdout
	}
	if p.flagNameConverter == nil {
		p.flagNameConverter = DefaultFlagNameConverter
	}
	if p.commandNameConverter == nil {
		p.commandNameConverter = DefaultCommandNameConverter
	}
	if p.maxDependencyDepth <= 0 {
		p.maxDependencyDepth = DefaultMaxDependencyDepth
	}
}

func (p *Parser) getArgumentInfoByID(id string) *FlagInfo {
	longName := p.lookup[id]
	if longName == "" {
		return nil
	}

	if info, found := p.acceptedFlags.Get(longName); found {
		return info
	}

	return nil
}

type flagCache struct {
	flags        map[string]map[string]*FlagInfo
	needsValue   map[string]bool
	isStandalone map[string]bool
}

func (p *Parser) buildFlagCache() *flagCache {
	cache := &flagCache{
		flags:        make(map[string]map[string]*FlagInfo),
		needsValue:   make(map[string]bool),
		isStandalone: make(map[string]bool),
	}

	for flag := p.acceptedFlags.Front(); flag != nil; flag = flag.Next() {
		fv := flag.Value
		longName := *flag.Key

		// Extract the base flag name (before @)
		baseName := splitPathFlag(longName)[0]

		if _, exists := cache.flags[baseName]; !exists {
			cache.flags[baseName] = make(map[string]*FlagInfo)
		}

		if shortName := fv.Argument.Short; shortName != "" {
			if _, exists := cache.flags[shortName]; !exists {
				cache.flags[shortName] = make(map[string]*FlagInfo)
			}
		}

		cmdPath := ""
		if fv.CommandPath != "" {
			cmdPath = fv.CommandPath
		}

		cache.flags[baseName][cmdPath] = fv
		if shortName := fv.Argument.Short; shortName != "" {
			cache.flags[shortName][cmdPath] = fv
			cache.isStandalone[shortName] = fv.Argument.TypeOf == types.Standalone
			cache.needsValue[shortName] = fv.Argument.TypeOf != types.Standalone
		}

		// Store needsValue and isStandalone with the full key including command path
		cache.isStandalone[longName] = fv.Argument.TypeOf == types.Standalone
		cache.needsValue[longName] = fv.Argument.TypeOf != types.Standalone

		// Also store with base name for global flags
		if cmdPath == "" {
			cache.isStandalone[baseName] = fv.Argument.TypeOf == types.Standalone
			cache.needsValue[baseName] = fv.Argument.TypeOf != types.Standalone
		}

		// Don't treat positional arguments as flags that need values
		if fv.Argument.Position != nil {
			cache.needsValue[longName] = false
			cache.needsValue[baseName] = false
		}
	}

	return cache
}

func (p *Parser) setPositionalArguments(state parse.State) {
	args := state.Args()
	positional := make([]PositionalArgument, 0, len(args))
	cache := p.buildFlagCache()

	declaredPos := make([]struct {
		key      string
		flag     *FlagInfo
		index    int
		required bool
	}, 0, p.acceptedFlags.Len())

	for flag := p.acceptedFlags.Front(); flag != nil; flag = flag.Next() {
		fv := flag.Value
		if fv.Argument.Position != nil {
			declaredPos = append(declaredPos, struct {
				key      string
				flag     *FlagInfo
				index    int
				required bool
			}{
				*flag.Key,
				fv,
				*fv.Argument.Position,
				fv.Argument.Required,
			})
		}
	}

	if len(declaredPos) > 0 {
		sort.SliceStable(declaredPos, func(i, j int) bool {
			return declaredPos[i].index < declaredPos[j].index
		})
	}

	skipNext := false
	currentCmdPath := make([]string, 0, 3) // Pre-allocate for typical command depth
	argPos := 0
	executedCommands := make(map[string]bool) // Track which commands were encountered
	executedCommands[""] = true               // Always check global positionals
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if p.isFlag(arg) {
			name := strings.TrimLeft(arg, "-")

			// Try to get canonical name from translation registry
			canonicalName := name
			if canonical, ok := p.translationRegistry.GetCanonicalFlagName(name, p.GetLanguage()); ok {
				canonicalName = canonical
			}

			// Check if this flag needs a value
			needsValue := false

			// First check if this flag exists in the current command context
			if len(currentCmdPath) > 0 {
				if flagInfo, exists := cache.flags[canonicalName]; exists {
					cmdPath := strings.Join(currentCmdPath, " ")
					if cmdFlagInfo, cmdExists := flagInfo[cmdPath]; cmdExists {
						// Found flag in command context
						needsValue = cmdFlagInfo.Argument.TypeOf != types.Standalone && cmdFlagInfo.Argument.Position == nil
					} else if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
						// Not in command context, but exists as global flag
						needsValue = globalFlagInfo.Argument.TypeOf != types.Standalone && globalFlagInfo.Argument.Position == nil
					}
				}
			} else {
				// No command context, check global flags
				if flagInfo, exists := cache.flags[canonicalName]; exists {
					if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
						needsValue = globalFlagInfo.Argument.TypeOf != types.Standalone && globalFlagInfo.Argument.Position == nil
					}
				}
			}

			if needsValue {
				skipNext = true
			}
			continue
		}

		// Special handling for standalone flags with boolean values
		// If previous arg was a standalone flag and current arg is a valid boolean,
		// skip it as it was consumed by the flag
		if i > 0 && p.isFlag(args[i-1]) {
			prevName := strings.TrimFunc(args[i-1], p.prefixFunc)

			// Try to get canonical name from translation registry
			canonicalPrevName := prevName
			if canonical, ok := p.translationRegistry.GetCanonicalFlagName(prevName, p.GetLanguage()); ok {
				canonicalPrevName = canonical
			}

			// Check if previous flag was standalone
			isStandalone := false
			if len(currentCmdPath) > 0 {
				if flagInfo, exists := cache.flags[canonicalPrevName]; exists {
					cmdPath := strings.Join(currentCmdPath, " ")
					if cmdFlagInfo, cmdExists := flagInfo[cmdPath]; cmdExists {
						isStandalone = cmdFlagInfo.Argument.TypeOf == types.Standalone
					} else if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
						isStandalone = globalFlagInfo.Argument.TypeOf == types.Standalone
					}
				}
			} else {
				if flagInfo, exists := cache.flags[canonicalPrevName]; exists {
					if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
						isStandalone = globalFlagInfo.Argument.TypeOf == types.Standalone
					}
				}
			}

			// If previous was standalone and current is a valid boolean, skip it
			if isStandalone {
				if _, err := strconv.ParseBool(arg); err == nil {
					continue
				}
			}
		}

		// Check command path
		isCmd := false
		canonicalArg := arg
		// Get canonical command name if it's a translated command
		if canonical, ok := p.translationRegistry.GetCanonicalCommandPath(arg, p.GetLanguage()); ok {
			canonicalArg = canonical
		}

		if len(currentCmdPath) == 0 {
			if p.isCommand(arg) {
				currentCmdPath = append(currentCmdPath, canonicalArg)
				isCmd = true
				argPos = 0 // Reset position counter for new command
			}
		} else {
			switch {
			case p.isCommand(strings.Join(append(currentCmdPath, arg), " ")):
				currentCmdPath = append(currentCmdPath, canonicalArg)
				isCmd = true
			case p.isCommand(arg):
				currentCmdPath = []string{canonicalArg}
				isCmd = true
				argPos = 0
			default:
			}
		}

		if isCmd {
			// Mark this command path as executed
			executedCommands[strings.Join(currentCmdPath, " ")] = true
			continue
		}

		// Store the positional with its command context
		pa := PositionalArgument{
			Position: i,
			Value:    arg,
			Argument: nil,
			ArgPos:   argPos,
		}

		// Find the matching declared positional for this command context
		cmdPath := strings.Join(currentCmdPath, " ")
		skipThisPositional := false
		for _, decl := range declaredPos {
			// Check if this declaration belongs to the current command path
			if decl.flag.CommandPath == cmdPath && *decl.flag.Argument.Position == argPos {
				lookup := buildPathFlag(decl.key, decl.flag.CommandPath)

				// Check if this flag was already explicitly set via flag syntax
				if _, alreadySet := p.options[lookup]; alreadySet {
					// This positional slot was filled via explicit flag syntax
					// Don't treat this value as a positional argument
					skipThisPositional = true
					continue
				}

				pa.Argument = decl.flag.Argument

				// Run validators on positional argument
				if len(decl.flag.Argument.Validators) > 0 {
					for _, validator := range decl.flag.Argument.Validators {
						if err := validator(arg); err != nil {
							p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, lookup))
						}
					}
				}

				p.registerFlagValue(lookup, arg, arg)
				p.options[lookup] = arg
				if err := p.setBoundVariable(arg, lookup); err != nil {
					p.addError(errs.ErrSettingBoundValue.WithArgs(lookup).Wrap(err))
				}
				break
			}
		}

		if !skipThisPositional {
			positional = append(positional, pa)
		}
		argPos++
	}

	// Check for missing required positionals and apply defaults
	// Since we already matched positionals during collection, we only need to check for missing ones
	for _, decl := range declaredPos {
		// Only check positionals for commands that were actually executed
		if !executedCommands[decl.flag.CommandPath] {
			continue
		}

		// Check if this positional was provided
		found := false
		for _, pos := range positional {
			if pos.Argument == decl.flag.Argument {
				found = true
				break
			}
		}

		if !found {
			// Check if a flag was provided that takes precedence
			flagFromArg := p.flagOrShortFlag(decl.key, decl.flag.CommandPath)
			if p.HasRawFlag(flagFromArg) {
				continue
			}

			if decl.flag.Argument.DefaultValue != "" {
				// Apply default value
				lookup := buildPathFlag(decl.key, decl.flag.CommandPath)
				p.registerFlagValue(lookup, decl.flag.Argument.DefaultValue, decl.flag.Argument.DefaultValue)
				p.options[lookup] = decl.flag.Argument.DefaultValue
				if err := p.setBoundVariable(decl.flag.Argument.DefaultValue, lookup); err != nil {
					p.addError(errs.ErrSettingBoundValue.WithArgs(lookup).Wrap(err))
				}

				// Add a positional argument entry for the default value
				positional = append(positional, PositionalArgument{
					Position: decl.index, // Use the declared position
					Value:    decl.flag.Argument.DefaultValue,
					Argument: decl.flag.Argument,
					ArgPos:   decl.index,
				})
			} else if decl.required {
				// Missing required positional
				p.addError(errs.ErrRequiredPositionalFlag.WithArgs(decl.key, decl.index))
			}
		}
	}

	newResult := make([]PositionalArgument, 0, len(positional))

	for i := range positional {
		// Keep positions that either:
		// 1. Have a non-empty value
		// 2. Are unbound (Argument == nil) and have any value
		if positional[i].Value != "" {
			newResult = append(newResult, positional[i])
		}
	}

	// Sort by position to maintain order
	sort.Slice(newResult, func(i, j int) bool {
		return newResult[i].Position < newResult[j].Position
	})

	p.positionalArgs = newResult
}

func (p *Parser) evalExecOnParse(lastCommandPath string) string {
	if p.callbackOnParse {
		err := p.ExecuteCommand()
		if err != nil {
			p.addError(errs.WrapOnce(err, errs.ErrProcessingCommand, lastCommandPath))
		}
	} else if cmd, ok := p.getCommand(lastCommandPath); ok && cmd.Callback != nil && cmd.ExecOnParse {
		err := p.ExecuteCommand()
		if err != nil {
			p.addError(errs.WrapOnce(err, errs.ErrProcessingCommand, lastCommandPath))
		}
	}

	return ""
}

func (p *Parser) evalFlagWithPath(state parse.State, currentCommandPath string) bool {
	if p.posixCompatible {
		return p.parsePosixFlag(state, currentCommandPath)
	} else {
		return p.parseFlag(state, currentCommandPath)
	}
}

func (p *Parser) flagOrShortFlag(flag string, commandPath ...string) string {
	// First check directly with the provided path
	pathFlag := buildPathFlag(flag, commandPath...)
	_, pathFound := p.acceptedFlags.Get(pathFlag)
	if pathFound {
		return pathFlag
	}

	// Check if it's a global flag
	globalFlag := splitPathFlag(flag)[0]
	_, found := p.acceptedFlags.Get(globalFlag)
	if found {
		return globalFlag
	}

	// NEW: Use context-aware short flag lookup
	if longFlag, found := p.shortFlagLookup(flag, commandPath...); found {
		return longFlag
	}

	// Try parent paths for the original flag
	if !pathFound && len(commandPath) > 0 && commandPath[0] != "" {
		parts := splitPathFlag(flag)
		if len(parts) > 1 {
			flag = parts[0]
		}
		pathString := strings.Join(commandPath, " ")
		pathParts := strings.Split(pathString, " ")

		for i := len(pathParts) - 1; i > 0; i-- {
			parentPath := strings.Join(pathParts[:i], " ")
			parentKey := buildPathFlag(flag, parentPath)
			if _, found := p.acceptedFlags.Get(parentKey); found {
				return parentKey
			}
		}
	}

	return pathFlag
}

func (p *Parser) isFlag(flag string) bool {
	if len(p.prefixes) == 0 {
		if strings.HasPrefix(flag, "-") {
			if n, ok := util.ParseNumeric(flag); ok && n.IsNegative {
				return false
			}
			return true
		}
		return false
	}

	for _, prefix := range p.prefixes {
		if strings.HasPrefix(flag, string(prefix)) {
			if prefix == '-' {
				if n, ok := util.ParseNumeric(flag); ok && n.IsNegative {
					return false
				}
			}
			return true
		}
	}

	return false
}

func (p *Parser) isCommand(arg string) bool {
	if _, ok := p.registeredCommands.Get(arg); ok {
		return true
	}

	// Check if it's a translated command name
	if canonical, ok := p.translationRegistry.GetCanonicalCommandPath(arg, p.GetLanguage()); ok {
		if _, ok := p.registeredCommands.Get(canonical); ok {
			return true
		}
	}

	return false
}

func (p *Parser) isGlobalFlag(arg string) bool {
	stripped := strings.TrimLeftFunc(arg, p.prefixFunc)
	flag, ok := p.acceptedFlags.Get(p.flagOrShortFlag(stripped))
	if ok {
		return flag.CommandPath == ""
	}

	// Check if it's a translated global flag
	if canonical, ok := p.translationRegistry.GetCanonicalFlagName(stripped, p.GetLanguage()); ok {
		flag, ok := p.acceptedFlags.Get(canonical)
		if ok {
			return flag.CommandPath == ""
		}
	}

	return false
}

func (p *Parser) addError(err error) {
	p.errors = append(p.errors, err)
}

func (p *Parser) getCommand(name string) (*Command, bool) {
	// First try canonical lookup
	cmd, found := p.registeredCommands.Get(name)

	// If not found, try translation lookup
	if !found {
		if canonical, ok := p.translationRegistry.GetCanonicalCommandPath(name, p.GetLanguage()); ok {
			cmd, found = p.registeredCommands.Get(canonical)
		}
	}

	return cmd, found
}

func (p *Parser) registerSecureValue(flag, value string) error {
	var err error
	p.rawArgs[flag] = value
	if value != "" {
		p.options[flag] = value
		err = p.setBoundVariable(value, flag)
	}

	return err
}

func (p *Parser) registerFlagValue(flag, value, rawValue string) {
	parts := splitPathFlag(flag)
	p.rawArgs[parts[0]] = rawValue
	p.rawArgs[rawValue] = rawValue

	// For Chained type flags that are repeated, append values
	if flagInfo, found := p.acceptedFlags.Get(flag); found && flagInfo.Argument.TypeOf == types.Chained {
		if existingValue, exists := p.options[flag]; exists && p.repeatedFlags[flag] {
			// Append with pipe separator (the default for chained values)
			p.options[flag] = existingValue + "|" + value
			return
		}
	}

	p.options[flag] = value
}

func (p *Parser) registerCommand(cmd *Command, name string) {
	if cmd.path == "" {
		return
	}

	p.rawArgs[name] = name

	p.commandOptions.Set(cmd.path, len(cmd.Subcommands) == 0)
}

func (p *Parser) queueSecureArgument(name string, argument *Argument) {
	if p.secureArguments == nil {
		p.secureArguments = orderedmap.NewOrderedMap[string, *types.Secure]()
	}

	pathFlag := splitPathFlag(name)
	p.rawArgs[pathFlag[0]] = name
	p.secureArguments.Set(name, &argument.Secure)
}

func (p *Parser) parseCommand(state parse.State, cmdQueue *queue.Q[*Command], commandPathSlice *[]string) bool {
	terminating := false
	currentArg := state.CurrentArg()

	// Check if we're dealing with a subcommand
	var (
		curSub *Command
		ok     bool
	)
	if cmdQueue.Len() > 0 {
		ok, curSub = p.checkSubCommands(cmdQueue, currentArg)
		if !ok {
			return false
		}
	}

	var cmd *Command
	if curSub != nil {
		cmd = curSub
	} else {
		if registered, found := p.getCommand(currentArg); found {
			cmd = registered
			p.registerCommand(cmd, currentArg)
			// If we found a root command and we already have a command path,
			// we're starting a new command chain - clear the path
			if len(*commandPathSlice) > 0 {
				*commandPathSlice = (*commandPathSlice)[:0]
			}
		}
	}

	if cmd != nil {
		// Use the canonical command name for the path
		*commandPathSlice = append(*commandPathSlice, cmd.Name)
		if len(cmd.Subcommands) == 0 {
			cmdQueue.Clear()
			terminating = true
		} else {
			cmdQueue.Push(cmd)
		}

		if cmd.Callback == nil && cmd.callbackLocation.IsValid() {
			cmd.Callback = cmd.callbackLocation.Interface().(CommandFunc)
		}

		// Queue the command callback (if any) after the command is fully recognized
		if cmd.Callback != nil {
			p.queueCommandCallback(cmd)
		}

	} else {
		// Command not found - check if it might be a typo
		// Only generate suggestions if we have registered commands and this looks like a command attempt
		if p.registeredCommands.Len() > 0 {
			suggestions, _ := p.findSimilarRootCommandsWithContext(currentArg)
			if len(suggestions) > 0 {
				// Check if any suggestion is very close (likely a typo)
				for _, suggestion := range suggestions {
					distance := util.LevenshteinDistance(currentArg, suggestion)
					if distance <= 2 {
						// Very likely a typo - generate error with suggestions
						p.addError(errs.ErrCommandNotFound.WithArgs(currentArg))

						// Display each suggestion in the form that was closest to user input
						displaySuggestions := make([]string, len(suggestions))
						for i, suggestion := range suggestions {
							// By default show canonical
							displaySuggestions[i] = suggestion

							// Check if we should show translated form
							if p.translationRegistry != nil {
								if cmd, found := p.registeredCommands.Get(suggestion); found && cmd.NameKey != "" {
									if translated, found := p.translationRegistry.GetCommandTranslation(suggestion, p.GetLanguage()); found {
										// Compare distances to determine which form to show
										canonicalDist := util.LevenshteinDistance(currentArg, suggestion)
										translatedDist := util.LevenshteinDistance(currentArg, translated)

										// Show the form that's closer to what user typed
										if translatedDist < canonicalDist {
											displaySuggestions[i] = translated
										} else if translatedDist == canonicalDist && translated != suggestion {
											// If equal distance and different words, show both forms
											displaySuggestions[i] = fmt.Sprintf("%s / %s", suggestion, translated)
										}
									}
								}
							}
						}

						// Format suggestions
						var formatted string
						if p.suggestionsFormatter != nil {
							formatted = p.suggestionsFormatter(displaySuggestions)
						} else {
							// Use i18n for "did you mean"
							didYouMean := p.layeredProvider.GetMessage(messages.MsgDidYouMeanKey)
							if len(displaySuggestions) == 1 {
								formatted = fmt.Sprintf("%s %s", didYouMean, displaySuggestions[0])
							} else {
								formatted = fmt.Sprintf("%s\n  %s", didYouMean, strings.Join(displaySuggestions, "\n  "))
							}
						}
						p.addError(fmt.Errorf("%s", formatted))
						return false
					}
				}
			}
		}
	}

	return terminating
}

func (p *Parser) queueCommandCallback(cmd *Command) {
	if cmd.Callback != nil {
		p.callbackQueue.Enqueue(cmd)
	}
}

func (p *Parser) processFlag(argument *Argument, state parse.State, flag string) {
	var err error
	if argument.Secure.IsSecure {
		if state.Pos() < state.Len()-1 {
			if !p.isFlag(state.Peek()) {
				state.Skip()
			}
		}
		p.queueSecureArgument(flag, argument)
	} else {
		var next string
		if state.Pos() < state.Len()-1 {
			next = state.Peek()
		}
		if (len(next) == 0 || p.isFlag(next)) && len(argument.DefaultValue) > 0 {
			next = argument.DefaultValue
		} else {
			state.Skip()
		}
		if state.Pos() >= state.Len()-1 && len(next) == 0 {
			p.addError(errs.ErrFlagExpectsValue.WithArgs(flag))
		} else {
			next, err = p.flagValue(argument, next, flag)
			if err != nil {
				p.addError(err)
			} else {
				if err = p.processValueFlag(flag, next, argument); err != nil {
					p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, flag))
				}
			}
		}
	}
}

func (p *Parser) flagValue(argument *Argument, next string, flag string) (arg string, err error) {
	if argument.TypeOf == types.File {
		next = expandVarExpr().ReplaceAllStringFunc(next, varFunc)
		next, err = filepath.Abs(next)
		if st, e := os.Stat(next); e != nil {
			err = errs.ErrNotFoundPathForFlag.WithArgs(flag, next).Wrap(e)
			return
		} else if st.IsDir() {
			err = errs.ErrNotFilePathForFlag.WithArgs(flag)
			return
		}
		next = filepath.Clean(next)
		if val, e := os.ReadFile(next); e != nil {
			err = errs.ErrFlagFileOperation.WithArgs(flag, next).Wrap(e)
		} else {
			arg = string(val)
		}
		p.registerFlagValue(flag, arg, next)
	} else {
		if p.isFlag(next) && argument.TypeOf == types.Single {
			stripped := strings.TrimLeftFunc(next, p.prefixFunc)
			if _, ok := p.acceptedFlags.Get(stripped); ok {
				p.addError(errs.ErrFlagExpectsValue.WithArgs(flag))
				return
			}
		}
		arg = next
		p.registerFlagValue(flag, next, next)
	}

	return arg, err
}

// findSimilarSubcommandsWithContext finds subcommands similar to the input and detects if input is likely translated
func (p *Parser) findSimilarSubcommandsWithContext(subcommands []Command, input string, parentPath string) ([]string, bool) {
	type subcommandSuggestion struct {
		canonicalName string
		distance      int
		isTranslated  bool
	}

	var allSuggestions []subcommandSuggestion
	threshold := p.cmdSuggestionThreshold
	if threshold == 0 {
		return nil, false // Suggestions disabled for commands
	}
	currentLang := p.GetLanguage()

	// Check all subcommands - both canonical and translated names
	for _, cmd := range subcommands {
		// Check canonical name
		distance := util.LevenshteinDistance(input, cmd.Name)
		if distance > 0 && distance <= threshold {
			allSuggestions = append(allSuggestions, subcommandSuggestion{
				canonicalName: cmd.Name,
				distance:      distance,
				isTranslated:  false,
			})
		}

		// Check translated name if available
		if p.translationRegistry != nil && cmd.NameKey != "" {
			// For subcommands, build the full path
			fullPath := cmd.Name
			if parentPath != "" {
				fullPath = parentPath + " " + cmd.Name
			}
			if translatedName, found := p.translationRegistry.GetCommandTranslation(fullPath, currentLang); found {
				translatedDistance := util.LevenshteinDistance(input, translatedName)
				if translatedDistance > 0 && translatedDistance <= threshold {
					// Check if we already have this command in suggestions
					found := false
					for i, s := range allSuggestions {
						if s.canonicalName == cmd.Name {
							// Update if translated is closer
							if translatedDistance < s.distance {
								allSuggestions[i].distance = translatedDistance
								allSuggestions[i].isTranslated = true
							}
							found = true
							break
						}
					}
					if !found {
						allSuggestions = append(allSuggestions, subcommandSuggestion{
							canonicalName: cmd.Name,
							distance:      translatedDistance,
							isTranslated:  true,
						})
					}
				}
			}
		}
	}

	// Find minimum distance
	minDistance := 3
	for _, s := range allSuggestions {
		if s.distance < minDistance {
			minDistance = s.distance
		}
	}

	// If we have distance 1 matches, only show those
	// Otherwise show all matches up to the configured threshold
	finalThreshold := minDistance
	if minDistance > 1 && p.cmdSuggestionThreshold > 1 {
		finalThreshold = p.cmdSuggestionThreshold
	}

	// Filter and collect canonical names
	var suggestions []string
	hasTranslated := false

	for _, s := range allSuggestions {
		if s.distance <= finalThreshold {
			suggestions = append(suggestions, s.canonicalName)
			if s.isTranslated {
				hasTranslated = true
			}
		}
	}

	// Remove duplicates
	uniqueSuggestions := make(map[string]bool)
	var result []string
	for _, s := range suggestions {
		if !uniqueSuggestions[s] {
			uniqueSuggestions[s] = true
			result = append(result, s)
		}
	}

	// Sort by distance
	sort.Slice(result, func(i, j int) bool {
		dist1 := 3
		dist2 := 3
		for _, s := range allSuggestions {
			if s.canonicalName == result[i] {
				dist1 = s.distance
			}
			if s.canonicalName == result[j] {
				dist2 = s.distance
			}
		}
		return dist1 < dist2
	})

	// Limit to top 3
	if len(result) > 3 {
		result = result[:3]
	}

	return result, hasTranslated
}

// findSimilarFlagsWithContext finds flags similar to the input and detects if input is likely translated
func (p *Parser) findSimilarFlagsWithContext(input string, commandPath string) ([]string, bool) {
	type flagSuggestion struct {
		canonicalName string
		distance      int
		isTranslated  bool
	}

	var allSuggestions []flagSuggestion
	threshold := p.flagSuggestionThreshold
	if threshold == 0 {
		return nil, false // Suggestions disabled for flags
	}

	// Remove prefix from input if present
	cleanInput := strings.TrimLeftFunc(input, p.prefixFunc)
	currentLang := p.GetLanguage()

	// Check all flags - both canonical and translated names
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		flagKey := *f.Key
		flagInfo := f.Value

		// Skip flags not in the current command context
		if commandPath != "" && flagInfo.CommandPath != commandPath && flagInfo.CommandPath != "" {
			continue
		}

		// Extract flag name without command path
		flagParts := splitPathFlag(flagKey)
		flagName := flagParts[0]

		// Check canonical name
		distance := util.LevenshteinDistance(cleanInput, flagName)
		if distance > 0 && distance <= threshold {
			allSuggestions = append(allSuggestions, flagSuggestion{
				canonicalName: flagName,
				distance:      distance,
				isTranslated:  false,
			})
		}

		// Check short form if available
		if flagInfo.Argument.Short != "" {
			shortDistance := util.LevenshteinDistance(cleanInput, flagInfo.Argument.Short)
			if shortDistance > 0 && shortDistance <= threshold {
				// Check if we already have this flag in suggestions
				found := false
				for i, s := range allSuggestions {
					if s.canonicalName == flagName {
						// Update if short form is closer
						if shortDistance < s.distance {
							allSuggestions[i].distance = shortDistance
						}
						found = true
						break
					}
				}
				if !found {
					allSuggestions = append(allSuggestions, flagSuggestion{
						canonicalName: flagName,
						distance:      shortDistance,
						isTranslated:  false,
					})
				}
			}
		}

		// Check translated name if available
		if p.translationRegistry != nil && flagInfo.Argument.NameKey != "" {
			if translatedName, found := p.translationRegistry.GetFlagTranslation(flagName, currentLang); found {
				translatedDistance := util.LevenshteinDistance(cleanInput, translatedName)
				if translatedDistance > 0 && translatedDistance <= threshold {
					// Check if we already have this flag in suggestions
					found := false
					for i, s := range allSuggestions {
						if s.canonicalName == flagName {
							// Update if translated is closer
							if translatedDistance < s.distance {
								allSuggestions[i].distance = translatedDistance
								allSuggestions[i].isTranslated = true
							}
							found = true
							break
						}
					}
					if !found {
						allSuggestions = append(allSuggestions, flagSuggestion{
							canonicalName: flagName,
							distance:      translatedDistance,
							isTranslated:  true,
						})
					}
				}
			}
		}
	}

	// Find minimum distance
	minDistance := 3
	for _, s := range allSuggestions {
		if s.distance < minDistance {
			minDistance = s.distance
		}
	}

	// If we have distance 1 matches, only show those
	// Otherwise show all matches up to the configured threshold
	finalThreshold := minDistance
	if minDistance > 1 && p.flagSuggestionThreshold > 1 {
		finalThreshold = p.flagSuggestionThreshold
	}

	// Filter and collect canonical names
	var suggestions []string
	hasTranslated := false

	for _, s := range allSuggestions {
		if s.distance <= finalThreshold {
			suggestions = append(suggestions, s.canonicalName)
			if s.isTranslated {
				hasTranslated = true
			}
		}
	}

	// Remove duplicates
	uniqueSuggestions := make(map[string]bool)
	var result []string
	for _, s := range suggestions {
		if !uniqueSuggestions[s] {
			uniqueSuggestions[s] = true
			result = append(result, s)
		}
	}

	// Sort by distance
	sort.Slice(result, func(i, j int) bool {
		dist1 := 3
		dist2 := 3
		for _, s := range allSuggestions {
			if s.canonicalName == result[i] {
				dist1 = s.distance
			}
			if s.canonicalName == result[j] {
				dist2 = s.distance
			}
		}
		return dist1 < dist2
	})

	// Limit to top 3
	if len(result) > 3 {
		result = result[:3]
	}

	return result, hasTranslated
}

// findSimilarRootCommandsWithContext finds root commands similar to the input
func (p *Parser) findSimilarRootCommandsWithContext(input string) ([]string, bool) {
	type suggestion struct {
		canonicalName string
		distance      int
		isTranslated  bool
	}

	var allSuggestions []suggestion
	currentLang := p.GetLanguage()
	threshold := p.cmdSuggestionThreshold
	if threshold == 0 {
		return nil, false // Suggestions disabled for commands
	}

	// Check all commands - both canonical and translated names
	for c := p.registeredCommands.Front(); c != nil; c = c.Next() {
		cmd := c.Value
		cmdName := *c.Key

		// Check canonical name
		distance := util.LevenshteinDistance(input, cmdName)
		if distance > 0 && distance <= threshold {
			allSuggestions = append(allSuggestions, suggestion{
				canonicalName: cmdName,
				distance:      distance,
				isTranslated:  false,
			})
		}

		// Check translated name if available
		if p.translationRegistry != nil && cmd.NameKey != "" {
			if translated, found := p.translationRegistry.GetCommandTranslation(cmdName, currentLang); found {
				translatedDistance := util.LevenshteinDistance(input, translated)
				if translatedDistance > 0 && translatedDistance <= threshold {
					// Check if we already have this command in suggestions
					found := false
					for i, s := range allSuggestions {
						if s.canonicalName == cmdName {
							// Update if translated is closer
							if translatedDistance < s.distance {
								allSuggestions[i].distance = translatedDistance
								allSuggestions[i].isTranslated = true
							}
							found = true
							break
						}
					}
					if !found {
						allSuggestions = append(allSuggestions, suggestion{
							canonicalName: cmdName,
							distance:      translatedDistance,
							isTranslated:  true,
						})
					}
				}
			}
		}
	}

	// Find minimum distance
	minDistance := 3
	for _, s := range allSuggestions {
		if s.distance < minDistance {
			minDistance = s.distance
		}
	}

	// If we have distance 1 matches, only show those
	// Otherwise show all matches up to the configured threshold
	finalThreshold := minDistance
	if minDistance > 1 && p.cmdSuggestionThreshold > 1 {
		finalThreshold = p.cmdSuggestionThreshold
	}

	// Filter and determine if we should show translated names
	var finalSuggestions []string
	hasTranslated := false

	for _, s := range allSuggestions {
		if s.distance <= finalThreshold {
			finalSuggestions = append(finalSuggestions, s.canonicalName)
			if s.isTranslated {
				hasTranslated = true
			}
		}
	}

	// Sort by distance
	sort.Slice(finalSuggestions, func(i, j int) bool {
		dist1 := 3
		dist2 := 3
		for _, s := range allSuggestions {
			if s.canonicalName == finalSuggestions[i] {
				dist1 = s.distance
			}
			if s.canonicalName == finalSuggestions[j] {
				dist2 = s.distance
			}
		}
		return dist1 < dist2
	})

	// Limit to top 3
	if len(finalSuggestions) > 3 {
		finalSuggestions = finalSuggestions[:3]
	}

	return finalSuggestions, hasTranslated
}

func (p *Parser) findSimilarRootCommands(input string) []string {
	suggestions, _ := p.findSimilarRootCommandsWithContext(input)
	return suggestions
}

// generateFlagError generates an error for an unknown flag with suggestions
func (p *Parser) generateFlagError(flagName string, commandPath string) {
	suggestions, _ := p.findSimilarFlagsWithContext(flagName, commandPath)
	if len(suggestions) > 0 {
		// Format suggestions with proper prefixes
		formattedSuggestions := make([]string, len(suggestions))

		// Remove prefix from input for comparison
		cleanInput := strings.TrimLeftFunc(flagName, p.prefixFunc)

		for i, s := range suggestions {
			// Decide whether to show canonical or translated based on distances
			displayName := s

			if p.translationRegistry != nil {
				// Check if there's a translation
				if translated, found := p.translationRegistry.GetFlagTranslation(s, p.GetLanguage()); found {
					// Compare distances to determine which form to show
					canonicalDist := util.LevenshteinDistance(cleanInput, s)
					translatedDist := util.LevenshteinDistance(cleanInput, translated)

					// Show the form that's closer to what user typed
					if translatedDist < canonicalDist {
						displayName = translated
					} else if translatedDist == canonicalDist && translated != s {
						// If equal distance and different words, show both forms
						displayName = s + " / " + translated
					}
				}
			}

			if len(displayName) == 1 || (strings.Contains(displayName, " / ") && len(s) == 1) {
				// Short flag
				formattedSuggestions[i] = string(p.prefixes[0]) + displayName
			} else {
				// Long flag
				if len(p.prefixes) > 1 {
					formattedSuggestions[i] = string(p.prefixes[1]) + string(p.prefixes[1]) + displayName
				} else {
					formattedSuggestions[i] = string(p.prefixes[0]) + string(p.prefixes[0]) + displayName
				}
			}
		}
		// Use custom formatter if set, otherwise default to comma-separated list with brackets
		var formatted string
		if p.suggestionsFormatter != nil {
			formatted = p.suggestionsFormatter(formattedSuggestions)
		} else {
			formatted = "[" + strings.Join(formattedSuggestions, ", ") + "]"
		}
		p.addError(errs.ErrUnknownFlagWithSuggestions.WithArgs(flagName, formatted))
	} else {
		p.addError(errs.ErrUnknownFlag.WithArgs(flagName))
	}
}

func (p *Parser) checkSubCommands(cmdQueue *queue.Q[*Command], currentArg string) (bool, *Command) {
	found := false
	var sub Command

	if cmdQueue.Len() == 0 {
		return false, nil
	}

	currentCmd, _ := cmdQueue.Pop()
	for _, sub = range currentCmd.Subcommands {
		if strings.EqualFold(sub.Name, currentArg) {
			found = true
			break
		}
	}

	// If not found, try translation lookup
	if !found {
		for _, sub = range currentCmd.Subcommands {
			// Check if currentArg is a translated name
			if sub.NameKey != "" {
				translator := p.GetTranslator()
				translatedName := translator.TL(p.GetLanguage(), sub.NameKey)
				if strings.EqualFold(translatedName, currentArg) {
					found = true
					break
				}
			}
		}
	}

	if found {
		p.registerCommand(&sub, currentArg)
		cmdQueue.Push(&sub) // Keep subcommands in the queue
		return true, &sub
	} else if len(currentCmd.Subcommands) > 0 {
		// Check if the current arg looks like it was meant to be a subcommand
		// (not a flag or positional argument)
		if !p.isFlag(currentArg) {
			// Find similar subcommands - pass the parent command's path
			suggestions, _ := p.findSimilarSubcommandsWithContext(currentCmd.Subcommands, currentArg, currentCmd.path)
			if len(suggestions) > 0 {
				// Create a more helpful error message
				p.addError(errs.ErrCommandNotFound.WithArgs(currentCmd.path + " " + currentArg))

				// Decide whether to show canonical or translated based on distances
				displaySuggestions := make([]string, len(suggestions))
				for i, suggestion := range suggestions {
					displaySuggestions[i] = suggestion

					// Find the subcommand to check for translation
					for _, sub := range currentCmd.Subcommands {
						if sub.Name == suggestion && sub.NameKey != "" && p.translationRegistry != nil {
							// Build the full path for the subcommand
							fullPath := suggestion
							if currentCmd.path != "" {
								fullPath = currentCmd.path + " " + suggestion
							}
							if translated, found := p.translationRegistry.GetCommandTranslation(fullPath, p.GetLanguage()); found {
								// Compare distances to determine which form to show
								canonicalDist := util.LevenshteinDistance(currentArg, suggestion)
								translatedDist := util.LevenshteinDistance(currentArg, translated)

								// Show the form that's closer to what user typed
								if translatedDist < canonicalDist {
									displaySuggestions[i] = translated
								} else if translatedDist == canonicalDist && translated != suggestion {
									// If equal distance and different words, show both forms
									displaySuggestions[i] = suggestion + " / " + translated
								}
							}
							break
						}
					}
				}

				// Format suggestions using the formatter if available
				var formatted string
				if p.suggestionsFormatter != nil {
					formatted = p.suggestionsFormatter(displaySuggestions)
				} else {
					// Default format with i18n
					didYouMean := p.layeredProvider.GetMessage(messages.MsgDidYouMeanKey)
					formatted = fmt.Sprintf("%s\n  %s", didYouMean, strings.Join(displaySuggestions, "\n  "))
				}
				p.addError(fmt.Errorf("%s", formatted))
			} else {
				// No similar commands found, show available subcommands
				p.addError(errs.ErrCommandExpectsSubcommand.WithArgs(currentCmd.Name, currentCmd.Subcommands))
			}
		}
	}

	return false, nil
}

func (p *Parser) processValueFlag(currentArg string, next string, argument *Argument) error {
	var processed string
	var validationPassed bool = true

	// Use processSingleValue if we have AcceptedValues or Validators
	if len(argument.AcceptedValues) > 0 || len(argument.Validators) > 0 {
		processed, validationPassed = p.processSingleValue(next, currentArg, argument)
	} else {
		haveFilters := argument.PreFilter != nil || argument.PostFilter != nil
		if argument.PreFilter != nil {
			processed = argument.PreFilter(next)
			p.registerFlagValue(currentArg, processed, next)
		}
		if argument.PostFilter != nil {
			if processed != "" {
				processed = argument.PostFilter(processed)
			} else {
				processed = argument.PostFilter(next)
			}
			p.registerFlagValue(currentArg, processed, next)
		}
		if !haveFilters {
			processed = next
		}
	}

	// Run validators on the final processed value
	// Skip for chained types as they are validated individually in checkMultiple
	// Also skip if validation already failed in processSingleValue
	if len(argument.Validators) > 0 && argument.TypeOf != types.Chained && validationPassed {
		for _, validator := range argument.Validators {
			if err := validator(processed); err != nil {
				return errs.WrapOnce(err, errs.ErrProcessingFlag, currentArg)
			}
		}
	}

	// Only set bound variable if validation passed
	if validationPassed {
		return p.setBoundVariable(processed, currentArg)
	}
	return nil
}

func (p *Parser) processSecureFlag(name string, config *types.Secure) {
	var prompt string
	if !p.HasFlag(name) {
		return
	}

	if !config.IsSecure {
		return
	}
	if config.Prompt == "" {
		prompt = "password: "
	} else {
		prompt = config.Prompt
	}
	if pass, err := input.GetSecureString(prompt, p.GetStderr(), p.GetTerminalReader()); err == nil {
		// Get the argument info to run validators
		if argInfo := p.getArgumentInfoByID(name); argInfo != nil && argInfo.Argument != nil {
			// Run validators on secure value
			if len(argInfo.Argument.Validators) > 0 {
				for _, validator := range argInfo.Argument.Validators {
					if err = validator(pass); err != nil {
						p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, name))
						return
					}
				}
			}
		}

		err = p.registerSecureValue(name, pass)
		if err != nil {
			p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, name))
		}
	} else {
		p.addError(errs.WrapOnce(err, errs.ErrSecureFlagExpectsValue, name))
	}
}

func (p *Parser) processSingleValue(next, key string, argument *Argument) (string, bool) {
	switch argument.TypeOf {
	case types.Single:
		return p.checkSingle(next, key, argument)
	case types.Chained:
		return p.checkMultiple(next, key, argument)
	}

	return "", false
}

// getPatternDescription returns the description for a PatternValue, checking i18n first
func (p *Parser) getPatternDescription(pv types.PatternValue) string {
	if pv.Description == "" {
		return pv.Pattern
	}

	// First check if it's a translation key
	msg := p.layeredProvider.GetMessage(pv.Description)
	if msg != pv.Description {
		// It was translated, return the translation
		return msg
	}

	// Otherwise return the description as-is (literal string)
	return pv.Description
}

func (p *Parser) checkSingle(next, flag string, argument *Argument) (string, bool) {
	var errBuf = strings.Builder{}
	var valid = false
	var value string
	if argument.PreFilter != nil {
		value = argument.PreFilter(next)
	} else {
		value = next
	}

	// Skip AcceptedValues check if we have validators (they're converted to validators now)
	if len(argument.Validators) == 0 && len(argument.AcceptedValues) > 0 {
		// Legacy path: Check AcceptedValues only if no validators
		for _, v := range argument.AcceptedValues {
			if v.Compiled != nil && v.Compiled.MatchString(value) {
				valid = true
				break
			}
		}

		if !valid {
			// Build error message with all accepted patterns
			lenValues := len(argument.AcceptedValues)
			for i, v := range argument.AcceptedValues {
				errBuf.WriteString(p.getPatternDescription(v))
				if i+1 < lenValues {
					errBuf.WriteString(", ")
				}
			}
			p.addError(errs.ErrInvalidArgument.WithArgs(next, flag, errBuf.String()))
			return "", false
		}
	}

	// Run validators (if any) - this includes converted AcceptedValues
	if len(argument.Validators) > 0 {
		for _, validator := range argument.Validators {
			if err := validator(value); err != nil {
				p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, flag))
				return "", false
			}
		}
	}

	// Step 3: Apply post-filter
	if argument.PostFilter != nil {
		value = argument.PostFilter(value)
	}

	// All validations passed
	p.registerFlagValue(flag, value, next)
	return value, true
}

func (p *Parser) checkMultiple(next, flag string, argument *Argument) (string, bool) {
	listDelimFunc := p.getListDelimiterFunc()
	args := strings.FieldsFunc(next, listDelimFunc)

	// Process each value in the list
	for i := 0; i < len(args); i++ {
		// Apply pre-filter
		if argument.PreFilter != nil {
			args[i] = argument.PreFilter(args[i])
		}

		// Skip AcceptedValues check if we have validators (they're converted to validators now)
		if len(argument.Validators) == 0 && len(argument.AcceptedValues) > 0 {
			// Legacy path: Check AcceptedValues only if no validators
			matched := false
			for _, v := range argument.AcceptedValues {
				if v.Compiled != nil && v.Compiled.MatchString(args[i]) {
					matched = true
					break
				}
			}

			if !matched {
				// Build error message with all accepted patterns
				var errBuf strings.Builder
				lenValues := len(argument.AcceptedValues)
				for j, v := range argument.AcceptedValues {
					errBuf.WriteString(p.getPatternDescription(v))
					if j+1 < lenValues {
						errBuf.WriteString(", ")
					}
				}
				p.addError(errs.ErrInvalidArgument.WithArgs(args[i], flag, errBuf.String()))
				return "", false
			}
		}

		if len(argument.Validators) > 0 {
			for _, validator := range argument.Validators {
				if err := validator(args[i]); err != nil {
					p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, flag))
					return "", false
				}
			}
		}

		// Step 3: Apply post-filter
		if argument.PostFilter != nil {
			args[i] = argument.PostFilter(args[i])
		}
	}

	// All validations passed for all values
	value := strings.Join(args, "|")
	p.registerFlagValue(flag, value, next)
	return value, true
}

func (p *Parser) validateProcessedOptions() {
	p.walkCommands()
	p.walkFlags()
}

func (p *Parser) walkFlags() {
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		flagInfo := f.Value
		visited := make(map[string]bool)
		if flagInfo.Argument.RequiredIf != nil {
			if required, msg := flagInfo.Argument.RequiredIf(p, *f.Key); required {
				p.addError(errors.New(msg))
			}
			continue
		}

		if !flagInfo.Argument.Required {
			if p.HasFlag(*f.Key) && flagInfo.Argument.TypeOf == types.Standalone {
				p.validateStandaloneFlag(*f.Key)
			}
			continue
		}

		mainKey := p.flagOrShortFlag(*f.Key)
		if _, found := p.options[mainKey]; found {
			if flagInfo.Argument.TypeOf == types.Standalone {
				p.validateStandaloneFlag(mainKey)
			}
			continue
		} else if flagInfo.Argument.Secure.IsSecure {
			pathFlag := splitPathFlag(mainKey)
			if len(pathFlag) == 2 {
				if !p.HasCommand(pathFlag[1]) {
					continue
				}
			}
			p.queueSecureArgument(mainKey, flagInfo.Argument)
			continue
		}

		cmdArg := splitPathFlag(mainKey)

		if !p.shouldValidateDependencies(flagInfo) {
			if len(cmdArg) == 1 || (len(cmdArg) == 2 && p.HasCommand(cmdArg[1])) {
				if flagInfo.Argument.Position != nil {
					p.addError(errs.ErrRequiredPositionalFlag.WithArgs(*f.Key, *flagInfo.Argument.Position))
				} else {
					p.addError(errs.ErrRequiredFlag.WithArgs(*f.Key))
				}
			}
		} else {
			p.validateDependencies(flagInfo, mainKey, visited, 0)
		}
	}
}

func (p *Parser) shouldValidateDependencies(flagInfo *FlagInfo) bool {
	return len(flagInfo.Argument.DependencyMap) > 0
}

func (p *Parser) validateStandaloneFlag(key string) {
	_, err := p.GetBool(key)
	if err != nil {
		p.addError(err)
	}
}

func (p *Parser) walkCommands() {
	stack := queue.New[*Command]()
	for kv := p.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		stack.Push(kv.Value)
	}
	for stack.Len() > 0 {
		cmd, _ := stack.Pop()
		matches := 0
		match := strings.Builder{}
		subCmdLen := len(cmd.Subcommands)
		matchedCommands := make([]Command, 0, 5)
		if subCmdLen == 0 {
			continue
		}

		for i, sub := range cmd.Subcommands {
			match.WriteString(sub.Name)
			if i < subCmdLen-1 {
				match.WriteString(", ")
			}
			if _, found := p.commandOptions.Get(sub.path); found {
				matchedCommands = append(matchedCommands, sub)
				matches++
			}
		}

		for _, m := range matchedCommands {
			for i := range m.Subcommands {
				sub := m.Subcommands[i]
				stack.Push(&sub)
			}
		}
	}
}

func (p *Parser) validateDependencies(flagInfo *FlagInfo, mainKey string, visited map[string]bool, depth int) {
	if depth > p.maxDependencyDepth {
		p.addError(errs.ErrRecursionDepthExceeded.WithArgs(p.maxDependencyDepth))
		return
	}

	if visited[mainKey] {
		p.addError(errs.ErrCircularDependency.WithArgs(mainKey))
		return
	}

	visited[mainKey] = true

	for _, depends := range p.getDependentFlags(flagInfo.Argument) {
		dependentFlag, found := p.getFlagInCommandPath(depends, flagInfo.CommandPath)
		if !found {
			p.addError(errs.ErrDependencyNotFound.WithArgs(mainKey, depends, flagInfo.CommandPath))
			continue
		}

		dependKey := p.options[depends]
		matches, allowedValues := p.checkDependencyValue(flagInfo.Argument, depends, dependKey)
		if !matches {
			p.addError(errs.ErrDependencyValueNotSpecified.WithArgs(mainKey, depends, allowedValues, dependKey))
		}

		p.validateDependencies(dependentFlag, depends, visited, depth+1)
	}

	visited[mainKey] = false
}

func (p *Parser) getFlagInCommandPath(flag string, commandPath string) (*FlagInfo, bool) {
	// First, check if the flag exists in the command-specific path
	if commandPath != "" {
		flagKey := buildPathFlag(flag, commandPath)
		if flagInfo, exists := p.acceptedFlags.Get(flagKey); exists {
			return flagInfo, true
		}
	}

	// Fallback to global flag
	if flagInfo, exists := p.acceptedFlags.Get(flag); exists {
		return flagInfo, true
	}

	return nil, false
}

func (p *Parser) setBoundVariable(value string, currentArg string) error {
	data, found := p.bind[currentArg]
	if !found {
		return nil
	}

	flagInfo, _ := p.acceptedFlags.Get(currentArg)
	if value == "" {
		value = flagInfo.Argument.DefaultValue
	}

	if len(p.customBind) > 0 {
		customProc, found := p.customBind[currentArg]
		if found {
			customProc(currentArg, value, data)
			return nil
		}
	}

	// For Chained type with repeated flag support, check if we need to append
	if flagInfo.Argument.TypeOf == types.Chained {
		return p.appendOrSetBoundVariable(value, data, currentArg, p.listFunc)
	}

	return util.ConvertString(value, data, currentArg, p.listFunc)
}

// appendOrSetBoundVariable handles repeated flags by appending to slice types
// or replacing the value for non-slice types. This enables the pattern:
// -o option1 -o option2 instead of -o "option1,option2"
func (p *Parser) appendOrSetBoundVariable(value string, data any, currentArg string, delimiterFunc types.ListDelimiterFunc) error {
	// Check if we've already seen this flag
	doAppend := true
	if !p.repeatedFlags[currentArg] {
		// First occurrence, mark it as seen and set normally
		p.repeatedFlags[currentArg] = true
		doAppend = false
	}

	return util.ConvertString(value, data, currentArg, delimiterFunc, doAppend)
}

func (p *Parser) prefixFunc(r rune) bool {
	for i := 0; i < len(p.prefixes); i++ {
		if r == p.prefixes[i] {
			return true
		}
	}

	return false
}

func (p *Parser) getListDelimiterFunc() types.ListDelimiterFunc {
	if p.listFunc != nil {
		return p.listFunc
	}

	return matchChainedSeparators
}

func (p *Parser) groupEnvVarsByCommand() map[string][]string {
	commandEnvVars := make(map[string][]string)
	if p.envNameConverter == nil {
		return commandEnvVars
	}
	for _, env := range p.envResolver.Environ() {
		kv := strings.Split(env, "=")
		v := p.envNameConverter(kv[0])
		if v == "" {
			continue
		}
		for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
			paths := splitPathFlag(*f.Key)
			length := len(paths)
			// Global flag (no command path)
			if length == 1 && paths[0] == v {
				commandEnvVars["global"] = append(commandEnvVars["global"], fmt.Sprintf("--%s", *f.Key), kv[1])
			}
			// Command-specific flag
			if length > 1 && paths[0] == v || v == f.Value.Argument.Short {
				commandEnvVars[paths[1]] = append(commandEnvVars[paths[1]], fmt.Sprintf("--%s", *f.Key), kv[1])
			}
		}
	}

	return commandEnvVars
}

func (p *Parser) mergeCmdLine(nestedCmdLine *Parser) error {
	for k, v := range nestedCmdLine.bind {
		if _, exists := p.bind[k]; exists {
			return errs.ErrFlagAlreadyExists.WithArgs(k)
		}
		p.bind[k] = v
	}
	for k, v := range nestedCmdLine.customBind {
		p.customBind[k] = v
	}
	for it := nestedCmdLine.acceptedFlags.Front(); it != nil; it = it.Next() {
		p.acceptedFlags.Set(*it.Key, it.Value)
	}
	for k, v := range nestedCmdLine.lookup {
		p.lookup[k] = v
	}
	for it := nestedCmdLine.registeredCommands.Front(); it != nil; it = it.Next() {
		// Check if command already exists and preserve its properties
		if existing, found := p.registeredCommands.Get(*it.Key); found {
			newCmd := it.Value
			// Preserve existing properties that shouldn't be overwritten
			if existing.NameKey != "" && newCmd.NameKey == "" {
				newCmd.NameKey = existing.NameKey
			}
			if existing.Description != "" && newCmd.Description == "" {
				newCmd.Description = existing.Description
			}
			if existing.DescriptionKey != "" && newCmd.DescriptionKey == "" {
				newCmd.DescriptionKey = existing.DescriptionKey
			}
			p.registeredCommands.Set(*it.Key, newCmd)
		} else {
			p.registeredCommands.Set(*it.Key, it.Value)
		}
	}

	// Merge errors from nested parser
	for _, err := range nestedCmdLine.errors {
		p.addError(err)
	}

	// Merge translation registry
	if nestedCmdLine.translationRegistry != nil && p.translationRegistry != nil {
		p.translationRegistry.Merge(nestedCmdLine.translationRegistry)
	}

	return nil
}

func showDependencies(dependencies []string) string {
	buf := strings.Builder{}
	dependLen := len(dependencies)
	if dependLen > 0 {
		for i, k := range dependencies {
			if len(strings.TrimSpace(k)) == 0 {
				continue
			}
			buf.WriteString("'" + k + "'")
			if i < dependLen-1 {
				buf.WriteString(" or ")
			}
		}
	}

	return buf.String()
}

func matchChainedSeparators(r rune) bool {
	return r == ',' || r == '|' || r == ' '
}

func pruneExecPathFromArgs(args *[]string) {
	if len(*args) > 0 {
		osBase := os.Args[0]
		if strings.EqualFold(osBase, (*args)[0]) {
			*args = (*args)[1:]
		}
	}
}

const (
	ExecDir = "${EXEC_DIR}"
)

func varFunc(s string) string {
	switch strings.ToUpper(s) {
	case ExecDir:
		p, e := os.Executable()
		if e != nil {
			return s
		}
		return filepath.Dir(p)
	default:
		return s
	}
}

func expandVarExpr() *regexp.Regexp {
	return regexp.MustCompile(`(\$\{.+})`)
}

func unmarshalTagsToArgument(bundle *i18n.Bundle, field reflect.StructField, arg *Argument) (name string, path string, err error) {
	if tag, ok := field.Tag.Lookup("goopt"); ok && strings.Contains(tag, ":") {
		config, err := parse.UnmarshalTagFormat(tag, field)
		if err != nil {
			return "", "", err
		}

		if bundle != nil && config.DescriptionKey != "" {
			if tr := bundle.T(config.DescriptionKey); tr != config.DescriptionKey {
				config.Description = tr
			}
		}

		newArg, err := toArgument(config)
		if err != nil {
			return "", "", err
		}
		*arg = *newArg
		return config.Name, config.Path, nil
	}

	if isStructOrSliceType(field) {
		return "", "", nil // For nested structs, slices and arrays nil config is valid
	}

	return "", "", errs.ErrNoValidTags
}

func toArgument(c *types.TagConfig) (*Argument, error) {

	configs := []ConfigureArgumentFunc{
		WithType(c.TypeOf),
		WithDescription(c.Description),
		WithDefaultValue(c.Default),
		WithDescriptionKey(c.DescriptionKey),
		WithNameKey(c.NameKey),
		WithDependencyMap(c.DependsOn),
		WithShortFlag(c.Short),
		WithRequired(c.Required),
		WithAcceptedValues(c.AcceptedValues),
		WithDefaultValue(c.Default),
	}

	// Convert AcceptedValues to validators for internal processing
	// but still store them as AcceptedValues for backward compatibility (help text, etc.)
	var acceptedValueValidators []validation.ValidatorFunc
	if len(c.AcceptedValues) > 0 {
		// Store AcceptedValues for help text and backward compatibility
		configs = append(configs, WithAcceptedValues(c.AcceptedValues))

		// Create validators from AcceptedValues
		for _, av := range c.AcceptedValues {
			// Each AcceptedValue becomes a regex validator
			validator := validation.Regex(av.Pattern, av.Description)
			acceptedValueValidators = append(acceptedValueValidators, validator)
		}
	}

	// Parse and add validators
	var allValidators []validation.ValidatorFunc

	// First add validators from accepted values (if any)
	if len(acceptedValueValidators) > 0 {
		// Use OneOf so any accepted value matches
		allValidators = append(allValidators, validation.OneOf(acceptedValueValidators...))
	}

	// Then add explicit validators
	if len(c.Validators) > 0 {
		validators, err := validation.ParseValidators(c.Validators)
		if err != nil {
			return nil, err
		}
		if len(validators) > 0 {
			allValidators = append(allValidators, validators...)
		}
	}

	// Add all validators to the argument
	if len(allValidators) > 0 {
		configs = append(configs, WithValidators(allValidators...))
	}

	arg := NewArg(configs...)

	arg.Secure = c.Secure
	arg.Position = c.Position

	return arg, nil
}

// CommandConfig holds configuration for building a command
type CommandConfig struct {
	Path           string
	Description    string
	DescriptionKey string
	NameKey        string
	Parent         *Command
}

func (p *Parser) buildCommand(commandPath, description, descriptionKey string, parent *Command) (*Command, error) {
	// Backward compatibility wrapper
	return p.buildCommandFromConfig(&CommandConfig{
		Path:           commandPath,
		Description:    description,
		DescriptionKey: descriptionKey,
		Parent:         parent,
	})
}

func (p *Parser) buildCommandFromConfig(config *CommandConfig) (*Command, error) {
	if config.Path == "" {
		return nil, errs.ErrEmptyCommandPath
	}

	commandNames := strings.Split(config.Path, " ")

	var topParent = config.Parent
	var currentCommand *Command

	for i, cmdName := range commandNames {
		found := false
		isLastCommand := (i == len(commandNames)-1)

		// If we're at the top level (parent is nil)
		if config.Parent == nil {
			// Look for the command at the top level
			if cmd, exists := p.registeredCommands.Get(cmdName); exists {
				currentCommand = cmd
				found = true
				// Update properties if this is the command being configured
				if len(commandNames) == 1 || isLastCommand {
					if config.NameKey != "" {
						currentCommand.NameKey = config.NameKey
					}
					if config.Description != "" || config.DescriptionKey != "" {
						p.resolveCommandDescription(config.Description, currentCommand, cmdName, config.DescriptionKey)
					}
				}
			} else {
				// Create a new top-level command
				newCommand := &Command{
					Name: cmdName,
				}

				// Only apply properties if this is the actual command being configured
				// For a path like "top middle", when processing "top", we only set its properties
				// if the full path is "top" (i.e., single command)
				if len(commandNames) == 1 || isLastCommand {
					newCommand.NameKey = config.NameKey
					p.resolveCommandDescription(config.Description, newCommand, cmdName, config.DescriptionKey)
				}
				p.registeredCommands.Set(cmdName, newCommand)
				currentCommand = newCommand
			}
		} else {
			if cmdName == config.Parent.Name {
				continue
			}
			for idx, subCmd := range config.Parent.Subcommands {
				if subCmd.Name == cmdName {
					currentCommand = &config.Parent.Subcommands[idx] // Use the existing subcommand
					found = true
					// Update properties if this is the command being configured
					if len(commandNames) == 1 || isLastCommand {
						if config.NameKey != "" {
							currentCommand.NameKey = config.NameKey
						}
						if config.Description != "" || config.DescriptionKey != "" {
							p.resolveCommandDescription(config.Description, currentCommand, cmdName, config.DescriptionKey)
						}
					}
					break
				}
			}

			if !found {
				newCommand := &Command{
					Name:        cmdName,
					Subcommands: []Command{},
					path:        config.Path,
				}
				// For single command paths, always apply properties
				// For multi-command paths, only apply to the last command
				if len(commandNames) == 1 || isLastCommand {
					newCommand.NameKey = config.NameKey
					p.resolveCommandDescription(config.Description, newCommand, cmdName, config.DescriptionKey)
				}
				config.Parent.Subcommands = append(config.Parent.Subcommands, *newCommand)
				currentCommand = &config.Parent.Subcommands[len(config.Parent.Subcommands)-1] // Update currentCommand to point to the new subcommand
			}
		}

		// Set the top parent (the first command in the hierarchy)
		if topParent == nil {
			topParent = currentCommand
		}

		// Move to the next level in the hierarchy
		config.Parent = currentCommand
	}

	// Add the top-level command if not already registered
	if topParent != nil && config.Parent == nil {
		if _, exists := p.registeredCommands.Get(topParent.Name); !exists {
			p.registeredCommands.Set(topParent.Name, topParent)
		}
	}

	return topParent, nil
}

func (p *Parser) resolveCommandDescription(description string, newCommand *Command, cmdName string, descriptionKey string) {
	if description != "" {
		newCommand.Description = description
	} else {
		newCommand.Description = fmt.Sprintf("Auto-generated command for %s", cmdName)
	}
	if descriptionKey != "" {
		newCommand.DescriptionKey = descriptionKey
		if p.userI18n != nil {
			if tr := p.userI18n.T(newCommand.DescriptionKey); tr != "" {
				newCommand.Description = tr
			}
		}
	}
}

func newParserFromReflectValue(structValue reflect.Value, flagPrefix, commandPath string, maxDepth, currentDepth int, config ...ConfigureCmdLineFunc) (*Parser, error) {
	if currentDepth > maxDepth {
		return nil, errs.ErrRecursionDepthExceeded.WithArgs(maxDepth)
	}

	var err error
	parser := NewParser()
	for _, cfg := range config {
		cfg(parser, &err)
		if err != nil {
			return nil, errs.ErrConfiguringParser.Wrap(err)
		}
	}

	// Unwrap the value and type
	unwrappedValue, err := util.UnwrapValue(structValue)
	if err != nil {
		return nil, errs.WrapOnce(err, errs.ErrUnwrappingValue)
	}

	st := util.UnwrapType(structValue.Type())
	if st.Kind() != reflect.Struct {
		return nil, errs.ErrOnlyStructsCanBeTagged
	}

	err = parser.processStructCommands(unwrappedValue, commandPath, currentDepth, maxDepth, nil)
	if err != nil {
		return nil, err
	}

	// Use unwrappedValue for field iteration
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		if _, ok := field.Tag.Lookup("ignore"); ok {
			continue
		}

		fieldValue := unwrappedValue.Field(i) // Use unwrappedValue here
		if !fieldValue.CanAddr() || !fieldValue.CanInterface() {
			continue
		}

		var (
			longName string
			pathTag  string
		)
		arg := &Argument{}
		longName, pathTag, err = unmarshalTagsToArgument(nil, field, arg)
		if err != nil {
			// ErrNoValidTags is not an error - it just means the field has no goopt tags
			if errors.Is(err, errs.ErrNoValidTags) {
				continue
			}

			// Check if this is a validator syntax error that should fail immediately
			var validatorErr *i18n.TrError
			if errors.As(err, &validatorErr) && errors.Is(err, errs.ErrValidatorMustUseParentheses) {
				return nil, errs.WrapOnce(err, errs.ErrProcessingField, field.Name)
			}

			// For other errors, decide based on field type
			if !isFunction(field) && !isStructOrSliceType(field) {
				// For simple fields with tag errors, we can skip them and continue
				parser.addError(errs.WrapOnce(err, errs.ErrProcessingField, field.Name))
				continue
			}
			// For structural fields (functions, nested structs, slices), fail fast
			if !isFunction(field) {
				if flagPrefix != "" {
					return nil, errs.WrapOnce(err, errs.ErrProcessingFieldWithPrefix, flagPrefix, field.Name)
				} else {
					return nil, errs.WrapOnce(err, errs.ErrProcessingField, field.Name)
				}
			}
			continue
		}

		isCommand := isFieldCommand(field)

		if longName == "" {
			if isCommand {
				longName = parser.commandNameConverter(field.Name)
			} else {
				longName = parser.flagNameConverter(field.Name)
			}
		}

		// Create new flag prefix only for non-command fields
		fieldFlagPath := flagPrefix
		if !isCommand {
			if flagPrefix == "" {
				fieldFlagPath = longName
			} else {
				fieldFlagPath = fmt.Sprintf("%s.%s", flagPrefix, longName)
			}
		}

		if isSliceType(field) && !isBasicType(field.Type) {
			// Process as nested structure
			if err := processSliceField(fieldFlagPath, commandPath, fieldValue, maxDepth, currentDepth, parser, config...); err != nil {
				return nil, err
			}
			continue
		}

		if isStructType(field) {
			// Skip if this is a Parser instance (e.g. embedded in another struct)
			if field.Type == reflect.TypeOf(Parser{}) {
				continue
			}
			newCommandPath := commandPath
			if isCommand {
				if newCommandPath == "" {
					newCommandPath = longName
				} else {
					newCommandPath = fmt.Sprintf("%s %s", newCommandPath, longName)
				}
			}

			if err = processNestedStruct(fieldFlagPath, newCommandPath, fieldValue, maxDepth, currentDepth, parser, config...); err != nil {
				// For structural errors during parser creation, fail immediately with the original error
				return nil, err
			}
			continue
		}

		// Use both flag prefix and command path appropriately
		fullFlagName := longName
		if flagPrefix != "" {
			fullFlagName = fmt.Sprintf("%s.%s", flagPrefix, longName)
		}

		// Process the path tag to associate the flag with commands or global
		if pathTag != "" {
			if err = parser.processPathTag(pathTag, fieldValue, fullFlagName, arg); err != nil {
				return parser, errs.WrapOnce(err, errs.ErrProcessingFlag, fullFlagName)
			}
		} else {
			// If no path specified, use current command path (if any)
			err = parser.bindArgument(commandPath, fieldValue, fullFlagName, arg)
			if err != nil {
				return parser, errs.WrapOnce(err, errs.ErrProcessingFlag, fullFlagName)
			}
		}
	}

	return parser, nil
}

func (p *Parser) processPathTag(pathTag string, fieldValue reflect.Value, fullFlagName string, arg *Argument) error {
	paths := strings.Split(pathTag, ",")
	for _, cmdPath := range paths {
		cmdPathComponents := strings.Split(cmdPath, " ")
		parentCommand := ""
		var cmd *Command
		var pCmd *Command
		var err error

		for i, cmdComponent := range cmdPathComponents {
			if i == 0 {
				if p, ok := p.registeredCommands.Get(cmdComponent); ok {
					pCmd = p
				}
			}
			if parentCommand == "" {
				parentCommand = cmdComponent
			} else {
				parentCommand = fmt.Sprintf("%s %s", parentCommand, cmdComponent)
			}

			if cmd, err = p.buildCommand(parentCommand, "", "", pCmd); err != nil {
				return errs.WrapOnce(err, errs.ErrProcessingCommand, parentCommand)
			}
		}

		if cmd != nil {
			err = p.AddCommand(cmd)
			if err != nil {
				return err
			}
		}

		err = p.BindFlag(fieldValue.Addr().Interface(), fullFlagName, arg, cmdPath)
		if err != nil {
			return err
		}
		if arg.DefaultValue != "" {
			err = p.setBoundVariable(arg.DefaultValue, buildPathFlag(fullFlagName, cmdPath))
			if err != nil {
				// Default value binding errors are not critical - collect them
				p.addError(errs.WrapOnce(err, errs.ErrSettingBoundValue, arg.DefaultValue))
			}
		}
	}

	return nil
}

func (p *Parser) bindArgument(commandPath string, fieldValue reflect.Value, fullFlagName string, arg *Argument) (err error) {
	if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
		// For nil pointers, add to errors but don't fail the entire parser construction
		p.addError(errs.ErrBindNil.WithArgs(fullFlagName))
		return nil
	}
	// Get the interface value - if it's already a pointer, use it directly
	var interfaceValue interface{}
	if fieldValue.Kind() == reflect.Ptr {
		interfaceValue = fieldValue.Interface()
	} else {
		interfaceValue = fieldValue.Addr().Interface()
	}

	if commandPath != "" {
		err = p.BindFlag(interfaceValue, fullFlagName, arg, commandPath)
	} else {
		// Global flag
		err = p.BindFlag(interfaceValue, fullFlagName, arg)
	}
	if err != nil {
		return err
	}
	if arg.DefaultValue != "" {
		if commandPath != "" {
			err = p.setBoundVariable(arg.DefaultValue, buildPathFlag(fullFlagName, commandPath))
		} else {
			err = p.setBoundVariable(arg.DefaultValue, fullFlagName)
		}
		if err != nil {
			// Default value binding errors are not critical - collect them
			p.addError(errs.WrapOnce(err, errs.ErrSettingBoundValue, arg.DefaultValue))
		}
	}

	return nil
}

func (p *Parser) processStructCommands(val reflect.Value, currentPath string, currentDepth, maxDepth int, callbackMap map[string]CommandFunc) error {
	if callbackMap == nil {
		callbackMap = make(map[string]CommandFunc)
	}

	// Handle case where the entire value is a Command type (not a struct containing commands)
	unwrappedValue, err := util.UnwrapValue(val)
	if err != nil {
		return errs.WrapOnce(err, errs.ErrUnwrappingValue)
	}

	if unwrappedValue.Type() == reflect.TypeOf(Command{}) {
		cmd := unwrappedValue.Interface().(Command)
		_, err = p.buildCommandFromConfig(&CommandConfig{
			Path:           cmd.path,
			Description:    cmd.Description,
			DescriptionKey: cmd.DescriptionKey,
			NameKey:        cmd.NameKey,
			Parent:         nil,
		})
		if err != nil {
			return errs.WrapOnce(err, errs.ErrProcessingCommand, cmd.path)
		}
		err = p.AddCommand(&cmd)
		if err != nil {
			return err
		}
		return nil
	}

	// Prevent infinite recursion
	typ := util.UnwrapType(val.Type())
	if currentDepth > maxDepth {
		return errs.ErrRecursionDepthExceeded.WithArgs(maxDepth)
	}

	// Process all fields in the struct
	for i := 0; i < unwrappedValue.NumField(); i++ {
		field := unwrappedValue.Field(i)
		fieldType := typ.Field(i)
		if !fieldType.IsExported() || fieldType.Tag.Get("ignore") != "" {
			continue
		}

		// Handle pointer fields
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				continue
			}
			fieldValue = field.Elem()
		}

		if field.Type().AssignableTo(reflect.TypeOf(CommandFunc(nil))) {
			// Only store if we're in a command context (currentPath is not empty)
			if currentPath != "" {
				cmd, ok := p.registeredCommands.Get(currentPath)
				if !ok {
					// Skip this field instead of failing - the command might not be registered yet due to an earlier error
					continue
				}

				// If the callback is already set (non-nil), use it directly
				if field.IsValid() && !field.IsZero() {
					callbackFunc := field.Interface().(CommandFunc)
					callbackMap[currentPath] = callbackFunc
				} else {
					// Store the field reference for later checking
					cmd.callbackLocation = field
				}
			}
			continue // Skip further processing for this field
		}

		// Process Command fields first - these are explicit command definitions
		if fieldType.Type == reflect.TypeOf(Command{}) {
			cmd := fieldValue.Interface().(Command)
			// Build path by combining current path with command name
			cmdPath := cmd.Name
			if currentPath != "" {
				cmdPath = currentPath + " " + cmd.Name
			}

			// Find the root parent command
			var parent *Command
			if currentPath != "" {
				parentPath := strings.Split(currentPath, " ")[0]
				if reg, ok := p.registeredCommands.Get(parentPath); ok {
					parent = reg
				}
			}

			buildCmd, err := p.buildCommandFromConfig(&CommandConfig{
				Path:           cmdPath,
				Description:    cmd.Description,
				DescriptionKey: cmd.DescriptionKey,
				NameKey:        cmd.NameKey,
				Parent:         parent,
			})
			if err != nil {
				return errs.ErrProcessingCommand.WithArgs(cmdPath).Wrap(err)
			}

			err = p.AddCommand(buildCmd)
			if err != nil {
				return err
			}
			continue
		}

		// Then process struct fields which might contain struct tags defining nested commands
		if fieldValue.Kind() == reflect.Struct && isFieldCommand(fieldType) {
			// Parse the goopt tag for command configuration
			config, err := parse.UnmarshalTagFormat(fieldType.Tag.Get("goopt"), fieldType)
			if err != nil {
				return errs.WrapOnce(err, errs.ErrUnmarshallingTag, fieldType.Name)
			}

			cmdName := config.Name
			if cmdName == "" {
				cmdName = p.commandNameConverter(fieldType.Name)
			}

			// Build the command path
			cmdPath := cmdName
			if currentPath != "" {
				cmdPath = currentPath + " " + cmdName
			}

			// Find parent command if we're nested
			var parent *Command
			if currentPath != "" {
				parentPath := strings.Split(currentPath, " ")[0]
				if reg, ok := p.registeredCommands.Get(parentPath); ok {
					parent = reg
				}
			}

			// Build and register the command
			buildCmd, err := p.buildCommandFromConfig(&CommandConfig{
				Path:           cmdPath,
				Description:    config.Description,
				DescriptionKey: config.DescriptionKey,
				NameKey:        config.NameKey,
				Parent:         parent,
			})
			if err != nil {
				return errs.WrapOnce(err, errs.ErrProcessingCommand, cmdPath)
			}

			err = p.AddCommand(buildCmd)
			if err != nil {
				return err
			}

			// Process nested structure with updated path
			if err := p.processStructCommands(fieldValue, cmdPath, currentDepth+1, maxDepth, callbackMap); err != nil {
				return err
			}
		} else if fieldValue.Kind() == reflect.Struct {
			// Process non-command struct fields for nested commands
			if err := p.processStructCommands(fieldValue, currentPath, currentDepth+1, maxDepth, callbackMap); err != nil {
				return err
			}
		}
	}

	// Process all callbacks collected at this level
	for cmdPath, callback := range callbackMap {
		// Get the command by path
		cmd, ok := p.registeredCommands.Get(cmdPath)
		if !ok {
			// Command might not be registered yet if it's in a nested structure
			// This is not an error during construction
			continue
		}

		// Skip if we've already processed this callback
		if cmd.Callback != nil {
			continue
		}

		// Check if this is a terminal command (no subcommands)
		if len(cmd.Subcommands) == 0 {
			// we can safely ignore the error because we know the command exists
			_ = p.SetCommand(cmdPath, WithCallback(callback))
		} else {
			// Callback on non-terminal command is a validation error, not structural
			// Check if we've already added this error to avoid duplicates
			errMsg := errs.ErrProcessingCommand.WithArgs(cmdPath).Wrap(errs.ErrCallbackOnNonTerminalCommand).Error()
			alreadyExists := false
			for _, existingErr := range p.errors {
				if existingErr.Error() == errMsg {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				p.addError(errs.ErrProcessingCommand.WithArgs(cmdPath).Wrap(errs.ErrCallbackOnNonTerminalCommand))
			}
		}
	}

	return nil
}

// checkDependencyValue checks if the provided value matches any of the required values
// for a given dependency
func (p *Parser) checkDependencyValue(arg *Argument, dependentFlag string, actualValue string) (bool, []string) {
	allowedValues, exists := arg.DependencyMap[dependentFlag]
	if !exists {
		// Flag not in dependency map means no dependency
		return true, nil
	}

	// nil or empty slice means any value is acceptable (flag just needs to be present)
	if len(allowedValues) == 0 {
		return true, nil
	}

	// Check if actual value matches any allowed value
	for _, allowed := range allowedValues {
		if strings.EqualFold(actualValue, allowed) {
			return true, allowedValues
		}
	}
	return false, allowedValues
}

// getDependentFlags returns all flags that this argument depends on
func (p *Parser) getDependentFlags(arg *Argument) []string {
	deps := make([]string, 0, len(arg.DependencyMap))
	for dep := range arg.DependencyMap {
		deps = append(deps, dep)
	}
	return deps
}

func (p *Parser) validateSlicePath(path string) error {
	parts := strings.Split(path, ".")
	currentPath := ""

	for i, part := range parts {
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += part

		// Try to parse as index
		if idx, err := strconv.Atoi(part); err == nil {
			// This part is a numeric index - check against parent's capacity
			parentPath := strings.Join(parts[:i], ".")
			if flagInfo, exists := p.acceptedFlags.Get(parentPath); exists {
				if flagInfo.Argument == nil {
					return errs.ErrMissingArgumentInfo.WithArgs(parentPath)
				}

				if flagInfo.Argument.Capacity <= 0 {
					return errs.ErrNegativeCapacity.WithArgs(parentPath, flagInfo.Argument.Capacity)
				}

				if idx < 0 || idx >= flagInfo.Argument.Capacity {
					return errs.ErrIndexOutOfBounds.WithArgs(idx, currentPath, flagInfo.Argument.Capacity-1)
				}
			}
		}
	}

	// Final check - does this path exist in our accepted flags?
	if _, exists := p.acceptedFlags.Get(path); !exists {
		return errs.ErrUnknownFlag.WithArgs(path)
	}

	return nil
}

func processSliceField(flagPrefix, commandPath string, fieldValue reflect.Value, maxDepth, currentDepth int, c *Parser, config ...ConfigureCmdLineFunc) error {
	// Get capacity from Argument if specified
	capacity := 0
	if flagInfo, exists := c.acceptedFlags.Get(flagPrefix); exists && flagInfo.Argument != nil {
		capacity = flagInfo.Argument.Capacity
	}

	// Unwrap the field value
	unwrappedValue, err := util.UnwrapValue(fieldValue)
	if err != nil {
		return errs.WrapOnce(err, errs.ErrUnwrappingValue, flagPrefix)
	}

	// Initialize or resize slice if needed
	if (unwrappedValue.Kind() == reflect.Slice && unwrappedValue.IsNil()) ||
		(capacity > 0 && unwrappedValue.Cap() != capacity) {
		newSlice := reflect.MakeSlice(unwrappedValue.Type(), capacity, capacity)
		if !unwrappedValue.IsNil() && unwrappedValue.Len() > 0 {
			copyLen := util.Min(unwrappedValue.Len(), capacity)
			reflect.Copy(newSlice.Slice(0, copyLen), unwrappedValue.Slice(0, copyLen))
		}
		unwrappedValue.Set(newSlice)
	}

	// Process each slice element
	for idx := 0; idx < unwrappedValue.Len(); idx++ {
		elem := unwrappedValue.Index(idx).Addr()

		// Create full path with the slice index
		elemPrefix := fmt.Sprintf("%s.%d", flagPrefix, idx)

		nestedCmdLine, err := newParserFromReflectValue(elem, elemPrefix, commandPath, maxDepth, currentDepth+1, config...)
		if err != nil {
			return errs.WrapOnce(err, errs.ErrProcessingField, flagPrefix, idx)
		}
		if err = c.mergeCmdLine(nestedCmdLine); err != nil {
			return errs.WrapOnce(err, errs.ErrProcessingField, flagPrefix, idx)
		}
	}

	return nil
}

func processNestedStruct(flagPrefix, commandPath string, fieldValue reflect.Value, maxDepth, currentDepth int, c *Parser, config ...ConfigureCmdLineFunc) error {
	unwrappedValue, err := util.UnwrapValue(fieldValue)
	if err != nil {
		if errors.Is(err, errs.ErrNilPointer) {
			// Nil pointer - this is fine
			return nil
		}
		return errs.WrapOnce(err, errs.ErrUnwrappingValue, flagPrefix)
	}

	var existingCmdDescription string
	if commandPath != "" {
		if existingCmd, found := c.registeredCommands.Get(commandPath); found && existingCmd.Description != "" {
			existingCmdDescription = existingCmd.Description
		}
	}

	nestedCmdLine, err := newParserFromReflectValue(unwrappedValue.Addr(), flagPrefix, commandPath, maxDepth, currentDepth+1, config...)
	if err != nil {
		return errs.WrapOnce(err, errs.ErrProcessingField, flagPrefix)
	}

	err = c.mergeCmdLine(nestedCmdLine)
	if err != nil {
		return err
	}
	if cmd, ok := c.registeredCommands.Get(commandPath); ok && existingCmdDescription != "" && cmd.Description != existingCmdDescription {
		cmd.Description = existingCmdDescription
	}

	return nil
}

func buildPathFlag(flag string, commandPath ...string) string {
	if strings.Count(flag, "@") == 0 && len(commandPath) > 0 && commandPath[0] != "" {
		return fmt.Sprintf("%s@%s", flag, strings.Join(commandPath, " "))
	}

	return flag
}

func splitPathFlag(flag string) []string {
	return strings.Split(flag, "@")
}

func getFlagPath(flag string) string {
	paths := splitPathFlag(flag)
	if len(paths) == 2 {
		return paths[1]
	}

	return ""
}

func addFlagToCompletionData(data *completion.CompletionData, cmd, flagName string, flagInfo *FlagInfo, renderer Renderer) {
	if flagInfo == nil || flagInfo.Argument == nil {
		return
	}

	// Create flag pair with type conversion
	pair := completion.FlagPair{
		Long:        flagName,
		Short:       flagInfo.Argument.Short,
		Description: renderer.FlagDescription(flagInfo.Argument),
		Type:        completion.FlagType(flagInfo.Argument.TypeOf),
	}

	// Add to appropriate flag list
	if cmd == "" {
		data.Flags = append(data.Flags, pair)
	} else {
		data.CommandFlags[cmd] = append(data.CommandFlags[cmd], pair)
	}

	// Handle flag values
	if len(flagInfo.Argument.AcceptedValues) > 0 {
		values := make([]completion.CompletionValue, len(flagInfo.Argument.AcceptedValues))
		for i, v := range flagInfo.Argument.AcceptedValues {
			values[i] = completion.CompletionValue{
				Pattern:     v.Pattern,
				Description: v.Description,
			}
		}

		// Add values for both forms if short exists
		if flagInfo.Argument.Short != "" {
			shortKey := pair.Short
			longKey := pair.Long
			if cmd != "" {
				shortKey = cmd + "@" + shortKey
				longKey = cmd + "@" + longKey
			}
			data.FlagValues[shortKey] = values
			data.FlagValues[longKey] = values
		}
	}
}

func isFieldCommand(field reflect.StructField) bool {
	// Use UnwrapType instead of manual pointer.go unwrapping
	typ := util.UnwrapType(field.Type)

	var isCommand bool
	if cmd, ok := field.Tag.Lookup("goopt"); ok {
		isCommand = strings.Contains(cmd, "kind:command")
	}

	if !isCommand && typ.Kind() == reflect.Struct {
		isCommand = typ == reflect.TypeOf(Command{})
	}

	return isCommand
}

// Matches patterns like:
// - field.0.inner
// - field.0.inner.1.more
// - field.0.inner.1.more.2.even.more
var nestedSlicePathRegex = regexp.MustCompile(`^([^.]+\.\d+\.[^.]+)(\.\d+\.[^.]+)*$`)

func isNestedSlicePath(path string) bool {
	return nestedSlicePathRegex.MatchString(path)
}

func isStructOrSliceType(field reflect.StructField) bool {
	return isSliceType(field) || isStructType(field)
}

func isFunction(field reflect.StructField) bool {
	unwrappedType := util.UnwrapType(field.Type)
	return unwrappedType.Kind() == reflect.Func
}

func isStructType(field reflect.StructField) bool {
	typ := field.Type
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return typ.Kind() == reflect.Struct
}

func isSliceType(field reflect.StructField) bool {
	typ := field.Type
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array
}

func isBasicType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	case reflect.Slice:
		// Check if it's a slice of basic types
		return isBasicType(t.Elem())
	case reflect.Ptr:
		// Check the type being pointed to
		return isBasicType(t.Elem())
	default:
		return false
	}
}

// shortFlagLookup performs context-aware lookup of short flags
// It checks from most specific (with full command path) to least specific (global)
func (p *Parser) shortFlagLookup(shortFlag string, commandPath ...string) (longFlag string, found bool) {
	// If not a short flag, return early
	if len(shortFlag) == 0 || len(shortFlag) > 1 && p.posixCompatible {
		return "", false
	}

	// Try with full command context first
	if len(commandPath) > 0 {
		contextualKey := buildPathFlag(shortFlag, commandPath...)
		if longFlag, found = p.lookup[contextualKey]; found {
			return longFlag, true
		}

		// Try parent contexts (walk up the command hierarchy)
		pathString := strings.Join(commandPath, " ")
		pathParts := strings.Split(pathString, " ")

		for i := len(pathParts) - 1; i > 0; i-- {
			parentPath := pathParts[:i]
			parentKey := buildPathFlag(shortFlag, parentPath...)
			if longFlag, found = p.lookup[parentKey]; found {
				return longFlag, true
			}
		}
	}

	// Try global context (no command path)
	if longFlag, found = p.lookup[shortFlag]; found {
		return longFlag, true
	}

	return "", false
}

// storeShortFlag stores a short flag with proper context in the lookup table
func (p *Parser) storeShortFlag(shortFlag, longFlag string, commandPath ...string) {
	if len(shortFlag) == 0 {
		return
	}

	// Store with context using @ notation
	contextualKey := buildPathFlag(shortFlag, commandPath...)
	p.lookup[contextualKey] = longFlag

	// For backward compatibility, also store without context if it's global
	if len(commandPath) == 0 {
		p.lookup[shortFlag] = longFlag
	}
}

// checkShortFlagConflict checks if a short flag would conflict in any context
func (p *Parser) checkShortFlagConflict(shortFlag, newFlag string, commandPath ...string) (conflictingFlag string, hasConflict bool) {
	if len(shortFlag) == 0 {
		return "", false
	}

	// Check exact context
	contextualKey := buildPathFlag(shortFlag, commandPath...)
	if existingFlag, exists := p.lookup[contextualKey]; exists && existingFlag != newFlag {
		return existingFlag, true
	}

	// If this is a global flag, check if it conflicts with any command-specific usage
	if len(commandPath) == 0 {
		// Check all entries in lookup table for conflicts
		for key, value := range p.lookup {
			// Skip non-short flag entries (UUIDs, etc)
			if len(key) > 1 && !strings.Contains(key, "@") {
				continue
			}

			// Check if this is the same short flag in a different context
			parts := strings.Split(key, "@")
			if parts[0] == shortFlag && value != newFlag {
				return value, true
			}
		}
	} else {
		// If this is a command flag, check if there's a global flag with the same short
		if globalFlag, exists := p.lookup[shortFlag]; exists {
			// This is a global short flag, so it conflicts
			return globalFlag, true
		}
	}

	return "", false
}

// detectBestStyle automatically selects the best help style based on CLI complexity
func (p *Parser) detectBestStyle() HelpStyle {
	flagCount := p.acceptedFlags.Count()
	cmdCount := p.registeredCommands.Count()

	// Large CLI with many flags and commands
	if float64(flagCount) > float64(p.helpConfig.CompactThreshold)*1.4 && cmdCount > 5 {
		return HelpStyleHierarchical
	}

	// Medium CLI with moderate complexity
	if flagCount > p.helpConfig.CompactThreshold {
		return HelpStyleCompact
	}

	// Small CLI with commands
	if cmdCount > 3 {
		return HelpStyleGrouped
	}

	// Simple CLI
	return HelpStyleFlat
}

// printFlatHelp prints traditional flat help
func (p *Parser) printFlatHelp(writer io.Writer) {
	p.PrintUsage(writer)
}

// printGroupedHelp prints help with flags grouped by command
func (p *Parser) printGroupedHelp(writer io.Writer) {
	p.PrintUsageWithGroups(writer)
}

// printCompactHelp prints deduplicated, compact help
func (p *Parser) printCompactHelp(writer io.Writer) {
	fmt.Fprintln(writer, p.layeredProvider.GetFormattedMessage(messages.MsgUsageKey, os.Args[0]))

	// Positional args if any
	p.PrintPositionalArgs(writer)

	// Global flags in compact form
	globalFlags := p.getGlobalFlags()
	if len(globalFlags) > 0 {
		fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgGlobalFlagsKey))
		for _, flag := range globalFlags {
			p.printCompactFlag(writer, flag)
		}
	}

	// Shared flag groups
	sharedGroups := p.detectSharedFlagGroups()
	if len(sharedGroups) > 0 {
		fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgSharedFlagsKey))

		// Sort groups for consistent output
		var groupNames []string
		for name := range sharedGroups {
			groupNames = append(groupNames, name)
		}
		sort.Strings(groupNames)

		for _, prefix := range groupNames {
			info := sharedGroups[prefix]
			if len(info.commands) > 1 {
				fmt.Fprintf(writer, "\n%s.* (%s: %s)\n",
					prefix,
					p.layeredProvider.GetMessage(messages.MsgUsedByKey),
					strings.Join(info.commands, ", "))

				// Show first few flags as examples
				for i, flag := range info.flags {
					if i >= 3 {
						fmt.Fprintf(writer, "  ... %s %d %s\n",
							p.layeredProvider.GetMessage(messages.MsgAndKey),
							len(info.flags)-3,
							p.layeredProvider.GetMessage(messages.MsgMoreKey))
						break
					}
					p.printCompactFlag(writer, flag)
				}
			}
		}
	}

	// Commands with flag counts
	if p.registeredCommands.Count() > 0 {
		fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgCommandsKey))
		for cmd := p.registeredCommands.Front(); cmd != nil; cmd = cmd.Next() {
			if cmd.Value.topLevel {
				flagCount := p.countCommandFlags(cmd.Value.Name)
				fmt.Fprintf(writer, "  %-15s %-40s",
					cmd.Value.Name,
					util.Truncate(p.renderer.CommandDescription(cmd.Value), 40))
				if flagCount > 0 {
					fmt.Fprintf(writer, " [%d %s]", flagCount, p.layeredProvider.GetMessage(messages.MsgFlagsKey))
				}
				fmt.Fprintln(writer)
			}
		}
	}

	fmt.Fprintf(writer, "\n%s\n", p.layeredProvider.GetMessage(messages.MsgHelpHintKey))
}

// printHierarchicalHelp prints hierarchical help for complex CLIs
func (p *Parser) printHierarchicalHelp(writer io.Writer) {
	fmt.Fprintf(writer, "%s\n\n", p.layeredProvider.GetFormattedMessage(messages.MsgUsageHierarchicalKey, os.Args[0]))

	// Only essential global flags
	globalFlags := p.getGlobalFlags()
	if len(globalFlags) > 0 {
		fmt.Fprintf(writer, "%s:\n", p.layeredProvider.GetMessage(messages.MsgGlobalFlagsKey))
		shown := 0
		maxToShow := p.helpConfig.MaxGlobals
		if maxToShow <= 0 {
			maxToShow = len(globalFlags) // Show all if MaxGlobals is 0 or negative
		}
		for _, flag := range globalFlags {
			// Only show help and essential flags
			if flag.Short == "h" || flag.Short == "help" || flag.Required {
				p.printCompactFlag(writer, flag)
				shown++
			}
			if shown >= maxToShow {
				break
			}
		}
		if len(globalFlags) > shown {
			fmt.Fprintf(writer, "  ... %s %d %s\n",
				p.layeredProvider.GetMessage(messages.MsgAndKey),
				len(globalFlags)-shown,
				p.layeredProvider.GetMessage(messages.MsgMoreKey))
		}
	}

	// Shared flag groups summary
	sharedGroups := p.detectSharedFlagGroups()
	if len(sharedGroups) > 0 {
		fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgSharedFlagGroupsKey))

		// Sort and display
		type groupInfo struct {
			name     string
			cmdCount int
		}
		var groups []groupInfo
		for prefix, info := range sharedGroups {
			if len(info.commands) > 1 {
				groups = append(groups, groupInfo{prefix, len(info.commands)})
			}
		}
		sort.Slice(groups, func(i, j int) bool {
			return groups[i].cmdCount > groups[j].cmdCount
		})

		for _, g := range groups {
			fmt.Fprintf(writer, "  %-20s %s %d %s\n",
				g.name+".*",
				p.layeredProvider.GetMessage(messages.MsgUsedByKey),
				g.cmdCount,
				p.layeredProvider.GetMessage(messages.MsgCommandsKey))
		}
	}

	// Command structure
	if p.registeredCommands.Count() > 0 {
		fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgCommandStructureKey))
		p.printCommandTree(writer)
	}

	// Examples
	fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgExamplesKey))
	fmt.Fprintf(writer, "  %s --help                    # %s\n",
		os.Args[0], p.layeredProvider.GetMessage(messages.MsgThisHelpKey))
	if p.registeredCommands.Count() > 0 {
		// Show first command as example
		if first := p.registeredCommands.Front(); first != nil {
			fmt.Fprintf(writer, "  %s %s --help              # %s\n",
				os.Args[0], first.Value.Name, p.layeredProvider.GetMessage(messages.MsgCommandHelpKey))
			if len(first.Value.Subcommands) > 0 {
				fmt.Fprintf(writer, "  %s %s %s --help       # %s\n",
					os.Args[0], first.Value.Name, first.Value.Subcommands[0].Name,
					p.layeredProvider.GetMessage(messages.MsgSubcommandHelpKey))
			}
		}
	}
}

// Helper functions

// getGlobalFlags returns flags with no command path
func (p *Parser) getGlobalFlags() []*Argument {
	var globalFlags []*Argument
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == "" && !f.Value.Argument.isPositional() {
			globalFlags = append(globalFlags, f.Value.Argument)
		}
	}
	return globalFlags
}

// detectSharedFlagGroups finds flags shared across multiple commands
func (p *Parser) detectSharedFlagGroups() map[string]*flagGroupInfo {
	groups := make(map[string]*flagGroupInfo)
	flagsByPrefix := make(map[string]map[string]bool) // Track unique flags per prefix

	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		flagName := *f.Key
		flagParts := splitPathFlag(flagName)
		baseName := flagParts[0] // Flag name is the first part

		// Extract prefix (e.g., "core.ldap" from "core.ldap.host")
		prefix := extractFlagPrefix(baseName)
		if prefix == "" {
			continue
		}

		if groups[prefix] == nil {
			groups[prefix] = &flagGroupInfo{
				flags:    []*Argument{},
				commands: []string{},
			}
			flagsByPrefix[prefix] = make(map[string]bool)
		}

		// Only add unique flags to the group
		if !flagsByPrefix[prefix][baseName] {
			groups[prefix].flags = append(groups[prefix].flags, f.Value.Argument)
			flagsByPrefix[prefix][baseName] = true
		}

		// Always track which commands use this flag
		if f.Value.CommandPath != "" {
			if !util.Contains(groups[prefix].commands, f.Value.CommandPath) {
				groups[prefix].commands = append(groups[prefix].commands, f.Value.CommandPath)
			}
		}
	}

	return groups
}

// flagGroupInfo holds information about a group of related flags
type flagGroupInfo struct {
	flags    []*Argument
	commands []string
}

// printCompactFlag prints a flag in compact format
func (p *Parser) printCompactFlag(writer io.Writer, arg *Argument) {
	name := p.renderer.FlagName(arg)

	// Build the flag representation
	var flagStr string
	if p.helpConfig.ShowShortFlags && arg.Short != "" {
		flagStr = fmt.Sprintf("--%s, -%s", name, arg.Short)
	} else {
		flagStr = fmt.Sprintf("--%s", name)
	}

	if p.helpConfig.ShowDescription {
		flagStr += fmt.Sprintf(" \"%s\"", p.renderer.FlagDescription(arg))
	}

	if p.helpConfig.ShowDefaults && arg.DefaultValue != "" {
		flagStr += fmt.Sprintf(" %s (%s)", p.layeredProvider.GetMessage(messages.MsgDefaultsToKey),
			arg.DefaultValue)
	}

	// Add required/optional indicator
	required := ""
	if p.helpConfig.ShowRequired {
		if arg.Required {
			required = fmt.Sprintf(" (%s)", p.layeredProvider.GetMessage(messages.MsgRequiredKey))
		} else if arg.RequiredIf != nil {
			required = fmt.Sprintf(" (%s)", p.layeredProvider.GetMessage(messages.MsgConditionalKey))
		}
	}

	fmt.Fprintf(writer, "  %s%s\n", flagStr, required)
}

// printCommandTree prints the command hierarchy as a tree
func (p *Parser) printCommandTree(writer io.Writer) {
	for cmd := p.registeredCommands.Front(); cmd != nil; cmd = cmd.Next() {
		ppConfig := p.DefaultPrettyPrintConfig()
		if cmd.Value.topLevel {
			desc := p.renderer.CommandDescription(cmd.Value)
			if desc != "" {
				fmt.Fprintf(writer, "\n%-20s %s\n", cmd.Value.Name, desc)
			} else {
				fmt.Fprintf(writer, "\n%s\n", cmd.Value.Name)
			}
			for i := range cmd.Value.Subcommands {
				prefix := ppConfig.DefaultPrefix
				if i == len(cmd.Value.Subcommands)-1 {
					prefix = ppConfig.TerminalPrefix
				}
				sub := &cmd.Value.Subcommands[i]
				// Look up the actual registered command to get the correct description
				subPath := cmd.Value.Name + " " + sub.Name
				if registeredSub, found := p.registeredCommands.Get(subPath); found {
					desc := util.Truncate(p.renderer.CommandDescription(registeredSub), 50)
					fmt.Fprintf(writer, "  %s %-20s %s\n", prefix, sub.Name, desc)
				} else {
					desc := util.Truncate(p.renderer.CommandDescription(sub), 50)
					fmt.Fprintf(writer, "  %s %-20s %s\n", prefix, sub.Name, desc)
				}
			}
		}
	}
}

// countCommandFlags counts flags specific to a command
func (p *Parser) countCommandFlags(cmdPath string) int {
	count := 0
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == cmdPath {
			count++
		}
	}
	return count
}

// extractFlagPrefix extracts the prefix from a flag name (e.g., "core.ldap" from "core.ldap.host")
func extractFlagPrefix(flagName string) string {
	parts := strings.Split(flagName, ".")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], ".")
	}
	return ""
}

// checkNamingConsistency checks if explicit flag and command names follow the naming convention
// defined by the converters. Returns warnings for any inconsistencies found.
func (p *Parser) checkNamingConsistency() []string {
	var warnings []string

	// Check flags
	for kv := p.acceptedFlags.Front(); kv != nil; kv = kv.Next() {
		flagName := *kv.Key
		flagInfo := kv.Value

		// Skip command-specific flags (handle them with their command context)
		if strings.Contains(flagName, "@") {
			parts := strings.Split(flagName, "@")
			if len(parts) == 2 {
				flagName = parts[0]
			}
		}

		// Check if the flag name matches what the converter would produce
		// This indicates the flag was explicitly named (not generated from struct field)
		converted := p.flagNameConverter(flagName)
		if flagName != converted {
			warnings = append(warnings,
				fmt.Sprintf("Flag '--%s' doesn't follow naming convention (converter would produce '--%s')",
					flagName, converted))
		}

		// Check flag translations
		if p.translationRegistry != nil && flagInfo.Argument.NameKey != "" {
			currentLang := p.GetLanguage()
			if currentLang != language.Und {
				translation, ok := p.translationRegistry.GetFlagTranslation(*kv.Key, currentLang)
				if ok && translation != "" && translation != flagName {
					convertedTranslation := p.flagNameConverter(translation)
					if translation != convertedTranslation {
						warnings = append(warnings,
							fmt.Sprintf("Translation '--%s' for flag '--%s' doesn't follow naming convention (converter would produce '--%s')",
								translation, flagName, convertedTranslation))
					}
				}
			}
		}
	}

	// Check commands
	for kv := p.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		cmdName := *kv.Key
		cmd := kv.Value

		// Check if the command name matches what the converter would produce
		converted := p.commandNameConverter(cmdName)
		if cmdName != converted {
			warnings = append(warnings,
				fmt.Sprintf("Command '%s' doesn't follow naming convention (converter would produce '%s')",
					cmdName, converted))
		}

		// Check command translations
		if p.translationRegistry != nil && cmd.NameKey != "" {
			currentLang := p.GetLanguage()
			if currentLang != language.Und {
				translation, ok := p.translationRegistry.GetCommandTranslation(cmdName, currentLang)
				if ok && translation != "" && translation != cmdName {
					convertedTranslation := p.commandNameConverter(translation)
					if translation != convertedTranslation {
						warnings = append(warnings,
							fmt.Sprintf("Translation '%s' for command '%s' doesn't follow naming convention (converter would produce '%s')",
								translation, cmdName, convertedTranslation))
					}
				}
			}
		}
	}

	return warnings
}

// wrapErrorIfTranslatable wraps a translatable error with the current language provider
// This is called lazily when errors are accessed, ensuring they use the current language
func (p *Parser) wrapErrorIfTranslatable(err error) error {
	if err == nil {
		return nil
	}

	// Check if already wrapped with provider
	if _, ok := err.(*errs.ErrWithProvider); ok {
		// Already wrapped, return as-is
		return err
	}

	// Check if it's a translatable error anywhere in the chain
	var te i18n.TranslatableError
	if errors.As(err, &te) {
		// Wrap with current language provider
		return errs.WithProvider(te, p.layeredProvider)
	}

	// Return non-translatable errors as-is
	return err
}
