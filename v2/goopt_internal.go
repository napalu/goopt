package goopt

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/input"
	"github.com/napalu/goopt/v2/internal/messages"

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

// splitFlagValue splits a flag argument on the first '=' to support --flag=value syntax.
// Returns the flag name (without prefix) and the embedded value (if any).
// If no '=' is present, returns the flag name and empty string.
func splitFlagValue(flagArg string) (flagName string, embeddedValue string, hasEmbeddedValue bool) {
	if idx := strings.Index(flagArg, "="); idx > 0 {
		return flagArg[:idx], flagArg[idx+1:], true
	}
	return flagArg, "", false
}

func (p *Parser) parseFlag(state parse.State, currentCommandPath string) bool {
	stripped := strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc)

	// Check for --flag=value syntax
	flagName, embeddedValue, hasEmbeddedValue := splitFlagValue(stripped)

	flag := p.flagOrShortFlag(flagName, currentCommandPath)
	flagInfo, found := p.acceptedFlags.Get(flag)

	if !found {
		flagInfo, found = p.acceptedFlags.Get(flagName)
		if found {
			flag = flagName
		}
	}

	// If not found, try translation lookup. Resolve the canonical name through the
	// same parent-walking resolver used for the canonical name above, so a
	// translated name inherits across command scope exactly as its canonical does
	// (e.g. --<translated> on a parent-command flag works under a subcommand).
	if !found {
		if canonical, ok := p.translationRegistry.GetCanonicalFlagName(flagName, p.GetLanguage()); ok {
			flag = p.flagOrShortFlag(canonical, currentCommandPath)
			flagInfo, found = p.acceptedFlags.Get(flag)
		}
	}

	if found {
		if hasEmbeddedValue {
			p.processFlagArgWithValue(state, flagInfo.Argument, flag, embeddedValue, currentCommandPath)
		} else {
			p.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
		}
		return true
	} else {
		// Don't add error here - return false to indicate not processed
		return false
	}
}

func (p *Parser) parsePosixFlag(state parse.State, currentCommandPath string) bool {
	stripped := strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc)

	// Check for -flag=value syntax
	flagName, embeddedValue, hasEmbeddedValue := splitFlagValue(stripped)

	flag := p.flagOrShortFlag(flagName)
	flagInfo, found := p.getFlagInCommandPath(flag, currentCommandPath)
	if !found {
		// two-pass process to account for flag values directly adjacent to a flag (e.g. `-f1` instead of `-f 1`)
		// Note: Don't normalize if we have an embedded value with =
		if !hasEmbeddedValue {
			p.normalizePosixArgs(state, flag, currentCommandPath)
			flag = p.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), p.prefixFunc))
			flagInfo, found = p.getFlagInCommandPath(flag, currentCommandPath)
		}
	}

	// If not found, try translation lookup
	if !found {
		if canonical, ok := p.translationRegistry.GetCanonicalFlagName(flagName, p.GetLanguage()); ok {
			flagInfo, found = p.getFlagInCommandPath(canonical, currentCommandPath)
		}
	}

	if found {
		if hasEmbeddedValue {
			p.processFlagArgWithValue(state, flagInfo.Argument, flag, embeddedValue, currentCommandPath)
		} else {
			p.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
		}
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
	for i := range len(currentArg) {
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
					if err := validator.Validate(boolVal); err != nil {
						p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(lookup)))
						return
					}
				}
			}

			p.registerFlagValue(lookup, boolVal, currentArg)
			p.options[lookup] = boolVal
			err := p.setBoundVariable(boolVal, lookup)
			if err != nil {
				p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(lookup)))
			}
		}
	case types.Single, types.Chained, types.File:
		p.processFlag(argument, state, lookup)
	}
}

