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
	"time"

	"github.com/araddon/dateparse"
	"github.com/napalu/goopt/completion"
	"github.com/napalu/goopt/parse"
	"github.com/napalu/goopt/types/orderedmap"
	"github.com/napalu/goopt/types/queue"
	"github.com/napalu/goopt/util"
)

func (s *Parser) parseFlag(state parse.State, currentCommandPath string) {
	stripped := strings.TrimLeftFunc(state.CurrentArg(), s.prefixFunc)
	flag := s.flagOrShortFlag(stripped, currentCommandPath)
	flagInfo, found := s.acceptedFlags.Get(flag)

	if !found {
		flagInfo, found = s.acceptedFlags.Get(stripped)
		if found {
			flag = stripped
		}
	}

	if found {
		s.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
	} else {
		s.addError(fmt.Errorf("unknown argument '%s' in command Path '%s'", flag, currentCommandPath))
	}
}

func (s *Parser) parsePosixFlag(state parse.State, currentCommandPath string) {
	flag := s.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), s.prefixFunc))
	flagInfo, found := s.getFlagInCommandPath(flag, currentCommandPath)
	if !found {
		// two-pass process to account for flag values directly adjacent to a flag (e.g. `-f1` instead of `-f 1`)
		s.normalizePosixArgs(state, flag, currentCommandPath)
		flag = s.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), s.prefixFunc))
		flagInfo, found = s.getFlagInCommandPath(flag, currentCommandPath)
	}

	if found {
		s.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
	} else {
		s.addError(fmt.Errorf("unknown argument '%s' in command Path '%s'", flag, currentCommandPath))
	}
}

