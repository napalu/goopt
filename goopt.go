// Copyright 2021, Florent Heyworth. All rights reserved.
// Use of this source code is governed by the MIT licensee
// which can be found in the LICENSE file.

// Package goopt provides support for command-line processing.
//
// It supports 3 types of flags:
//   Single - a flag which expects a value
//   Chained - flag which expects a delimited value representing elements in a list (and is evaluated as a list)
//   Standalone - a boolean switch which takes no value
//
// Additionally, commands and sub-commands (Command) are supported. Commands can be nested to represent sub-commands. Unlike
// the official go.Flag package commands and sub-commands may be placed before, after or mixed in with flags.
//
package goopt

import (
	"errors"
	"fmt"
	"github.com/ef-ds/deque"
	"github.com/google/shlex"
	"github.com/wk8/go-ordered-map"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// NewCmdLineOption convenience initialization method. Does not support fluent configuration. Use NewCmdLine to
// configure CmdLineOption fluently.
func NewCmdLineOption() *CmdLineOption {
	return &CmdLineOption{
		acceptedFlags:      orderedmap.New(),
		lookup:             map[string]string{},
		options:            map[string]string{},
		errors:             []string{},
		bind:               make(map[string]interface{}, 1),
		customBind:         map[string]ValueSetFunc{},
		registeredCommands: map[string]Command{},
		commandOptions:     map[string]path{},
		positionalArgs:     []PositionalArgument{},
		listFunc:           matchChainedSeparators,
		callbackQueue:      deque.New(),
		callbackResults:    map[string]error{},
		secureArguments:    map[string]Secure{},
		prefixes:           []rune{'-', '/'},
	}
}

// NewArgument convenience initialization method to describe Flags. Does not support fluent configuration. Use NewArg to
// configure Argument fluently.
func NewArgument(shortFlag string, description string, typeOf OptionType, required bool, secure Secure, defaultValue string) *Argument {
	return &Argument{
		Description:  description,
		TypeOf:       typeOf,
		Required:     required,
		DependsOn:    []string{},
		OfValue:      []string{},
		Secure:       secure,
		Short:        shortFlag,
		DefaultValue: defaultValue,
	}
}

// ExecuteCommands command callbacks are placed on a FIFO queue during parsing until ExecuteCommands is called.
// Returns the count of errors encountered during execution.
func (s *CmdLineOption) ExecuteCommands() int {
	callbackErrors := 0
	for s.callbackQueue.Len() > 0 {

		ele, _ := s.callbackQueue.PopFront()
		call := ele.(commandCallback)
		if call.callback != nil && len(call.arguments) == 2 {
			cmd, cmdOk := call.arguments[0].(*Command)
			arg, argOk := call.arguments[1].(string)
			if cmdOk && argOk {
				err := call.callback(s, cmd, arg)
				s.callbackResults[cmd.Name] = err
				if err != nil {
					callbackErrors++
				}
			}
		}
	}

	return callbackErrors
}

// GetCommandExecutionError returns the error which occurred during execution of a command callback
// after ExecuteCommands has been called. Returns nil on no error. Returns a CommandNotFound error when
// no callback is associated with commandName
func (s *CmdLineOption) GetCommandExecutionError(commandName string) error {
	if err, found := s.callbackResults[commandName]; found {
		return err
	}

	return &CommandNotFoundError{msg: fmt.Sprintf("%s was not found or has no associated callback", commandName)}
}

// AddFlagFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
func (s *CmdLineOption) AddFlagFilter(flag string, proc FilterFunc) error {
	mainKey := s.flagOrShortFlag(flag)
	if arg, found := s.acceptedFlags.Get(mainKey); found {
		arg.(*Argument).Filter = proc

		return nil
	}

	return fmt.Errorf("flag '%s' was not found", flag)
}

// HasFilter returns true when an option has a transform/evaluate function which is called on Parse
func (s *CmdLineOption) HasFilter(flag string) bool {
	mainKey := s.flagOrShortFlag(flag)
	if arg, found := s.acceptedFlags.Get(mainKey); found {
		return arg.(*Argument).Filter != nil
	}

	return false
}

// GetFilter retrieve Flag transform/evaluate function
func (s *CmdLineOption) GetFilter(flag string) (FilterFunc, error) {
	mainKey := s.flagOrShortFlag(flag)
	if arg, found := s.acceptedFlags.Get(mainKey); found {
		if arg.(*Argument).Filter != nil {
			return arg.(*Argument).Filter, nil
		}
	}

	return nil, errors.New("no filters for flag " + flag)
}

// HasAcceptedValues returns true when a Flag defines a set of valid values it will accept
func (s *CmdLineOption) HasAcceptedValues(flag string) bool {
	arg, found := s.acceptedFlags.Get(s.flagOrShortFlag(flag))
	if found {
		return len(arg.(*Argument).AcceptedValues) > 0
	}

	return !found
}

// AddCommand used to define a Command/sub-command chain
// Unlike a flag which starts with a '-' or '/' a Command represents a verb or action
func (s *CmdLineOption) AddCommand(cmdArg *Command) error {
	_, err := s.validateCommand(cmdArg, 0, 100)
	if err != nil {
		return err
	}

	s.registeredCommands[cmdArg.Name] = *cmdArg

	return nil
}

// Parse this function should be called on os.Args (or a user-defined array of arguments). Returns true when
// user command line arguments match the defined Flag and Command rules
func (s *CmdLineOption) Parse(args []string) bool {
	s.ensureInit()
	pruneExecPathFromArgs(&args)

	state := &parseState{
		endOf: len(args),
		skip:  -1,
		args:  args,
	}
	var cmdQueue deque.Deque
	for state.pos = 0; state.pos < state.endOf; state.pos++ {
		if state.skip == state.pos {
			continue
		}

		if s.isFlag(args[state.pos]) {
			s.parseFlag(args, state)
		} else {
			s.parseCommand(args, state, &cmdQueue)
		}
	}

	s.validateProcessedOptions()
	s.setPositionalArguments(args)

	success := len(s.errors) == 0
	if success {
		for key, secure := range s.secureArguments {
			s.processSecureFlag(key, secure)
		}
	}
	s.secureArguments = nil

	return success
}

// ParseString calls Parse
func (s *CmdLineOption) ParseString(argString string) bool {
	args, err := shlex.Split(argString)
	if err != nil {
		return false
	}

	return s.Parse(args)
}

// ParseWithDefaults calls Parse supplementing missing arguments in args array with default values from defaults
func (s *CmdLineOption) ParseWithDefaults(defaults map[string]string, args []string) bool {
	argLen := len(args)
	argMap := make(map[string]string, argLen)

	for i := 0; i < argLen; i++ {
		if s.isFlag(args[i]) {
			arg := strings.TrimLeftFunc(args[i], s.prefixFunc)
			if flag, found := s.acceptedFlags.Get(arg); found &&
				(flag.(*Argument).TypeOf != Standalone || !flag.(*Argument).Secure.IsSecure) {
				if i < argLen-1 {
					argMap[arg] = args[i+1]
				}
			}
		}
	}

	for key, val := range defaults {
		if _, found := argMap[key]; !found {
			args = append(args, string(s.prefixes[0])+key)
			args = append(args, val)
		}
	}

	return s.Parse(args)
}

// ParseStringWithDefaults calls Parse supplementing missing arguments in argString with default values from defaults
func (s *CmdLineOption) ParseStringWithDefaults(defaults map[string]string, argString string) bool {
	args, err := shlex.Split(argString)
	if err != nil {
		return false
	}

	return s.ParseWithDefaults(defaults, args)
}

func (s *CmdLineOption) SetPosix(posixCompatible bool) bool {
	oldValue := s.posixCompatible

	s.posixCompatible = posixCompatible

	return oldValue
}

// GetOrDefault returns the value of a defined Flag or defaultValue if no value is set
func (s *CmdLineOption) GetOrDefault(flag string, defaultValue string) string {
	value, found := s.Get(flag)
	if found {
		return value
	}

	return defaultValue
}

// GetPositionalArgs TODO explain
func (s *CmdLineOption) GetPositionalArgs() []PositionalArgument {
	return s.positionalArgs
}

// GetPositionalArgCount TODO explain
func (s *CmdLineOption) GetPositionalArgCount() int {
	return len(s.positionalArgs)
}

// HasPositionalArgs TODO explain
func (s *CmdLineOption) HasPositionalArgs() bool {
	return s.GetPositionalArgCount() > 0
}

// GetCommandValue returns the value of a command path if found.
// Example:
//   in the structure Command{Name : "Test", Subcommands: []Command{{Name: "User"}}}
//   the path to User would be expressed as "Test User"
func (s *CmdLineOption) GetCommandValue(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("paths is empty")
	}

	entry, found := s.commandOptions[path]
	if !found {
		return "", fmt.Errorf("not found. no commands stored under path")
	}

	return entry.value, nil
}