// processFlagArgWithValue handles flag processing when an embedded value is provided via --flag=value syntax
func (p *Parser) processFlagArgWithValue(state parse.State, argument *Argument, currentArg string, embeddedValue string, currentCommandPath ...string) {
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
			boolVal := cmp.Or(embeddedValue, "true")

			// Run validators on standalone flag value
			if len(argument.Validators) > 0 {
				for _, validator := range argument.Validators {
					if err := validator.Validate(boolVal); err != nil {
						p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(lookup)))
						return
					}
				}
			}

			p.registerFlagValue(lookup, boolVal, currentArg)
			p.options[lookup] = boolVal
			err := p.setBoundVariable(boolVal, lookup)
			if err != nil {
				p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(lookup)))
			}
		}
	case types.Single, types.Chained, types.File:
		p.processFlagWithValue(argument, embeddedValue, lookup)
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

	for i := range len(cmdArg.Subcommands) {
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

func (p *Parser) evalExecOnParse(lastCommandPath string) string {
	if p.completionMode {
		return "" // completion must never execute command callbacks
	}
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

// chainedInternalSep is the internal separator for stored Chained (list) values: the
// ASCII Unit Separator. It is deliberately a control byte that cannot be typed on a
// command line (or appear in config/env data), so it never collides with a value and
// is never a user-facing list separator. The user's ListDelimiterFunc remains the
// sole authority on ELEMENT separation; this marker only delimits the dimensions the
// user never specifies — repeated-occurrence boundaries and the validated-element
// rejoin. Stored values are read back by splitting on (user delimiter ∪ this marker),
// see chainedSplitFunc.
const chainedInternalSep = "\x1f"
const chainedInternalSepRune = '\x1f'

// chainedSplitFunc returns the predicate for splitting a STORED chained value back
// into elements: the configured input delimiter UNION the internal marker. Input
// parsing uses the user delimiter alone; reading a stored value must additionally
// break on the marker that joins occurrences and validated elements.
func (p *Parser) chainedSplitFunc() types.ListDelimiterFunc {
	input := p.getListDelimiterFunc()
	return func(r rune) bool { return r == chainedInternalSepRune || input(r) }
}

// chainedRegisteredDownstream reports whether a Chained flag's options value is
// written by a step after flagValue: checkMultiple (when it has validators or
// accepted-values) or the filter block in processValueFlag (when it has pre/post
// filters). Such flags must NOT also be registered in flagValue, to keep exactly one
// options write per occurrence.
func (p *Parser) chainedRegisteredDownstream(argument *Argument) bool {
	return argument.TypeOf == types.Chained &&
		(len(argument.Validators) > 0 || len(argument.AcceptedValues) > 0 ||
			argument.PreFilter != nil || argument.PostFilter != nil)
}

func (p *Parser) registerFlagValue(flag, value, rawValue string) {
	parts := splitPathFlag(flag)
	p.rawArgs[parts[0]] = rawValue
	p.rawArgs[rawValue] = rawValue

	// For Chained (list) flags, accumulate repeated occurrences. We store each
	// occurrence's token verbatim (preserving the user's declared list separator) and
	// join occurrences with the internal marker — NOT a list separator of our own.
	// repeatedFlags is now marked per-occurrence for EVERY chained flag (in
	// processValueFlag), so accumulation no longer depends on the flag being bound to
	// a variable, and registerFlagValue is called exactly once per occurrence (see the
	// deferral in flagValue) so a repeat never double-appends.
	if flagInfo, found := p.acceptedFlags.Get(flag); found && flagInfo.Argument.TypeOf == types.Chained {
		if existingValue, exists := p.options[flag]; exists && p.repeatedFlags[flag] {
			p.options[flag] = existingValue + chainedInternalSep + value
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

func (p *Parser) parseCommand(state parse.State, cmdQueue *queue.Q[*Command], commandPathSlice *[]string, lastCommandPath string) (bool, *Command) {
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
			return false, nil
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
		if len(cmd.Subcommands) == 0 || cmd.Greedy {
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
		// Command not found - if positionals are declared for this context,
		// let the arg fall through to setPositionalArguments() instead of suggesting.
		// Check both the current path and the last terminating command's path,
		// since commandPathSlice is cleared after a terminating command.
		currentPath := strings.Join(*commandPathSlice, " ")
		hasPositionals := len(p.getPositionalsForCommand(currentPath)) > 0
		if !hasPositionals && lastCommandPath != "" {
			hasPositionals = len(p.getPositionalsForCommand(lastCommandPath)) > 0
		}

		// Only generate suggestions if no positionals could claim this arg
		if !hasPositionals && p.registeredCommands.Len() > 0 {
			suggestions, _ := p.findSimilarRootCommandsWithContext(currentArg)
			if len(suggestions) > 0 {
				// Check if any suggestion is very close (likely a typo)
				for _, suggestion := range suggestions {
					distance := util.DamerauLevenshteinDistance(currentArg, suggestion)
					if distance <= 2 {
						// Very likely a typo - generate error with suggestions
						p.addError(errs.ErrCommandNotFound.WithArgs(currentArg))

						// Render each suggestion in the form closest to user input.
						displaySuggestions := p.localizeSuggestions(currentArg, suggestions, func(key string) (string, bool) {
							if p.translationRegistry == nil {
								return "", false
							}
							if cmd, found := p.registeredCommands.Get(key); found && cmd.NameKey != "" {
								return p.translationRegistry.GetCommandTranslation(key, p.GetLanguage())
							}
							return "", false
						})

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
						return false, nil
					}
				}
			}
		}
	}

	return terminating, cmd
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
			p.addError(errs.ErrFlagExpectsValue.WithArgs(p.formatFlagForError(flag)))
		} else {
			next, err = p.flagValue(argument, next, flag)
			if err != nil {
				p.addError(err)
			} else {
				if err = p.processValueFlag(flag, next, argument); err != nil {
					p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(flag)))
				}
			}
		}
	}
}

// processFlagWithValue processes a flag with an embedded value from --flag=value syntax
func (p *Parser) processFlagWithValue(argument *Argument, embeddedValue string, flag string) {
	var err error
	if argument.Secure.IsSecure {
		// For secure arguments with embedded values, we just queue them
		p.queueSecureArgument(flag, argument)
	} else {
		// Use the embedded value directly
		value := embeddedValue
		if len(value) == 0 && len(argument.DefaultValue) > 0 {
			value = argument.DefaultValue
		}
		if len(value) == 0 {
			p.addError(errs.ErrFlagExpectsValue.WithArgs(p.formatFlagForError(flag)))
		} else {
			value, err = p.flagValue(argument, value, flag)
			if err != nil {
				p.addError(err)
			} else {
				if err = p.processValueFlag(flag, value, argument); err != nil {
					p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(flag)))
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
			err = errs.ErrNotFoundPathForFlag.WithArgs(p.formatFlagForError(flag), next).Wrap(e)
			return
		} else if st.IsDir() {
			err = errs.ErrNotFilePathForFlag.WithArgs(p.formatFlagForError(flag))
			return
		}
		next = filepath.Clean(next)
		if val, e := os.ReadFile(next); e != nil {
			err = errs.ErrFlagFileOperation.WithArgs(p.formatFlagForError(flag), next).Wrap(e)
		} else {
			arg = string(val)
		}
		p.registerFlagValue(flag, arg, next)
	} else {
		if p.isFlag(next) && argument.TypeOf == types.Single {
			stripped := strings.TrimLeftFunc(next, p.prefixFunc)
			if _, ok := p.acceptedFlags.Get(stripped); ok {
				p.addError(errs.ErrFlagExpectsValue.WithArgs(p.formatFlagForError(flag)))
				return
			}
		}
		arg = next
		// A Chained flag that has validators/accepted-values (split+validated in
		// checkMultiple) or filters (applied in processValueFlag) is registered by
		// that downstream step instead. Writing the raw token here too would store it
		// twice — and, now that repeated accumulation is bind-independent, append it
		// twice on a repeat. Plain chained flags have no downstream write, so they are
		// stored here.
		if !p.chainedRegisteredDownstream(argument) {
			p.registerFlagValue(flag, next, next)
		}
	}

	return arg, err
}

// findSimilarSubcommandsWithContext finds subcommands similar to the input and detects if input is likely translated
// suggestionItem is one candidate for fuzzy "did you mean" matching: a key (the
// value returned and deduped on — a command name, a full command path, or a flag
// name) plus the texts to score the user's input against. names are canonical
// score targets (the name itself, plus aliases such as a flag's short form); i18n
// are translated score targets. Empty targets are ignored. This is the seam that
// keeps each entity's gathering distinct while the ranking below stays shared.
type suggestionItem struct {
	key   string
	names []string
	i18n  []string
}

// rankSuggestions is the single ranking core shared by every "did you mean"
// matcher — root commands, subcommands, the help command tree, and flags. For
// each item it keeps the closest in-threshold distance, preferring a canonical
// match over a translated one on a tie, then applies the shared filter: when any
// distance-1 match exists show only those, otherwise widen to the threshold; dedup
// by key, sort by ascending distance, cap at topN. threshold<=0 disables
// suggestions. Returns the display keys and whether any surfaced match came from a
// translated name (so callers can decide whether to render localized forms).
//
// The legitimate differences between flags and commands (separate thresholds,
// short forms, translation API, display prefixes) all live in the per-entity
// gatherers and callers — NOT here. Keep it that way: this is the one place the
// scoring/filtering must not drift between subsystems.
func (p *Parser) rankSuggestions(input string, items []suggestionItem, threshold, topN int) ([]string, bool) {
	if threshold <= 0 {
		return nil, false
	}

	type scored struct {
		key          string
		distance     int
		isTranslated bool
	}
	var all []scored
	for _, it := range items {
		best := -1
		bestTranslated := false
		consider := func(text string, translated bool) {
			if text == "" {
				return
			}
			d := util.DamerauLevenshteinDistance(input, text)
			if d <= 0 || d > threshold {
				return
			}
			// Strictly-closer wins; canonical wins ties (it is considered first).
			if best == -1 || d < best {
				best = d
				bestTranslated = translated
			}
		}
		for _, n := range it.names {
			consider(n, false)
		}
		for _, t := range it.i18n {
			consider(t, true)
		}
		if best != -1 {
			all = append(all, scored{key: it.key, distance: best, isTranslated: bestTranslated})
		}
	}
	if len(all) == 0 {
		return nil, false
	}

	// If any distance-1 match exists, show only those; otherwise widen to the
	// configured threshold.
	minDistance := 3
	for _, s := range all {
		if s.distance < minDistance {
			minDistance = s.distance
		}
	}
	finalThreshold := minDistance
	if minDistance > 1 && threshold > 1 {
		finalThreshold = threshold
	}

	seen := make(map[string]bool)
	var result []string
	hasTranslated := false
	for _, s := range all {
		if s.distance <= finalThreshold && !seen[s.key] {
			seen[s.key] = true
			result = append(result, s.key)
			if s.isTranslated {
				hasTranslated = true
			}
		}
	}

	slices.SortFunc(result, func(a, b string) int {
		da, db := 3, 3
		for _, s := range all {
			if s.key == a {
				da = s.distance
			}
			if s.key == b {
				db = s.distance
			}
		}
		return cmp.Compare(da, db)
	})
	if len(result) > topN {
		result = result[:topN]
	}
	return result, hasTranslated
}

func (p *Parser) findSimilarSubcommandsWithContext(subcommands []Command, input string, parentPath string) ([]string, bool) {
	currentLang := p.GetLanguage()
	items := make([]suggestionItem, 0, len(subcommands))
	for _, cmd := range subcommands {
		it := suggestionItem{key: cmd.Name, names: []string{cmd.Name}}
		if p.translationRegistry != nil && cmd.NameKey != "" {
			fullPath := cmd.Name
			if parentPath != "" {
				fullPath = parentPath + " " + cmd.Name
			}
			if t, found := p.translationRegistry.GetCommandTranslation(fullPath, currentLang); found {
				it.i18n = append(it.i18n, t)
			}
		}
		items = append(items, it)
	}
	return p.rankSuggestions(input, items, p.cmdSuggestionThreshold, 3)
}

// localizeSuggestions renders ranked suggestion keys in the form closest to what the
// user typed: the translated name when it is a nearer match, "canonical / translated"
// on a tie, otherwise the canonical key. translate resolves a key's translated form
// (and whether one exists) — the only per-entity difference (command name vs full
// path vs flag API) lives in that closure. This is the display counterpart to
// rankSuggestions: the single place the canonical/translated/both choice lives, so it
// cannot drift between the flag, command and subcommand "did you mean" paths.
func (p *Parser) localizeSuggestions(input string, keys []string, translate func(key string) (string, bool)) []string {
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = key
		translated, ok := translate(key)
		if !ok {
			continue
		}
		canonicalDist := util.DamerauLevenshteinDistance(input, key)
		translatedDist := util.DamerauLevenshteinDistance(input, translated)
		if translatedDist < canonicalDist {
			out[i] = translated
		} else if translatedDist == canonicalDist && translated != key {
			out[i] = key + " / " + translated
		}
	}
	return out
}