func (s *Parser) normalizePosixArgs(state parse.State, currentArg string, commandPath string) {
	newArgs := make([]string, 0, state.Len())
	statePos := state.CurrentPos()
	if statePos > 0 {
		newArgs = append(newArgs, state.Args()[:statePos]...)
	}

	value := ""
	for i := 0; i < len(currentArg); i++ {
		cf := s.flagOrShortFlag(currentArg[i:i+1], commandPath)
		if _, found := s.acceptedFlags.Get(cf); found {
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

func (s *Parser) processFlagArg(state parse.State, argument *Argument, currentArg string, currentCommandPath ...string) {
	lookup := buildPathFlag(currentArg, currentCommandPath...)
	switch argument.TypeOf {
	case Standalone:
		if argument.Secure.IsSecure {
			s.queueSecureArgument(lookup, argument)
		} else {
			boolVal := "true"
			if state.CurrentPos()+1 < state.Len() {
				nextArg := state.Peek()
				_, found := s.registeredCommands.Get(nextArg)
				if !found && !s.isFlag(state.Peek()) {
					boolVal = nextArg
					state.SkipCurrent()
				}
			}
			s.options[lookup] = boolVal
			err := s.setBoundVariable(boolVal, lookup)
			if err != nil {
				s.addError(fmt.Errorf(
					"could not process input argument '%s' - the following error occurred: %s", lookup, err))
			}
		}
	case Single, Chained, File:
		s.processFlag(argument, state, lookup)
	}
}

func (s *Parser) registerCommandRecursive(cmd *Command) {
	// Add the current command to the map
	cmd.topLevel = strings.Count(cmd.path, " ") == 0
	s.registeredCommands.Set(cmd.path, cmd)

	// Recursively register all subcommands
	for i := range cmd.Subcommands {
		subCmd := &cmd.Subcommands[i]
		s.registerCommandRecursive(subCmd)
	}

}

func (s *Parser) validateCommand(cmdArg *Command, level, maxDepth int) (bool, error) {
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
		if ok, err := s.validateCommand(&cmdArg.Subcommands[i], level+1, maxDepth); err != nil {
			return ok, err
		}
	}

	return true, nil
}

func (s *Parser) ensureInit() {
	if s.options == nil {
		s.options = map[string]string{}
	}
	if s.acceptedFlags == nil {
		s.acceptedFlags = orderedmap.NewOrderedMap[string, *FlagInfo]()
	}
	if s.lookup == nil {
		s.lookup = map[string]string{}
	}
	if s.errors == nil {
		s.errors = []error{}
	}
	if s.bind == nil {
		s.bind = make(map[string]interface{}, 1)
	}
	if s.customBind == nil {
		s.customBind = map[string]ValueSetFunc{}
	}
	if s.registeredCommands == nil {
		s.registeredCommands = orderedmap.NewOrderedMap[string, *Command]()
	}
	if s.commandOptions == nil {
		s.commandOptions = orderedmap.NewOrderedMap[string, bool]()
	}
	if s.positionalArgs == nil {
		s.positionalArgs = []PositionalArgument{}
	}
	if s.rawArgs == nil {
		s.rawArgs = map[string]string{}
	}
	if s.callbackQueue == nil {
		s.callbackQueue = queue.New[commandCallback]()
	}
	if s.callbackResults == nil {
		s.callbackResults = map[string]error{}
	}
	if s.secureArguments == nil {
		s.secureArguments = orderedmap.NewOrderedMap[string, *Secure]()
	}
	if s.stderr == nil {
		s.stderr = os.Stderr
	}
	if s.stdout == nil {
		s.stdout = os.Stdout
	}
	if s.flagNameConverter == nil {
		s.flagNameConverter = DefaultFlagNameConverter
	}
	if s.commandNameConverter == nil {
		s.commandNameConverter = DefaultCommandNameConverter
	}
	if s.maxDependencyDepth <= 0 {
		s.maxDependencyDepth = DefaultMaxDependencyDepth
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
		a.AcceptedValues = []PatternValue{}
	}
	if a.DependencyMap == nil {
		a.DependencyMap = map[string][]string{}
	}
}

func (s *Parser) setPositionalArguments(args []string, commandPath ...string) {
	var positional []PositionalArgument
	for i, seen := range args {
		seen = s.flagOrShortFlag(strings.TrimLeftFunc(seen, s.prefixFunc), commandPath...)
		if _, found := s.rawArgs[seen]; !found {
			positional = append(positional, PositionalArgument{i, seen})
		}
	}

	s.positionalArgs = positional
}

func (s *Parser) evalFlagWithPath(state parse.State, currentCommandPath string) {
	if s.posixCompatible {
		s.parsePosixFlag(state, currentCommandPath)
	} else {
		s.parseFlag(state, currentCommandPath)
	}
}

func (s *Parser) flagOrShortFlag(flag string, commandPath ...string) string {
	pathFlag := buildPathFlag(flag, commandPath...)
	_, pathFound := s.acceptedFlags.Get(pathFlag)
	if !pathFound {
		globalFlag := splitPathFlag(flag)[0]
		_, found := s.acceptedFlags.Get(globalFlag)
		if found {
			return globalFlag
		}
		item, found := s.lookup[flag]
		if found {
			pathFlag = buildPathFlag(item, commandPath...)
			if _, found := s.acceptedFlags.Get(pathFlag); found {
				return pathFlag
			}
			return item
		}
	}

	return pathFlag
}

func (s *Parser) isFlag(flag string) bool {
	return strings.HasPrefix(flag, "-")
}

func (s *Parser) isGlobalFlag(arg string) bool {
	flag, ok := s.acceptedFlags.Get(s.flagOrShortFlag(strings.TrimLeftFunc(arg, s.prefixFunc)))
	if ok {
		return flag.CommandPath == ""
	}

	return false
}

func (s *Parser) addError(err error) {
	s.errors = append(s.errors, err)
}

func (s *Parser) getCommand(name string) (*Command, bool) {
	cmd, found := s.registeredCommands.Get(name)

	return cmd, found
}

func (s *Parser) registerSecureValue(flag, value string) error {
	var err error
	s.rawArgs[flag] = value
	if value != "" {
		s.options[flag] = value
		err = s.setBoundVariable(value, flag)
	}

	return err
}

func (s *Parser) registerFlagValue(flag, value, rawValue string) {
	parts := splitPathFlag(flag)
	s.rawArgs[parts[0]] = rawValue

	s.options[flag] = value
}

func (s *Parser) registerCommand(cmd *Command, name string) {
	if cmd.path == "" {
		return
	}

	s.rawArgs[name] = name

	s.commandOptions.Set(cmd.path, len(cmd.Subcommands) == 0)
}

func (s *Parser) queueSecureArgument(name string, argument *Argument) {
	if s.secureArguments == nil {
		s.secureArguments = orderedmap.NewOrderedMap[string, *Secure]()
	}

	s.rawArgs[name] = name
	s.secureArguments.Set(name, &argument.Secure)
}

func (s *Parser) parseCommand(state parse.State, cmdQueue *queue.Q[*Command], commandPathSlice *[]string) bool {
	terminating := false
	currentArg := state.CurrentArg()

	// Check if we're dealing with a subcommand
	var (
		curSub *Command
		ok     bool
	)
	if cmdQueue.Len() > 0 {
		ok, curSub = s.checkSubCommands(cmdQueue, currentArg)
		if !ok {
			return false
		}
	}

	var cmd *Command
	if curSub != nil {
		cmd = curSub
	} else {
		if registered, found := s.getCommand(currentArg); found {
			cmd = registered
			s.registerCommand(cmd, currentArg)
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
			s.queueCommandCallback(cmd)
		}

	} else if state.CurrentPos() == 0 && !s.isFlag(currentArg) {
		s.addError(fmt.Errorf("options should be prefixed by '-'"))
	}

	return terminating
}

func (s *Parser) queueCommandCallback(cmd *Command) {
	if cmd.Callback != nil {
		s.callbackQueue.Push(commandCallback{
			callback:  cmd.Callback,
			arguments: []interface{}{s, cmd},
		})
	}
}

func (s *Parser) processFlag(argument *Argument, state parse.State, flag string) {
	var err error
	if argument.Secure.IsSecure {
		if state.CurrentPos() < state.Len()-1 {
			if !s.isFlag(state.Peek()) {
				state.SkipCurrent()
			}
		}
		s.queueSecureArgument(flag, argument)
	} else {
		var next string
		if state.CurrentPos() < state.Len()-1 {
			next = state.Peek()
		}
		if (len(next) == 0 || s.isFlag(next)) && len(argument.DefaultValue) > 0 {
			next = argument.DefaultValue
		} else {
			state.SkipCurrent()
		}
		if state.CurrentPos() >= state.Len()-1 && len(next) == 0 {
			s.addError(fmt.Errorf("flag '%s' expects a value", flag))
		} else {
			next, err = s.flagValue(argument, next, flag)
			if err != nil {
				s.addError(err)
			} else {
				if err = s.processValueFlag(flag, next, argument); err != nil {
					s.addError(fmt.Errorf("failed to process your input for Flag '%s': %s", flag, err))
				}
			}
		}
	}
}

func (s *Parser) flagValue(argument *Argument, next string, flag string) (arg string, err error) {
	if argument.TypeOf == File {
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
		s.registerFlagValue(flag, arg, next)
	} else {
		arg = next
		s.registerFlagValue(flag, next, next)
	}

	return arg, err
}

func (s *Parser) checkSubCommands(cmdQueue *queue.Q[*Command], currentArg string) (bool, *Command) {
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
		s.registerCommand(&sub, currentArg)
		cmdQueue.Push(&sub) // Keep subcommands in the queue
		return true, &sub
	} else if len(currentCmd.Subcommands) > 0 {
		s.addError(fmt.Errorf("command %s expects one of the following: %v",
			currentCmd.Name, currentCmd.Subcommands))
	}

	return false, nil
}

func (s *Parser) processValueFlag(currentArg string, next string, argument *Argument) error {
	var processed string
	if len(argument.AcceptedValues) > 0 {
		processed = s.processSingleValue(next, currentArg, argument)
	} else {
		haveFilters := argument.PreFilter != nil || argument.PostFilter != nil
		if argument.PreFilter != nil {
			processed = argument.PreFilter(next)
			s.registerFlagValue(currentArg, processed, next)
		}
		if argument.PostFilter != nil {
			if processed != "" {
				processed = argument.PostFilter(processed)
			} else {
				processed = argument.PostFilter(next)
			}
			s.registerFlagValue(currentArg, processed, next)
		}
		if !haveFilters {
			processed = next
		}
	}

	return s.setBoundVariable(processed, currentArg)
}

func (s *Parser) processSecureFlag(name string, config *Secure) {
	var prompt string
	if !s.HasFlag(name) {
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
	if pass, err := util.GetSecureString(prompt, s.GetStderr(), s.GetTerminalReader()); err == nil {
		err = s.registerSecureValue(name, pass)
		if err != nil {
			s.addError(fmt.Errorf("failed to process flag '%s' secure value: %s", name, err))
		}
	} else {
		s.addError(fmt.Errorf("secure flag '%s' expects a value but we failed to obtain one: %s", name, err))
	}
}

func (s *Parser) processSingleValue(next, key string, argument *Argument) string {
	switch argument.TypeOf {
	case Single:
		return s.checkSingle(next, key, argument)
	case Chained:
		return s.checkMultiple(next, key, argument)
	}

	return ""
}

func (s *Parser) checkSingle(next, flag string, argument *Argument) string {
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
		if v.value.MatchString(value) {
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
		s.registerFlagValue(flag, value, next)
	} else {
		s.addError(fmt.Errorf(
			"invalid argument '%s' for flag '%s'. Accepted values: %s", next, flag, errBuf.String()))
	}

	return value
}

func (s *Parser) checkMultiple(next, flag string, argument *Argument) string {
	valid := 0
	errBuf := strings.Builder{}
	listDelimFunc := s.getListDelimiterFunc()
	args := strings.FieldsFunc(next, listDelimFunc)

	for i := 0; i < len(args); i++ {
		if argument.PreFilter != nil {
			args[i] = argument.PreFilter(args[i])
		}

		for _, v := range argument.AcceptedValues {
			if v.value.MatchString(args[i]) {
				valid++
			}
		}

		if argument.PostFilter != nil {
			args[i] = argument.PostFilter(args[i])
		}
	}

	value := strings.Join(args, "|")
	if valid == len(args) {
		s.registerFlagValue(flag, value, next)
	} else {
		lenValues := len(argument.AcceptedValues)
		for i := 0; i < lenValues; i++ {
			v := argument.AcceptedValues[i]
			errBuf.WriteString(v.Describe())
			if i+1 < lenValues {
				errBuf.WriteString(", ")
			}
		}
		s.addError(fmt.Errorf(
			"invalid argument '%s' for flag '%s'. Accepted values: %s", next, flag, errBuf.String()))
	}

	return value
}

func (s *Parser) validateProcessedOptions() {
	s.walkCommands()
	s.walkFlags()
}

func (s *Parser) walkFlags() {
	for f := s.acceptedFlags.Front(); f != nil; f = f.Next() {
		flagInfo := f.Value
		visited := make(map[string]bool)
		if flagInfo.Argument.RequiredIf != nil {
			if required, msg := flagInfo.Argument.RequiredIf(s, *f.Key); required {
				s.addError(errors.New(msg))
			}
			continue
		}

		if !flagInfo.Argument.Required {
			if s.HasFlag(*f.Key) && flagInfo.Argument.TypeOf == Standalone {
				s.validateStandaloneFlag(*f.Key)
			}
			continue
		}

		mainKey := s.flagOrShortFlag(*f.Key)
		if _, found := s.options[mainKey]; found {
			if flagInfo.Argument.TypeOf == Standalone {
				s.validateStandaloneFlag(mainKey)
			}
			continue
		} else if flagInfo.Argument.Secure.IsSecure {
			s.queueSecureArgument(mainKey, flagInfo.Argument)
			continue
		}

		cmdArg := splitPathFlag(mainKey)
		if len(flagInfo.Argument.DependsOn) == 0 {
			if len(cmdArg) == 1 || (len(cmdArg) == 2 && s.HasCommand(cmdArg[1])) {
				s.addError(fmt.Errorf("flag '%s' is mandatory but missing from the command line", *f.Key))
			}

		} else {
			s.validateDependencies(flagInfo, mainKey, visited, 0)
		}
	}
}

func (s *Parser) validateStandaloneFlag(key string) {
	_, err := s.GetBool(key)
	if err != nil {
		s.addError(err)
	}
}

func (s *Parser) walkCommands() {
	stack := queue.New[*Command]()
	for kv := s.registeredCommands.Front(); kv != nil; kv = kv.Next() {
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
			if _, found := s.commandOptions.Get(sub.path); found {
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

func (s *Parser) validateDependencies(flagInfo *FlagInfo, mainKey string, visited map[string]bool, depth int) {
	if depth > s.maxDependencyDepth {
		s.addError(fmt.Errorf("maximum dependency depth exceeded for flag '%s'", mainKey))
		return
	}

	if visited[mainKey] {
		s.addError(fmt.Errorf("circular dependency detected: flag '%s' is involved in a circular chain of dependencies", mainKey))
		return
	}

	visited[mainKey] = true

	for _, depends := range s.getDependentFlags(flagInfo.Argument) {
		dependentFlag, found := s.getFlagInCommandPath(depends, flagInfo.CommandPath)
		if !found {
			s.addError(fmt.Errorf("flag '%s' depends on '%s', but it is missing from command group '%s' or global flags",
				mainKey, depends, flagInfo.CommandPath))
			continue
		}

		dependKey := s.options[depends]
		matches, allowedValues := s.checkDependencyValue(flagInfo.Argument, depends, dependKey)
		if !matches {
			s.addError(fmt.Errorf("flag '%s' requires flag '%s' to have one of these values: %v (got '%s')",
				mainKey, depends, allowedValues, dependKey))
		}

		s.validateDependencies(dependentFlag, depends, visited, depth+1)
	}

	visited[mainKey] = false
}

func (s *Parser) getFlagInCommandPath(flag string, commandPath string) (*FlagInfo, bool) {
	// First, check if the flag exists in the command-specific path
	if commandPath != "" {
		flagKey := buildPathFlag(flag, commandPath)
		if flagInfo, exists := s.acceptedFlags.Get(flagKey); exists {
			return flagInfo, true
		}
	}

	// Fallback to global flag
	if flagInfo, exists := s.acceptedFlags.Get(flag); exists {
		return flagInfo, true
	}

	return nil, false
}

func (s *Parser) setBoundVariable(value string, currentArg string) error {
	data, found := s.bind[currentArg]
	if !found {
		return nil
	}

	flagInfo, _ := s.acceptedFlags.Get(currentArg)
	if value == "" {
		value = flagInfo.Argument.DefaultValue
	}

	if len(s.customBind) > 0 {
		customProc, found := s.customBind[currentArg]
		if found {
			customProc(currentArg, value, data)
			return nil
		}
	}

	return convertString(value, data, currentArg, s.listFunc)
}

func (s *Parser) prefixFunc(r rune) bool {
	for i := 0; i < len(s.prefixes); i++ {
		if r == s.prefixes[i] {
			return true
		}
	}

	return false
}

func (s *Parser) getListDelimiterFunc() ListDelimiterFunc {
	if s.listFunc != nil {
		return s.listFunc
	}

	return matchChainedSeparators
}

func (s *Parser) groupEnvVarsByCommand() map[string][]string {
	commandEnvVars := make(map[string][]string)
	if s.envNameConverter == nil {
		return commandEnvVars
	}
	for _, env := range os.Environ() {
		kv := strings.Split(env, "=")
		v := s.envNameConverter(kv[0])
		if v == "" {
			continue
		}
		for f := s.acceptedFlags.Front(); f != nil; f = f.Next() {
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

func canConvert(data interface{}, optionType OptionType) (bool, error) {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return false, fmt.Errorf("%w: we expect a pointer to a variable", ErrUnsupportedTypeConversion)
	}

	supported := true
	var err error
	if optionType == Standalone {
		switch data.(type) {
		case *bool:
			return true, nil
		default:
			return false, fmt.Errorf("%w: Standalone fields can only be bound to a boolean variable", ErrUnsupportedTypeConversion)
		}
	}

	switch t := data.(type) {
	case *string:
	case *[]string:
	case *complex64:
	case *int:
	case *[]int:
	case *int64:
	case *[]int64:
	case *int32:
	case *[]int32:
	case *int16:
	case *[]int16:
	case *int8:
	case *[]int8:
	case *uint:
	case *[]uint:
	case *uint64:
	case *[]uint64:
	case *uint32:
	case *[]uint32:
	case *uint16:
	case *[]uint16:
	case *uint8:
	case *[]uint8:
	case *float64:
	case *[]float64:
	case *float32:
	case *[]float32:
	case *bool:
	case *[]bool:
	case *time.Time:
	case *[]time.Time:
	default:
		supported = false
		err = fmt.Errorf("%w: unsupported data type %v", ErrUnsupportedTypeConversion, t)
	}

	return supported, err
}

func convertString(value string, data any, arg string, delimiterFunc ListDelimiterFunc) error {
	var err error

	switch t := data.(type) {
	case *string:
		*(t) = value
	case *[]string:
		values := strings.FieldsFunc(value, delimiterFunc)
		*(t) = values
	case *complex64:
		if val, err := strconv.ParseComplex(value, 64); err == nil {
			*(t) = complex64(val)
		}
	case *int:
		if val, err := strconv.Atoi(value); err == nil {
			*(t) = val
		}
	case *[]int:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int, len(values))
		for i, v := range values {
			if val, err := strconv.Atoi(v); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *int64:
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			*(t) = val
		}
	case *[]int64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 64); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *int32:
		if val, err := strconv.ParseInt(value, 10, 32); err == nil {
			*(t) = int32(val)
		}
	case *[]int32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 32); err == nil {
				temp[i] = int32(val)
			}
		}
		*(t) = temp
	case *int16:
		if val, err := strconv.ParseInt(value, 10, 16); err == nil {
			*(t) = int16(val)
		}
	case *[]int16:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int16, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 16); err == nil {
				temp[i] = int16(val)
			}
		}
		*(t) = temp
	case *int8:
		if val, err := strconv.ParseInt(value, 10, 8); err == nil {
			*(t) = int8(val)
		}
	case *[]int8:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int8, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 8); err == nil {
				temp[i] = int8(val)
			}
		}
		*(t) = temp
	case *uint:
		if val, err := strconv.ParseUint(value, 10, strconv.IntSize); err == nil {
			*(t) = uint(val)
		}
	case *[]uint:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, strconv.IntSize); err == nil {
				temp[i] = uint(val)
			}
		}
		*(t) = temp
	case *uint64:
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			*(t) = val
		}
	case *[]uint64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 64); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *uint32:
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			*(t) = uint32(val)
		}
	case *[]uint32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 32); err == nil {
				temp[i] = uint32(val)
			}
		}
		*(t) = temp
	case *uint16:
		if val, err := strconv.ParseUint(value, 10, 16); err == nil {
			*(t) = uint16(val)
		}
	case *[]uint16:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint16, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 16); err == nil {
				temp[i] = uint16(val)
			}
		}
		*(t) = temp
	case *uint8:
		if val, err := strconv.ParseUint(value, 10, 8); err == nil {
			*(t) = uint8(val)
		}
	case *[]uint8:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint8, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 8); err == nil {
				temp[i] = uint8(val)
			}
		}
		*(t) = temp
	case *float64:
		if val, err := strconv.ParseFloat(value, 64); err == nil {
			*(t) = val
		}
	case *[]float64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]float64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseFloat(v, 64); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *float32:
		if val, err := strconv.ParseFloat(value, 32); err == nil {
			*(t) = float32(val)
		}
	case *[]float32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]float32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseFloat(v, 32); err == nil {
				temp[i] = float32(val)
			}
		}
		*(t) = temp
	case *bool:
		if val, err := strconv.ParseBool(value); err == nil {
			*(t) = val
		}
	case *[]bool:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]bool, len(values))
		for i, v := range values {
			if val, err := strconv.ParseBool(v); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *time.Time:
		if val, err := dateparse.ParseLocal(value); err == nil {
			*(t) = val
		}
	case *[]time.Time:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]time.Time, len(values))
		for i, v := range values {
			if val, err := dateparse.ParseLocal(v); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	default:
		err = fmt.Errorf("%w: unsupported data type %v for argument %s", ErrUnsupportedTypeConversion, t, arg)
	}

	return err
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

