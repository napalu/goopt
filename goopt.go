// Copyright 2021-2024, Florent Heyworth. All rights reserved.
// Use of this source code is governed by the MIT licensee
// which can be found in the LICENSE file.

// Package goopt provides support for command-line processing.
//
// It supports 4 types of flags:
//
//	Single - a flag which expects a value
//	Chained - flag which expects a delimited value representing elements in a list (and is evaluated as a list)
//	Standalone - a boolean flag which by default takes no value (defaults to true) but may accept a value which evaluates to true or false
//	File - a flag which expects a file path
//
// Additionally, commands and sub-commands (Command) are supported. Commands can be nested to represent sub-commands. Unlike
// the official go.Flag package commands and sub-commands may be placed before, after or mixed in with flags.
package goopt

import (
	"fmt"
	"io"
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

// NewParser convenience initialization method. Use NewCmdLine to
// configure CmdLineOption using option functions.
func NewParser() *Parser {
	return &Parser{
		acceptedFlags:        orderedmap.NewOrderedMap[string, *FlagInfo](),
		lookup:               map[string]string{},
		options:              map[string]string{},
		errors:               []error{},
		bind:                 make(map[string]interface{}, 1),
		customBind:           map[string]ValueSetFunc{},
		registeredCommands:   orderedmap.NewOrderedMap[string, *Command](),
		commandOptions:       orderedmap.NewOrderedMap[string, bool](),
		positionalArgs:       []PositionalArgument{},
		listFunc:             matchChainedSeparators,
		callbackQueue:        queue.New[commandCallback](),
		callbackResults:      map[string]error{},
		secureArguments:      orderedmap.NewOrderedMap[string, *types.Secure](),
		prefixes:             []rune{'-'},
		stderr:               os.Stderr,
		stdout:               os.Stdout,
		flagNameConverter:    DefaultFlagNameConverter,
		commandNameConverter: DefaultCommandNameConverter,
		maxDependencyDepth:   DefaultMaxDependencyDepth,
	}
}

// NewCmdLineOption creates a new parser with default initialization.
//
// Deprecated: Use NewParser instead. This function will be removed in v2.0.0.
func NewCmdLineOption() *Parser {
	return NewParser()
}

// NewParserFromStruct creates a new Parser from a struct.
// By default, all fields are treated as flags unless:
//   - They are tagged with `ignore`
//   - They are unexported
//   - They are nested structs or slices of structs
//
// Default field behavior:
//   - Type: Single
//   - Name: derived from field name
//   - Flag: true
//
// Use tags to override defaults:
//
//	`goopt:"name:custom;type:chained"`
func NewParserFromStruct[T any](structWithTags *T) (*Parser, error) {
	return NewParserFromStructWithLevel(structWithTags, 5)
}

// NewCmdLineFromStruct is an alias for NewParserFromStruct.
//
// Deprecated: Use NewParserFromStruct instead. This function will be removed in v2.0.0.
func NewCmdLineFromStruct[T any](structWithTags *T) (*Parser, error) {
	return NewParserFromStruct(structWithTags)
}

// NewParserFromStructWithLevel parses a struct and binds its fields to command-line flags up to maxDepth levels
func NewParserFromStructWithLevel[T any](structWithTags *T, maxDepth int) (*Parser, error) {
	return newParserFromReflectValue(reflect.ValueOf(structWithTags), "", "", maxDepth, 0)
}

// NewCmdLineFromStructWithLevel is an alias for NewParserFromStructWithLevel.
//
// Deprecated: Use NewParserFromStructWithLevel instead. This function will be removed in v2.0.0.
func NewCmdLineFromStructWithLevel[T any](structWithTags *T, maxDepth int) (*Parser, error) {
	return NewParserFromStructWithLevel(structWithTags, maxDepth)
}

// NewParserFromInterface creates a new parser from an interface{} that should be a struct or a pointer to a struct
func NewParserFromInterface(i interface{}) (*Parser, error) {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		// If not a pointer, create one
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		v = ptr
	}
	return newParserFromReflectValue(v, "", "", 5, 0)
}