// GetCommandValues returns the list of all commands seen on command-line
func (s *CmdLineOption) GetCommandValues() []PathValue {
	pathValues := make([]PathValue, 0, len(s.commandOptions))
	for k, v := range s.commandOptions {
		if v.isTerminating {
			pathValues = append(pathValues, PathValue{
				Path:  k,
				Value: v.value,
			})
		}
	}

	return pathValues
}

// Get returns a combination of a Flag's value as string and true if found. Returns an empty string and false otherwise
func (s *CmdLineOption) Get(flag string) (string, bool) {
	mainKey := s.flagOrShortFlag(flag)
	value, found := s.options[mainKey]
	if found {
		accepted, _ := s.acceptedFlags.Get(mainKey)
		if accepted.(*Argument).Secure.IsSecure {
			s.options[mainKey] = ""
		}
	}

	return value, found
}

// GetBool attempts to convert the string value of a Flag to a boolean.
func (s *CmdLineOption) GetBool(flag string) (bool, error) {
	value, success := s.Get(flag)
	if !success {
		return false, fmt.Errorf("no option with flag '%s' exists", flag)
	}

	val, err := strconv.ParseBool(value)

	return val, err
}

// GetInt attempts to convert the string value of a Flag to an int64.
func (s *CmdLineOption) GetInt(flag string, bitSize int) (int64, error) {
	value, success := s.Get(flag)
	if !success {
		return 0, fmt.Errorf("no option with flag '%s' exists", flag)
	}

	val, err := strconv.ParseInt(value, 0, bitSize)

	return val, err
}