func typeOfFlagFromString(s string) OptionType {
	switch strings.ToUpper(s) {
	case "STANDALONE":
		return Standalone
	case "CHAINED":
		return Chained
	case "FILE":
		return File
	case "SINGLE":
		return Single
	default:
		return Empty
	}
}

func (s *Parser) mergeCmdLine(nestedCmdLine *Parser) error {
	for k, v := range nestedCmdLine.bind {
		if _, exists := s.bind[k]; exists {
			return fmt.Errorf("conflict: flag '%s' is already bound in this CmdLineOption", k)
		}
		s.bind[k] = v
	}
	for k, v := range nestedCmdLine.customBind {
		s.customBind[k] = v
	}
	for it := nestedCmdLine.acceptedFlags.Front(); it != nil; it = it.Next() {
		s.acceptedFlags.Set(*it.Key, it.Value)
	}
	for k, v := range nestedCmdLine.lookup {
		s.lookup[k] = v
	}
	for it := nestedCmdLine.registeredCommands.Front(); it != nil; it = it.Next() {
		s.registeredCommands.Set(*it.Key, it.Value)
	}

	return nil
}

func legacyUnmarshalTagFormat(field reflect.StructField) (*tagConfig, error) {
	config := &tagConfig{
		kind: kindFlag,
	}

	tagNames := []string{
		"long", "short", "description", "required", "type", "default",
		"secure", "prompt", "path", "accepted", "depends",
	}

	for _, tag := range tagNames {
		value, ok := field.Tag.Lookup(tag)
		if !ok {
			continue
		}

		switch tag {
		case "long":
			config.name = value
		case "short":
			config.short = value
		case "description":
			config.description = value
		case "type":
			config.typeOf = typeOfFlagFromString(value)
		case "default":
			config.default_ = value
		case "required":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'required' tag value for field %s: %w", field.Name, err)
			}
			config.required = boolVal
		case "secure":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'secure' tag value for field %s: %w", field.Name, err)
			}
			if boolVal {
				config.secure = Secure{IsSecure: boolVal}
			}
		case "prompt":
			if config.secure.IsSecure {
				config.secure.Prompt = value
			}
		case "path":
			config.path = value
		case "accepted":
			patterns, err := parse.PatternValues(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'accepted' tag value for field %s: %w", field.Name, err)
			}
			// Convert to PatternValue
			config.acceptedValues = make([]PatternValue, len(patterns))
			for i, p := range patterns {
				pv, err := convertPattern(p, field.Name)
				if err != nil {
					return nil, err
				}
				config.acceptedValues[i] = *pv
			}
		case "depends":
			deps, err := parse.Dependencies(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'depends' tag value for field %s: %w", field.Name, err)
			}
			config.dependsOn = deps
		default:
			return nil, fmt.Errorf("unrecognized tag '%s' on field %s", tag, field.Name)
		}
	}

	// Validate type if specified
	if typeStr, ok := field.Tag.Lookup("type"); ok {
		switch strings.ToLower(typeStr) {
		case "single", "standalone", "chained", "file":
			config.typeOf = typeOfFlagFromString(typeStr)
		case "":
			config.typeOf = Single
		default:
			config.typeOf = Empty
		}
	}

	return config, nil
}