// NewArgument convenience initialization method to describe Flags. Alternatively, Use NewArg to
// configure Argument using option functions.
func NewArgument(shortFlag string, description string, typeOf types.OptionType, required bool, secure types.Secure, defaultValue string) *Argument {
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

func (p *Parser) SetExecOnParse(value bool) {
	p.callbackOnParse = value
}

// SetCommandNameConverter allows setting a custom name converter for command names
func (p *Parser) SetCommandNameConverter(converter NameConversionFunc) NameConversionFunc {
	oldConverter := p.commandNameConverter
	p.commandNameConverter = converter

	return oldConverter
}

// SetFlagNameConverter allows setting a custom name converter for flag names
func (p *Parser) SetFlagNameConverter(converter NameConversionFunc) NameConversionFunc {
	oldConverter := p.flagNameConverter
	p.flagNameConverter = converter

	return oldConverter
}

// SetEnvNameConverter allows setting a custom name converter for environment variable names
// If set and the environment variable exists, it will be prepended to the args array
func (p *Parser) SetEnvNameConverter(converter NameConversionFunc) NameConversionFunc {
	oldConverter := p.envNameConverter
	p.envNameConverter = converter

	return oldConverter
}

// ExecuteCommands command callbacks are placed on a FIFO queue during parsing until ExecuteCommands is called.
// Returns the count of errors encountered during execution.
func (p *Parser) ExecuteCommands() int {
	callbackErrors := 0
	for p.callbackQueue.Len() > 0 {
		call, _ := p.callbackQueue.Pop()
		if call.callback != nil && len(call.arguments) == 2 {
			cmdLine, cmdLineOk := call.arguments[0].(*Parser)
			cmd, cmdOk := call.arguments[1].(*Command)
			if cmdLineOk && cmdOk {
				err := call.callback(cmdLine, cmd)
				p.callbackResults[cmd.Name] = err
				if err != nil {
					callbackErrors++
				}
			}
		}
	}

	return callbackErrors
}

// ExecuteCommand command callbacks are placed on a FIFO queue during parsing until ExecuteCommands is called.
// Returns the error which occurred during execution of a command callback.
func (p *Parser) ExecuteCommand() error {
	if p.callbackQueue.Len() > 0 {
		call, _ := p.callbackQueue.Pop()
		if call.callback != nil && len(call.arguments) == 2 {
			cmdLine, cmdLineOk := call.arguments[0].(*Parser)
			cmd, cmdOk := call.arguments[1].(*Command)
			if cmdLineOk && cmdOk {
				err := call.callback(cmdLine, cmd)
				p.callbackResults[cmd.Name] = err
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// GetCommandExecutionError returns the error which occurred during execution of a command callback
// after ExecuteCommands has been called. Returns nil on no error. Returns a CommandNotFound error when
// no callback is associated with commandName
func (p *Parser) GetCommandExecutionError(commandName string) error {
	if err, found := p.callbackResults[commandName]; found {
		return err
	}

	return fmt.Errorf("%w: %s was not found or has no associated callback", types.ErrCommandNotFound, commandName)
}

// GetCommandExecutionErrors returns the errors which occurred during execution of command callbacks
// after ExecuteCommands has been called. Returns a KeyValue list of command name and error
func (p *Parser) GetCommandExecutionErrors() []types.KeyValue[string, error] {
	errors := []types.KeyValue[string, error]{}
	for key, err := range p.callbackResults {
		if err != nil {
			errors = append(errors, types.KeyValue[string, error]{Key: key, Value: err})
		}
	}

	return errors
}

// AddFlagPreValidationFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
// *before* AcceptedValues are checked
func (p *Parser) AddFlagPreValidationFilter(flag string, proc FilterFunc, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		flagInfo.Argument.PreFilter = proc

		return nil
	}

	return fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
}

// AddFlagPostValidationFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
// *after* AcceptedValues are checked
func (p *Parser) AddFlagPostValidationFilter(flag string, proc FilterFunc, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		flagInfo.Argument.PostFilter = proc

		return nil
	}

	return fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
}

// HasPreValidationFilter returns true when an option has a transform/evaluate function which is called on Parse
// before checking for acceptable values
func (p *Parser) HasPreValidationFilter(flag string, commandPath ...string) bool {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		return flagInfo.Argument.PreFilter != nil
	}

	return false
}

// GetPreValidationFilter retrieve Flag transform/evaluate function which is called on Parse before checking for
// acceptable values
func (p *Parser) GetPreValidationFilter(flag string, commandPath ...string) (FilterFunc, error) {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		if flagInfo.Argument.PreFilter != nil {
			return flagInfo.Argument.PreFilter, nil
		}
	}

	return nil, fmt.Errorf("%w: no pre-validation filters for flag %s", types.ErrValidationFailed, flag)
}

// HasPostValidationFilter returns true when an option has a transform/evaluate function which is called on Parse
// after checking for acceptable values
func (p *Parser) HasPostValidationFilter(flag string, commandPath ...string) bool {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		return flagInfo.Argument.PostFilter != nil
	}

	return false
}

// GetPostValidationFilter retrieve Flag transform/evaluate function which is called on Parse after checking for
// acceptable values
func (p *Parser) GetPostValidationFilter(flag string, commandPath ...string) (FilterFunc, error) {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		if flagInfo.Argument.PostFilter != nil {
			return flagInfo.Argument.PostFilter, nil
		}
	}

	return nil, fmt.Errorf("%w: no post-validation filters for flag %s", types.ErrValidationFailed, flag)
}

// HasAcceptedValues returns true when a Flag defines a set of valid values it will accept
func (p *Parser) HasAcceptedValues(flag string, commandPath ...string) bool {
	flagInfo, found := p.acceptedFlags.Get(p.flagOrShortFlag(flag, commandPath...))
	if found {
		return len(flagInfo.Argument.AcceptedValues) > 0
	}

	return false
}

// AddCommand used to define a Command/sub-command chain
// Unlike a flag which starts with a '-' or '/' a Command represents a verb or action
func (p *Parser) AddCommand(cmd *Command) error {
	// Validate the command hierarchy and ensure unique paths
	if ok, err := p.validateCommand(cmd, 0, 100); !ok {
		return err
	}

	// Add the command and all its subcommands to registeredCommands
	p.registerCommandRecursive(cmd)

	return nil
}