// suggestSubcommands returns display-ready "did you mean" suggestions for an unknown
// subcommand `input` under `parentPath`. It is the single source of truth shared by the
// parse path and the help system: matching is i18n-aware (findSimilarSubcommandsWithContext)
// and display is localized (localizeSuggestions). Returns an empty slice when nothing is
// similar enough. Keeping both call sites on this helper is what prevents the help system
// from drifting back to canonical-only suggestions.
func (p *Parser) suggestSubcommands(subcommands []Command, input, parentPath string) []string {
	suggestions, _ := p.findSimilarSubcommandsWithContext(subcommands, input, parentPath)
	if len(suggestions) == 0 {
		return nil
	}
	return p.localizeSuggestions(input, suggestions, func(key string) (string, bool) {
		if p.translationRegistry == nil {
			return "", false
		}
		for _, sub := range subcommands {
			if sub.Name == key && sub.NameKey != "" {
				fullPath := key
				if parentPath != "" {
					fullPath = parentPath + " " + key
				}
				return p.translationRegistry.GetCommandTranslation(fullPath, p.GetLanguage())
			}
		}
		return "", false
	})
}

// findSimilarFlagsWithContext finds flags similar to the input and detects if input is likely translated
func (p *Parser) findSimilarFlagsWithContext(input string, commandPath string) ([]string, bool) {
	cleanInput := strings.TrimLeftFunc(input, p.prefixFunc)
	currentLang := p.GetLanguage()

	// Gather one item per flag name; the short form and translated name become
	// additional score targets. Flags sharing a name across command paths merge.
	var items []suggestionItem
	index := map[string]int{}
	for flagKey, flagInfo := range p.acceptedFlags.All() {
		// Skip flags not in the current command context
		if commandPath != "" && flagInfo.CommandPath != commandPath && flagInfo.CommandPath != "" {
			continue
		}
		flagName := splitPathFlag(flagKey)[0]
		i, ok := index[flagName]
		if !ok {
			i = len(items)
			index[flagName] = i
			items = append(items, suggestionItem{key: flagName, names: []string{flagName}})
		}
		if flagInfo.Argument.Short != "" {
			items[i].names = append(items[i].names, flagInfo.Argument.Short)
		}
		if p.translationRegistry != nil && flagInfo.Argument.NameKey != "" {
			if t, found := p.translationRegistry.GetFlagTranslation(flagName, currentLang); found {
				items[i].i18n = append(items[i].i18n, t)
			}
		}
	}
	return p.rankSuggestions(cleanInput, items, p.flagSuggestionThreshold, 3)
}