// GetFloat attempts to convert the string value of a Flag to a float64
func (s *CmdLineOption) GetFloat(flag string, bitSize int) (float64, error) {
	value, success := s.Get(flag)
	if !success {
		return 0, fmt.Errorf("no option with flag '%s' exists", flag)
	}

	val, err := strconv.ParseFloat(value, bitSize)

	return val, err
}

// GetList attempts to split the string value of a Chained Flag to a string slice
// by default the value is split on '|', ',' or ' ' delimiters
func (s *CmdLineOption) GetList(flag string) ([]string, error) {
	arg, found := s.acceptedFlags.Get(flag)
	notFound := fmt.Errorf("failed to retrieve value for flag '%s'", flag)
	listDelimFunc := s.getListDelimiterFunc()
	if found {
		if arg.(*Argument).TypeOf == Chained {
			value, success := s.Get(flag)
			if !success {
				return []string{}, notFound
			}

			return strings.FieldsFunc(value, listDelimFunc), nil
		}

		return []string{}, fmt.Errorf("invalid Argument type for flag '%s' - use typeOf = Chained instead", flag)
	}

	return []string{}, notFound
}

// SetListDelimiterFunc TODO explain
func (s *CmdLineOption) SetListDelimiterFunc(delimiterFunc ListDelimiterFunc) error {
	if delimiterFunc != nil {
		s.listFunc = delimiterFunc

		return nil
	}

	return fmt.Errorf("invalid ListDelimiterFunc (should not be null)")
}