// Parse this function should be called on os.Args (or a user-defined array of arguments). Returns true when
// user command line arguments match the defined Flag and Command rules
// Parse processes user command line arguments matching the defined Flag and Command rules.
func (p *Parser) Parse(args []string) bool {
	p.ensureInit()
	pruneExecPathFromArgs(&args)

	var (
		envFlagsByCommand  = p.groupEnvVarsByCommand() // Get env flags split by command
		envInserted        = make(map[string]int)
		lastCommandPath    string
		cmdQueue           = queue.New[*Command]()
		ctxStack           = queue.New[string]() // Stack for command contexts
		commandPathSlice   []string
		currentCommandPath string
		processedStack     bool
	)

	state := parse.NewState(args)
	if g, ok := envFlagsByCommand["global"]; ok && len(g) > 0 {
		state.InsertArgsAt(0, g...)
	}

	for state.Advance() {
		cur := state.CurrentArg()
		if p.isFlag(state.CurrentArg()) {
			if p.isGlobalFlag(cur) {
				p.evalFlagWithPath(state, "")
			} else {
				// We now iterate over the stack to handle flags for each command context
				for i := ctxStack.Len() - 1; i >= 0; i-- {
					cmdContext, ok := ctxStack.At(i)
					if ok {
						currentCommandPath = cmdContext
					} else {
						currentCommandPath = ""
					}

					p.evalFlagWithPath(state, currentCommandPath)

					processedStack = true
				}

				if !processedStack {
					// fallback - possibly POSIX style
					p.evalFlagWithPath(state, "")
				}
			}

		} else {
			// Parse the next command
			terminating := p.parseCommand(state, cmdQueue, &commandPathSlice)
			currentCommandPath = strings.Join(commandPathSlice, " ")
			// Inject relevant environment variables for the current command context
			if instanceCount, exists := envInserted[currentCommandPath]; !exists || instanceCount < cmdQueue.Len() {
				if len(envFlagsByCommand[currentCommandPath]) > 0 {
					state.InsertArgsAt(state.CurrentPos()+1, envFlagsByCommand[currentCommandPath]...)
				}
				envInserted[currentCommandPath]++
			}

			if lastCommandPath != "" && p.callbackOnParse {
				err := p.ExecuteCommand()
				if err != nil {
					p.addError(err)
				}
				lastCommandPath = ""
			}

			if terminating {
				if processedStack {
					ctxStack.Clear()
					processedStack = false
				}
				if currentCommandPath != "" {
					ctxStack.Push(currentCommandPath)
				}
				lastCommandPath = currentCommandPath
				commandPathSlice = commandPathSlice[:0]
			}
		}
	}

	// Execute any remaining command callback after parsing is done
	if p.callbackOnParse && lastCommandPath != "" {
		err := p.ExecuteCommand()
		if err != nil {
			p.addError(err)
		}
	}

	// Validate all processed options
	p.validateProcessedOptions()
	p.setPositionalArguments(state, currentCommandPath)

	// Process secure arguments if parsing succeeded
	success := len(p.errors) == 0
	if success {
		for f := p.secureArguments.Front(); f != nil; f = f.Next() {
			p.processSecureFlag(*f.Key, f.Value)
		}
		success = len(p.errors) == 0
	}
	p.secureArguments = nil

	return success
}

// ParseString calls Parse
func (p *Parser) ParseString(argString string) bool {
	args, err := parse.Split(argString)
	if err != nil {
		return false
	}

	return p.Parse(args)
}

// ParseWithDefaults calls Parse supplementing missing arguments in args array with default values from defaults
func (p *Parser) ParseWithDefaults(defaults map[string]string, args []string) bool {
	argLen := len(args)
	argMap := make(map[string]string, argLen)

	for i := 0; i < argLen; i++ {
		if p.isFlag(args[i]) {
			arg := p.flagOrShortFlag(strings.TrimLeftFunc(args[i], p.prefixFunc))
			if i < argLen-1 {
				argMap[arg] = args[i+1]
				if args[i] != arg {
					argMap[args[i]] = args[i+1]
				}
			}
		}
	}

	for key, val := range defaults {
		if _, found := argMap[key]; !found {
			args = append(args, string(p.prefixes[0])+key)
			args = append(args, val)
		}
	}

	return p.Parse(args)
}

// ParseStringWithDefaults calls Parse supplementing missing arguments in argString with default values from defaults
func (p *Parser) ParseStringWithDefaults(defaults map[string]string, argString string) bool {
	args, err := parse.Split(argString)
	if err != nil {
		return false
	}

	return p.ParseWithDefaults(defaults, args)
}

// SetPosix sets the posixCompatible flag.
func (p *Parser) SetPosix(posixCompatible bool) bool {
	oldValue := p.posixCompatible

	p.posixCompatible = posixCompatible

	return oldValue
}

// GetPositionalArgs returns the list of positional arguments - a positional argument is a command line argument that is
// neither a flag, a flag value, nor a command
func (p *Parser) GetPositionalArgs() []PositionalArgument {
	return p.positionalArgs
}

// GetPositionalArgCount returns the number of positional arguments
func (p *Parser) GetPositionalArgCount() int {
	return len(p.positionalArgs)
}

// HasPositionalArgs returns true if there are positional arguments
func (p *Parser) HasPositionalArgs() bool {
	return p.GetPositionalArgCount() > 0
}

// GetCommands returns the list of all commands seen on command-line
func (p *Parser) GetCommands() []string {
	pathValues := make([]string, 0, p.commandOptions.Count())
	for kv := p.commandOptions.Front(); kv != nil; kv = kv.Next() {
		if kv.Value {
			pathValues = append(pathValues, *kv.Key)
		}
	}

	return pathValues
}

// Get returns a combination of a Flag's value as string and true if found. If a flag is not set but has a configured default value
// the default value is registered and is returned. Returns an empty string and false otherwise
func (p *Parser) Get(flag string, commandPath ...string) (string, bool) {
	lookup := buildPathFlag(flag, commandPath...)
	mainKey := p.flagOrShortFlag(lookup)
	value, found := p.options[mainKey]
	flagInfo, ok := p.acceptedFlags.Get(mainKey)
	if ok {
		if found {
			if flagInfo.Argument.Secure.IsSecure {
				p.options[mainKey] = ""
			}
		} else {
			if flagInfo.Argument.DefaultValue != "" {
				p.options[mainKey] = flagInfo.Argument.DefaultValue
				value = flagInfo.Argument.DefaultValue
				err := p.setBoundVariable(value, mainKey)
				if err != nil {
					p.addError(fmt.Errorf("error setting bound variable value %w", err))
				}
				found = true
			}
		}
	}

	return value, found
}

// GetOrDefault returns the value of a defined Flag or defaultValue if no value is set
func (p *Parser) GetOrDefault(flag string, defaultValue string, commandPath ...string) string {
	value, found := p.Get(flag, commandPath...)
	if found {
		return value
	}

	return defaultValue
}

// GetBool attempts to convert the string value of a Flag to boolean.
func (p *Parser) GetBool(flag string, commandPath ...string) (bool, error) {
	value, success := p.Get(flag, commandPath...)
	if !success {
		return false, fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
	}

	val, err := strconv.ParseBool(value)

	return val, err
}