// findSimilarRootCommandsWithContext finds root commands similar to the input
func (p *Parser) findSimilarRootCommandsWithContext(input string) ([]string, bool) {
	currentLang := p.GetLanguage()
	var items []suggestionItem
	for cmdName, cmd := range p.registeredCommands.All() {
		it := suggestionItem{key: cmdName, names: []string{cmdName}}
		if p.translationRegistry != nil && cmd.NameKey != "" {
			if t, found := p.translationRegistry.GetCommandTranslation(cmdName, currentLang); found {
				it.i18n = append(it.i18n, t)
			}
		}
		items = append(items, it)
	}
	return p.rankSuggestions(input, items, p.cmdSuggestionThreshold, 3)
}

// generateFlagError generates an error for an unknown flag with suggestions
func (p *Parser) generateFlagError(flagName string, commandPath string) {
	suggestions, _ := p.findSimilarFlagsWithContext(flagName, commandPath)
	if len(suggestions) > 0 {
		// Format suggestions with proper prefixes
		formattedSuggestions := make([]string, len(suggestions))

		// Decide each display form (translated/canonical/both) closest to user input,
		// then apply flag prefixes below.
		cleanInput := strings.TrimLeftFunc(flagName, p.prefixFunc)
		displayNames := p.localizeSuggestions(cleanInput, suggestions, func(key string) (string, bool) {
			if p.translationRegistry == nil {
				return "", false
			}
			return p.translationRegistry.GetFlagTranslation(key, p.GetLanguage())
		})

		for i, displayName := range displayNames {
			s := suggestions[i]
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
		// If the parent command has positionals declared, the unknown arg
		// might be a positional value — let it fall through instead of suggesting
		hasPositionals := len(p.getPositionalsForCommand(currentCmd.path)) > 0
		if !hasPositionals && !p.isFlag(currentArg) {
			// Find similar subcommands - pass the parent command's path
			displaySuggestions := p.suggestSubcommands(currentCmd.Subcommands, currentArg, currentCmd.path)
			if len(displaySuggestions) > 0 {
				// Create a more helpful error message
				p.addError(errs.ErrCommandNotFound.WithArgs(currentCmd.path + " " + currentArg))

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
		processed = next
		if argument.PreFilter != nil {
			processed = argument.PreFilter(next)
		}
		if argument.PostFilter != nil {
			if processed != "" {
				processed = argument.PostFilter(processed)
			} else {
				processed = argument.PostFilter(next)
			}
		}
		if haveFilters {
			// Register once, after both filters — registering after each would write
			// (and, for a repeated chained flag, accumulate) twice per occurrence.
			p.registerFlagValue(currentArg, processed, next)
		}
	}

	// Run validators on the final processed value
	// Skip for chained types as they are validated individually in checkMultiple
	// Also skip if validation already failed in processSingleValue
	if len(argument.Validators) > 0 && argument.TypeOf != types.Chained && validationPassed {
		for _, validator := range argument.Validators {
			if err := validator.Validate(processed); err != nil {
				return errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(currentArg))
			}
		}
	}

	// Only set bound variable if validation passed
	if validationPassed {
		err := p.setBoundVariable(processed, currentArg)
		// Mark the occurrence complete for chained flags so a SUBSEQUENT occurrence
		// accumulates (in both options and any bound slice) rather than replacing —
		// independent of whether the flag is bound. This is the single per-occurrence
		// boundary that both registerFlagValue and appendOrSetBoundVariable read.
		if argument.TypeOf == types.Chained {
			p.repeatedFlags[currentArg] = true
		}
		return err
	}
	return nil
}

func (p *Parser) processSecureFlag(name string, config *types.Secure) {
	if !p.HasFlag(name) {
		return
	}

	if !config.IsSecure {
		return
	}

	var pass string
	var err error

	// Try environment variable first (opt-in: only when envNameConverter is set)
	if p.envNameConverter != nil {
		if envValue := p.resolveSecureEnvVar(name); envValue != "" {
			pass = envValue
		}
	}

	// No env value — prompt interactively
	if pass == "" {
		prompt := "password: "
		if config.Prompt != "" {
			prompt = config.Prompt
		}
		pass, err = input.GetSecureString(prompt, p.GetStderr(), p.GetTerminalReader())
		if err != nil {
			p.addError(errs.WrapOnce(err, errs.ErrSecureFlagExpectsValue, p.formatFlagForError(name)))
			return
		}
	}

	// Run validators on the value (regardless of source)
	if flagInfo, found := p.acceptedFlags.Get(name); found && flagInfo.Argument != nil {
		if len(flagInfo.Argument.Validators) > 0 {
			for _, validator := range flagInfo.Argument.Validators {
				if err = validator.Validate(pass); err != nil {
					p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(name)))
					return
				}
			}
		}
	}

	// Register value
	if err = p.registerSecureValue(name, pass); err != nil {
		p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(name)))
	}
}