func unmarshalTagFormat(tag string, field reflect.StructField) (*tagConfig, error) {
	config := &tagConfig{}
	parts := strings.Split(tag, ";")

	for _, part := range parts {
		key, value, found := strings.Cut(part, ":")
		if !found {
			return nil, fmt.Errorf("invalid tag format in field %s: %s", field.Name, part)
		}

		switch key {
		case "kind":
			switch kind(value) {
			case kindFlag, kindCommand, kindEmpty:
				config.kind = kind(value)
			default:
				return nil, fmt.Errorf("invalid kind in field %s: %s (must be 'command', 'flag', or empty)",
					field.Name, value)
			}
		case "name":
			config.name = value
		case "short":
			config.short = value
		case "type":
			config.typeOf = typeOfFlagFromString(value)
		case "desc":
			config.description = value
		case "default":
			config.default_ = value
		case "required":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'required' value in field %s: %w", field.Name, err)
			}
			config.required = boolVal
		case "secure":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'secure' value in field %s: %w", field.Name, err)
			}
			if boolVal {
				config.secure = Secure{IsSecure: boolVal}
			}
		case "prompt":
			if config.secure.IsSecure {
				config.secure.Prompt = value
			}
		case "path":
			config.path = value
		case "accepted":
			patterns, err := parse.PatternValues(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'accepted' value in field %s: %w", field.Name, err)
			}
			for i, p := range patterns {
				config.acceptedValues = make([]PatternValue, len(patterns))
				pv, err := convertPattern(p, field.Name)
				if err != nil {
					return nil, err
				}
				config.acceptedValues[i] = *pv
			}
		case "depends":
			deps, err := parse.Dependencies(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'depends' value in field %s: %w", field.Name, err)
			}
			config.dependsOn = deps
		default:
			return nil, fmt.Errorf("unrecognized key '%s' in field %s", key, field.Name)
		}
	}

	// If kind is empty, treat as flag
	if config.kind == kindEmpty {
		config.kind = kindFlag
	}

	// Validate type if specified
	if typeStr, ok := field.Tag.Lookup("type"); ok {
		switch typeStr {
		case "single", "standalone", "chained", "file":
			config.typeOf = typeOfFlagFromString(typeStr)
		default:
			return nil, fmt.Errorf("invalid type value: %s", typeStr)
		}
	}

	return config, nil
}