// GetInt attempts to convert the string value of a Flag to an int64.
func (p *Parser) GetInt(flag string, bitSize int, commandPath ...string) (int64, error) {
	value, success := p.Get(flag, commandPath...)
	if !success {
		return 0, fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
	}

	val, err := strconv.ParseInt(value, 10, bitSize)

	return val, err
}

// GetFloat attempts to convert the string value of a Flag to a float64
func (p *Parser) GetFloat(flag string, bitSize int, commandPath ...string) (float64, error) {
	value, success := p.Get(flag, commandPath...)
	if !success {
		return 0, fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
	}

	val, err := strconv.ParseFloat(value, bitSize)

	return val, err
}

// GetList attempts to split the string value of a Chained Flag to a string slice
// by default the value is split on '|', ',' or ' ' delimiters
func (p *Parser) GetList(flag string, commandPath ...string) ([]string, error) {
	arg, err := p.GetArgument(flag, commandPath...)
	if err == nil {
		if arg.TypeOf == types.Chained {
			value, success := p.Get(flag, commandPath...)
			if !success {
				return []string{}, fmt.Errorf("failed to retrieve value for flag '%s'", flag)
			}

			listDelimFunc := p.getListDelimiterFunc()

			return strings.FieldsFunc(value, listDelimFunc), nil
		}

		return []string{}, fmt.Errorf("invalid Argument type for flag '%s' - use typeOf = Chained instead", flag)
	}

	return []string{}, err
}

// SetListDelimiterFunc sets the value delimiter function for Chained flags
func (p *Parser) SetListDelimiterFunc(delimiterFunc types.ListDelimiterFunc) error {
	if delimiterFunc != nil {
		p.listFunc = delimiterFunc

		return nil
	}

	return fmt.Errorf("invalid ListDelimiterFunc (should not be null)")
}

// SetArgumentPrefixes sets the flag argument prefixes
func (p *Parser) SetArgumentPrefixes(prefixes []rune) error {
	prefixesLen := len(prefixes)
	if prefixesLen == 0 {
		return fmt.Errorf("can't parse with empty argument prefix list")
	}

	p.prefixes = prefixes

	return nil
}

// GetConsistencyWarnings is a helper function which provides information about eventual option consistency warnings.
// It is intended for users of the library rather than for end-users
func (p *Parser) GetConsistencyWarnings() []string {
	var configWarnings []string
	for opt := range p.options {
		mainKey := p.flagOrShortFlag(opt)
		flagInfo, found := p.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}
		if flagInfo.Argument.TypeOf == types.Standalone && flagInfo.Argument.DefaultValue != "" {
			configWarnings = append(configWarnings,
				fmt.Sprintf("Flag '%s' is a Standalone (boolean) flag and has a default value specified. "+
					"The default value is ignored.", mainKey))
		}
	}

	return configWarnings
}

// GetWarnings returns a string slice of all warnings (non-fatal errors) - a warning is set when optional dependencies
// are not met - for instance, specifying the value of a Flag which relies on a missing argument
func (p *Parser) GetWarnings() []string {
	var warnings []string
	for opt := range p.options {
		mainKey := p.flagOrShortFlag(opt)
		flagInfo, found := p.acceptedFlags.Get(mainKey)
		if !found {
			continue
		}

		dependentFlags := p.getDependentFlags(flagInfo.Argument)
		if len(dependentFlags) == 0 {
			continue
		}

		for _, depFlag := range dependentFlags {
			dependKey := p.flagOrShortFlag(depFlag)
			dependValue, hasKey := p.options[dependKey]

			if !hasKey {
				warnings = append(warnings,
					fmt.Sprintf("Flag '%s' depends on '%s' which was not specified.", mainKey, depFlag))
				continue
			}

			matches, allowedValues := p.checkDependencyValue(flagInfo.Argument, depFlag, dependValue)
			if !matches && len(allowedValues) > 0 {
				warnings = append(warnings, fmt.Sprintf(
					"Flag '%s' depends on '%s' with value %s which was not specified. (got '%s')",
					mainKey, dependKey, showDependencies(allowedValues), dependValue))
			}
		}
	}

	return warnings
}

// GetOptions returns a slice of KeyValue pairs which have been supplied on the command-line.
func (p *Parser) GetOptions() []types.KeyValue[string, string] {
	keyValues := make([]types.KeyValue[string, string], len(p.options))
	i := 0
	for key, value := range p.options {
		keyValues[i].Key = key
		keyValues[i].Value = value
		i++
	}

	return keyValues
}

// AddFlag is used to define a Flag. A Flag represents a command line option
// with a "long" name and an optional "short" form prefixed by '-', '--' or '/'.
// This version supports both global flags and command-specific flags using the optional commandPath argument.
func (p *Parser) AddFlag(flag string, argument *Argument, commandPath ...string) error {
	argument.ensureInit()

	if flag == "" {
		return fmt.Errorf("can't set empty flag")
	}

	// Use the helper function to generate the lookup key
	lookupFlag := buildPathFlag(flag, commandPath...)

	// Ensure no duplicate flags for the same command path or globally
	if _, exists := p.acceptedFlags.Get(lookupFlag); exists {
		return fmt.Errorf("flag '%s' already exists for the given command path", lookupFlag)
	}

	if lenS := len(argument.Short); lenS > 0 {
		if p.posixCompatible && lenS > 1 {
			return fmt.Errorf("%w: flag %s has short form %s which is not posix compatible (length > 1)", types.ErrPosixIncompatible, flag, argument.Short)
		}

		// Check for short flag conflicts only for global flags
		if len(commandPath) == 0 { // Global flag
			if arg, exists := p.lookup[argument.Short]; exists {
				return fmt.Errorf("short flag '%s' on global flag %s already exists as %v", argument.Short, flag, arg)
			}
		}

		p.lookup[argument.Short] = flag
	}

	if argument.TypeOf == types.Empty {
		argument.TypeOf = types.Single
	}

	p.acceptedFlags.Set(lookupFlag, &FlagInfo{
		Argument:    argument,
		CommandPath: strings.Join(commandPath, " "), // Keep track of the command path
	})

	if argument.Capacity < 0 {
		return fmt.Errorf("negative capacity not allowed: %d", argument.Capacity)
	}

	if argument.Capacity > 0 {
		// Register each index
		for i := 0; i < argument.Capacity; i++ {
			indexPath := fmt.Sprintf("%s.%d", lookupFlag, i)
			p.acceptedFlags.Set(indexPath, &FlagInfo{
				Argument:    argument,
				CommandPath: strings.Join(commandPath, " "),
			})
		}
	}

	return nil
}

