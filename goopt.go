// Copyright 2021, Florent Heyworth. All rights reserved.
// Use of this source code is governed by the MIT licensee
// which can be found in the LICENSE file.

// Package goopt provides support for command-line processing.
//
// It supports 3 types of flags:
//
//	Single - a flag which expects a value
//	Chained - flag which expects a delimited value representing elements in a list (and is evaluated as a list)
//	Standalone - a boolean flag which by default takes no value (defaults to true) but may accept a value which evaluates to true or false
//
// Additionally, commands and sub-commands (Command) are supported. Commands can be nested to represent sub-commands. Unlike
// the official go.Flag package commands and sub-commands may be placed before, after or mixed in with flags.
package goopt

import (
	"fmt"
	"github.com/ef-ds/deque"
	"github.com/napalu/goopt/parse"
	"github.com/napalu/goopt/types/orderedmap"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// NewCmdLineOption convenience initialization method. Does not support fluent configuration. Use NewCmdLine to
// configure CmdLineOption fluently.
func NewCmdLineOption() *CmdLineOption {
	return &CmdLineOption{
		acceptedFlags:      orderedmap.NewOrderedMap[string, *FlagInfo](),
		lookup:             map[string]string{},
		options:            map[string]string{},
		errors:             []error{},
		bind:               make(map[string]interface{}, 1),
		customBind:         map[string]ValueSetFunc{},
		registeredCommands: orderedmap.NewOrderedMap[string, Command](),
		commandOptions:     orderedmap.NewOrderedMap[string, bool](),
		positionalArgs:     []PositionalArgument{},
		listFunc:           matchChainedSeparators,
		callbackQueue:      deque.New(),
		callbackResults:    map[string]error{},
		secureArguments:    orderedmap.NewOrderedMap[string, *Secure](),
		prefixes:           []rune{'-', '/'},
	}
}

// NewCmdLineFromStruct parses a struct and binds its fields to command-line flags
func NewCmdLineFromStruct[T any](structWithTags *T) (*CmdLineOption, error) {
	return NewCmdLineFromStructWithLevel(structWithTags, 5)
}

// NewCmdLineFromStructWithLevel parses a struct and binds its fields to command-line flags up to maxDepth levels
func NewCmdLineFromStructWithLevel[T any](structWithTags *T, maxDepth int) (*CmdLineOption, error) {
	return newCmdLineFromReflectValue(reflect.ValueOf(structWithTags), "", maxDepth, 0)
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

// SetEnvFilter allows setting an environment name lookup function
// If set and the environment variable exists, it will be prepended to the args array
func (s *CmdLineOption) SetEnvFilter(env EnvFunc) EnvFunc {
	oldFilter := s.envFilter
	s.envFilter = env

	return oldFilter
}

// ExecuteCommands command callbacks are placed on a FIFO queue during parsing until ExecuteCommands is called.
// Returns the count of errors encountered during execution.
func (s *CmdLineOption) ExecuteCommands() int {
	callbackErrors := 0
	for s.callbackQueue.Len() > 0 {

		ele, _ := s.callbackQueue.PopFront()
		call := ele.(commandCallback)
		if call.callback != nil && len(call.arguments) == 2 {
			cmdLine, cmdLineOk := call.arguments[0].(*CmdLineOption)
			cmd, cmdOk := call.arguments[1].(*Command)
			if cmdLineOk && cmdOk {
				err := call.callback(cmdLine, cmd)
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

	return fmt.Errorf("%w: %s was not found or has no associated callback", ErrCommandNotFound, commandName)
}

// AddFlagPreValidationFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
// *before* AcceptedValues are checked
func (s *CmdLineOption) AddFlagPreValidationFilter(flag string, proc FilterFunc) error {
	mainKey := s.flagOrShortFlag(flag)
	if flagInfo, found := s.acceptedFlags.Get(mainKey); found {
		flagInfo.Argument.PreFilter = proc

		return nil
	}

	return fmt.Errorf("%w: %s", ErrFlagNotFound, flag)
}

// AddFlagPostValidationFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
// *after* AcceptedValues are checked
func (s *CmdLineOption) AddFlagPostValidationFilter(flag string, proc FilterFunc) error {
	mainKey := s.flagOrShortFlag(flag)
	if flagInfo, found := s.acceptedFlags.Get(mainKey); found {
		flagInfo.Argument.PostFilter = proc

		return nil
	}

	return fmt.Errorf("%w: %s", ErrFlagNotFound, flag)
}

// HasPreValidationFilter returns true when an option has a transform/evaluate function which is called on Parse
// before checking for acceptable values
func (s *CmdLineOption) HasPreValidationFilter(flag string) bool {
	mainKey := s.flagOrShortFlag(flag)
	if flagInfo, found := s.acceptedFlags.Get(mainKey); found {
		return flagInfo.Argument.PreFilter != nil
	}

	return false
}

// GetPreValidationFilter retrieve Flag transform/evaluate function which is called on Parse before checking for
// acceptable values
func (s *CmdLineOption) GetPreValidationFilter(flag string) (FilterFunc, error) {
	mainKey := s.flagOrShortFlag(flag)
	if flagInfo, found := s.acceptedFlags.Get(mainKey); found {
		if flagInfo.Argument.PreFilter != nil {
			return flagInfo.Argument.PreFilter, nil
		}
	}

	return nil, fmt.Errorf("%w: no pre-validation filters for flag %s", ErrValidationFailed, flag)
}

// HasPostValidationFilter returns true when an option has a transform/evaluate function which is called on Parse
// after checking for acceptable values
func (s *CmdLineOption) HasPostValidationFilter(flag string) bool {
	mainKey := s.flagOrShortFlag(flag)
	if flagInfo, found := s.acceptedFlags.Get(mainKey); found {
		return flagInfo.Argument.PostFilter != nil
	}

	return false
}

// GetPostValidationFilter retrieve Flag transform/evaluate function which is called on Parse after checking for
// acceptable values
func (s *CmdLineOption) GetPostValidationFilter(flag string) (FilterFunc, error) {
	mainKey := s.flagOrShortFlag(flag)
	if flagInfo, found := s.acceptedFlags.Get(mainKey); found {
		if flagInfo.Argument.PostFilter != nil {
			return flagInfo.Argument.PostFilter, nil
		}
	}

	return nil, fmt.Errorf("%w: no post-validation filters for flag %s", ErrValidationFailed, flag)
}

// HasAcceptedValues returns true when a Flag defines a set of valid values it will accept
func (s *CmdLineOption) HasAcceptedValues(flag string) bool {
	flagInfo, found := s.acceptedFlags.Get(s.flagOrShortFlag(flag))
	if found {
		return len(flagInfo.Argument.AcceptedValues) > 0
	}

	return false
}

// AddCommand used to define a Command/sub-command chain
// Unlike a flag which starts with a '-' or '/' a Command represents a verb or action
func (s *CmdLineOption) AddCommand(cmd *Command) error {
	// Validate the command hierarchy and ensure unique paths
	if ok, err := s.validateCommand(cmd, 0, 100); !ok {
		return err
	}

	// Add the command and all its subcommands to registeredCommands
	s.registerCommandRecursive(cmd)

	return nil
}

// Parse this function should be called on os.Args (or a user-defined array of arguments). Returns true when
// user command line arguments match the defined Flag and Command rules
func (s *CmdLineOption) Parse(args []string) bool {
	s.ensureInit()
	pruneExecPathFromArgs(&args)

	if s.envFilter != nil {
		args = s.envToFlags(args)
	}

	state := &parseState{
		endOf: len(args),
		skip:  -1,
	}

	var cmdQueue deque.Deque
	var commandPathSlice []string
	var currentCommandPath string

	for state.pos = 0; state.pos < state.endOf; state.pos++ {
		if state.skip == state.pos {
			continue
		}

		if s.isFlag(args[state.pos]) {
			if s.posixCompatible {
				args = s.parsePosixFlag(args, state, currentCommandPath)
			} else {
				s.parseFlag(args, state, currentCommandPath)
			}
		} else {
			s.parseCommand(args, state, &cmdQueue, &commandPathSlice)
			currentCommandPath = strings.Join(commandPathSlice, " ")
		}
	}

	s.validateProcessedOptions()
	s.setPositionalArguments(args)

	success := len(s.errors) == 0
	if success {
		for f := s.secureArguments.Front(); f != nil; f = f.Next() {
			s.processSecureFlag(*f.Key, f.Value)
		}
	}
	s.secureArguments = nil

	return success
}

// ParseString calls Parse
func (s *CmdLineOption) ParseString(argString string) bool {
	args, err := parse.Split(argString)
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
			arg := s.flagOrShortFlag(strings.TrimLeftFunc(args[i], s.prefixFunc))
			if flagInfo, found := s.acceptedFlags.Get(arg); found &&
				(flagInfo.Argument.TypeOf != Standalone || !flagInfo.Argument.Secure.IsSecure) {
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
	args, err := parse.Split(argString)
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

// GetCommands returns the list of all commands seen on command-line
func (s *CmdLineOption) GetCommands() []string {
	pathValues := make([]string, 0, s.commandOptions.Count())
	for kv := s.commandOptions.Front(); kv != nil; kv = kv.Next() {
		if kv.Value {
			pathValues = append(pathValues, *kv.Key)
		}
	}

	return pathValues
}

// Get returns a combination of a Flag's value as string and true if found. Returns an empty string and false otherwise
func (s *CmdLineOption) Get(flag string) (string, bool) {
	mainKey := s.flagOrShortFlag(flag)
	value, found := s.options[mainKey]
	if found {
		if flagInfo, ok := s.acceptedFlags.Get(mainKey); ok {
			if flagInfo.Argument.Secure.IsSecure {
				s.options[mainKey] = ""
			}
		}
	}

	return value, found
}

// GetBool attempts to convert the string value of a Flag to boolean.
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

	val, err := strconv.ParseInt(value, 10, bitSize)

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
	arg, err := s.GetArgument(flag)
	listDelimFunc := s.getListDelimiterFunc()
	if err == nil {
		if arg.TypeOf == Chained {
			value, success := s.Get(flag)
			if !success {
				return []string{}, fmt.Errorf("failed to retrieve value for flag '%s'", flag)
			}

			return strings.FieldsFunc(value, listDelimFunc), nil
		}

		return []string{}, fmt.Errorf("invalid Argument type for flag '%s' - use typeOf = Chained instead", flag)
	}

	return []string{}, err
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
		flagInfo, found := s.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}
		if flagInfo.Argument.TypeOf == Standalone && flagInfo.Argument.DefaultValue != "" {
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
		flagInfo, found := s.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}
		if len(flagInfo.Argument.DependsOn) == 0 {
			continue
		}
		for _, k := range flagInfo.Argument.DependsOn {
			dependKey := s.flagOrShortFlag(k)
			_, hasKey := s.options[dependKey]
			if !hasKey {
				warnings = append(warnings,
					fmt.Sprintf("Flag '%s' depends on '%s' which was not specified.", mainKey, k))
			}

			if len(flagInfo.Argument.OfValue) == 0 {
				continue
			}
			dependValue, found := s.Get(dependKey)
			if !found {
				continue
			}

			valueFound := false
			for i := 0; i < len(flagInfo.Argument.OfValue); i++ {
				if strings.EqualFold(flagInfo.Argument.OfValue[i], dependValue) {
					valueFound = true
					break
				}
			}

			if !valueFound {
				warnings = append(warnings, fmt.Sprintf(
					"Flag '%s' depends on '%s' with value %s which was not specified. (got '%s')",
					mainKey, dependKey, showDependencies(flagInfo.Argument.OfValue), dependValue))
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

// AddFlag is used to define a Flag. A Flag represents a command line option
// with a "long" name and an optional "short" form prefixed by '-', '--' or '/'.
// This version supports both global flags and command-specific flags using the optional commandPath argument.
func (s *CmdLineOption) AddFlag(flag string, argument *Argument, commandPath ...string) error {
	argument.ensureInit()

	if flag == "" {
		return fmt.Errorf("can't set empty flag")
	}

	// Use the helper function to generate the lookup key
	lookupFlag := buildLookupFlag(flag, commandPath...)

	// Ensure no duplicate flags for the same command path or globally
	if _, exists := s.acceptedFlags.Get(lookupFlag); exists {
		return fmt.Errorf("flag '%s' already exists for the given command Path", lookupFlag)
	}

	// Handle short flag validation
	if lenS := len(argument.Short); lenS > 0 {
		if s.posixCompatible && lenS > 1 {
			return fmt.Errorf("%w: flag %s has short form %s which is not posix compatible (length > 1)", ErrPosixIncompatible, flag, argument.Short)
		}
		if arg, exists := s.lookup[argument.Short]; exists {
			return fmt.Errorf("short flag '%s' on flag %s already exists as %v", argument.Short, flag, arg)
		}
		s.lookup[argument.Short] = flag
	}

	// Store the flag in acceptedFlags
	s.acceptedFlags.Set(lookupFlag, &FlagInfo{
		Argument:    argument,
		CommandPath: strings.Join(commandPath, " "),
	})

	return nil
}

// AddFlagToCommand maps a flag to a specific command path
/*func (s *CmdLineOption) AddFlagToCommand(commandPath string, flag string, argument *Argument, varPtr interface{}) error {
	// Split the command path into individual command names
	pathParts := strings.Split(commandPath, " ")

	// Navigate the command hierarchy
	currentCmd := s.getOrCreateCommand(pathParts[0])
	for _, part := range pathParts[1:] {
		currentCmd = currentCmd.getOrCreateSubcommand(part)
	}

	// Add the flag to the final command in the hierarchy
	currentCmd.addFlag(argument)

	// Ensure we bind the flag to the variable pointer for command-specific parsing
	if varPtr != nil {
		if err := s.BindFlag(varPtr, flag, argument); err != nil {
			return fmt.Errorf("error binding flag '%s' to variable: %w", flag, err)
		}
	}

	// Also, add the flag to the commandFlags map for quick lookup
	if s.commandFlags == nil {
		s.commandFlags = make(map[string][]*Argument)
	}
	s.commandFlags[commandPath] = append(s.commandFlags[commandPath], argument)

	return nil
}*/

// BindFlagToCmdLine is a helper function to allow passing generics to the CmdLineOption.BindFlag method
func BindFlagToCmdLine[T Bindable](s *CmdLineOption, data *T, flag string, argument *Argument, commandPath ...string) error {
	if s == nil {
		return fmt.Errorf("can't bind flag to nil CmdLineOption pointer")
	}

	return s.BindFlag(data, flag, argument, commandPath...)
}

// CustomBindFlagToCmdLine is a helper function to allow passing generics to the CmdLineOption.CustomBindFlag method
func CustomBindFlagToCmdLine[T any](s *CmdLineOption, data *T, proc ValueSetFunc, flag string, argument *Argument, commandPath ...string) error {
	if s == nil {
		return fmt.Errorf("can't bind flag to nil CmdLineOption pointer")
	}

	return s.CustomBindFlag(data, proc, flag, argument, commandPath...)
}

// BindFlag is used to bind a *pointer* to string, int, uint, bool, float or time.Time scalar or slice variable with a Flag
// which is set when Parse is invoked.
// An error is returned if data cannot be bound - for compile-time safety use BindFlagToCmdLine instead
func (s *CmdLineOption) BindFlag(bindPtr interface{}, flag string, argument *Argument, commandPath ...string) error {
	if bindPtr == nil {
		return fmt.Errorf("can't bind flag to nil CmdLineOption pointer")
	}
	if ok, err := canConvert(bindPtr, argument.TypeOf); !ok {
		return err
	}

	if reflect.ValueOf(bindPtr).Kind() != reflect.Ptr {
		return fmt.Errorf("BindFlag only accepts pointer types")
	}

	lookupFlag := buildLookupFlag(flag, commandPath...)

	if err := s.AddFlag(lookupFlag, argument); err != nil {
		return err
	}
	// Bind the flag to the variable
	s.bind[lookupFlag] = bindPtr

	return nil
}

/*func (s *CmdLineOption) BindFlag(bindPtr interface{}, flag string, argument *Argument, commandPath ...string) error {
	if bindPtr == nil {
		return fmt.Errorf("can't bind flag to nil CmdLineOption pointer")
	}
	if ok, err := canConvert(bindPtr, argument.TypeOf); !ok {
		return err
	}

	// Determine if the flag is being bound to a specific command path or globally
	var actualCommandPath string
	if len(commandPath) > 0 {
		actualCommandPath = commandPath[0]
	}

	// Handle the case for shared flags
	if _, found := s.sharedFlags.Get(flag); found {
		if s.sharedBind == nil {
			s.sharedBind = make(map[string]map[string]interface{})
		}
		// Check if there is already a map for this flag
		if _, exists := s.sharedBind[flag]; !exists {
			s.sharedBind[flag] = make(map[string]interface{})
		}
		// Add the binding for this specific command path
		s.sharedBind[flag][actualCommandPath] = bindPtr
	} else {
		// Regular flag binding (either global or for a single command path)
		if actualCommandPath != "" {
			s.bind[flag+"@"+actualCommandPath] = bindPtr
		} else {
			s.bind[flag] = bindPtr
		}
	}

	return nil
}*/

// CustomBindFlag works like BindFlag but expects a ValueSetFunc callback which is called when a Flag is evaluated on Parse.
// When the Flag is seen on the command like the ValueSetFunc is called with the user-supplied value. Allows binding
// complex structures not supported by BindFlag
func (s *CmdLineOption) CustomBindFlag(data any, proc ValueSetFunc, flag string, argument *Argument, commandPath ...string) error {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return fmt.Errorf("we expect a pointer to a variable")
	}

	if !reflect.ValueOf(data).Elem().IsValid() {
		return fmt.Errorf("can't bind to invalid value field")
	}

	if err := s.AddFlag(flag, argument, commandPath...); err != nil {
		return err
	}

	lookupFlag := buildLookupFlag(flag, commandPath...)

	s.bind[lookupFlag] = data
	s.customBind[lookupFlag] = proc

	return nil
}

// AcceptPattern is used to define an acceptable value for a Flag. The 'pattern' argument is compiled to a regular expression
// and the description argument is used to provide a human-readable description of the pattern.
// Returns an error if the regular expression cannot be compiled or if the Flag does not support values (Standalone).
// Example:
//
//		a Flag which accepts only whole numbers could be defined as:
//	 	AcceptPattern("times", PatternValue{Pattern: `^[\d]+`, Description: "Please supply a whole number"}).
func (s *CmdLineOption) AcceptPattern(flag string, val PatternValue) error {
	return s.AcceptPatterns(flag, []PatternValue{val})
}

// AcceptPatterns same as PatternValue but acts on a list of patterns and descriptions. When specified, the patterns defined
// in AcceptPatterns represent a set of values, of which one must be supplied on the command-line. The patterns are evaluated
// on Parse, if no command-line options match one of the PatternValue, Parse returns false.
func (s *CmdLineOption) AcceptPatterns(flag string, acceptVal []PatternValue) error {
	arg, err := s.GetArgument(flag)
	if err != nil {
		return err
	}

	lenValues := len(acceptVal)
	if arg.AcceptedValues == nil {
		arg.AcceptedValues = make([]LiterateRegex, 0, lenValues)
	}

	for i := 0; i < lenValues; i++ {
		if err := arg.accept(acceptVal[i]); err != nil {
			return *err
		}
	}

	return nil
}

// GetAcceptPatterns takes a flag string and returns an error if the flag does not exist, a slice of LiterateRegex otherwise
func (s *CmdLineOption) GetAcceptPatterns(flag string) ([]LiterateRegex, error) {
	arg, err := s.GetArgument(flag)
	if err != nil {
		return []LiterateRegex{}, err
	}

	if arg.AcceptedValues == nil {
		return []LiterateRegex{}, nil
	}

	return arg.AcceptedValues, nil
}

// GetArgument returns the Argument corresponding to the long or short flag or an error when not found
func (s *CmdLineOption) GetArgument(flag string) (*Argument, error) {
	mainKey := s.flagOrShortFlag(flag)
	v, found := s.acceptedFlags.Get(mainKey)
	if !found {
		return nil, fmt.Errorf("option with flag %s was not set", flag)
	}

	return v.Argument, nil
}

// SetArgument sets an Argument configuration. Returns an error if the Argument is not found or the
// configuration results in an error
func (s *CmdLineOption) SetArgument(flag string, configs ...ConfigureArgumentFunc) error {
	arg, err := s.GetArgument(flag)
	if err != nil {
		return err
	}

	return arg.Set(configs...)
}

// GetShortFlag maps a long flag to its equivalent short flag. Short flags are concise alternatives to
// their verbose counterparts.
//
// The function returns the corresponding short flag if it exists. If not, it returns an error
// indicating that no short flag is defined for the provided long flag.
//
// Params:
// flag (string): The long flag for which the function should retrieve the corresponding short flag.
//
// Returns:
// string: The short flag variant if it exists.
// error: An error if no short flag is defined for the provided long flag, or if any other error occurs.
func (s *CmdLineOption) GetShortFlag(flag string) (string, error) {
	argument, err := s.GetArgument(flag)
	if err == nil {
		if argument.Short != "" {
			return argument.Short, nil
		}

		return "", fmt.Errorf("flag %s has no short flag defined", flag)
	}

	return "", err
}

// HasFlag returns true when the Flag has been seen on the command line.
func (s *CmdLineOption) HasFlag(flag string) bool {
	mainKey := s.flagOrShortFlag(flag)
	_, found := s.options[mainKey]
	if !found && s.secureArguments != nil {
		// secure arguments are evaluated after all others - if a callback (ex. RequiredIf) relies
		// on HasFlag during Parse then we need to check secureArguments
		_, found = s.secureArguments.Get(mainKey)
	}

	return found
}

// HasCommand return true when the name has been seen on the command line.
func (s *CmdLineOption) HasCommand(path string) bool {
	_, found := s.commandOptions.Get(path)

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
	flagInfo, found := s.acceptedFlags.Get(mainKey)
	if found {
		flagInfo.Argument.Description = description

		return nil
	}

	return fmt.Errorf("%w: %s", ErrFlagNotFound, flag)
}

// GetDescription retrieves a Flag's description as set by DescribeFlag
func (s *CmdLineOption) GetDescription(flag string) string {
	mainKey := s.flagOrShortFlag(flag)
	flagInfo, found := s.acceptedFlags.Get(mainKey)
	if found {
		return flagInfo.Argument.Description
	}

	return ""
}

// SetFlag is used to re-define a Flag or define a new Flag at runtime. This can be sometimes useful for dynamic
// evaluation of combinations of options and values which can't be expressed statically. For instance, when the user
// should supply these during a program's execution but after command-line options have been parsed. If the Flag is of type
// File the value is stored in the file.
func (s *CmdLineOption) SetFlag(flag, value string) error {
	mainKey := s.flagOrShortFlag(flag)
	key := ""
	_, found := s.options[flag]
	if found {
		s.options[mainKey] = value
		key = mainKey
	} else {
		s.options[flag] = value
		key = flag
	}
	arg, err := s.GetArgument(key)
	if err != nil {
		return err
	}

	if arg.TypeOf == File {
		path := s.rawArgs[key]
		if path == "" {
			path = arg.DefaultValue
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		err = os.WriteFile(abs, []byte(value), 0600)
		if err != nil {
			return err
		}

	}

	return nil
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
	flagInfo, found := s.acceptedFlags.Get(mainKey)
	if found {
		flagInfo.Argument.DependsOn = append(flagInfo.Argument.DependsOn, dependsOn)

		return nil
	}

	return fmt.Errorf("%w: %s", ErrFlagNotFound, flag)
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
	flagInfo, found := s.acceptedFlags.Get(mainKey)
	if found {
		flagInfo.Argument.DependsOn = append(flagInfo.Argument.DependsOn, dependsOn)
		if len(flagInfo.Argument.OfValue) == 0 {
			flagInfo.Argument.OfValue = make([]string, 1, 5)
			flagInfo.Argument.OfValue[0] = ofValue
		} else {
			flagInfo.Argument.OfValue = append(flagInfo.Argument.OfValue, ofValue)
		}

		return nil
	}

	return fmt.Errorf("%w: %s", ErrFlagNotFound, flag)
}

// GetErrors returns a list of the errors encountered during Parse
func (s *CmdLineOption) GetErrors() []error {
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
	if s.registeredCommands.Count() > 0 {
		_, _ = writer.Write([]byte("\ncommands:\n"))
		s.PrintCommands(writer)
	}
}

// PrintUsageWithGroups pretty prints accepted Flags and show command-specific Flags grouped by Commands to io.Writer.
func (s *CmdLineOption) PrintUsageWithGroups(writer io.Writer) {
	// Print the program usage
	_, _ = writer.Write([]byte(fmt.Sprintf("usage: %s\n", os.Args[0])))

	// Print global flags
	s.PrintGlobalFlags(writer)

	// Print command-specific flags and commands
	if s.registeredCommands.Count() > 0 {
		_, _ = writer.Write([]byte("\nCommands:\n"))
		s.PrintCommandsWithFlags(writer, &PrettyPrintConfig{
			NewCommandPrefix:     " +  ",
			DefaultPrefix:        " │─ ",
			TerminalPrefix:       " └─ ",
			InnerLevelBindPrefix: " ** ",
			OuterLevelBindPrefix: " |  ",
		})
	}
}

// PrintGlobalFlags prints global (non-command-specific) flags
func (s *CmdLineOption) PrintGlobalFlags(writer io.Writer) {
	_, _ = writer.Write([]byte("\nGlobal Flags:\n\n"))

	for f := s.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == "" { // Global flags have no command path
			shortFlag, _ := s.GetShortFlag(*f.Key)
			requiredOrOptional := "optional"
			if f.Value.Argument.Required {
				requiredOrOptional = "required"
			}
			_, _ = writer.Write([]byte(fmt.Sprintf(" --%s or -%s \"%s\" (%s)\n", *f.Key, shortFlag, f.Value.Argument.Description, requiredOrOptional)))
		}
	}
}

// PrintCommandsWithFlags prints commands with their respective flags
func (s *CmdLineOption) PrintCommandsWithFlags(writer io.Writer, config *PrettyPrintConfig) {
	for kv := s.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		if kv.Value.TopLevel {
			kv.Value.Visit(func(cmd *Command, level int) bool {
				// Determine the correct prefix based on command level and position
				var prefix string
				if level == 0 {
					prefix = config.NewCommandPrefix
				} else if len(cmd.Subcommands) == 0 {
					prefix = config.TerminalPrefix
				} else {
					prefix = config.DefaultPrefix
				}

				// Print the command itself with proper indentation
				command := fmt.Sprintf("%s%s%s \"%s\"\n", prefix, strings.Repeat(config.InnerLevelBindPrefix, level), cmd.Path, cmd.Description)
				if _, err := writer.Write([]byte(command)); err != nil {
					return false
				}

				// Print flags specific to this command
				s.PrintCommandSpecificFlags(writer, cmd.Path, level, config)

				return true
			}, 0)
		}
	}
}

// PrintCommandSpecificFlags print flags for a specific command with the appropriate indentation
func (s *CmdLineOption) PrintCommandSpecificFlags(writer io.Writer, commandPath string, level int, config *PrettyPrintConfig) {
	hasFlags := false
	for f := s.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == commandPath {
			if !hasFlags {
				// Print the header for flags of this command
				//flagHeader := fmt.Sprintf("%sFlags for command '%s':\n", strings.Repeat(config.OuterLevelBindPrefix, level+1), commandPath)
				//_, _ = writer.Write([]byte(flagHeader))
				hasFlags = true
			}

			// Determine if the flag is required or optional
			requiredOrOptional := "optional"
			if f.Value.Argument.Required {
				requiredOrOptional = "required"
			}

			// Print each flag with proper indentation, aligning with the command hierarchy
			flag := fmt.Sprintf("%s--%s \"%s\" (%s)\n", strings.Repeat(config.OuterLevelBindPrefix, level+1), splitCommandFlag(*f.Key)[0], f.Value.Argument.Description, requiredOrOptional)
			_, _ = writer.Write([]byte(flag))
		}
	}
}

// PrintFlags pretty prints accepted command-line switches to io.Writer
func (s *CmdLineOption) PrintFlags(writer io.Writer) {
	var shortOption string
	for f := s.acceptedFlags.Front(); f != nil; f = f.Next() {
		shortFlag, err := s.GetShortFlag(*f.Key)
		if err == nil {
			shortOption = " or -" + shortFlag + " "
		} else {
			shortOption = " "
		}
		requiredOrOptional := "optional"
		if f.Value.Argument.Required {
			requiredOrOptional = "required"
		}
		_, _ = writer.Write([]byte(fmt.Sprintf("\n --%s%s\"%s\" (%s)",
			*f.Key, shortOption, s.GetDescription(*f.Key), requiredOrOptional)))
	}
}

// PrintCommands writes the list of accepted Command structs to io.Writer.
func (s *CmdLineOption) PrintCommands(writer io.Writer) {
	s.PrintCommandsUsing(writer, &PrettyPrintConfig{
		NewCommandPrefix:     " +",
		DefaultPrefix:        " │",
		TerminalPrefix:       " └",
		OuterLevelBindPrefix: "─",
	})
}

// PrintCommandsUsing writes the list of accepted Command structs to io.Writer using PrettyPrintConfig.
// PrettyPrintConfig.NewCommandPrefix precedes the start of a new command
// PrettyPrintConfig.DefaultPrefix precedes sub-commands by default
// PrettyPrintConfig.TerminalPrefix precedes terminal, i.e. Command structs which don't have sub-commands
// PrettyPrintConfig.OuterLevelBindPrefix is used for indentation. The indentation is repeated for each Level under the
//
//	command root. The Command root is at Level 0.
func (s *CmdLineOption) PrintCommandsUsing(writer io.Writer, config *PrettyPrintConfig) {
	for kv := s.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		if kv.Value.TopLevel {
			kv.Value.Visit(func(cmd *Command, level int) bool {
				var start = config.DefaultPrefix
				switch {
				case level == 0:
					start = config.NewCommandPrefix
				case len(cmd.Subcommands) == 0:
					start = config.TerminalPrefix
				}
				command := fmt.Sprintf("%s%s %s \"%s\"\n", start, strings.Repeat(config.OuterLevelBindPrefix, level),
					cmd.Name, cmd.Description)
				if _, err := writer.Write([]byte(command)); err != nil {
					return false
				}
				return true

			}, 0)
		}
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
