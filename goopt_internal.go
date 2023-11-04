package goopt

import (
	"errors"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/ef-ds/deque"
	"github.com/napalu/goopt/types/orderedmap"
	"github.com/napalu/goopt/util"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (s *CmdLineOption) parseFlag(args []string, state *parseState) {
	currentArg := s.flagOrShortFlag(strings.TrimLeftFunc(args[state.pos], s.prefixFunc))
	argument, found := s.acceptedFlags.Get(currentArg)
	if found {
		s.processFlagArg(args, state, argument, currentArg)

	} else {
		s.addError(fmt.Errorf("unknown argument '%s'", currentArg))
	}
}

func (s *CmdLineOption) parsePosixFlag(args []string, state *parseState) []string {
	currentArg := s.flagOrShortFlag(strings.TrimLeftFunc(args[state.pos], s.prefixFunc))
	argument, found := s.acceptedFlags.Get(currentArg)
	if found {
		s.processFlagArg(args, state, argument, currentArg)
		return args
	}

	// two-pass process to account for flag values directly adjacent to a flag (e.g. `-f1` instead of `-f 1`)
	// 1. we first scan for any value which is not a flag and extend args if any are found
	for startPos := 0; startPos < len(currentArg); startPos++ {
		cf := s.flagOrShortFlag(currentArg[startPos : startPos+1])
		_, found := s.acceptedFlags.Get(cf)
		if !found {
			args = insert(args, cf, state.pos+1)
			state.endOf++
		}
	}

	// 2. process as usual
	for startPos := 0; startPos < len(currentArg); startPos++ {
		cf := s.flagOrShortFlag(currentArg[startPos : startPos+1])
		argument, found := s.acceptedFlags.Get(cf)
		if found {
			s.processFlagArg(args, state, argument, cf)
		}
	}

	return args
}

func (s *CmdLineOption) processFlagArg(args []string, state *parseState, argument *Argument, currentArg string) {
	switch argument.TypeOf {
	case Standalone:
		if argument.Secure.IsSecure {
			s.queueSecureArgument(currentArg, argument)
		} else {
			boolVal := "true"
			if state.pos+1 < state.endOf {
				_, found := s.registeredCommands[args[state.pos+1]]
				if !found && !s.isFlag(args[state.pos+1]) {
					boolVal = args[state.pos+1]
					state.skip = state.pos + 1
				}
			}
			s.options[currentArg] = boolVal
			err := s.setBoundVariable(boolVal, currentArg)
			if err != nil {
				s.addError(fmt.Errorf(
					"could not process input argument '%s' - the following error occurred: %s", currentArg, err))
			}
		}
	case Single, Chained:
		s.processSingleOrChainedFlag(args, argument, state, currentArg)
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
		cmdArg.path = cmdArg.Name
	}

	if _, found := s.registeredCommands[cmdArg.path]; found {
		return false, fmt.Errorf("duplicate command '%s' already exists", cmdArg.Name)
	}

	for i := 0; i < len(cmdArg.Subcommands); i++ {
		cmdArg.Subcommands[i].path = cmdArg.path + " " + cmdArg.Subcommands[i].Name
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
		s.acceptedFlags = orderedmap.NewOrderedMap[string, *Argument]()
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
		s.registeredCommands = map[string]Command{}
	}
	if s.commandOptions == nil {
		s.commandOptions = map[string]path{}
	}
	if s.positionalArgs == nil {
		s.positionalArgs = []PositionalArgument{}
	}
	if s.rawArgs == nil {
		s.rawArgs = map[string]bool{}
	}
	if s.callbackQueue == nil {
		s.callbackQueue = deque.New()
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

func (s *CmdLineOption) setPositionalArguments(args []string) {
	var positional []PositionalArgument
	for i, seen := range args {
		seen = s.flagOrShortFlag(strings.TrimLeftFunc(seen, s.prefixFunc))
		if _, found := s.rawArgs[seen]; !found {
			positional = append(positional, PositionalArgument{i, seen})
		}
	}

	s.rawArgs = map[string]bool{}
	s.positionalArgs = positional
}

func (s *CmdLineOption) flagOrShortFlag(flag string) string {
	_, found := s.acceptedFlags.Get(flag)
	if !found {
		item, found := s.lookup[flag]
		if found {
			return item
		}
	}

	return flag
}

func (s *CmdLineOption) isFlag(flag string) bool {
	return strings.HasPrefix(flag, "/") || strings.HasPrefix(flag, "-")
}

func (s *CmdLineOption) addError(err error) {
	s.errors = append(s.errors, err)
}

func (s *CmdLineOption) getCommand(name string) (Command, bool) {
	cmd, found := s.registeredCommands[name]

	return cmd, found
}

func (s *CmdLineOption) registerSecureValue(flag, value string) error {
	var err error
	s.rawArgs[flag] = true
	if value != "" {
		s.options[flag] = value
		err = s.setBoundVariable(value, flag)
	}

	return err
}

func (s *CmdLineOption) registerFlagValue(flag, value, rawValue string) {
	s.rawArgs[flag] = true
	if rawValue != "" {
		s.rawArgs[rawValue] = true
	}
	s.options[flag] = value
}

func (s *CmdLineOption) registerCommandValue(cmd *Command, name, value, rawValue string) {
	s.rawArgs[name] = true
	if rawValue != "" {
		s.rawArgs[rawValue] = true
	}

	if cmd.path == "" {
		return
	}

	s.commandOptions[cmd.path] = path{value: value, isTerminating: len(cmd.Subcommands) == 0}
}

func (s *CmdLineOption) queueSecureArgument(name string, argument *Argument) {
	if s.secureArguments == nil {
		s.secureArguments = orderedmap.NewOrderedMap[string, *Secure]()
	}

	s.secureArguments.Set(name, &argument.Secure)
}

func (s *CmdLineOption) parseCommand(args []string, state *parseState, cmdQueue *deque.Deque) {
	currentArg := args[state.pos]
	var next string

	if state.pos < state.endOf-1 {
		next = args[state.pos+1]
	}

	if cmdQueue.Len() > 0 {
		if !s.checkSubCommands(cmdQueue, currentArg, next, state) {
			return
		}
	}

	if cmd, found := s.getCommand(currentArg); found {
		s.registerCommandValue(&cmd, currentArg, next, next)
		if len(cmd.Subcommands) == 0 {
			s.checkCommandValue(cmd, currentArg, args, state)
		} else {
			cmdQueue.PushBack(cmd)
		}
	} else if state.pos == 0 && !s.isFlag(currentArg) {
		s.addError(fmt.Errorf("options should be prefixed by either '-' or '/'"))
	}
}

func (s *CmdLineOption) processSingleOrChainedFlag(args []string, argument *Argument, state *parseState, currentArg string) {
	if argument.Secure.IsSecure {
		if state.pos < state.endOf-1 {
			if !s.isFlag(args[state.pos+1]) {
				state.skip = state.pos + 1
			}
		}
		s.queueSecureArgument(currentArg, argument)
	} else {
		var next string
		if state.pos < state.endOf-1 {
			next = args[state.pos+1]
		}
		if (len(next) == 0 || s.isFlag(next)) && len(argument.DefaultValue) > 0 {
			next = argument.DefaultValue
		} else {
			state.skip = state.pos + 1
		}
		if state.pos >= state.endOf-1 && len(next) == 0 {
			s.addError(fmt.Errorf("flag '%s' expects a value", currentArg))
		} else {
			if err := s.processValueFlag(currentArg, next, argument); err != nil {
				s.addError(fmt.Errorf("failed to process your input for Flag '%s': %s", currentArg, err))
			}
		}
	}
}

func (s *CmdLineOption) checkCommandValue(cmd Command, currentArg string, args []string, state *parseState) {
	var next string
	if state.pos < state.endOf-1 {
		next = args[state.pos+1]
	}
	if (len(next) == 0 || s.isFlag(next)) && len(cmd.DefaultValue) > 0 {
		next = cmd.DefaultValue
	} else {
		state.skip = state.pos + 1
	}
	if state.pos >= state.endOf-1 && len(next) == 0 {
		s.addError(fmt.Errorf("command '%s' expects a value", currentArg))
	}
	if cmd.Callback != nil {
		s.callbackQueue.PushBack(commandCallback{
			callback:  cmd.Callback,
			arguments: []interface{}{s, &cmd, next},
		})
	}
}

func (s *CmdLineOption) checkSubCommands(cmdQueue *deque.Deque, currentArg string, next string, state *parseState) bool {
	found := false
	var sub Command
	if cmdQueue.Len() == 0 {
		return false
	}
	currentCmd, _ := cmdQueue.PopFront()
	for _, sub = range currentCmd.(Command).Subcommands {
		if strings.EqualFold(sub.Name, currentArg) {
			found = true
			break
		}
	}
	if found {
		if len(sub.Subcommands) == 0 {
			if (len(next) == 0 || s.isFlag(next)) && len(sub.DefaultValue) > 0 {
				next = sub.DefaultValue
			} else {
				state.skip = state.pos + 1
			}
			if state.pos >= state.endOf-1 && len(next) == 0 {
				s.addError(fmt.Errorf("command '%s' expects a value", currentArg))
				return false
			} else if s.isFlag(next) {
				s.addError(fmt.Errorf("command '%s' expects a value but we received a flag '%s'", currentArg, next))
				return false
			}
		}

		s.registerCommandValue(&sub, currentArg, next, next)
		cmdQueue.PushBack(sub)
		if sub.Callback != nil {
			s.callbackQueue.PushBack(commandCallback{
				callback:  sub.Callback,
				arguments: []interface{}{&sub, next},
			})
		}
	} else if cmdQueue.Len() > 0 {
		test, _ := cmdQueue.PopFront()
		if len(test.(Command).Subcommands) > 0 {
			s.addError(fmt.Errorf("command %s expects one of the following commands %v",
				test.(Command).Name, test.(Command).Subcommands))
		} else {
			cmdQueue.PushFront(test)
		}
	}

	return true
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
		} else {
			processed = next
		}
		s.registerFlagValue(currentArg, processed, next)
	}

	return s.setBoundVariable(processed, currentArg)
}

func (s *CmdLineOption) processSecureFlag(name string, config *Secure) {
	var prompt string
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
		if f.Value.RequiredIf != nil {
			if required, msg := f.Value.RequiredIf(s, *f.Key); required {
				s.addError(errors.New(msg))
			}
			continue
		}
		if !f.Value.Required {
			if s.HasFlag(*f.Key) && f.Value.TypeOf == Standalone {
				s.validateStandaloneFlag(*f.Key)
			}
			continue
		}

		mainKey := s.flagOrShortFlag(*f.Key)
		if _, found := s.options[mainKey]; found {
			if f.Value.TypeOf == Standalone {
				s.validateStandaloneFlag(mainKey)
			}
			continue
		} else if f.Value.Secure.IsSecure {
			s.queueSecureArgument(mainKey, f.Value)
			continue
		}

		dependsLen := len(f.Value.DependsOn)
		if dependsLen == 0 {
			s.addError(fmt.Errorf("flag '%s' is mandatory but missing from the command line", *f.Key))
		} else {
			s.validateDependencies(f.Value, mainKey)
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
	stack := deque.New()
	for _, cmd := range s.registeredCommands {
		stack.PushBack(cmd)
	}
	for stack.Len() > 0 {
		current, _ := stack.PopBack()
		cmd := current.(Command)
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
			if _, found := s.commandOptions[sub.path]; found {
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
				stack.PushFront(sub)
			}
		}
	}
}

func (s *CmdLineOption) validateDependencies(arg *Argument, mainKey string) {
	for _, depends := range arg.DependsOn {
		dependKey, found := s.options[depends]
		if found {
			continue
		}

		argument, found := s.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}

		for _, k := range argument.OfValue {
			if strings.EqualFold(dependKey, k) {
				s.addError(fmt.Errorf("flag '%s' is mandatory but missing from the command line", k))
			}
		}
	}
}

func (s *CmdLineOption) setBoundVariable(value string, currentArg string) error {
	data, found := s.bind[currentArg]
	if !found {
		return nil
	}

	accepted, _ := s.acceptedFlags.Get(currentArg)
	if value == "" {
		value = accepted.DefaultValue
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

func insert[T any](a []T, c T, i int) []T {
	var v T
	a = append(a, v)
	return append(a[:i], append([]T{c}, a[i:len(a)-1]...)...)
}