// BindFlagToParser is a helper function to allow passing generics to the Parser.BindFlag method
func BindFlagToParser[T Bindable](s *Parser, data *T, flag string, argument *Argument, commandPath ...string) error {
	if s == nil {
		return types.ErrBindNilPointer
	}

	return s.BindFlag(data, flag, argument, commandPath...)
}

// BindFlagToCmdLine is an alias for BindFlagToParser.
//
// Deprecated: Use BindFlagToParser instead. This function will be removed in v2.0.0.
func BindFlagToCmdLine[T Bindable](s *Parser, data *T, flag string, argument *Argument, commandPath ...string) error {
	return BindFlagToParser(s, data, flag, argument, commandPath...)
}

// CustomBindFlagToParser is a helper function to allow passing generics to the Parser.CustomBindFlag method
func CustomBindFlagToParser[T any](s *Parser, data *T, proc ValueSetFunc, flag string, argument *Argument, commandPath ...string) error {
	if s == nil {
		return types.ErrBindNilPointer
	}

	return s.CustomBindFlag(data, proc, flag, argument, commandPath...)
}

// CustomBindFlagToCmdLine is an alias for CustomBindFlagToParser.
//
// Deprecated: Use CustomBindFlagToParser instead. This function will be removed in v2.0.0.
func CustomBindFlagToCmdLine[T any](s *Parser, data *T, proc ValueSetFunc, flag string, argument *Argument, commandPath ...string) error {
	return CustomBindFlagToParser(s, data, proc, flag, argument, commandPath...)
}

// BindFlag is used to bind a *pointer* to string, int, uint, bool, float or time.Time scalar or slice variable with a Flag
// which is set when Parse is invoked.
// An error is returned if data cannot be bound - for compile-time safety use BindFlagToParser instead
func (p *Parser) BindFlag(bindPtr interface{}, flag string, argument *Argument, commandPath ...string) error {
	if bindPtr == nil {
		return types.ErrBindNilPointer
	}
	if ok, err := util.CanConvert(bindPtr, argument.TypeOf); !ok {
		return err
	}

	v := reflect.ValueOf(bindPtr)
	if v.Kind() != reflect.Ptr {
		return types.ErrVariableNotAPointer
	}

	elem := v.Elem()
	if !elem.IsValid() {
		return fmt.Errorf("can't bind to invalid value field")
	}

	lookupFlag := buildPathFlag(flag, commandPath...)
	if elem.Kind() == reflect.Slice && argument != nil && argument.Capacity > 0 {
		// Create or resize slice to match capacity
		newSlice := reflect.MakeSlice(elem.Type(), argument.Capacity, argument.Capacity)
		// If resizing existing slice, preserve values where possible
		if elem.Len() > 0 {
			copyLen := util.Min(elem.Len(), argument.Capacity)
			reflect.Copy(newSlice.Slice(0, copyLen), elem.Slice(0, copyLen))
		}
		elem.Set(newSlice)

		for i := 0; i < argument.Capacity; i++ {
			indexPath := fmt.Sprintf("%s.%d", lookupFlag, i)
			p.bind[indexPath] = bindPtr
		}
	}

	if err := p.AddFlag(flag, argument, commandPath...); err != nil {
		return err
	}

	if argument.TypeOf == types.Empty {
		argument.TypeOf = parse.InferFieldType(reflect.ValueOf(bindPtr).Elem().Type())
	}

	// Bind the flag to the variable
	p.bind[lookupFlag] = bindPtr

	return nil
}

// CustomBindFlag works like BindFlag but expects a ValueSetFunc callback which is called when a Flag is evaluated on Parse.
// When the Flag is seen on the command like the ValueSetFunc is called with the user-supplied value. Allows binding
// complex structures not supported by BindFlag
func (p *Parser) CustomBindFlag(data any, proc ValueSetFunc, flag string, argument *Argument, commandPath ...string) error {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return fmt.Errorf("we expect a pointer to a variable")
	}

	if !reflect.ValueOf(data).Elem().IsValid() {
		return fmt.Errorf("can't bind to invalid value field")
	}

	if err := p.AddFlag(flag, argument, commandPath...); err != nil {
		return err
	}

	lookupFlag := buildPathFlag(flag, commandPath...)

	p.bind[lookupFlag] = data
	p.customBind[lookupFlag] = proc

	return nil
}

// AcceptPattern is used to define an acceptable value for a Flag. The 'pattern' argument is compiled to a regular expression
// and the description argument is used to provide a human-readable description of the pattern.
// Returns an error if the regular expression cannot be compiled or if the Flag does not support values (Standalone).
// Example:
//
//		a Flag which accepts only whole numbers could be defined as:
//	 	AcceptPattern("times", PatternValue{Pattern: `^[\d]+`, Description: "Please supply a whole number"}).
func (p *Parser) AcceptPattern(flag string, val types.PatternValue, commandPath ...string) error {
	return p.AcceptPatterns(flag, []types.PatternValue{val}, commandPath...)
}

