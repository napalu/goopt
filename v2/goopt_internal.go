package goopt

import (
	"errors"
	"fmt"
	"github.com/napalu/goopt/v2/input"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/completion"
	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/internal/parse"
	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/types/orderedmap"
	"github.com/napalu/goopt/v2/types/queue"
)

func (p *Parser) parseFlag(state parse.State, currentCommandPath string) {
	stripped := strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc)
	flag := p.flagOrShortFlag(stripped, currentCommandPath)
	flagInfo, found := p.acceptedFlags.Get(flag)

	if !found {
		flagInfo, found = p.acceptedFlags.Get(stripped)
		if found {
			flag = stripped
		}
	}

	if found {
		p.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
	} else {
		p.addError(errs.ErrUnknownFlagInCommandPath.WithArgs(flag, currentCommandPath))
	}
}

func (p *Parser) parsePosixFlag(state parse.State, currentCommandPath string) {
	flag := p.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc))
	flagInfo, found := p.getFlagInCommandPath(flag, currentCommandPath)
	if !found {
		// two-pass process to account for flag values directly adjacent to a flag (e.g. `-f1` instead of `-f 1`)
		p.normalizePosixArgs(state, flag, currentCommandPath)
		flag = p.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc))
		flagInfo, found = p.getFlagInCommandPath(flag, currentCommandPath)
	}

	if found {
		p.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
	} else {
		p.addError(errs.ErrUnknownFlagInCommandPath.WithArgs(flag, currentCommandPath))
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
			p.addError(errs.ErrProcessingField.WithArgs(lookup).Wrap(err))
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
			p.registerFlagValue(lookup, boolVal, currentArg)
			p.options[lookup] = boolVal
			err := p.setBoundVariable(boolVal, lookup)
			if err != nil {
				p.addError(errs.ErrProcessingFlag.WithArgs(lookup).Wrap(err))
			}
		}
	case types.Single, types.Chained, types.File:
		p.processFlag(argument, state, lookup)
	}
}