// resolveSecureEnvVar checks if an environment variable matches the secure flag name.
// Uses the same prefix/converter pattern as groupEnvVarsByCommand().
func (p *Parser) resolveSecureEnvVar(name string) string {
	baseName := splitPathFlag(name)[0]
	for _, envEntry := range p.envResolver.Environ() {
		kv := strings.SplitN(envEntry, "=", 2)
		if len(kv) != 2 {
			continue
		}
		varName := kv[0]
		if p.envVarPrefix != "" && !strings.HasPrefix(varName, p.envVarPrefix) {
			continue
		}
		stripped := strings.Replace(varName, p.envVarPrefix, "", 1)
		if stripped == "" {
			continue
		}
		converted := p.envNameConverter(stripped)
		if converted == p.envNameConverter(baseName) {
			return kv[1]
		}
	}
	return ""
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
			p.addError(errs.ErrInvalidArgument.WithArgs(next, p.formatFlagForError(flag), errBuf.String()))
			return "", false
		}
	}

	// Run validators (if any) - this includes converted AcceptedValues
	if len(argument.Validators) > 0 {
		for _, validator := range argument.Validators {
			if err := validator.Validate(value); err != nil {
				p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(flag)))
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
	for i := range len(args) {
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
				p.addError(errs.ErrInvalidArgument.WithArgs(args[i], p.formatFlagForError(flag), errBuf.String()))
				return "", false
			}
		}

		if len(argument.Validators) > 0 {
			for _, validator := range argument.Validators {
				if err := validator.Validate(args[i]); err != nil {
					p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, p.formatFlagForError(flag)))
					return "", false
				}
			}
		}

		// Step 3: Apply post-filter
		if argument.PostFilter != nil {
			args[i] = argument.PostFilter(args[i])
		}
	}

	// All validations passed for all values. Join the validated elements with the
	// internal marker (not a user-facing separator); GetList / ConvertString recover
	// them by splitting on (user delimiter ∪ marker).
	value := strings.Join(args, chainedInternalSep)
	p.registerFlagValue(flag, value, next)
	return value, true
}

func (p *Parser) validateProcessedOptions() {
	p.walkCommands()
	p.walkFlags()
	p.validateContracts()
}