// AcceptPatterns same as PatternValue but acts on a list of patterns and descriptions. When specified, the patterns defined
// in AcceptPatterns represent a set of values, of which one must be supplied on the command-line. The patterns are evaluated
// on Parse, if no command-line options match one of the PatternValue, Parse returns false.
func (p *Parser) AcceptPatterns(flag string, acceptVal []types.PatternValue, commandPath ...string) error {
	arg, err := p.GetArgument(flag, commandPath...)
	if err != nil {
		return err
	}

	lenValues := len(acceptVal)
	arg.AcceptedValues = acceptVal

	for i := 0; i < lenValues; i++ {
		re, err := regexp.Compile(acceptVal[i].Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile pattern %s: %w", acceptVal[i].Pattern, err)
		}
		acceptVal[i].Compiled = re
	}

	return nil
}

// GetAcceptPatterns takes a flag string and returns an error if the flag does not exist, a slice of LiterateRegex otherwise
func (p *Parser) GetAcceptPatterns(flag string, commandPath ...string) ([]types.PatternValue, error) {
	arg, err := p.GetArgument(flag, commandPath...)
	if err != nil {
		return []types.PatternValue{}, err
	}

	if arg.AcceptedValues == nil {
		return []types.PatternValue{}, nil
	}

	return arg.AcceptedValues, nil
}

// GetArgument returns the Argument corresponding to the long or short flag or an error when not found
func (p *Parser) GetArgument(flag string, commandPath ...string) (*Argument, error) {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	v, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return nil, fmt.Errorf("option with flag %s was not set", flag)
	}

	return v.Argument, nil
}

// SetArgument sets an Argument configuration. Returns an error if the Argument is not found or the
// configuration results in an error
func (p *Parser) SetArgument(flag string, paths []string, configs ...ConfigureArgumentFunc) error {
	var args = make([]*Argument, 0, 1)

	if len(paths) == 0 {
		arg, err := p.GetArgument(flag)
		if err != nil {
			return err
		}
		args = append(args, arg)

	} else {
		for _, path := range paths {
			arg, err := p.GetArgument(flag, path)
			if err != nil {
				return err
			}
			args = append(args, arg)
		}

	}

	for _, arg := range args {
		err := arg.Set(configs...)
		if err != nil {
			return err
		}
	}

	return nil
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
func (p *Parser) GetShortFlag(flag string, commandPath ...string) (string, error) {
	argument, err := p.GetArgument(flag, commandPath...)
	if err == nil {
		if argument.Short != "" {
			return argument.Short, nil
		}

		return "", fmt.Errorf("flag %s has no short flag defined", flag)
	}

	return "", err
}

// HasFlag returns true when the Flag has been seen on the command line.
func (p *Parser) HasFlag(flag string, commandPath ...string) bool {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	_, found := p.options[mainKey]
	if !found && p.secureArguments != nil {
		// secure arguments are evaluated after all others - if a callback (ex. RequiredIf) relies
		// on HasFlag during Parse then we need to check secureArguments - we only do this
		// if the argument has been passed on the command line
		flagParts := splitPathFlag(mainKey)
		if _, found = p.rawArgs[flagParts[0]]; found {
			_, found = p.secureArguments.Get(mainKey)
		}
	}

	return found
}

// HasRawFlag returns true when the Flag has been seen on the command line - can be used to check if a flag
// was specified on the command line irrespective of the command context.
func (p *Parser) HasRawFlag(flag string) bool {
	mainKey := p.flagOrShortFlag(flag)
	flagParts := splitPathFlag(mainKey)
	if _, found := p.rawArgs[flagParts[0]]; found {
		return true
	}

	return false
}

// HasCommand return true when the name has been seen on the command line.
func (p *Parser) HasCommand(path string) bool {
	_, found := p.commandOptions.Get(path)

	return found
}

// ClearAll clears all parsed options and  commands as well as filters and acceptedValues (guards).
// Configured flags and registered commands are not cleared. Use this when parsing a command line
// repetitively.
func (p *Parser) ClearAll() {
	p.Clear(ClearConfig{})
}

// Clear can be used to selectively clear sensitive options or when re-defining options on the fly.
func (p *Parser) Clear(config ClearConfig) {
	if !config.KeepOptions {
		p.options = nil
	}
	if !config.KeepErrors {
		p.errors = p.errors[:0]
	}
	if !config.KeepCommands {
		p.commandOptions = nil
	}
	if !config.KeepPositional {
		p.positionalArgs = nil
	}
}

// DescribeFlag is used to provide a description of a Flag
func (p *Parser) DescribeFlag(flag, description string, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if found {
		flagInfo.Argument.Description = description

		return nil
	}

	return fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
}

// GetDescription retrieves a Flag's description as set by DescribeFlag
func (p *Parser) GetDescription(flag string, commandPath ...string) string {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if found {
		return flagInfo.Argument.Description
	}

	return ""
}

// SetCommand allows for setting of Command fields via option functions
// Example:
//
//	 s.Set("user create",
//		WithCommandDescription("create user),
//		WithCommandCallback(callbackFunc),
//	)
func (p *Parser) SetCommand(commandPath string, configs ...ConfigureCommandFunc) error {
	if cmd, ok := p.registeredCommands.Get(commandPath); ok {
		cmd.Set(configs...)
		return nil
	} else {
		return fmt.Errorf("command path %s not found", commandPath)
	}
}

// SetFlag is used to re-define a Flag or define a new Flag at runtime. This can be sometimes useful for dynamic
// evaluation of combinations of options and values which can't be expressed statically. For instance, when the user
// should supply these during a program's execution but after command-line options have been parsed. If the Flag is of type
// File the value is stored in the file.
func (p *Parser) SetFlag(flag, value string, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	key := ""
	_, found := p.options[flag]
	if found {
		p.options[mainKey] = value
		key = mainKey
	} else {
		p.options[flag] = value
		key = flag
	}
	arg, err := p.GetArgument(key)
	if err != nil {
		return err
	}

	if arg.TypeOf == types.File {
		path := p.rawArgs[key]
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
func (p *Parser) Remove(flag string, commandPath ...string) bool {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	_, found := p.options[mainKey]
	if found {
		delete(p.options, mainKey)

		return true
	}

	return false
}

// DependsOnFlag adds a dependency without value constraints
func (p *Parser) DependsOnFlag(flag, dependsOn string, commandPath ...string) error {
	return p.AddDependency(flag, dependsOn, commandPath...)
}

// DependsOnFlagValue adds a dependency with specific value constraints
func (p *Parser) DependsOnFlagValue(flag, dependsOn, ofValue string, commandPath ...string) error {
	return p.AddDependencyValue(flag, dependsOn, []string{ofValue}, commandPath...)
}

// AddDependency adds a dependency without value constraints
func (p *Parser) AddDependency(flag, dependsOn string, commandPath ...string) error {
	if flag == "" {
		return fmt.Errorf("can't set dependency on empty flag")
	}

	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
	}

	// Initialize DependencyMap if needed
	if flagInfo.Argument.DependencyMap == nil {
		flagInfo.Argument.DependencyMap = make(map[string][]string)
	}

	dependsOnKey := p.flagOrShortFlag(dependsOn, commandPath...)
	// Empty slice means the flag just needs to be present
	flagInfo.Argument.DependencyMap[dependsOnKey] = nil
	return nil
}

// AddDependencyValue adds or updates a dependency with specific allowed values
func (p *Parser) AddDependencyValue(flag, dependsOn string, allowedValues []string, commandPath ...string) error {
	if flag == "" {
		return fmt.Errorf("can't set dependency on empty flag")
	}

	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
	}

	// Initialize DependencyMap if needed
	if flagInfo.Argument.DependencyMap == nil {
		flagInfo.Argument.DependencyMap = make(map[string][]string)
	}

	dependsOnKey := p.flagOrShortFlag(dependsOn, commandPath...)
	// Update or add the dependency values
	if existing, exists := flagInfo.Argument.DependencyMap[dependsOnKey]; exists {
		// Append new values to existing ones
		flagInfo.Argument.DependencyMap[dependsOnKey] = append(existing, allowedValues...)
	} else {
		flagInfo.Argument.DependencyMap[dependsOnKey] = allowedValues
	}
	return nil
}

// RemoveDependency removes a dependency
func (p *Parser) RemoveDependency(flag, dependsOn string, commandPath ...string) error {
	if flag == "" {
		return fmt.Errorf("can't remove dependency from empty flag")
	}

	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return fmt.Errorf(FmtErrorWithString, types.ErrFlagNotFound, flag)
	}

	dependsOnKey := p.flagOrShortFlag(dependsOn, commandPath...)
	if flagInfo.Argument.DependencyMap != nil {
		delete(flagInfo.Argument.DependencyMap, dependsOnKey)
	}
	return nil
}