func (s *CmdLineOption) SetArgumentPrefixes(prefixes []rune) error {
	prefixesLen := len(prefixes)
	if prefixesLen == 0 {
		return fmt.Errorf("can't parse with empty argument prefix list")
	}

	s.prefixes = prefixes

	return nil
}

// GetConsistencyWarnings is a helper function which provides information about eventual option consistency warnings.
// It is intended for users of the library rather than for end-users
func (s *CmdLineOption) GetConsistencyWarnings() []string {
	var configWarnings []string
	for opt := range s.options {
		mainKey := s.flagOrShortFlag(opt)
		arg, found := s.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}
		argument := arg.(*Argument)
		if argument.TypeOf == Standalone && argument.DefaultValue != "" {
			configWarnings = append(configWarnings,
				fmt.Sprintf("Flag '%s' is a Standalone (boolean) flag and has a default value specified. "+
					"The default value is ignored.", mainKey))
		}
	}

	return configWarnings
}

// GetWarnings returns a string slice of all warnings (non-fatal errors) - a warning is set when optional dependencies
// are not met - for instance, specifying the value of a Flag which relies on a missing argument
func (s *CmdLineOption) GetWarnings() []string {
	var warnings []string
	for opt := range s.options {
		mainKey := s.flagOrShortFlag(opt)
		arg, found := s.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}
		argument := arg.(*Argument)
		if len(argument.DependsOn) == 0 {
			continue
		}
		for _, k := range argument.DependsOn {
			dependKey := s.flagOrShortFlag(k)
			_, hasKey := s.options[dependKey]
			if !hasKey {
				warnings = append(warnings,
					fmt.Sprintf("Flag '%s' depends on '%s' which was not specified.", mainKey, k))
			}

			if len(argument.OfValue) == 0 {
				continue
			}
			dependValue, found := s.Get(dependKey)
			if !found {
				continue
			}

			valueFound := false
			for i := 0; i < len(argument.OfValue); i++ {
				if strings.EqualFold(argument.OfValue[i], dependValue) {
					valueFound = true
					break
				}
			}

			if !valueFound {
				warnings = append(warnings, fmt.Sprintf(
					"Flag '%s' depends on '%s' with value %s which was not specified. (got '%s')",
					mainKey, dependKey, showDependencies(argument.OfValue), dependValue))
			}
		}
	}

	return warnings
}

// GetOptions returns a slice of KeyValue pairs which have been supplied on the command-line.
func (s *CmdLineOption) GetOptions() []KeyValue {
	keyValues := make([]KeyValue, len(s.options))
	i := 0
	for key, value := range s.options {
		keyValues[i].Key = key
		keyValues[i].Value = value
		i++
	}

	return keyValues
}

// AddFlag used to define a Flag - a Flag represents a command line option as a "long" and optional "short" form
// which is prefixed by '-', '--' or '/'.
func (s *CmdLineOption) AddFlag(flag string, argument *Argument) error {
	argument.ensureInit()

	if flag == "" {
		return fmt.Errorf("can't set empty flag")
	}

	if len(argument.Short) > 0 {
		s.lookup[argument.Short] = flag
	}
	s.acceptedFlags.Set(flag, argument)

	return nil
}