func (p *Parser) registerCommandRecursive(cmd *Command) {
	// Add the current command to the map
	cmd.topLevel = strings.Count(cmd.path, " ") == 0
	p.registeredCommands.Set(cmd.path, cmd)

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
	if p.i18n == nil {
		p.i18n = i18n.Default()
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

		if _, exists := cache.flags[longName]; !exists {
			cache.flags[longName] = make(map[string]*FlagInfo)
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

		cache.flags[longName][cmdPath] = fv
		if shortName := fv.Argument.Short; shortName != "" {
			cache.flags[shortName][cmdPath] = fv
			cache.isStandalone[shortName] = fv.Argument.TypeOf == types.Standalone
			cache.needsValue[shortName] = fv.Argument.TypeOf != types.Standalone
		}

		cache.isStandalone[longName] = fv.Argument.TypeOf == types.Standalone
		cache.needsValue[longName] = fv.Argument.TypeOf != types.Standalone
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
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if p.isFlag(arg) {
			name := strings.TrimLeft(arg, "-")
			if len(currentCmdPath) > 0 {
				name = buildPathFlag(name, currentCmdPath...)
			}
			if cache.needsValue[name] {
				skipNext = true
			}
			continue
		}

		// Handle previous flag's value
		if i > 0 && p.isFlag(args[i-1]) {
			prevName := strings.TrimFunc(args[i-1], p.prefixFunc)
			if len(currentCmdPath) > 0 {
				prevName = buildPathFlag(prevName, currentCmdPath...)
			}
			if cache.needsValue[prevName] {
				skipNext = true
				continue
			}
			if cache.isStandalone[prevName] {
				if _, err := strconv.ParseBool(arg); err == nil {
					continue
				}
			}
		}

		// Check command path
		isCmd := false
		if len(currentCmdPath) == 0 {
			if p.isCommand(arg) {
				currentCmdPath = append(currentCmdPath, arg)
				isCmd = true
			}
		} else {
			if p.isCommand(strings.Join(append(currentCmdPath, arg), " ")) {
				currentCmdPath = append(currentCmdPath, arg)
				isCmd = true
			} else {
				currentCmdPath = currentCmdPath[:0]
			}
		}

		if isCmd {
			continue
		}

		positional = append(positional, PositionalArgument{
			Position: i,
			Value:    arg,
			Argument: nil,
			ArgPos:   argPos,
		})
		argPos++
	}

	maxDeclaredIdx := 0
	if len(declaredPos) > 0 {
		maxDeclaredIdx = declaredPos[len(declaredPos)-1].index
	}
	// Match and register positionals in one pass
	for _, decl := range declaredPos {
		if decl.index >= len(positional) {
			if decl.flag.Argument.DefaultValue != "" {
				// Only extend if within reasonable bounds
				if decl.index <= maxDeclaredIdx {
					// Extend result slice if needed
					if decl.index >= len(positional) {
						newResult := make([]PositionalArgument, decl.index+1)
						copy(newResult, positional)
						positional = newResult
					}
					positional[decl.index] = PositionalArgument{
						Position: decl.index,
						Value:    decl.flag.Argument.DefaultValue,
						Argument: decl.flag.Argument,
						ArgPos:   *decl.flag.Argument.Position,
					}

				} else {
					continue
				}
			} else {
				continue
			}
		}

		pos := &positional[decl.index]
		if pos.ArgPos != *decl.flag.Argument.Position {
			continue
		}

		flagFromArg := p.flagOrShortFlag(decl.key, decl.flag.CommandPath) // Use resolved name
		// Precedence check is done here to check whether the flag was provided on the command line.
		if p.HasRawFlag(flagFromArg) {
			// The flag was explicitly provided. Flag takes precedence.
			// Do NOT bind the value from 'pos' to this field ('decl').
			// Mark this positional argument slot as unbound in the final results.
			pos.Argument = nil
			continue
		}

		pos.Argument = decl.flag.Argument
		lookup := buildPathFlag(decl.key, decl.flag.CommandPath)
		p.registerFlagValue(lookup, pos.Value, pos.Value)
		p.options[lookup] = pos.Value
		if err := p.setBoundVariable(pos.Value, lookup); err != nil {
			p.addError(errs.ErrSettingBoundValue.WithArgs(lookup).Wrap(err))
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
			p.addError(errs.ErrProcessingCommand.Wrap(err).WithArgs(lastCommandPath))
		}
	} else if cmd, ok := p.getCommand(lastCommandPath); ok && cmd.Callback != nil && cmd.ExecOnParse {
		err := p.ExecuteCommand()
		if err != nil {
			p.addError(errs.ErrProcessingCommand.Wrap(err).WithArgs(lastCommandPath))
		}
	}

	return ""
}

func (p *Parser) evalFlagWithPath(state parse.State, currentCommandPath string) {
	if p.posixCompatible {
		p.parsePosixFlag(state, currentCommandPath)
	} else {
		p.parseFlag(state, currentCommandPath)
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
	return false
}

func (p *Parser) isGlobalFlag(arg string) bool {
	flag, ok := p.acceptedFlags.Get(p.flagOrShortFlag(strings.TrimLeftFunc(arg, p.prefixFunc)))
	if ok {
		return flag.CommandPath == ""
	}

	return false
}

func (p *Parser) addError(err error) {
	p.errors = append(p.errors, err)
}

func (p *Parser) getCommand(name string) (*Command, bool) {
	cmd, found := p.registeredCommands.Get(name)

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
		}
	}

	if cmd != nil {
		*commandPathSlice = append(*commandPathSlice, currentArg)
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

	}

	return terminating
}