// FlagPath returns the command part of a Flag or an empty string when not.
func (p *Parser) FlagPath(flag string) string {
	return getFlagPath(flag)
}

// GetErrors returns a list of the errors encountered during Parse
func (p *Parser) GetErrors() []error {
	return p.errors
}

// GetErrorCount is greater than zero when errors were encountered during Parse.
func (p *Parser) GetErrorCount() int {
	return len(p.errors)
}

// GetCompletionData populates a CompletionData struct containing information for command line completion
func (p *Parser) GetCompletionData() completion.CompletionData {
	data := completion.CompletionData{
		Commands:            make([]string, 0),
		Flags:               make([]completion.FlagPair, 0),
		CommandFlags:        make(map[string][]completion.FlagPair),
		FlagValues:          make(map[string][]completion.CompletionValue),
		CommandDescriptions: make(map[string]string),
	}

	// Process flags
	for iter := p.acceptedFlags.Front(); iter != nil; iter = iter.Next() {
		flag := *iter.Key
		flagInfo := iter.Value
		flagParts := splitPathFlag(flag)

		cmd := ""
		flagName := flag
		if len(flagParts) > 1 {
			cmd = flagParts[0]
			flagName = flagParts[1]
		}

		addFlagToCompletionData(&data, cmd, flagName, flagInfo)
	}

	// Process commands
	for kv := p.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		cmd := kv.Value
		if cmd != nil {
			data.Commands = append(data.Commands, cmd.path)
			data.CommandDescriptions[cmd.path] = cmd.Description
		}
	}

	return data
}

// GenerateCompletion generates completion scripts for the given shell and program name
func (p *Parser) GenerateCompletion(shell, programName string) string {
	generator := completion.GetGenerator(shell)
	return generator.Generate(programName, p.GetCompletionData())
}

// PrintUsage pretty prints accepted Flags and Commands to io.Writer.
func (p *Parser) PrintUsage(writer io.Writer) {
	_, _ = writer.Write([]byte(fmt.Sprintf("usage: %s", []byte(os.Args[0]))))
	p.PrintFlags(writer)
	if p.registeredCommands.Count() > 0 {
		_, _ = writer.Write([]byte("\ncommands:\n"))
		p.PrintCommands(writer)
	}
}

// PrintUsageWithGroups pretty prints accepted Flags and show command-specific Flags grouped by Commands to io.Writer.
func (p *Parser) PrintUsageWithGroups(writer io.Writer) {
	// Print the program usage
	_, _ = writer.Write([]byte(fmt.Sprintf("usage: %s\n", os.Args[0])))

	// Print global flags
	p.PrintGlobalFlags(writer)

	// Print command-specific flags and commands
	if p.registeredCommands.Count() > 0 {
		_, _ = writer.Write([]byte("\nCommands:\n"))
		p.PrintCommandsWithFlags(writer, &PrettyPrintConfig{
			NewCommandPrefix:     " +  ",
			DefaultPrefix:        " │─ ",
			TerminalPrefix:       " └─ ",
			InnerLevelBindPrefix: " ** ",
			OuterLevelBindPrefix: " |  ",
		})
	}
}