// BindFlag is used to bind a *pointer* to a string, int, uint, bool, float or time.Time variable with a Flag
// which is set when Parse is invoked.
func (s *CmdLineOption) BindFlag(data interface{}, flag string, argument *Argument) error {
	if ok, err := canConvert(data, argument.TypeOf); !ok {
		return err
	}

	if err := s.AddFlag(flag, argument); err != nil {
		return err
	}

	s.bind[flag] = data

	return nil
}

// CustomBindFlag works like BindFlag but expects a ValueSetFunc callback which is called when a Flag is evaluated on Parse.
// When the Flag is seen on the command like the ValueSetFunc is called with the user-supplied value. Allows binding
// complex structures not supported by BindFlag
func (s *CmdLineOption) CustomBindFlag(data interface{}, proc ValueSetFunc, flag string, argument *Argument) error {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return fmt.Errorf("we expect a pointer to a variable")
	}

	if !reflect.ValueOf(data).Elem().IsValid() {
		return fmt.Errorf("can't bind to invalid value field")
	}

	if err := s.AddFlag(flag, argument); err != nil {
		return err
	}

	s.bind[flag] = data
	s.customBind[flag] = proc

	return nil
}

// AcceptValue is used to define an acceptable value for a Flag. The 'pattern' argument is compiled to a regular expression
// and the description argument is used to provide a human-readable description of the pattern.
// Returns an error if the regular expression cannot be compiled or if the Flag does not support values (Standalone).
// Example:
//  	a Flag which accepts only whole numbers could be defined as:
//   	AcceptValue("times", `^[\d]+`, "Please supply a whole number").
func (s *CmdLineOption) AcceptValue(flag string, pattern string, description string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	mainKey := s.flagOrShortFlag(flag)
	accepted, found := s.acceptedFlags.Get(mainKey)
	if !found {
		return fmt.Errorf("option with flag %s was not set", flag)
	}
	arg := accepted.(*Argument)
	if arg.TypeOf == Standalone {
		return fmt.Errorf("option with flag %s does not accept a value (Standalone)", flag)
	}

	if arg.AcceptedValues == nil {
		arg.AcceptedValues = make([]LiterateRegex, 1, 5)
		arg.AcceptedValues[0] = LiterateRegex{
			value:   re,
			explain: description,
		}
	} else {
		arg.AcceptedValues = append(arg.AcceptedValues, LiterateRegex{
			value:   re,
			explain: description,
		})
	}

	return nil
}

// AcceptValues same as AcceptValue but acts on a list of patterns and descriptions. When specified, the patterns defined
// in AcceptValues represent a set of values, of which one must be supplied on the command-line. The patterns are evaluated
// on Parse, if no command-line options match one of the AcceptValues, Parse returns false.
func (s *CmdLineOption) AcceptValues(flag string, patterns, descriptions []string) error {
	lenDesc := len(descriptions)
	var desc = ""

	for i, pattern := range patterns {
		if i < lenDesc {
			desc = descriptions[i]
		}
		if err := s.AcceptValue(flag, pattern, desc); err != nil {
			return err
		}
	}

	return nil
}

func (s *CmdLineOption) GetShortFlag(flag string) (string, error) {
	mainKey := s.flagOrShortFlag(flag)
	argument, found := s.acceptedFlags.Get(mainKey)
	if found {
		if argument.(*Argument).Short != "" {
			return argument.(*Argument).Short, nil
		}

		return "", fmt.Errorf("flag %s has no short flag defined", flag)
	}

	return "", fmt.Errorf("flag %s was not found", flag)
}

// HasFlag returns true when the Flag has been seen on the command line.
func (s *CmdLineOption) HasFlag(flag string) bool {
	mainKey := s.flagOrShortFlag(flag)
	_, found := s.options[mainKey]

	return found
}

// HasCommand return true when the name has been seen on the command line.
func (s *CmdLineOption) HasCommand(path string) bool {
	_, found := s.commandOptions[path]

	return found
}