func (p *Parser) queueCommandCallback(cmd *Command) {
	if cmd.Callback != nil {
		p.callbackQueue.Push(cmd)
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
					p.addError(errs.ErrProcessingFlag.WithArgs(flag).Wrap(err))
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

	if found {
		p.registerCommand(&sub, currentArg)
		cmdQueue.Push(&sub) // Keep subcommands in the queue
		return true, &sub
	} else if len(currentCmd.Subcommands) > 0 {
		p.addError(errs.ErrCommandExpectsSubcommand.WithArgs(currentCmd.Name, currentCmd.Subcommands))
	}

	return false, nil
}

func (p *Parser) processValueFlag(currentArg string, next string, argument *Argument) error {
	var processed string
	if len(argument.AcceptedValues) > 0 {
		processed = p.processSingleValue(next, currentArg, argument)
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

	return p.setBoundVariable(processed, currentArg)
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
		err = p.registerSecureValue(name, pass)
		if err != nil {
			p.addError(errs.ErrProcessingFlag.WithArgs(name).Wrap(err))
		}
	} else {
		p.addError(errs.ErrSecureFlagExpectsValue.WithArgs(name).Wrap(err))
	}
}

func (p *Parser) processSingleValue(next, key string, argument *Argument) string {
	switch argument.TypeOf {
	case types.Single:
		return p.checkSingle(next, key, argument)
	case types.Chained:
		return p.checkMultiple(next, key, argument)
	}

	return ""
}

func (p *Parser) checkSingle(next, flag string, argument *Argument) string {
	var errBuf = strings.Builder{}
	var valid = false
	var value string
	if argument.PreFilter != nil {
		value = argument.PreFilter(next)
	} else {
		value = next
	}

	lenValues := len(argument.AcceptedValues)
	for i, v := range argument.AcceptedValues {
		if v.Compiled.MatchString(value) {
			valid = true
		} else {
			errBuf.WriteString(v.Describe())
			if i+1 < lenValues {
				errBuf.WriteString(", ")
			}
		}
	}

	if argument.PostFilter != nil {
		value = argument.PostFilter(value)
	}
	if valid {
		p.registerFlagValue(flag, value, next)
	} else {
		p.addError(errs.ErrInvalidArgument.WithArgs(next, flag, errBuf.String()))
	}

	return value
}

func (p *Parser) checkMultiple(next, flag string, argument *Argument) string {
	valid := 0
	errBuf := strings.Builder{}
	listDelimFunc := p.getListDelimiterFunc()
	args := strings.FieldsFunc(next, listDelimFunc)

	for i := 0; i < len(args); i++ {
		if argument.PreFilter != nil {
			args[i] = argument.PreFilter(args[i])
		}

		for _, v := range argument.AcceptedValues {
			if v.Compiled.MatchString(args[i]) {
				valid++
			}
		}

		if argument.PostFilter != nil {
			args[i] = argument.PostFilter(args[i])
		}
	}

	value := strings.Join(args, "|")
	if valid == len(args) {
		p.registerFlagValue(flag, value, next)
	} else {
		lenValues := len(argument.AcceptedValues)
		for i := 0; i < lenValues; i++ {
			v := argument.AcceptedValues[i]
			errBuf.WriteString(v.Describe())
			if i+1 < lenValues {
				errBuf.WriteString(", ")
			}
		}
		p.addError(errs.ErrInvalidArgument.WithArgs(next, flag, errBuf.String()))
	}

	return value
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

	return util.ConvertString(value, data, currentArg, p.listFunc)
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
	for _, env := range os.Environ() {
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
		p.registeredCommands.Set(*it.Key, it.Value)
	}
	
	// Merge errors from nested parser
	for _, err := range nestedCmdLine.errors {
		p.addError(err)
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

		*arg = *toArgument(config)
		return config.Name, config.Path, nil
	}

	if isStructOrSliceType(field) {
		return "", "", nil // For nested structs, slices and arrays nil config is valid
	}

	return "", "", errs.ErrNoValidTags
}

func toArgument(c *types.TagConfig) *Argument {
	arg := NewArg(WithType(c.TypeOf),
		WithDescription(c.Description),
		WithDefaultValue(c.Default),
		WithDescriptionKey(c.DescriptionKey),
		WithAcceptedValues(c.AcceptedValues),
		WithDependencyMap(c.DependsOn),
		WithShortFlag(c.Short),
		WithRequired(c.Required),
	)
	if c.Secure.IsSecure {
		arg.Secure = c.Secure
	}

	if c.Position != nil {
		arg.Position = c.Position
	}

	return arg
}

func (p *Parser) buildCommand(commandPath, description, descriptionKey string, parent *Command) (*Command, error) {
	if commandPath == "" {
		return nil, errs.ErrEmptyCommandPath
	}

	commandNames := strings.Split(commandPath, " ")

	var topParent = parent
	var currentCommand *Command

	for _, cmdName := range commandNames {
		found := false

		// If we're at the top level (parent is nil)
		if parent == nil {
			// Look for the command at the top level
			if cmd, exists := p.registeredCommands.Get(cmdName); exists {
				currentCommand = cmd
				found = true
			} else {
				// Create a new top-level command
				newCommand := &Command{
					Name: cmdName,
				}

				p.resolveCommandDescription(description, newCommand, cmdName, descriptionKey)
				p.registeredCommands.Set(cmdName, newCommand)
				currentCommand = newCommand
			}
		} else {
			if cmdName == parent.Name {
				continue
			}
			for idx, subCmd := range parent.Subcommands {
				if subCmd.Name == cmdName {
					currentCommand = &parent.Subcommands[idx] // Use the existing subcommand
					found = true
					break
				}
			}

			if !found {
				newCommand := &Command{
					Name:        cmdName,
					Subcommands: []Command{},
					path:        commandPath,
				}
				p.resolveCommandDescription(description, newCommand, cmdName, descriptionKey)
				parent.Subcommands = append(parent.Subcommands, *newCommand)
				currentCommand = &parent.Subcommands[len(parent.Subcommands)-1] // Update currentCommand to point to the new subcommand
			}
		}

		// Set the top parent (the first command in the hierarchy)
		if topParent == nil {
			topParent = currentCommand
		}

		// Move to the next level in the hierarchy
		parent = currentCommand
	}

	// Add the top-level command if not already registered
	if topParent != nil && parent == nil {
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
		return nil, errs.ErrUnwrappingValue.Wrap(err)
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
		var bundle *i18n.Bundle
		if parser.userI18n != nil {
			bundle = parser.userI18n
		} else if parser.i18n != nil {
			bundle = parser.i18n
		}

		if bundle == nil {
			bundle = i18n.Default()
		}
		longName, pathTag, err = unmarshalTagsToArgument(bundle, field, arg)
		if err != nil {
			// ErrNoValidTags is not an error - it just means the field has no goopt tags
			if errors.Is(err, errs.ErrNoValidTags) {
				continue
			}
			
			// For other errors, decide based on field type
			if !isFunction(field) && !isStructOrSliceType(field) {
				// For simple fields with tag errors, we can skip them and continue
				parser.addError(errs.ErrProcessingField.WithArgs(field.Name).Wrap(err))
				continue
			}
			// For structural fields (functions, nested structs, slices), fail fast
			if !isFunction(field) {
				if flagPrefix != "" {
					return nil, errs.ErrProcessingFieldWithPrefix.WithArgs(flagPrefix, field.Name).Wrap(err)
				} else {
					return nil, errs.ErrProcessingField.WithArgs(field.Name).Wrap(err)
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
				return parser, errs.ErrProcessingFlag.WithArgs(fullFlagName).Wrap(err)
			}
		} else {
			// If no path specified, use current command path (if any)
			err = parser.bindArgument(commandPath, fieldValue, fullFlagName, arg)
			if err != nil {
				return parser, errs.ErrProcessingFlag.WithArgs(fullFlagName).Wrap(err)
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
				return errs.ErrProcessingCommand.WithArgs(parentCommand).Wrap(err)
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
				p.addError(errs.ErrSettingBoundValue.WithArgs(arg.DefaultValue).Wrap(err))
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
			p.addError(errs.ErrSettingBoundValue.WithArgs(arg.DefaultValue).Wrap(err))
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
		return errs.ErrUnwrappingValue.Wrap(err)
	}

	if unwrappedValue.Type() == reflect.TypeOf(Command{}) {
		cmd := unwrappedValue.Interface().(Command)
		_, err := p.buildCommand(cmd.path, cmd.Description, cmd.DescriptionKey, nil)
		if err != nil {
			return errs.ErrProcessingCommand.WithArgs(cmd.path).Wrap(err)
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

			buildCmd, err := p.buildCommand(cmdPath, cmd.Description, cmd.DescriptionKey, parent)
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
				return errs.ErrUnmarshallingTag.WithArgs(fieldType.Name).Wrap(err)
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
			buildCmd, err := p.buildCommand(cmdPath, config.Description, config.DescriptionKey, parent)
			if err != nil {
				return errs.ErrProcessingCommand.WithArgs(cmdPath).Wrap(err)
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
		return errs.ErrUnwrappingValue.WithArgs(flagPrefix).Wrap(err)
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
			return errs.ErrProcessingField.WithArgs(flagPrefix, idx).Wrap(err)
		}
		if err = c.mergeCmdLine(nestedCmdLine); err != nil {
			return errs.ErrProcessingField.WithArgs(flagPrefix, idx).Wrap(err)
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
		return errs.ErrUnwrappingValue.WithArgs(flagPrefix).Wrap(err)
	}

	var existingCmdDescription string
	if commandPath != "" {
		if existingCmd, found := c.registeredCommands.Get(commandPath); found && existingCmd.Description != "" {
			existingCmdDescription = existingCmd.Description
		}
	}

	nestedCmdLine, err := newParserFromReflectValue(unwrappedValue.Addr(), flagPrefix, commandPath, maxDepth, currentDepth+1, config...)
	if err != nil {
		return errs.ErrProcessingField.WithArgs(flagPrefix).Wrap(err)
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
	if unwrappedType.Kind() == reflect.Func {
		return true
	}

	return false
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