func convertPattern(p parse.TagPatternValue, fieldName string) (*PatternValue, error) {
	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid 'accepted' value in field %s: %w", fieldName, err)
	}

	return &PatternValue{
		Pattern:     p.Pattern,
		Description: p.Description,
		value:       re,
	}, nil
}

func unmarshalTagsToArgument(field reflect.StructField, arg *Argument) (name string, path string, err error) {
	// Try new format first
	if tag, ok := field.Tag.Lookup("goopt"); ok && strings.Contains(tag, ":") {
		config, err := unmarshalTagFormat(tag, field)
		if err != nil {
			return "", "", err
		}
		*arg = *config.toArgument()
		return config.name, config.path, nil
	}

	// Legacy format handling
	config, err := legacyUnmarshalTagFormat(field)
	if err != nil {
		return "", "", err
	}
	*arg = *config.toArgument()

	return config.name, config.path, nil
}

func (c tagConfig) toArgument() *Argument {
	return &Argument{
		Short:          c.short,
		Description:    c.description,
		TypeOf:         c.typeOf,
		DefaultValue:   c.default_,
		Required:       c.required,
		Secure:         c.secure,
		AcceptedValues: c.acceptedValues,
		DependencyMap:  c.dependsOn,
	}
}

func (s *Parser) buildCommand(commandPath, description string, parent *Command) (*Command, error) {
	commandNames := strings.Split(commandPath, " ")

	var topParent = parent
	var currentCommand *Command

	for _, cmdName := range commandNames {
		found := false

		// If we're at the top level (parent is nil)
		if parent == nil {
			// Look for the command at the top level
			if cmd, exists := s.registeredCommands.Get(cmdName); exists {
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
				s.registeredCommands.Set(cmdName, newCommand)
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
		if _, exists := s.registeredCommands.Get(topParent.Name); !exists {
			s.registeredCommands.Set(topParent.Name, topParent)
		}
	}

	return topParent, nil
}

func newParserFromReflectValue(structValue reflect.Value, prefix string, maxDepth, currentDepth int) (*Parser, error) {
	if currentDepth > maxDepth {
		return nil, fmt.Errorf("recursion depth exceeded: max depth is %d", maxDepth)
	}

	c := NewParser()
	st := structValue.Type()
	if st.Kind() == reflect.Ptr {
		if structValue.IsNil() {
			return nil, fmt.Errorf("nil pointer encountered")
		}
		st = st.Elem()
		structValue = structValue.Elem()
	}
	if st.Kind() != reflect.Struct {
		return nil, fmt.Errorf("only structs can be tagged")
	}

	err := c.processStructCommands(structValue, "", 0, maxDepth)
	if err != nil {
		c.addError(err)
	}

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		if _, ok := field.Tag.Lookup("ignore"); ok {
			continue
		}

		fieldValue := structValue.Field(i)
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
			if prefix != "" {
				c.addError(fmt.Errorf("error processing field %s.%s: %w", prefix, field.Name, err))
			} else {
				c.addError(fmt.Errorf("error processing field %s: %w", field.Name, err))
			}
			continue
		}

		// Fallback to field name if no tag found
		if longName == "" {
			longName = c.flagNameConverter(field.Name)
		}

		// Create a new prefix for nested fields
		fieldPath := longName
		if prefix != "" {
			fieldPath = fmt.Sprintf("%s.%s", prefix, longName)
		}

		if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			if err := processSliceField(fieldPath, fieldValue, maxDepth, currentDepth, c); err != nil {
				c.addError(fmt.Errorf("error processing slice field %s: %w", fieldPath, err))
			}
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			if err := processNestedStruct(fieldPath, fieldValue, maxDepth, currentDepth, c); err != nil {
				c.addError(fmt.Errorf("error processing nested struct %s: %w", fieldPath, err))
			}
			continue
		}

		// Avoid leading dot if prefix is empty
		fullFlagName := longName
		if prefix != "" {
			fullFlagName = fmt.Sprintf("%s.%s", prefix, longName)
		}

		// Process the path tag to associate the flag with commands or global
		if pathTag != "" {
			paths := strings.Split(pathTag, ",")
			for _, cmdPath := range paths {
				cmdPathComponents := strings.Split(cmdPath, " ")
				parentCommand := ""
				var cmd *Command
				var pCmd *Command

				for i, cmdComponent := range cmdPathComponents {
					if i == 0 {
						if p, ok := c.registeredCommands.Get(cmdComponent); ok {
							pCmd = p
						}
					}
					if parentCommand == "" {
						parentCommand = cmdComponent
					} else {
						parentCommand = fmt.Sprintf("%s %s", parentCommand, cmdComponent)
					}

					// Ensure the command hierarchy exists up to this point
					if cmd, err = c.buildCommand(parentCommand, "", pCmd); err != nil {
						c.addError(fmt.Errorf("error processing command %s: %w", parentCommand, err))
					}
				}

				if cmd != nil {
					err = c.AddCommand(cmd)
					if err != nil {
						return nil, err
					}
				}

				// Bind the flag to the last command in the path
				err = c.BindFlag(fieldValue.Addr().Interface(), fullFlagName, arg, cmdPath)
				if err != nil {
					return nil, err
				}
				if arg.DefaultValue != "" {
					err = c.setBoundVariable(arg.DefaultValue, buildPathFlag(fullFlagName, cmdPath))
					if err != nil {
						c.addError(fmt.Errorf("error processing default value %s: %w", arg.DefaultValue, err))
					}
				}
			}

		} else {
			// If no path is specified, consider it a global flag
			err = c.BindFlag(fieldValue.Addr().Interface(), fullFlagName, arg)
			if err != nil {
				return nil, err
			}
			if arg.DefaultValue != "" {
				err = c.setBoundVariable(arg.DefaultValue, fullFlagName)
				if err != nil {
					c.addError(fmt.Errorf("error processing default value %s: %w", arg.DefaultValue, err))
				}
			}
		}
	}

	return c, nil
}