// ClearAll clears all parsed options and  commands as well as filters and acceptedValues (guards).
// Configured flags and registered commands are not cleared. Use this when parsing a command line
// repetitively.
func (s *CmdLineOption) ClearAll() {
	s.Clear(ClearConfig{})
}

// Clear can be used to selectively clear sensitive options or when re-defining options on the fly.
func (s *CmdLineOption) Clear(config ClearConfig) {
	if !config.KeepOptions {
		s.options = nil
	}
	if !config.KeepErrors {
		s.errors = s.errors[:0]
	}
	if !config.KeepCommands {
		s.commandOptions = nil
	}
	if !config.KeepPositional {
		s.positionalArgs = nil
	}
}

// DescribeFlag is used to provide a description of a Flag
func (s *CmdLineOption) DescribeFlag(flag, description string) error {
	mainKey := s.flagOrShortFlag(flag)
	accepted, found := s.acceptedFlags.Get(mainKey)
	if found {
		accepted.(*Argument).Description = description

		return nil
	}

	return fmt.Errorf("flag '%s' was not found", flag)
}

// GetDescription retrieves a Flag's description as set by DescribeFlag
func (s *CmdLineOption) GetDescription(flag string) string {
	mainKey := s.flagOrShortFlag(flag)
	accepted, found := s.acceptedFlags.Get(mainKey)
	if found {
		return accepted.(*Argument).Description
	}

	return ""
}

// SetFlag is used to re-define a Flag or define a new Flag at runtime. This can be sometimes useful for dynamic
// evaluation of combinations of options and values which can't be expressed statically. For instance, when the user
// should supply these during a program's execution but after command-line options have been parsed.
func (s *CmdLineOption) SetFlag(flag, value string) {
	mainKey := s.flagOrShortFlag(flag)
	_, found := s.options[flag]
	if found {
		s.options[mainKey] = value
	} else {
		s.options[flag] = value
	}
}

// Remove used to remove a defined-flag at runtime - returns false if the Flag was not found and true on removal.
func (s *CmdLineOption) Remove(flag string) bool {
	mainKey := s.flagOrShortFlag(flag)
	_, found := s.options[mainKey]
	if found {
		delete(s.options, mainKey)

		return true
	}

	return false
}

// DependsOnFlag same as DependsOnFlagValue but does not specify that the Flag it depends on must have a particular value
// to be valid.
func (s *CmdLineOption) DependsOnFlag(flag, dependsOn string) error {
	if flag == "" {
		return fmt.Errorf("can't set dependency on empty flag")
	}

	mainKey := s.flagOrShortFlag(flag)
	accepted, found := s.acceptedFlags.Get(mainKey)
	if found {
		accepted.(*Argument).DependsOn = append(accepted.(*Argument).DependsOn, dependsOn)

		return nil
	}

	return fmt.Errorf("flag '%s' was not found", flag)
}

// DependsOnFlagValue is used to describe flag dependencies. For example, a '--modify' flag could be specified to
// depend on a '--group' Flag with a value of 'users'. If the '--group' Flag is not specified with a value of
// 'users' on the command line a warning will be set during Parse.
func (s *CmdLineOption) DependsOnFlagValue(flag, dependsOn, ofValue string) error {
	if flag == "" {
		return fmt.Errorf("can't set dependency on empty flag")
	}

	if ofValue == "" {
		return fmt.Errorf("can't set dependency when value is empty")
	}

	mainKey := s.flagOrShortFlag(flag)
	accepted, found := s.acceptedFlags.Get(mainKey)
	if found {
		accepted.(*Argument).DependsOn = append(accepted.(*Argument).DependsOn, dependsOn)
		if len(accepted.(*Argument).OfValue) == 0 {
			accepted.(*Argument).OfValue = make([]string, 1, 5)
			accepted.(*Argument).OfValue[0] = ofValue
		} else {
			accepted.(*Argument).OfValue = append(accepted.(*Argument).OfValue, ofValue)
		}

		return nil
	}

	return fmt.Errorf("flag '%s' was not found", flag)
}

