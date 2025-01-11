package goopt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/napalu/goopt/completion"
	"github.com/napalu/goopt/parse"
	"github.com/napalu/goopt/types"
	"github.com/napalu/goopt/types/orderedmap"
	"github.com/napalu/goopt/types/queue"
	"github.com/napalu/goopt/util"
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
		p.addError(fmt.Errorf("unknown argument '%s' in command Path '%s'", flag, currentCommandPath))
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
		p.addError(fmt.Errorf("unknown argument '%s' in command Path '%s'", flag, currentCommandPath))
	}
}

func (p *Parser) normalizePosixArgs(state parse.State, currentArg string, commandPath string) {
	newArgs := make([]string, 0, state.Len())
	statePos := state.CurrentPos()
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
			p.addError(fmt.Errorf("invalid slice access for flag %s: %w", lookup, err))
			return
		}
	}

	switch argument.TypeOf {
	case types.Standalone:
		if argument.Secure.IsSecure {
			p.queueSecureArgument(lookup, argument)
		} else {
			boolVal := "true"
			if state.CurrentPos()+1 < state.Len() {
				nextArg := state.Peek()
				if !p.isCommand(nextArg) && !p.isFlag(nextArg) {
					if _, err := strconv.ParseBool(nextArg); err == nil {
						boolVal = nextArg
						state.SkipCurrent()
					}
				}
			}
			p.registerFlagValue(lookup, boolVal, currentArg)
			p.options[lookup] = boolVal
			err := p.setBoundVariable(boolVal, lookup)
			if err != nil {
				p.addError(fmt.Errorf(
					"could not process input argument '%s' - the following error occurred: %s", lookup, err))
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
		return false, fmt.Errorf("max command depth of %d exceeded", maxDepth)
	}

	var commandType string
	if level > 0 {
		commandType = "sub-command"
	} else {
		commandType = "command"
	}
	if cmdArg.Name == "" {
		return false, fmt.Errorf("the 'Name' property is missing from %s on Level %d: %+v", commandType, level, cmdArg)
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
		p.callbackQueue = queue.New[commandCallback]()
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

func (a *Argument) ensureInit() {
	if a.DependsOn == nil {
		a.DependsOn = []string{}
	}
	if a.OfValue == nil {
		a.OfValue = []string{}
	}
	if a.AcceptedValues == nil {
		a.AcceptedValues = []types.PatternValue{}
	}
	if a.DependencyMap == nil {
		a.DependencyMap = map[string][]string{}
	}
}

func (p *Parser) setPositionalArguments(state parse.State, commandPath ...string) {
	var positional []PositionalArgument
	args := state.Args()

	// First pass: collect positional arguments
	for i, seen := range args {
		seen = p.flagOrShortFlag(strings.TrimLeftFunc(seen, p.prefixFunc), commandPath...)
		if _, found := p.rawArgs[seen]; !found {
			positional = append(positional, PositionalArgument{
				Position: i,
				Value:    seen,
				Argument: nil,
			})
		}
	}

	// Second pass: match and validate positional arguments
	for i := range positional {
		if arg := p.findMatchingPositionalArg(state, positional[i].Position); arg != nil {
			positional[i].Argument = arg
		}
	}

	// Validate all positional requirements were met
	p.validatePositionalRequirements(state, positional)

	p.positionalArgs = positional
}

func (p *Parser) validatePositionalRequirements(state parse.State, found []PositionalArgument) {
	firstNonPos := p.findFirstNonPositionalArg(state)
	lastNonPos := p.findLastNonPositionalArg(state)

	for flag := p.acceptedFlags.Front(); flag != nil; flag = flag.Next() {
		arg := flag.Value.Argument
		if arg.Position == nil || arg.RelativeIndex == nil {
			continue
		}

		// Skip if this was provided as a flag
		if _, exists := p.rawArgs[*flag.Key]; exists {
			continue
		}

		// Check if we found this positional argument
		matched := false
		for _, pos := range found {
			if pos.Argument == arg {
				// Validate position requirements first
				switch *arg.Position {
				case types.AtStart:
					if pos.Position >= firstNonPos {
						p.addError(fmt.Errorf("argument '%s' must appear before position %d (before flags/commands)",
							*flag.Key, firstNonPos))
						continue
					}
				case types.AtEnd:
					if pos.Position <= lastNonPos {
						p.addError(fmt.Errorf("argument '%s' must appear after position %d (after flags/commands)",
							*flag.Key, lastNonPos))
						continue
					}
				}
				matched = true
				break
			}
		}

		if !matched {
			p.addError(fmt.Errorf("argument '%s' must appear at %s position",
				*flag.Key, positionTypeString(*arg.Position)))
		}
	}
}

func positionTypeString(pos types.PositionType) string {
	switch pos {
	case types.AtStart:
		return "start"
	case types.AtEnd:
		return "end"
	default:
		return "unknown"
	}
}

func (p *Parser) findMatchingPositionalArg(state parse.State, pos int) *Argument {
	// Find first non-positional argument position
	firstNonPos := p.findFirstNonPositionalArg(state)
	// Find last non-positional argument position
	lastNonPos := p.findLastNonPositionalArg(state)

	for flag := p.acceptedFlags.Front(); flag != nil; flag = flag.Next() {
		arg := flag.Value.Argument
		if arg.Position == nil || arg.RelativeIndex == nil {
			continue
		}

		switch *arg.Position {
		case types.AtStart:
			// Must appear before first non-positional arg
			if pos < firstNonPos && pos == *arg.RelativeIndex {
				return arg
			}
		case types.AtEnd:
			// Must appear after last non-positional arg
			if pos > lastNonPos && (pos-lastNonPos-1) == *arg.RelativeIndex {
				return arg
			}
		}
	}

	return nil
}

func (p *Parser) findFirstNonPositionalArg(state parse.State) int {
	for i := 0; i < state.Len(); i++ {
		arg := state.Args()[i]
		if p.isFlag(arg) || p.isCommand(arg) {
			return i
		}
	}
	return state.Len()
}

func (p *Parser) findLastNonPositionalArg(state parse.State) int {
	for i := state.Len() - 1; i >= 0; i-- {
		arg := state.Args()[i]
		if p.isFlag(arg) || p.isCommand(arg) {
			return i
		}
	}
	return -1
}

func (p *Parser) evalFlagWithPath(state parse.State, currentCommandPath string) {
	if p.posixCompatible {
		p.parsePosixFlag(state, currentCommandPath)
	} else {
		p.parseFlag(state, currentCommandPath)
	}
}

func (p *Parser) flagOrShortFlag(flag string, commandPath ...string) string {
	pathFlag := buildPathFlag(flag, commandPath...)
	_, pathFound := p.acceptedFlags.Get(pathFlag)
	if !pathFound {
		globalFlag := splitPathFlag(flag)[0]
		_, found := p.acceptedFlags.Get(globalFlag)
		if found {
			return globalFlag
		}
		item, found := p.lookup[flag]
		if found {
			pathFlag = buildPathFlag(item, commandPath...)
			if _, found := p.acceptedFlags.Get(pathFlag); found {
				return pathFlag
			}
			return item
		}
	}

	return pathFlag
}

func (p *Parser) isFlag(flag string) bool {
	if len(p.prefixes) == 0 {
		return strings.HasPrefix(flag, "-")
	}

	for _, prefix := range p.prefixes {
		if strings.HasPrefix(flag, string(prefix)) {
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

	p.rawArgs[name] = name
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

		// Queue the command callback (if any) after the command is fully recognized
		if cmd.Callback != nil {
			p.queueCommandCallback(cmd)
		}

	}

	return terminating
}

func (p *Parser) queueCommandCallback(cmd *Command) {
	if cmd.Callback != nil {
		p.callbackQueue.Push(commandCallback{
			callback:  cmd.Callback,
			arguments: []interface{}{p, cmd},
		})
	}
}

func (p *Parser) processFlag(argument *Argument, state parse.State, flag string) {
	var err error
	if argument.Secure.IsSecure {
		if state.CurrentPos() < state.Len()-1 {
			if !p.isFlag(state.Peek()) {
				state.SkipCurrent()
			}
		}
		p.queueSecureArgument(flag, argument)
	} else {
		var next string
		if state.CurrentPos() < state.Len()-1 {
			next = state.Peek()
		}
		if (len(next) == 0 || p.isFlag(next)) && len(argument.DefaultValue) > 0 {
			next = argument.DefaultValue
		} else {
			state.SkipCurrent()
		}
		if state.CurrentPos() >= state.Len()-1 && len(next) == 0 {
			p.addError(fmt.Errorf("flag '%s' expects a value", flag))
		} else {
			next, err = p.flagValue(argument, next, flag)
			if err != nil {
				p.addError(err)
			} else {
				if err = p.processValueFlag(flag, next, argument); err != nil {
					p.addError(fmt.Errorf("failed to process your input for Flag '%s': %s", flag, err))
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
			err = fmt.Errorf("flag '%s' should be a valid Path but could not find %s - error %s", flag, next, e.Error())
			return
		} else if st.IsDir() {
			err = fmt.Errorf("flag '%s' should be a file but is a directory", flag)
			return
		}
		next = filepath.Clean(next)
		if val, e := os.ReadFile(next); e != nil {
			err = fmt.Errorf("flag '%s' should be a valid file but reading from %s produces error %s ", flag, next, e.Error())
		} else {
			arg = string(val)
		}
		p.registerFlagValue(flag, arg, next)
	} else {
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
		p.addError(fmt.Errorf("command %s expects one of the following: %v",
			currentCmd.Name, currentCmd.Subcommands))
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
	if pass, err := util.GetSecureString(prompt, p.GetStderr(), p.GetTerminalReader()); err == nil {
		err = p.registerSecureValue(name, pass)
		if err != nil {
			p.addError(fmt.Errorf("failed to process flag '%s' secure value: %s", name, err))
		}
	} else {
		p.addError(fmt.Errorf("secure flag '%s' expects a value but we failed to obtain one: %s", name, err))
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
		p.addError(fmt.Errorf(
			"invalid argument '%s' for flag '%s'. Accepted values: %s", next, flag, errBuf.String()))
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
		p.addError(fmt.Errorf(
			"invalid argument '%s' for flag '%s'. Accepted values: %s", next, flag, errBuf.String()))
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
			p.queueSecureArgument(mainKey, flagInfo.Argument)
			continue
		}

		cmdArg := splitPathFlag(mainKey)

		if !p.shouldValidateDependencies(flagInfo) {
			if len(cmdArg) == 1 || (len(cmdArg) == 2 && p.HasCommand(cmdArg[1])) {
				p.addError(fmt.Errorf("flag '%s' is mandatory but missing from the command line", *f.Key))
			}
		} else {
			p.validateDependencies(flagInfo, mainKey, visited, 0)
		}
	}
}

func (p *Parser) shouldValidateDependencies(flagInfo *FlagInfo) bool {
	return len(flagInfo.Argument.DependsOn) > 0 || (flagInfo.Argument.DependencyMap != nil && len(flagInfo.Argument.DependsOn) > 0)
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
			for _, sub := range m.Subcommands {
				stack.Push(&sub)
			}
		}
	}
}

func (p *Parser) validateDependencies(flagInfo *FlagInfo, mainKey string, visited map[string]bool, depth int) {
	if depth > p.maxDependencyDepth {
		p.addError(fmt.Errorf("maximum dependency depth exceeded for flag '%s'", mainKey))
		return
	}

	if visited[mainKey] {
		p.addError(fmt.Errorf("circular dependency detected: flag '%s' is involved in a circular chain of dependencies", mainKey))
		return
	}

	visited[mainKey] = true

	for _, depends := range p.getDependentFlags(flagInfo.Argument) {
		dependentFlag, found := p.getFlagInCommandPath(depends, flagInfo.CommandPath)
		if !found {
			p.addError(fmt.Errorf("flag '%s' depends on '%s', but it is missing from command group '%s' or global flags",
				mainKey, depends, flagInfo.CommandPath))
			continue
		}

		dependKey := p.options[depends]
		matches, allowedValues := p.checkDependencyValue(flagInfo.Argument, depends, dependKey)
		if !matches {
			p.addError(fmt.Errorf("flag '%s' requires flag '%s' to have one of these values: %v (got '%s')",
				mainKey, depends, allowedValues, dependKey))
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
			return fmt.Errorf("conflict: flag '%s' is already bound in this CmdLineOption", k)
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

func unmarshalTagsToArgument(field reflect.StructField, arg *Argument) (name string, path string, err error) {
	// Try new format first
	if tag, ok := field.Tag.Lookup("goopt"); ok && strings.Contains(tag, ":") {
		config, err := parse.UnmarshalTagFormat(tag, field)
		if err != nil {
			return "", "", err
		}

		*arg = *toArgument(config)
		return config.Name, config.Path, nil
	}

	// Legacy format handling
	config, err := parse.LegacyUnmarshalTagFormat(field)
	if err != nil {
		return "", "", err
	}
	if config == nil {
		if isStructOrSliceType(field) {
			return "", "", nil // For nested structs, slices and arrays nil config is valid
		}
		return "", "", fmt.Errorf("no valid tags found for field %s", field.Name)
	}
	*arg = *toArgument(config)

	return config.Name, config.Path, nil
}

func toArgument(c *types.TagConfig) *Argument {
	return &Argument{
		Short:          c.Short,
		Description:    c.Description,
		TypeOf:         c.TypeOf,
		DefaultValue:   c.Default,
		Required:       c.Required,
		Secure:         c.Secure,
		AcceptedValues: c.AcceptedValues,
		DependencyMap:  c.DependsOn,
	}
}

func (p *Parser) buildCommand(commandPath, description string, parent *Command) (*Command, error) {
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
					Name:        cmdName,
					Subcommands: []Command{},
				}
				if description != "" {
					newCommand.Description = description
				} else {
					newCommand.Description = fmt.Sprintf("Auto-generated command for %s", cmdName)
				}
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
				newCommand := Command{
					Name:        cmdName,
					Subcommands: []Command{},
					path:        commandPath,
				}
				if description != "" {
					newCommand.Description = description
				} else {
					newCommand.Description = fmt.Sprintf("Auto-generated command for %s", cmdName)
				}
				parent.Subcommands = append(parent.Subcommands, newCommand)
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

func newParserFromReflectValue(structValue reflect.Value, flagPrefix, commandPath string, maxDepth, currentDepth int, config ...ConfigureCmdLineFunc) (*Parser, error) {
	if currentDepth > maxDepth {
		return nil, fmt.Errorf("recursion depth exceeded: max depth is %d", maxDepth)
	}

	var err error
	parser := NewParser()
	for _, cfg := range config {
		cfg(parser, &err)
		if err != nil {
			return nil, fmt.Errorf("error configuring parser: %w", err)
		}
	}

	// Unwrap the value and type
	unwrappedValue, err := util.UnwrapValue(structValue)
	if err != nil {
		return nil, fmt.Errorf("error unwrapping value: %w", err)
	}

	st := util.UnwrapType(structValue.Type())
	if st.Kind() != reflect.Struct {
		return nil, fmt.Errorf("only structs can be tagged")
	}

	err = parser.processStructCommands(unwrappedValue, commandPath, currentDepth, maxDepth)
	if err != nil {
		parser.addError(err)
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
		longName, pathTag, err = unmarshalTagsToArgument(field, arg)
		if err != nil {
			if flagPrefix != "" {
				parser.addError(fmt.Errorf("error processing field %s.%s: %w", flagPrefix, field.Name, err))
			} else {
				parser.addError(fmt.Errorf("error processing field %s: %w", field.Name, err))
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
				parser.addError(fmt.Errorf("error processing slice field %s: %w", fieldFlagPath, err))
			}
			continue
		}

		if isStructType(field) {
			newCommandPath := commandPath
			if isCommand {
				if newCommandPath == "" {
					newCommandPath = longName
				} else {
					newCommandPath = fmt.Sprintf("%s %s", newCommandPath, longName)
				}
			}

			if err = processNestedStruct(fieldFlagPath, newCommandPath, fieldValue, maxDepth, currentDepth, parser, config...); err != nil {
				parser.addError(fmt.Errorf("error processing nested struct %s: %w", fieldFlagPath, err))
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
				return parser, fmt.Errorf("error processing flag %s: %w", fullFlagName, err)
			}
		} else {
			// If no path specified, use current command path (if any)
			err = parser.bindArgument(commandPath, fieldValue, fullFlagName, arg)
			if err != nil {
				return parser, fmt.Errorf("error processing flag %s: %w", fullFlagName, err)
			}
		}
	}

	return parser, nil
}

func (p *Parser) bindArgument(commandPath string, fieldValue reflect.Value, fullFlagName string, arg *Argument) (err error) {
	if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
		p.addError(fmt.Errorf("unexpected - field pointer %s is nilskipping", fullFlagName))
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
			p.addError(fmt.Errorf("error processing default value %s: %w", arg.DefaultValue, err))
		}
	}

	return nil
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

			if cmd, err = p.buildCommand(parentCommand, "", pCmd); err != nil {
				p.addError(fmt.Errorf("error processing command %s: %w", parentCommand, err))
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
				p.addError(fmt.Errorf("error processing default value %s: %w", arg.DefaultValue, err))
			}
		}
	}

	return nil
}

func (p *Parser) processStructCommands(val reflect.Value, currentPath string, currentDepth, maxDepth int) error {
	// Handle case where the entire value is a Command type (not a struct containing commands)
	unwrappedValue, err := util.UnwrapValue(val)
	if err != nil {
		return fmt.Errorf("error unwrapping value: %w", err)
	}

	if unwrappedValue.Type() == reflect.TypeOf(Command{}) {
		cmd := unwrappedValue.Interface().(Command)
		_, err := p.buildCommand(cmd.path, cmd.Description, nil)
		if err != nil {
			return fmt.Errorf("error ensuring command hierarchy for path %s: %w", cmd.path, err)
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
		return fmt.Errorf("max nesting depth exceeded: %d", maxDepth)
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

			buildCmd, err := p.buildCommand(cmdPath, cmd.Description, parent)
			if err != nil {
				return fmt.Errorf("error ensuring command hierarchy for path %s: %w", cmdPath, err)
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
				return err
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
			buildCmd, err := p.buildCommand(cmdPath, config.Description, parent)
			if err != nil {
				return fmt.Errorf("error processing command %s: %w", cmdPath, err)
			}

			err = p.AddCommand(buildCmd)
			if err != nil {
				return err
			}

			// Process nested structure with updated path
			if err := p.processStructCommands(fieldValue, cmdPath, currentDepth+1, maxDepth); err != nil {
				return err
			}
		} else if fieldValue.Kind() == reflect.Struct {
			// Process non-command struct fields for nested commands
			if err := p.processStructCommands(fieldValue, currentPath, currentDepth+1, maxDepth); err != nil {
				return err
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
					return fmt.Errorf("internal error: missing argument info for %s", parentPath)
				}

				if flagInfo.Argument.Capacity <= 0 {
					return fmt.Errorf("slice at '%s' has no capacity set", parentPath)
				}

				if idx < 0 || idx >= flagInfo.Argument.Capacity {
					return fmt.Errorf("index %d out of bounds at '%s': valid range is 0-%d",
						idx, currentPath, flagInfo.Argument.Capacity-1)
				}
			}
		}
	}

	// Final check - does this path exist in our accepted flags?
	if _, exists := p.acceptedFlags.Get(path); !exists {
		return fmt.Errorf("unknown flag: %s", path)
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
		return fmt.Errorf("error unwrapping slice field %s: %w", flagPrefix, err)
	}

	// Initialize or resize slice if needed
	if (unwrappedValue.Kind() == reflect.Slice && unwrappedValue.IsNil()) ||
		(capacity > 0 && unwrappedValue.Cap() != capacity) {
		// TODO add a check to ensure capacity is not too large?

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
			return fmt.Errorf("error processing slice element %s[%d]: %w", flagPrefix, idx, err)
		}
		if err = c.mergeCmdLine(nestedCmdLine); err != nil {
			return fmt.Errorf("error merging slice element %s[%d]: %w", flagPrefix, idx, err)
		}
	}

	return nil
}

func processNestedStruct(flagPrefix, commandPath string, fieldValue reflect.Value, maxDepth, currentDepth int, c *Parser, config ...ConfigureCmdLineFunc) error {
	unwrappedValue, err := util.UnwrapValue(fieldValue)
	if err != nil {
		return fmt.Errorf("error unwrapping nested struct: %w", err)
	}

	nestedCmdLine, err := newParserFromReflectValue(unwrappedValue.Addr(), flagPrefix, commandPath, maxDepth, currentDepth+1, config...)
	if err != nil {
		return fmt.Errorf("error processing nested struct %s: %w", flagPrefix, err)
	}

	err = c.mergeCmdLine(nestedCmdLine)
	if err != nil {
		return err
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

func describeRequired(argument *Argument) string {
	requiredOrOptional := "optional"
	if argument.Required {
		requiredOrOptional = "required"
	} else if argument.RequiredIf != nil {
		requiredOrOptional = "conditional"
	}

	return requiredOrOptional
}

func formatFlagDescription(arg *Argument) string {
	status := ""
	if arg.Required {
		status = "(required) "
	} else if len(arg.DependsOn) > 0 {
		status = "(conditional) "
	}
	return status + arg.Description
}

func addFlagToCompletionData(data *completion.CompletionData, cmd, flagName string, flagInfo *FlagInfo) {
	if flagInfo == nil || flagInfo.Argument == nil {
		return
	}

	// Create flag pair with type conversion
	pair := completion.FlagPair{
		Long:        flagName,
		Short:       flagInfo.Argument.Short,
		Description: formatFlagDescription(flagInfo.Argument),
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
