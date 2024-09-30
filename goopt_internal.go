package goopt

import (
	"errors"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/parse"
	"github.com/napalu/goopt/types/orderedmap"
	"github.com/napalu/goopt/types/queue"
	"github.com/napalu/goopt/util"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (s *CmdLineOption) parseFlag(state parse.State, currentCommandPath string) {
	flag := s.flagOrShortFlag(strings.TrimLeftFunc(state.CurrentArg(), s.prefixFunc), currentCommandPath)

	// Try finding the flag in the current command path
	flagInfo, found := s.acceptedFlags.Get(flag)

	// If not found in the current command path, check for global flags
	if !found {
		flagInfo, found = s.acceptedFlags.Get(state.CurrentArg())
		if found {
			flag = state.CurrentArg()
		}
	}

	if found {
		s.processFlagArg(state, flagInfo.Argument, flag, currentCommandPath)
	} else {
		s.addError(fmt.Errorf("unknown argument '%s' in command Path '%s'", flag, currentCommandPath))
	}
}

func (s *CmdLineOption) parsePosixFlag(state parse.State, currentCommandPath string) {
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

func (s *CmdLineOption) normalizePosixArgs(state parse.State, currentArg string, commandPath string) {
	sb := strings.Builder{}
	lenS := len(currentArg)
	statePos := state.CurrentPos()
	stateEndOf := state.Len()
	newArgs := make([]string, 0, state.Len())
	if statePos > 0 {
		newArgs = append(newArgs, state.Args()[:statePos]...)
	}
	startPos := 0
	for startPos < lenS {
		cf := s.flagOrShortFlag(currentArg[startPos:startPos+1], commandPath)
		_, found := s.acceptedFlags.Get(cf)
		if found {
			newArgs = append(newArgs, fmt.Sprintf("-%s", cf))
			startPos++
		} else {
			v := splitPathFlag(cf)
			sb.WriteString(v[0])
			startPos++
			for startPos < lenS {
				cf = s.flagOrShortFlag(currentArg[startPos:startPos+1], commandPath)
				_, found = s.acceptedFlags.Get(cf)
				if found {
					break
				} else {
					v = splitPathFlag(cf)
					sb.WriteString(v[0])
					startPos++
				}
			}
			newArgs = append(newArgs, sb.String())
			sb.Reset()
		}
		stateEndOf++
	}

	if startPos > 0 {
		stateEndOf--
	}

	if state.Len() > statePos+1 {
		newArgs = append(newArgs, state.Args()[statePos+1:]...)
	}

	state.ReplaceArgs(newArgs...)
}

func (s *CmdLineOption) processFlagArg(state parse.State, argument *Argument, currentArg string, currentCommandPath ...string) {
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

func (s *CmdLineOption) registerCommandRecursive(cmd *Command) {
	// Add the current command to the map
	cmd.TopLevel = strings.Count(cmd.Path, " ") == 0
	s.registeredCommands.Set(cmd.Path, cmd)

	// Recursively register all subcommands
	for i := range cmd.Subcommands {
		subCmd := &cmd.Subcommands[i]
		s.registerCommandRecursive(subCmd)
	}

}

func (s *CmdLineOption) validateCommand(cmdArg *Command, level, maxDepth int) (bool, error) {
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
		cmdArg.Path = cmdArg.Name
	}

	for i := 0; i < len(cmdArg.Subcommands); i++ {
		cmdArg.Subcommands[i].Path = cmdArg.Path + " " + cmdArg.Subcommands[i].Name
		if ok, err := s.validateCommand(&cmdArg.Subcommands[i], level+1, maxDepth); err != nil {
			return ok, err
		}
	}

	return true, nil
}

func (s *CmdLineOption) ensureInit() {
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
}

func (a *Argument) ensureInit() {
	if a.DependsOn == nil {
		a.DependsOn = []string{}
	}
	if a.OfValue == nil {
		a.OfValue = []string{}
	}
}

func (s *CmdLineOption) setPositionalArguments(args []string, commandPath ...string) {
	var positional []PositionalArgument
	for i, seen := range args {
		seen = s.flagOrShortFlag(strings.TrimLeftFunc(seen, s.prefixFunc), commandPath...)
		if _, found := s.rawArgs[seen]; !found {
			positional = append(positional, PositionalArgument{i, seen})
		}
	}

	s.positionalArgs = positional
}

func (s *CmdLineOption) evalFlagWithPath(state parse.State, currentCommandPath string) {
	if s.posixCompatible {
		s.parsePosixFlag(state, currentCommandPath)
	} else {
		s.parseFlag(state, currentCommandPath)
	}
}

func (s *CmdLineOption) flagOrShortFlag(flag string, commandPath ...string) string {
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

func (s *CmdLineOption) isFlag(flag string) bool {
	return strings.HasPrefix(flag, "-")
}

func (s *CmdLineOption) isGlobalFlag(arg string) bool {
	flag, ok := s.acceptedFlags.Get(s.flagOrShortFlag(strings.TrimLeftFunc(arg, s.prefixFunc)))
	if ok {
		return flag.CommandPath == ""
	}

	return false
}

func (s *CmdLineOption) addError(err error) {
	s.errors = append(s.errors, err)
}

func (s *CmdLineOption) getCommand(name string) (*Command, bool) {
	cmd, found := s.registeredCommands.Get(name)

	return cmd, found
}

func (s *CmdLineOption) registerSecureValue(flag, value string) error {
	var err error
	s.rawArgs[flag] = value
	if value != "" {
		s.options[flag] = value
		err = s.setBoundVariable(value, flag)
	}

	return err
}

func (s *CmdLineOption) registerFlagValue(flag, value, rawValue string) {
	parts := splitPathFlag(flag)
	s.rawArgs[parts[0]] = rawValue

	s.options[flag] = value
}

func (s *CmdLineOption) registerCommand(cmd *Command, name string) {
	if cmd.Path == "" {
		return
	}

	s.rawArgs[name] = name

	s.commandOptions.Set(cmd.Path, len(cmd.Subcommands) == 0)
}

func (s *CmdLineOption) queueSecureArgument(name string, argument *Argument) {
	if s.secureArguments == nil {
		s.secureArguments = orderedmap.NewOrderedMap[string, *Secure]()
	}

	s.secureArguments.Set(name, &argument.Secure)
}

func (s *CmdLineOption) parseCommand(state parse.State, cmdQueue *queue.Q[*Command], commandPathSlice *[]string) bool {
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
			cmdQueue.Push(cmd) // Add subcommands to queue
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

func (s *CmdLineOption) queueCommandCallback(cmd *Command) {
	if cmd.Callback != nil {
		s.callbackQueue.Push(commandCallback{
			callback:  cmd.Callback,
			arguments: []interface{}{s, cmd},
		})
	}
}

func (s *CmdLineOption) processFlag(argument *Argument, state parse.State, flag string) {
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

func (s *CmdLineOption) flagValue(argument *Argument, next string, flag string) (arg string, err error) {
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

func (s *CmdLineOption) checkSubCommands(cmdQueue *queue.Q[*Command], currentArg string) (bool, *Command) {
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

func (a *Argument) accept(val PatternValue) *error {
	re, err := regexp.Compile(val.Pattern)
	if err != nil {
		return &err
	}

	if a.AcceptedValues == nil {
		a.AcceptedValues = make([]LiterateRegex, 1, 5)
		a.AcceptedValues[0] = LiterateRegex{
			value:   re,
			explain: val.Description,
		}
	} else {
		a.AcceptedValues = append(a.AcceptedValues, LiterateRegex{
			value:   re,
			explain: val.Description,
		})
	}

	return nil
}

func (s *CmdLineOption) processValueFlag(currentArg string, next string, argument *Argument) error {
	var processed string
	if len(argument.AcceptedValues) > 0 {
		processed = s.processSingleValue(next, currentArg, argument)
	} else {
		if argument.PreFilter != nil {
			processed = argument.PreFilter(next)
			s.registerFlagValue(currentArg, processed, next)
		} else {
			processed = next
		}
	}

	return s.setBoundVariable(processed, currentArg)
}

func (s *CmdLineOption) processSecureFlag(name string, config *Secure) {
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
	if pass, err := util.GetSecureString(prompt, os.Stderr); err == nil {
		err = s.registerSecureValue(name, pass)
		if err != nil {
			s.addError(fmt.Errorf("failed to process flag '%s' secure value: %s", name, err))
		}
	} else {
		s.addError(fmt.Errorf("flag IsSecure '%s' expects a value but we failed to obtain one: %s", name, err))
	}
}

func (s *CmdLineOption) processSingleValue(next, key string, argument *Argument) string {
	switch argument.TypeOf {
	case Single:
		return s.checkSingle(next, key, argument)
	case Chained:
		return s.checkMultiple(next, key, argument)
	}

	return ""
}

func (s *CmdLineOption) checkSingle(next, flag string, argument *Argument) string {
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

func (s *CmdLineOption) checkMultiple(next, flag string, argument *Argument) string {
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

func (s *CmdLineOption) validateProcessedOptions() {
	s.walkCommands()
	s.walkFlags()
}

func (s *CmdLineOption) walkFlags() {
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

		cmdArg := strings.Split(mainKey, "@")
		if len(flagInfo.Argument.DependsOn) == 0 {
			if len(cmdArg) == 1 || (len(cmdArg) == 2 && s.HasCommand(cmdArg[1])) {
				s.addError(fmt.Errorf("flag '%s' is mandatory but missing from the command line", *f.Key))
			}

		} else {
			s.validateDependencies(flagInfo, mainKey, visited, 0)
		}
	}
}

func (s *CmdLineOption) validateStandaloneFlag(key string) {
	_, err := s.GetBool(key)
	if err != nil {
		s.addError(err)
	}
}

func (s *CmdLineOption) walkCommands() {
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
			if _, found := s.commandOptions.Get(sub.Path); found {
				matchedCommands = append(matchedCommands, sub)
				matches++
			}
		}

		if matches == 0 && cmd.Required {
			s.addError(fmt.Errorf("command '%s' was not given but is expected with one of commands [%s] to be specified",
				cmd.Name, match.String()))
		}

		for _, m := range matchedCommands {
			for _, sub := range m.Subcommands {
				stack.Push(&sub)
			}
		}
	}
}

func (s *CmdLineOption) validateDependencies(flagInfo *FlagInfo, mainKey string, visited map[string]bool, depth int) {
	// Set a max depth to avoid too deep recursion
	const maxDepth = 10
	if depth > maxDepth {
		s.addError(fmt.Errorf("maximum dependency depth exceeded for flag '%s'", mainKey))
		return
	}

	// Circular dependency check
	if visited[mainKey] {
		s.addError(fmt.Errorf("circular dependency detected: flag '%s' is involved in a circular chain of dependencies", mainKey))
		return
	}

	// Mark the current flag as visited
	visited[mainKey] = true

	// Process the dependencies of the current flag
	for _, depends := range flagInfo.Argument.DependsOn {
		// First, check if the dependent flag exists in the same command path
		dependentFlag, found := s.getFlagInCommandPath(depends, flagInfo.CommandPath)
		if !found {
			s.addError(fmt.Errorf("flag '%s' depends on '%s', but it is missing from command group '%s' or global flags", mainKey, depends, flagInfo.CommandPath))
			continue
		}

		// Check specific flag values (OfValue)
		dependKey := s.options[depends]
		for _, k := range dependentFlag.Argument.OfValue {
			if strings.EqualFold(dependKey, k) {
				s.addError(fmt.Errorf("flag '%s' requires flag '%s' to be present with value '%s'", mainKey, depends, k))
			}
		}

		// Recursively validate the dependencies of the dependent flag, while tracking visited flags and depth
		s.validateDependencies(dependentFlag, depends, visited, depth+1)
	}

	// Unmark the flag as visited to allow other validation chains to proceed
	visited[mainKey] = false
}

func (s *CmdLineOption) getFlagInCommandPath(flag string, commandPath string) (*FlagInfo, bool) {
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

func (s *CmdLineOption) setBoundVariable(value string, currentArg string) error {
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

func (s *CmdLineOption) prefixFunc(r rune) bool {
	for i := 0; i < len(s.prefixes); i++ {
		if r == s.prefixes[i] {
			return true
		}
	}

	return false
}

func (s *CmdLineOption) getListDelimiterFunc() ListDelimiterFunc {
	if s.listFunc != nil {
		return s.listFunc
	}

	return matchChainedSeparators
}

func (s *CmdLineOption) preSplitEnvVarsByCommand() map[string][]string {
	commandEnvVars := make(map[string][]string)
	if s.envFilter == nil {
		return commandEnvVars
	}
	for _, env := range os.Environ() {
		kv := strings.Split(env, "=")
		v := s.envFilter(kv[0])
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
	return regexp.MustCompile(`(\$\{.+\})`)
}

func typeOfFromString(s string) OptionType {
	switch strings.ToUpper(s) {
	case "STANDALONE":
		return Standalone
	case "CHAINED":
		return Chained
	case "FILE":
		return File
	case "SINGLE":
		fallthrough
	default:
		return Single
	}
}

func (s *CmdLineOption) mergeCmdLine(nestedCmdLine *CmdLineOption) error {
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

	return nil
}

// unmarshalTagsToArgument populates the Argument struct based on struct tags
func unmarshalTagsToArgument(field reflect.StructField, arg *Argument) error {
	tagNames := []string{"long", "short", "description", "required", "type", "default", "secure", "prompt", "path"}

	for _, tag := range tagNames {
		value, ok := field.Tag.Lookup(tag)
		if !ok {
			continue
		}

		switch tag {
		case "long":
			// This will be handled separately
		case "short":
			arg.Short = value
		case "description":
			arg.Description = value
		case "type":
			arg.TypeOf = typeOfFromString(value)
		case "default":
			arg.DefaultValue = value
		case "required":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid 'required' tag value for field %s: %w", field.Name, err)
			}
			arg.Required = boolVal
		case "secure":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid 'secure' tag value for field %s: %w", field.Name, err)
			}
			if boolVal {
				arg.Secure = Secure{IsSecure: boolVal}
			}
		case "prompt":
			if arg.Secure.IsSecure {
				arg.Secure.Prompt = value
			}
		case "path":
			// Path is handled separately.
		default:
			return fmt.Errorf("unrecognized tag '%s' on field %s", tag, field.Name)
		}
	}

	return nil
}

func (s *CmdLineOption) buildCommand(commandPath string, parent *Command) (*Command, error) {
	// Split the path into segments (commands)
	commandNames := strings.Split(commandPath, " ")

	var topParent *Command = parent
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
					Description: fmt.Sprintf("Auto-generated command for %s", cmdName),
					Subcommands: []Command{},
				}
				s.registeredCommands.Set(cmdName, newCommand)
				currentCommand = newCommand
			}
		} else {
			if cmdName == parent.Name {
				continue
			}
			// Look for the command as a subcommand of the parent
			for idx, subCmd := range parent.Subcommands {
				if subCmd.Name == cmdName {
					currentCommand = &parent.Subcommands[idx] // Use the existing subcommand
					found = true
					break
				}
			}

			// If not found, add a new subcommand to the parent immediately
			if !found {
				newCommand := Command{
					Name:        cmdName,
					Description: fmt.Sprintf("Auto-generated command for %s", cmdName),
					Subcommands: []Command{},
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

// Non-generic helper that works with reflect.Value for recursion
func newCmdLineFromReflectValue(structValue reflect.Value, prefix string, maxDepth, currentDepth int) (*CmdLineOption, error) {
	if currentDepth > maxDepth {
		return nil, fmt.Errorf("recursion depth exceeded: max depth is %d", maxDepth)
	}

	c := NewCmdLineOption()
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

	err := c.processStructCommands(structValue, 0, maxDepth)
	if err != nil {
		c.addError(err)
	}
	countZeroTags := 0

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		if _, ok := field.Tag.Lookup("ignore"); ok {
			continue
		}

		fieldValue := structValue.Field(i)
		if !fieldValue.CanAddr() || !fieldValue.CanInterface() {
			continue // Skip unexported fields
		}

		// Get the 'long' tag for the flag name
		longName, ok := field.Tag.Lookup("long")
		if !ok {
			// If no 'long' tag is provided, use the field name in lower camel case
			longName = strcase.ToLowerCamel(field.Name)
		}

		// Create a new prefix for nested fields
		fieldPath := longName
		if prefix != "" {
			fieldPath = fmt.Sprintf("%s.%s", prefix, longName)
		}

		// Handle slice of structs
		if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			if err := processSliceField(fieldPath, fieldValue, maxDepth, currentDepth, c); err != nil {
				c.addError(fmt.Errorf("error processing slice field %s: %w", fieldPath, err))
			}
			continue
		}

		// Handle nested structs
		if field.Type.Kind() == reflect.Struct {
			if err := processNestedStruct(fieldPath, fieldValue, maxDepth, currentDepth, c); err != nil {
				c.addError(fmt.Errorf("error processing nested struct %s: %w", fieldPath, err))
			}
			continue
		}

		// Regular field handling
		arg := &Argument{}
		err := unmarshalTagsToArgument(field, arg)
		if err != nil {
			c.addError(fmt.Errorf("error processing field %s: %w", fieldPath, err))
		}

		if reflect.DeepEqual(*arg, Argument{}) {
			countZeroTags++
			continue
		}

		// Avoid leading dot if prefix is empty
		fullFlagName := longName
		if prefix != "" {
			fullFlagName = fmt.Sprintf("%s.%s", prefix, longName)
		}

		// Process the path tag to associate the flag with commands or global
		pathTag := field.Tag.Get("path")
		if pathTag != "" {
			// Inside the loop for processing path tags
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
					if cmd, err = c.buildCommand(parentCommand, pCmd); err != nil {
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
			}

		} else {
			// If no path is specified, consider it a global flag
			err = c.BindFlag(fieldValue.Addr().Interface(), fullFlagName, arg)
			if err != nil {
				return nil, err
			}
		}
	}

	if countZeroTags == st.NumField() {
		return nil, fmt.Errorf("struct %s is not properly tagged", prefix)
	}

	return c, nil
}

func (s *CmdLineOption) processStructCommands(val reflect.Value, currentDepth, maxDepth int) error {
	typ := val.Type()

	// Check for nesting limit
	if currentDepth > maxDepth {
		return fmt.Errorf("max nesting depth exceeded: %d", maxDepth)
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		if _, ok := fieldType.Tag.Lookup("ignore"); ok {
			continue
		}

		if !fieldType.IsExported() {
			continue
		}

		// Check if the field is a Command
		if fieldType.Type == reflect.TypeOf(Command{}) {
			cmd := field.Interface().(Command)
			_, err := s.buildCommand(cmd.Path, &cmd)
			if err != nil {
				return fmt.Errorf("error ensuring command hierarchy for path %s: %w", cmd.Path, err)
			}
		} else if field.Kind() == reflect.Struct {
			// Recursively process nested structs if the field is a struct
			if err := s.processStructCommands(field, currentDepth+1, maxDepth); err != nil {
				return fmt.Errorf("error processing nested struct %s: %w", fieldType.Name, err)
			}
		}
	}

	return nil
}

func processSliceField(prefix string, fieldValue reflect.Value, maxDepth, currentDepth int, c *CmdLineOption) error {
	if fieldValue.IsNil() {
		fieldValue.Set(reflect.MakeSlice(fieldValue.Type(), 0, 0))
	}

	for idx := 0; idx < fieldValue.Len(); idx++ {
		elem := fieldValue.Index(idx).Addr()

		// Create full path with the slice index
		elemPrefix := fmt.Sprintf("%s.%d", prefix, idx)

		// Recursively process the element with the new non-generic helper
		nestedCmdLine, err := newCmdLineFromReflectValue(elem, elemPrefix, maxDepth, currentDepth+1)
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

// Adjusted function to process nested structs
func processNestedStruct(prefix string, fieldValue reflect.Value, maxDepth, currentDepth int, c *CmdLineOption) error {
	// Recursively process the nested struct with the new non-generic helper
	nestedCmdLine, err := newCmdLineFromReflectValue(fieldValue.Addr(), prefix, maxDepth, currentDepth+1)
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