func (s *Parser) processStructCommands(val reflect.Value, currentPath string, currentDepth, maxDepth int) error {
	// Handle case where the entire value is a Command type (not a struct containing commands)
	if val.Type() == reflect.TypeOf(Command{}) {
		cmd := val.Interface().(Command)
		_, err := s.buildCommand(cmd.path, cmd.Description, nil)
		if err != nil {
			return fmt.Errorf("error ensuring command hierarchy for path %s: %w", cmd.path, err)
		}
		err = s.AddCommand(&cmd)
		if err != nil {
			return err
		}
		return nil
	}

	// Prevent infinite recursion
	typ := val.Type()
	if currentDepth > maxDepth {
		return fmt.Errorf("max nesting depth exceeded: %d", maxDepth)
	}

	// Process all fields in the struct
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		if !fieldType.IsExported() || fieldType.Tag.Get("ignore") != "" {
			continue
		}

		// Process Command fields first - these are explicit command definitions
		if fieldType.Type == reflect.TypeOf(Command{}) {
			cmd := field.Interface().(Command)
			// Build path by combining current path with command name
			cmdPath := cmd.Name
			if currentPath != "" {
				cmdPath = currentPath + " " + cmd.Name
			}

			// Find the root parent command (first part of the path)
			var parent *Command
			if currentPath != "" {
				parentPath := strings.Split(currentPath, " ")[0]
				if p, ok := s.registeredCommands.Get(parentPath); ok {
					parent = p
				}
			}

			// Register the command with its full path
			buildCmd, err := s.buildCommand(cmdPath, cmd.Description, parent)
			if err != nil {
				return fmt.Errorf("error ensuring command hierarchy for path %s: %w", cmdPath, err)
			}

			err = s.AddCommand(buildCmd)
			if err != nil {
				return err
			}

			continue // no need to process nested structs since we've already processed the Command hierarchy at this level
		}

		// Then process struct fields which might contain struct tags defining nested commands
		if field.Kind() == reflect.Struct {
			// Parse the goopt tag for command configuration
			config, err := unmarshalTagFormat(fieldType.Tag.Get("goopt"), fieldType)
			if err != nil {
				return err
			}

			if config.kind == kindCommand {
				cmdName := config.name
				if cmdName == "" {
					cmdName = s.commandNameConverter(fieldType.Name)
				}

				// Build the command path
				cmdPath := cmdName
				if currentPath != "" {
					cmdPath = currentPath + " " + cmdName
				}

				// Handle root-level commands
				if currentPath == "" {
					buildCmd, err := s.buildCommand(cmdPath, config.description, nil)
					if err != nil {
						return fmt.Errorf("error processing command %s: %w", cmdPath, err)
					}

					err = s.AddCommand(buildCmd)
					if err != nil {
						return err
					}
				} else {
					// Handle nested commands by finding their root parent
					parentPath := strings.Split(currentPath, " ")[0]
					if p, ok := s.registeredCommands.Get(parentPath); ok {
						buildCmd, err := s.buildCommand(cmdPath, config.description, p)
						if err != nil {
							return fmt.Errorf("error processing command %s: %w", cmdPath, err)
						}
						err = s.AddCommand(buildCmd)
						if err != nil {
							return err
						}
					} else {
						return fmt.Errorf("parent command %s not found", parentPath)
					}
				}

				// Update current path for nested processing
				currentPath = cmdPath
			}

			// Recursively process nested structs with the updated path
			if err := s.processStructCommands(field, currentPath, currentDepth+1, maxDepth); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkDependencyValue checks if the provided value matches any of the required values
// for a given dependency
func (s *Parser) checkDependencyValue(arg *Argument, dependentFlag string, actualValue string) (bool, []string) {
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
func (s *Parser) getDependentFlags(arg *Argument) []string {
	deps := make([]string, 0, len(arg.DependencyMap))
	for dep := range arg.DependencyMap {
		deps = append(deps, dep)
	}
	return deps
}

func processSliceField(prefix string, fieldValue reflect.Value, maxDepth, currentDepth int, c *Parser) error {
	if fieldValue.IsNil() {
		fieldValue.Set(reflect.MakeSlice(fieldValue.Type(), 0, 0))
	}

	for idx := 0; idx < fieldValue.Len(); idx++ {
		elem := fieldValue.Index(idx).Addr()

		// Create full path with the slice index
		elemPrefix := fmt.Sprintf("%s.%d", prefix, idx)

		nestedCmdLine, err := newParserFromReflectValue(elem, elemPrefix, maxDepth, currentDepth+1)
		if err != nil {
			return fmt.Errorf("error processing slice element %s[%d]: %w", prefix, idx, err)
		}
		err = c.mergeCmdLine(nestedCmdLine)
		if err != nil {
			return err
		}
	}

	return nil
}

func processNestedStruct(prefix string, fieldValue reflect.Value, maxDepth, currentDepth int, c *Parser) error {
	nestedCmdLine, err := newParserFromReflectValue(fieldValue.Addr(), prefix, maxDepth, currentDepth+1)
	if err != nil {
		return fmt.Errorf("error processing nested struct %s: %w", prefix, err)
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
	if len(paths) > 1 {
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