func (p *Parser) walkFlags() {
	visited := orderedmap.NewOrderedMap[string, bool]()
	for flagKey, flagInfo := range p.acceptedFlags.All() {
		if flagInfo.Argument.isPositional() {
			continue
		}
		if flagInfo.Argument.RequiredIf != nil {
			if required, msg := flagInfo.Argument.RequiredIf(p, flagKey); required {
				p.addError(errors.New(msg))
			}
			continue
		}

		if !flagInfo.Argument.Required {
			if p.HasFlag(flagKey) && flagInfo.Argument.TypeOf == types.Standalone {
				p.validateStandaloneFlag(flagKey)
			}
			continue
		}

		mainKey := p.flagOrShortFlag(flagKey)
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
					p.addError(errs.ErrRequiredPositionalFlag.WithArgs(p.formatFlagForError(flagKey), *flagInfo.Argument.Position))
				} else {
					p.addError(errs.ErrRequiredFlag.WithArgs(p.formatFlagForError(flagKey)))
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
	for _, cmd := range p.registeredCommands.All() {
		stack.Push(cmd)
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

func (p *Parser) validateDependencies(flagInfo *FlagInfo, mainKey string, visited *orderedmap.OrderedMap[string, bool], depth int) {
	if depth > p.maxDependencyDepth {
		p.addError(errs.ErrRecursionDepthExceeded.WithArgs(p.maxDependencyDepth))
		return
	}

	if _, onPath := visited.Get(mainKey); onPath {
		// mainKey is already on the active DFS path — a genuine cycle. Because we
		// Set on enter and Delete on backtrack, the ordered keys are exactly that
		// path, so we can render the chain that closes back on mainKey.
		var chain []string
		started := false
		for k := range visited.Keys() {
			if k == mainKey {
				started = true
			}
			if started {
				chain = append(chain, splitPathFlag(k)[0])
			}
		}
		chain = append(chain, splitPathFlag(mainKey)[0]) // close the loop
		p.addError(errs.ErrCircularDependency.WithArgs(
			p.formatFlagForError(mainKey),
			strings.Join(chain, " → "),
		))
		return
	}

	visited.Set(mainKey, true)

	for _, depends := range p.getDependentFlags(flagInfo.Argument) {
		dependentFlag, found := p.getFlagInCommandPath(depends, flagInfo.CommandPath)
		if !found {
			p.addError(errs.ErrDependencyNotFound.WithArgs(p.formatFlagForError(mainKey), p.formatFlagForError(depends), flagInfo.CommandPath))
			continue
		}

		dependKey := p.options[depends]
		matches, allowedValues := p.checkDependencyValue(flagInfo.Argument, depends, dependKey)
		if !matches {
			p.addError(errs.ErrDependencyValueNotSpecified.WithArgs(p.formatFlagForError(mainKey), p.formatFlagForError(depends), allowedValues, dependKey))
		}

		p.validateDependencies(dependentFlag, depends, visited, depth+1)
	}

	visited.Delete(mainKey)
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
	if p.completionMode {
		return nil // completion must never write to the user's bound variables
	}
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

	// For Chained type with repeated flag support, check if we need to append. The
	// value may carry the internal marker (from checkMultiple's validated rejoin), so
	// split on (user delimiter ∪ marker) — the same recovery GetList uses — to keep
	// the bound slice and GetList in lockstep.
	if flagInfo.Argument.TypeOf == types.Chained {
		return p.appendOrSetBoundVariable(value, data, currentArg, p.chainedSplitFunc())
	}

	return util.ConvertString(value, data, currentArg, p.listFunc)
}

// appendOrSetBoundVariable handles repeated flags by appending to slice types
// or replacing the value for non-slice types. This enables the pattern:
// -o option1 -o option2 instead of -o "option1,option2"
func (p *Parser) appendOrSetBoundVariable(value string, data any, currentArg string, delimiterFunc types.ListDelimiterFunc) error {
	// Append when this flag was already completed in a PRIOR occurrence; otherwise
	// set. The per-occurrence mark is owned by processValueFlag (bind-independent), so
	// the first occurrence reads false (set) and later ones read true (append).
	doAppend := p.repeatedFlags[currentArg]

	return util.ConvertString(value, data, currentArg, delimiterFunc, doAppend)
}

func (p *Parser) prefixFunc(r rune) bool {
	for i := range len(p.prefixes) {
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
		var v string
		kv := strings.SplitN(env, "=", 2)
		if p.envVarPrefix != "" && !strings.HasPrefix(kv[0], p.envVarPrefix) {
			continue
		}
		v = strings.Replace(kv[0], p.envVarPrefix, "", 1)
		if v == "" {
			continue
		}
		v = p.envNameConverter(v)
		for flagKey := range p.acceptedFlags.Keys() {
			paths := splitPathFlag(flagKey)
			length := len(paths)
			// Global flag (no command path)
			if length == 1 && p.envNameConverter(paths[0]) == v {
				commandEnvVars["global"] = append(commandEnvVars["global"], fmt.Sprintf("--%s", flagKey), kv[1])
			}
			// Command-specific flag
			if length > 1 && p.envNameConverter(paths[0]) == v {
				commandEnvVars[paths[1]] = append(commandEnvVars[paths[1]], fmt.Sprintf("--%s", flagKey), kv[1])
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
	for k, v := range nestedCmdLine.acceptedFlags.All() {
		p.acceptedFlags.Set(k, v)
	}
	for k, v := range nestedCmdLine.lookup {
		p.lookup[k] = v
	}
	for cmdKey, cmdVal := range nestedCmdLine.registeredCommands.All() {
		// Check if command already exists and preserve its properties
		if existing, found := p.registeredCommands.Get(cmdKey); found {
			newCmd := cmdVal
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
			p.registeredCommands.Set(cmdKey, newCmd)
		} else {
			p.registeredCommands.Set(cmdKey, cmdVal)
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
	var acceptedValueValidators []validation.Validator
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
	var allValidators []validation.Validator

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

		// If no accepted tag was provided, derive AcceptedValues from isoneof
		// so that completion data can see the allowed values.
		if len(c.AcceptedValues) == 0 {
			if values := validation.ExtractIsOneOfValues(c.Validators); len(values) > 0 {
				pvs := make([]types.PatternValue, len(values))
				for i, v := range values {
					escaped := regexp.QuoteMeta(v)
					pvs[i] = types.PatternValue{
						Pattern:     "^" + escaped + "$",
						Description: v,
						Compiled:    regexp.MustCompile("^" + escaped + "$"),
					}
				}
				configs = append(configs, WithAcceptedValues(pvs))
			}
		}
	}

	// Add all validators to the argument
	if len(allValidators) > 0 {
		configs = append(configs, WithValidators(allValidators...))
	}

	// Parse and add cross-flag contracts
	if len(c.Contracts) > 0 {
		contracts, err := parseContracts(c.Contracts)
		if err != nil {
			return nil, err
		}
		if len(contracts) > 0 {
			configs = append(configs, WithContracts(contracts...))
		}
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
	Greedy         bool
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
					Name:   cmdName,
					Greedy: config.Greedy,
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
					Greedy:      config.Greedy,
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
	for i := range st.NumField() {
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
			Greedy:         cmd.Greedy,
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
	for i := range unwrappedValue.NumField() {
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
				Greedy:         cmd.Greedy,
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
				Greedy:         config.Greedy,
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
		return errs.ErrUnknownFlag.WithArgs(p.formatFlagForError(path))
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
			copyLen := min(unwrappedValue.Len(), capacity)
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

// formatFlagForError formats a flag name for user-facing error messages.
// Converts "flag@command" to "'flag' (in command 'command')" for clarity.
func (p *Parser) formatFlagForError(flag string) string {
	parts := splitPathFlag(flag)
	if len(parts) == 2 && parts[1] != "" {
		// Omit the "(in command 'x')" qualifier when that command was actually
		// invoked: the user already knows which command they ran, so repeating it
		// (e.g. once per flag in a contract error) is noise. The qualifier is kept
		// only for a flag of a command that was not invoked, where it genuinely
		// disambiguates.
		for _, cmd := range p.GetCommands() {
			if cmd == parts[1] || strings.HasPrefix(cmd, parts[1]+" ") {
				return p.quoteForError(parts[0])
			}
		}
		// Format as: 'flag' (in command 'command')
		inCommandMsg := p.layeredProvider.GetMessage(messages.MsgInCommandKey)
		return p.quoteForError(parts[0]) + " (" + inCommandMsg + " " + p.quoteForError(parts[1]) + ")"
	}
	// No command context, just quote the flag name (use first part to strip any trailing @)
	return p.quoteForError(parts[0])
}

// quoteForError wraps s in the locale's quotation glyphs. The glyphs are sourced
// from the message bundle (defaulting to ASCII ') so each language can use its own
// quotation marks without code changes.
func (p *Parser) quoteForError(s string) string {
	open := p.layeredProvider.GetMessage(messages.MsgQuoteOpenKey)
	closing := p.layeredProvider.GetMessage(messages.MsgQuoteCloseKey)
	return open + s + closing
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

	// CLI with any commands uses grouped style for clarity
	if cmdCount > 0 {
		return HelpStyleGrouped
	}

	// Simple CLI with no commands - flat style
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

// printGroupedCleanHelp prints grouped help with clean, compact formatting (no ** markers, tighter spacing)
func (p *Parser) printGroupedCleanHelp(writer io.Writer) {
	cleanConfig := &PrettyPrintConfig{
		NewCommandPrefix:     " +  ",
		DefaultPrefix:        " ├─ ",
		TerminalPrefix:       " └─ ",
		InnerLevelBindPrefix: "  ", // 2 spaces - clean and compact
		OuterLevelBindPrefix: " │  ",
	}
	p.PrintUsageWithGroups(writer, cleanConfig)
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
		// Pre-pass: compute max command name width for alignment
		maxCmdWidth := 0
		for _, cmd := range p.registeredCommands.All() {
			if cmd.topLevel {
				cmdName := p.buildCommandNameWithPositionals(cmd)
				if len(cmdName) > maxCmdWidth {
					maxCmdWidth = len(cmdName)
				}
			}
		}
		maxCmdWidth += 2 // add padding after longest name

		fmt.Fprintf(writer, "\n%s:\n", p.layeredProvider.GetMessage(messages.MsgCommandsKey))
		for _, cmd := range p.registeredCommands.All() {
			if cmd.topLevel {
				flagCount := p.countCommandFlags(cmd.Name)
				cmdName := p.buildCommandNameWithPositionals(cmd)
				desc := p.renderer.CommandDescription(cmd)

				if desc != "" {
					quotedDesc := fmt.Sprintf("\"%s\"", util.Truncate(desc, 40))
					fmt.Fprintf(writer, "  %-*s %-42s",
						maxCmdWidth, cmdName,
						quotedDesc)
				} else {
					fmt.Fprintf(writer, "  %-*s %-42s",
						maxCmdWidth, cmdName, "")
				}
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
		slices.SortFunc(groups, func(a, b groupInfo) int {
			return cmp.Compare(b.cmdCount, a.cmdCount)
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
	for _, flagInfo := range p.acceptedFlags.All() {
		if flagInfo.CommandPath == "" && !flagInfo.Argument.isPositional() {
			globalFlags = append(globalFlags, flagInfo.Argument)
		}
	}
	return globalFlags
}

// detectSharedFlagGroups finds flags shared across multiple commands
func (p *Parser) detectSharedFlagGroups() map[string]*flagGroupInfo {
	groups := make(map[string]*flagGroupInfo)
	flagsByPrefix := make(map[string]map[string]bool) // Track unique flags per prefix

	for flagName, flagInfo := range p.acceptedFlags.All() {
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
			groups[prefix].flags = append(groups[prefix].flags, flagInfo.Argument)
			flagsByPrefix[prefix][baseName] = true
		}

		// Always track which commands use this flag
		if flagInfo.CommandPath != "" {
			if !slices.Contains(groups[prefix].commands, flagInfo.CommandPath) {
				groups[prefix].commands = append(groups[prefix].commands, flagInfo.CommandPath)
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
		flagStr += fmt.Sprintf(" (%s: %s)", p.layeredProvider.GetMessage(messages.MsgDefaultsToKey),
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

// printCommandTree prints the registered commands as a proper indented tree: every
// node gets a connector, children nest under their parent with guide rails, and
// descriptions are column-aligned. Recurses to arbitrary depth (the old version only
// drew two levels and left top-level commands flush-left — never actually a tree).
// RTL names/descriptions are bidi-isolated so they can't scramble the tree drawing.
func (p *Parser) printCommandTree(writer io.Writer) {
	pp := p.DefaultPrettyPrintConfig()
	isRTL := i18n.IsRTL(p.GetLanguage())
	emptyGuide := strings.Repeat(" ", len([]rune(pp.OuterLevelBindPrefix)))

	type node struct {
		prefix string // accumulated guides + this node's connector
		label  string // translated short name + positionals
		desc   string
	}
	var nodes []node
	var walk func(cmds []*Command, parentPath, guides string)
	walk = func(cmds []*Command, parentPath, guides string) {
		for i := range cmds {
			c := cmds[i]
			last := i == len(cmds)-1
			conn := pp.DefaultPrefix
			childGuides := guides + pp.OuterLevelBindPrefix
			if last {
				conn = pp.TerminalPrefix
				childGuides = guides + emptyGuide
			}
			path := c.Name
			if parentPath != "" {
				path = parentPath + " " + c.Name
			}
			label := p.renderer.CommandName(c)
			for _, pos := range p.getPositionalsForCommand(path) {
				fn := pos.Value
				if idx := strings.LastIndex(fn, "@"); idx >= 0 {
					fn = fn[:idx]
				}
				if pos.Argument.Required {
					label += " <" + fn + ">"
				} else {
					label += " [" + fn + "]"
				}
			}
			nodes = append(nodes, node{guides + conn, label, p.renderer.CommandDescription(c)})

			kids := make([]*Command, 0, len(c.Subcommands))
			for j := range c.Subcommands {
				kids = append(kids, &c.Subcommands[j])
			}
			walk(kids, path, childGuides)
		}
	}
	var roots []*Command
	for _, c := range p.registeredCommands.All() {
		if c.topLevel {
			roots = append(roots, c)
		}
	}
	walk(roots, "", "")

	// Align descriptions to a common column (width measured pre-isolation).
	maxW := 0
	for _, n := range nodes {
		if w := len([]rune(n.prefix + n.label)); w > maxW {
			maxW = w
		}
	}
	// Overview descriptions are truncated to fit HelpConfig.MaxWidth (full text shows
	// in `--help <command>`). The budget adapts to the alignment column, so a deeper
	// tree leaves less room; MaxWidth <= 0 means unlimited. Applied to every node,
	// where the old code truncated subcommands only.
	descCol := maxW + 2
	maxWidth := p.GetHelpConfig().MaxWidth
	for _, n := range nodes {
		label, desc := n.label, n.desc
		if maxWidth > 0 && desc != "" {
			budget := maxWidth - descCol - 2 // 2 for the surrounding quotes
			if budget < 10 {
				budget = 10 // floor so something always shows
			}
			desc = util.Truncate(desc, budget)
		}
		if isRTL {
			label = i18n.Isolate(label)
			if desc != "" {
				desc = i18n.Isolate(desc)
			}
		}
		if n.desc == "" {
			fmt.Fprintf(writer, "%s%s\n", n.prefix, label)
			continue
		}
		pad := maxW - len([]rune(n.prefix+n.label)) + 2
		fmt.Fprintf(writer, "%s%s%s\"%s\"\n", n.prefix, label, strings.Repeat(" ", pad), desc)
	}
}

// countCommandFlags counts flags specific to a command
func (p *Parser) countCommandFlags(cmdPath string) int {
	count := 0
	for _, flagInfo := range p.acceptedFlags.All() {
		if flagInfo.CommandPath == cmdPath {
			count++
		}
	}
	return count
}

// buildCommandNameWithPositionals builds a command name string including its positional args
// (e.g., "cp <source> <dest>" or "ls [subfolder]")
func (p *Parser) buildCommandNameWithPositionals(cmd *Command) string {
	cmdName := cmd.Name
	positionals := p.getPositionalsForCommand(cmd.path)
	for _, pos := range positionals {
		flagName := pos.Value
		if idx := strings.LastIndex(flagName, "@"); idx >= 0 {
			flagName = flagName[:idx]
		}
		if pos.Argument.Required {
			cmdName += " <" + flagName + ">"
		} else {
			cmdName += " [" + flagName + "]"
		}
	}
	return cmdName
}

// buildSubcommandNameWithPositionals builds a subcommand name string including its positional args
func (p *Parser) buildSubcommandNameWithPositionals(sub *Command, subPath string) string {
	subName := sub.Name
	subPositionals := p.getPositionalsForCommand(subPath)
	for _, pos := range subPositionals {
		flagName := pos.Value
		if idx := strings.LastIndex(flagName, "@"); idx >= 0 {
			flagName = flagName[:idx]
		}
		if pos.Argument.Required {
			subName += " <" + flagName + ">"
		} else {
			subName += " [" + flagName + "]"
		}
	}
	return subName
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
	for flagKey, flagInfo := range p.acceptedFlags.All() {
		flagName := flagKey

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
				translation, ok := p.translationRegistry.GetFlagTranslation(flagKey, currentLang)
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
	for cmdName, cmd := range p.registeredCommands.All() {

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