// GetErrors returns a list of the errors encountered during Parse
func (s *CmdLineOption) GetErrors() []string {
	return s.errors
}

// GetErrorCount is greater than zero when errors were encountered during Parse.
func (s *CmdLineOption) GetErrorCount() int {
	return len(s.errors)
}

// PrintUsage pretty prints accepted Flags and Commands to io.Writer.
func (s *CmdLineOption) PrintUsage(writer io.Writer) {
	_, _ = writer.Write([]byte(fmt.Sprintf("usage: %s", []byte(os.Args[0]))))
	s.PrintFlags(writer)
	_, _ = writer.Write([]byte("\ncommands:\n"))
	s.PrintCommands(writer)
}

// PrintFlags pretty prints accepted command-line switches to io.Writer
func (s *CmdLineOption) PrintFlags(writer io.Writer) {
	for pair, i := s.acceptedFlags.Oldest(), 0; pair != nil; pair = pair.Next() {
		shortFlag, err := s.GetShortFlag(pair.Key.(string))
		if err != nil {
			shortFlag = "-" + shortFlag
		}
		requiredOrOptional := "optional"
		if pair.Value.(*Argument).Required {
			requiredOrOptional = "required"
		}
		_, _ = writer.Write([]byte(fmt.Sprintf("\n --%s or -%s \"%s\" (%s)",
			pair.Key, shortFlag, s.GetDescription(pair.Key.(string)), requiredOrOptional)))
		i++
	}
}

// PrintCommands writes the list of accepted Command structs to io.Writer.
func (s *CmdLineOption) PrintCommands(writer io.Writer) {
	s.PrintCommandsUsing(writer, &PrettyPrintConfig{
		NewCommandPrefix: " +",
		DefaultPrefix:    " │",
		TerminalPrefix:   " └",
		LevelBindPrefix:  "─",
	})
}

// PrintCommandsUsing writes the list of accepted Command structs to io.Writer using PrettyPrintConfig.
// PrettyPrintConfig.NewCommandPrefix precedes the start of a new command
// PrettyPrintConfig.DefaultPrefix precedes sub-commands by default
// PrettyPrintConfig.TerminalPrefix precedes terminal, i.e. Command structs which don't have sub-commands
// PrettyPrintConfig.LevelBindPrefix is used for indentation. The indentation is repeated for each Level under the
//  command root. The Command root is at Level 0.
func (s *CmdLineOption) PrintCommandsUsing(writer io.Writer, config *PrettyPrintConfig) {
	for _, cmd := range s.registeredCommands {
		cmd.Visit(func(cmd *Command, level int) bool {
			var start = config.DefaultPrefix
			switch {
			case level == 0:
				start = config.NewCommandPrefix
			case len(cmd.Subcommands) == 0:
				start = config.TerminalPrefix
			}
			command := fmt.Sprintf("%s%s %s \"%s\"\n", start, strings.Repeat(config.LevelBindPrefix, level),
				cmd.Name, cmd.Description)
			if _, err := writer.Write([]byte(command)); err != nil {
				return false
			}
			return true

		}, 0)
	}
}

// Visit traverse a command and its subcommands from top to bottom
func (c *Command) Visit(visitor func(cmd *Command, level int) bool, level int) {
	if visitor != nil {
		if !visitor(c, level) {
			return
		}
	}

	for _, cmd := range c.Subcommands {
		cmd.Visit(visitor, level+1)
	}
}

// Describe a LiterateRegex (regular expression with a human-readable explanation of the pattern)
func (r *LiterateRegex) Describe() string {
	if len(r.explain) > 0 {
		return r.explain
	}

	return r.value.String()
}

func (e *UnsupportedTypeConversionError) Error() string {
	return e.msg
}

func (e *CommandNotFoundError) Error() string {
	return e.msg
}