// PrintGlobalFlags prints global (non-command-specific) flags
func (p *Parser) PrintGlobalFlags(writer io.Writer) {
	_, _ = writer.Write([]byte("\nGlobal Flags:\n\n"))

	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == "" { // Global flags have no command path
			shortFlag, _ := p.GetShortFlag(*f.Key)
			requiredOrOptional := describeRequired(f.Value.Argument)
			_, _ = writer.Write([]byte(fmt.Sprintf(" --%s or -%s \"%s\" (%s)\n", *f.Key, shortFlag, f.Value.Argument.Description, requiredOrOptional)))
		}
	}
}

// PrintCommandsWithFlags prints commands with their respective flags
func (p *Parser) PrintCommandsWithFlags(writer io.Writer, config *PrettyPrintConfig) {
	for kv := p.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		if kv.Value.topLevel {
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
				command := fmt.Sprintf("%s%s%s \"%s\"\n", prefix, strings.Repeat(config.InnerLevelBindPrefix, level), cmd.path, cmd.Description)
				if _, err := writer.Write([]byte(command)); err != nil {
					return false
				}

				// Print flags specific to this command
				p.PrintCommandSpecificFlags(writer, cmd.path, level, config)

				return true
			}, 0)
		}
	}
}

// PrintCommandSpecificFlags print flags for a specific command with the appropriate indentation
func (p *Parser) PrintCommandSpecificFlags(writer io.Writer, commandPath string, level int, config *PrettyPrintConfig) {
	hasFlags := false
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == commandPath {
			if !hasFlags {
				hasFlags = true
			}

			flagParts := splitPathFlag(*f.Key)
			flagDesc := fmt.Sprintf("--%s", flagParts[0])
			if f.Value.Argument != nil && f.Value.Argument.Short != "" {
				flagDesc = fmt.Sprintf("%s or -%s", flagDesc, f.Value.Argument.Short)
			}

			requiredOrOptional := describeRequired(f.Value.Argument)
			flag := fmt.Sprintf("%s%s \"%s\" (%s)\n", strings.Repeat(config.OuterLevelBindPrefix, level+1), flagDesc, f.Value.Argument.Description, requiredOrOptional)

			_, _ = writer.Write([]byte(flag))
		}
	}
}

// PrintFlags pretty prints accepted command-line switches to io.Writer
func (p *Parser) PrintFlags(writer io.Writer) {
	var shortOption string
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		shortFlag, err := p.GetShortFlag(*f.Key)
		if err == nil {
			shortOption = " or -" + shortFlag + " "
		} else {
			shortOption = " "
		}

		requiredOrOptional := describeRequired(f.Value.Argument)
		_, _ = writer.Write([]byte(fmt.Sprintf("\n --%s %s\"%s\" (%s)",
			*f.Key, shortOption, p.GetDescription(*f.Key), requiredOrOptional)))
	}
}

// PrintCommands writes the list of accepted Command structs to io.Writer.
func (p *Parser) PrintCommands(writer io.Writer) {
	p.PrintCommandsUsing(writer, &PrettyPrintConfig{
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
// command root. The Command root is at Level 0.
func (p *Parser) PrintCommandsUsing(writer io.Writer, config *PrettyPrintConfig) {
	for kv := p.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		if kv.Value.topLevel {
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

// SetTerminalReader sets the terminal reader for secure input (by default, the terminal reader is the real terminal)
// this is useful for testing or mocking the terminal reader or for setting a custom terminal reader
// the returned value is the old terminal reader, so it can be restored later
// this is a low-level function and should not be used by most users - by default terminal reader is nil and the real terminal is used
func (p *Parser) SetTerminalReader(t util.TerminalReader) util.TerminalReader {
	current := p.terminalReader
	p.terminalReader = t
	return current
}

// GetTerminalReader returns the current terminal reader
// this is a low-level function and should not be used by most users - by default terminal reader is nil and the real terminal is used
func (p *Parser) GetTerminalReader() util.TerminalReader {
	return p.terminalReader
}

// SetStderr sets the stderr writer and returns the old writer
// this is a low-level function and should not be used by most users - by default stderr is os.Stderr
func (p *Parser) SetStderr(w io.Writer) io.Writer {
	current := p.stderr
	p.stderr = w
	return current
}

// GetStderr returns the current stderr writer
// this is a low-level function and should not be used by most users - by default stderr is os.Stderr
func (p *Parser) GetStderr() io.Writer {
	return p.stderr
}

// SetStdout sets the stdout writer and returns the old writer
// this is a low-level function and should not be used by most users - by default stdout is os.Stdout
func (p *Parser) SetStdout(w io.Writer) io.Writer {
	current := p.stdout
	p.stdout = w
	return current
}

// GetStdout returns the current stdout writer
// this is a low-level function and should not be used by most users - by default stdout is os.Stdout
func (p *Parser) GetStdout() io.Writer {
	return p.stdout
}

// SetMaxDependencyDepth sets the maximum allowed depth for flag dependencies.
// If depth is less than 1, it will be set to DefaultMaxDependencyDepth.
func (p *Parser) SetMaxDependencyDepth(depth int) {
	if depth < 1 {
		depth = DefaultMaxDependencyDepth
	}
	p.maxDependencyDepth = depth
}

// GetMaxDependencyDepth returns the current maximum allowed depth for flag dependencies.
// If not explicitly set, returns DefaultMaxDependencyDepth.
func (p *Parser) GetMaxDependencyDepth() int {
	if p.maxDependencyDepth == 0 {
		return DefaultMaxDependencyDepth
	}
	return p.maxDependencyDepth
}

// Path returns the full path of the command
func (c *Command) Path() string {
	return c.path
}

// IsTopLevel returns whether this is a top-level command
func (c *Command) IsTopLevel() bool {
	return c.topLevel
}
