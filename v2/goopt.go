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
	"errors"
	"fmt"
	"sync"

	"github.com/napalu/goopt/v2/input"
	"github.com/napalu/goopt/v2/internal/messages"
	"github.com/napalu/goopt/v2/validation"
	"golang.org/x/text/language"

	"io"
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

// NewParser convenience initialization method. Use NewCmdLine to
// configure CmdLineOption using option functions.
func NewParser() *Parser {
	defaultBundle := i18n.Default()
	systemBundle := i18n.NewEmptyBundle()

	// Create layered provider
	layeredProvider := i18n.NewLayeredMessageProvider(defaultBundle, systemBundle, nil)

	p := &Parser{
		acceptedFlags:        orderedmap.NewOrderedMap[string, *FlagInfo](),
		lookup:               map[string]string{},
		options:              map[string]string{},
		errors:               []error{},
		bind:                 make(map[string]interface{}, 1),
		customBind:           map[string]ValueSetFunc{},
		registeredCommands:   orderedmap.NewOrderedMap[string, *Command](),
		commandOptions:       orderedmap.NewOrderedMap[string, bool](),
		positionalArgs:       []PositionalArgument{},
		repeatedFlags:        map[string]bool{},
		listFunc:             matchChainedSeparators,
		callbackQueue:        queue.New[*Command](),
		callbackResults:      map[string]error{},
		secureArguments:      orderedmap.NewOrderedMap[string, *types.Secure](),
		prefixes:             []rune{'-'},
		stderr:               os.Stderr,
		stdout:               os.Stdout,
		flagNameConverter:    DefaultFlagNameConverter,
		commandNameConverter: DefaultCommandNameConverter,
		maxDependencyDepth:   DefaultMaxDependencyDepth,
		defaultBundle:        defaultBundle,
		systemBundle:         systemBundle,
		layeredProvider:      layeredProvider,
		helpConfig:           DefaultHelpConfig,
		autoHelp:             true,
		helpFlags:            []string{"help", "h"},
		helpExecuted:         false,
		helpEndFunc: func() error {
			os.Exit(0)
			return nil
		},
		autoRegisteredHelp:      make(map[string]bool),
		autoVersion:             true,
		versionFlags:            []string{"version", "v"},
		showVersionInHelp:       false,
		versionExecuted:         false,
		autoRegisteredVersion:   make(map[string]bool),
		autoLanguage:            true,
		languageEnvVar:          "GOOPT_LANG",
		languageFlags:           []string{"language", "lang", "l"},
		autoRegisteredLanguage:  make(map[string]bool),
		globalPreHooks:          []PreHookFunc{},
		globalPostHooks:         []PostHookFunc{},
		commandPreHooks:         make(map[string][]PreHookFunc),
		commandPostHooks:        make(map[string][]PostHookFunc),
		hookOrder:               OrderGlobalFirst,
		translationRegistry:     nil, // Will be initialized after parser is created
		flagSuggestionThreshold: 2,   // Default threshold for flag suggestions
		cmdSuggestionThreshold:  2,   // Default threshold for command suggestions
		mu:                      sync.Mutex{},
	}
	p.translationRegistry = NewJITTranslationRegistry(p)
	p.renderer = NewRenderer(p)

	return p
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
func NewParserFromStruct[T any](structWithTags *T, config ...ConfigureCmdLineFunc) (*Parser, error) {
	return NewParserFromStructWithLevel(structWithTags, 5, config...)
}

// NewParserFromStructWithLevel parses a struct and binds its fields to command-line flags up to maxDepth levels
func NewParserFromStructWithLevel[T any](structWithTags *T, maxDepth int, config ...ConfigureCmdLineFunc) (*Parser, error) {
	p, err := newParserFromReflectValue(reflect.ValueOf(structWithTags), "", "", maxDepth, 0, config...)
	if p != nil {
		p.structCtx = structWithTags
	}

	return p, err
}

// NewParserFromInterface creates a new parser from an interface{} that should be a struct or a pointer to a struct
func NewParserFromInterface(i interface{}, config ...ConfigureCmdLineFunc) (*Parser, error) {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		// If not a pointer, create one
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		v = ptr
	}

	p, err := newParserFromReflectValue(v, "", "", 5, 0, config...)
	if p != nil {
		p.structCtx = i
	}

	return p, err
}

// SetExecOnParse executes command callbacks as soon as the command and associated flags have been parsed *during* the
// Parse call. This is useful for executing commands which may require setting configuration or flag values
// during command execution.
func (p *Parser) SetExecOnParse(value bool) {
	p.callbackOnParse = value
}

// SetExecOnParseComplete sets whether command callbacks should execute upon successful parsing completion.
func (p *Parser) SetExecOnParseComplete(value bool) {
	p.callbackOnParseComplete = value
}

// SetLanguage sets the parser language to the specified language tag.
// This sets the language at the layered provider level, affecting all translations.
func (p *Parser) SetLanguage(lang language.Tag) error {
	// Set language at the provider level - this is now the single source of truth
	p.layeredProvider.SetDefaultLanguage(lang)
	return nil
}

// GetLanguage returns the current language configured for the Parser, sourced from the layered message provider.
// This returns the actual matched language, which may differ from what was requested if language matching was used.
func (p *Parser) GetLanguage() language.Tag {
	return p.layeredProvider.GetDefaultLanguage()
}

// GetSupportedLanguages returns all languages supported across all bundles (default, system, and user)
func (p *Parser) GetSupportedLanguages() []language.Tag {
	langs := make(map[language.Tag]bool)

	// Collect from all bundles
	if p.defaultBundle != nil {
		for _, lang := range p.defaultBundle.Languages() {
			langs[lang] = true
		}
	}
	if p.systemBundle != nil {
		for _, lang := range p.systemBundle.Languages() {
			langs[lang] = true
		}
	}
	if p.userI18n != nil {
		for _, lang := range p.userI18n.Languages() {
			langs[lang] = true
		}
	}

	// Convert to slice
	result := make([]language.Tag, 0, len(langs))
	for lang := range langs {
		result = append(result, lang)
	}

	return result
}

// SetEndHelpFunc sets a custom function to be executed at the end of the help output. By default, os.exit(0) is called
// after help is shown, with the assumption that no further processing is expected after help is shown. This
// behaviour can be overridden by setting a custom endFunc. You can then check WasHelpShown do determine if help
// was displayed.
func (p *Parser) SetEndHelpFunc(endFunc func() error) {
	p.helpEndFunc = endFunc
}

// SetCommandNameConverter allows setting a custom name converter for command names
func (p *Parser) SetCommandNameConverter(converter NameConversionFunc) NameConversionFunc {
	oldConverter := p.commandNameConverter
	p.commandNameConverter = converter

	return oldConverter
}

// SetSystemLocales loads and sets system locales and their translations into the parser's system bundle. Returns an error on failure.
func (p *Parser) SetSystemLocales(locales ...i18n.Locale) error {
	for _, locale := range locales {
		if errLoad := p.systemBundle.LoadFromString(locale.Tag, locale.Translations); errLoad != nil {
			return errLoad
		}
	}

	return nil
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

// SetRenderer allows overriding the built-in flag and command renderer used for formatting flags and commands
func (p *Parser) SetRenderer(customRenderer Renderer) {
	p.renderer = customRenderer
}

// GetTranslator returns the translator interface for i18n operations.
// Use this to access both translation methods (T, TL) and locale-aware formatting (GetPrinter).
func (p *Parser) GetTranslator() i18n.Translator {
	return p.layeredProvider
}

// GetStructCtx returns the current struct context stored within the Parser instance.
// The struct context is a pointer to the struct that was supplied to NewParserFromStruct, NewParserFromStructWithLevel
// or NewParserFromInterface.
func (p *Parser) GetStructCtx() interface{} {
	return p.structCtx
}

// HasStructCtx returns true if the Parser instance has a struct context stored.
// The struct context is a pointer to the struct that was supplied to NewParserFromStruct, NewParserFromStructWithLevel
// or NewParserFromInterface.
func (p *Parser) HasStructCtx() bool {
	return p.structCtx != nil
}

// GetStructCtxAs returns the current struct context stored within the Parser instance as a specific type.
// The struct context is a pointer to the struct that was supplied to NewParserFromStruct, NewParserFromStructWithLevel
// or NewParserFromInterface.
func GetStructCtxAs[T any](p *Parser) (T, bool) {
	var zero T

	if p == nil || p.structCtx == nil {
		return zero, false
	}

	val, ok := p.structCtx.(T)
	return val, ok
}

// ExecuteCommands command callbacks are placed on a FIFO queue during parsing until ExecuteCommands is called.
// Returns the count of errors encountered during execution.
func (p *Parser) ExecuteCommands() int {
	callbackErrors := 0
	for p.callbackQueue.Len() > 0 {
		cmd, _ := p.callbackQueue.Pop()
		if cmd.Callback != nil {
			// Execute pre-hooks
			preErr := p.executePreHooks(cmd)
			if preErr != nil {
				p.callbackResults[cmd.path] = preErr
				callbackErrors++
				// Execute post-hooks even on pre-hook failure
				_ = p.executePostHooks(cmd, preErr)
				continue
			}

			// Execute the command
			cmdErr := cmd.Callback(p, cmd)
			p.callbackResults[cmd.path] = cmdErr
			if cmdErr != nil {
				callbackErrors++
			}

			// Execute post-hooks
			postErr := p.executePostHooks(cmd, cmdErr)
			if postErr != nil && cmdErr == nil {
				// Only count post-hook error if command succeeded
				p.callbackResults[cmd.path] = postErr
				callbackErrors++
			}
		}
	}

	return callbackErrors
}

// ExecuteCommand command callbacks are placed on a FIFO queue during parsing until ExecuteCommands is called.
// Returns the error which occurred during execution of a command callback.
func (p *Parser) ExecuteCommand() error {
	if p.callbackQueue.Len() > 0 {
		cmd, _ := p.callbackQueue.Pop()
		if cmd.Callback != nil {
			// Execute pre-hooks
			if preErr := p.executePreHooks(cmd); preErr != nil {
				p.callbackResults[cmd.path] = preErr
				// Execute post-hooks even on pre-hook failure
				_ = p.executePostHooks(cmd, preErr)
				return preErr
			}

			// Execute the command
			cmdErr := cmd.Callback(p, cmd)
			p.callbackResults[cmd.path] = cmdErr

			// Execute post-hooks
			if postErr := p.executePostHooks(cmd, cmdErr); postErr != nil {
				if cmdErr == nil {
					p.callbackResults[cmd.path] = postErr
					return postErr
				}
			}

			return cmdErr
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

	return errs.ErrCommandNotFound.WithArgs(commandName)
}

// GetCommandExecutionErrors returns the errors which occurred during execution of command callbacks
// after ExecuteCommands has been called. Returns a KeyValue list of command name and error
func (p *Parser) GetCommandExecutionErrors() []types.KeyValue[string, error] {
	var errors []types.KeyValue[string, error]
	for key, err := range p.callbackResults {
		if err != nil {
			errors = append(errors, types.KeyValue[string, error]{Key: key, Value: err})
		}
	}

	return errors
}

// AddFlagPreValidationFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
// *before* AcceptedValues and Validators are checked
func (p *Parser) AddFlagPreValidationFilter(flag string, proc FilterFunc, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		flagInfo.Argument.PreFilter = proc

		return nil
	}

	return errs.ErrFlagNotFound.WithArgs(flag)
}

// AddFlagPostValidationFilter adds a filter (user-defined transform/evaluate function) which is called on the Flag value during Parse
// *after* AcceptedValues and Validators are checked
func (p *Parser) AddFlagPostValidationFilter(flag string, proc FilterFunc, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	if flagInfo, found := p.acceptedFlags.Get(mainKey); found {
		flagInfo.Argument.PostFilter = proc

		return nil
	}

	return errs.ErrFlagNotFound.WithArgs(flag)
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

	return nil, errs.ErrNoPreValidationFilters.WithArgs(flag)
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

	return nil, errs.ErrNoPostValidationFilters.WithArgs(flag)
}

// HasAcceptedValues returns true when a Flag defines a set of valid values it will accept
//
// Deprecated. AcceptedValues is deprecated in favor of the more powerful and composable
// validation system. See HasValidators instead.
func (p *Parser) HasAcceptedValues(flag string, commandPath ...string) bool {
	flagInfo, found := p.acceptedFlags.Get(p.flagOrShortFlag(flag, commandPath...))
	if found {
		return len(flagInfo.Argument.AcceptedValues) > 0
	}

	return false
}

// HasValidators checks if the specified flag has any associated validators for the given command path.
func (p *Parser) HasValidators(flag string, commandPath ...string) bool {
	flagInfo, found := p.acceptedFlags.Get(p.flagOrShortFlag(flag, commandPath...))
	if found {
		return len(flagInfo.Argument.Validators) > 0
	}

	return false
}

// registerFlagTranslations registers flag metadata for JIT translation
func (p *Parser) registerFlagTranslations(flagName string, argument *Argument, commandPath ...string) {
	// With JIT, we always register metadata, even for flags without translation keys
	// This allows direct matching to work for all flags
	p.translationRegistry.RegisterFlagMetadata(flagName, argument, strings.Join(commandPath, " "))
}

// registerCommandTranslations registers command metadata for JIT translation
func (p *Parser) registerCommandTranslations(cmd *Command) {
	if cmd.NameKey == "" {
		return
	}

	// With JIT, we only register metadata
	p.translationRegistry.RegisterCommandMetadata(cmd.path, cmd)
}

// GetCanonicalFlagName returns the canonical name for a potentially translated flag name
func (p *Parser) GetCanonicalFlagName(name string) (string, bool) {
	return p.translationRegistry.GetCanonicalFlagName(name, p.GetLanguage())
}

// GetCanonicalCommandPath returns the canonical path for a potentially translated command name
func (p *Parser) GetCanonicalCommandPath(name string) (string, bool) {
	return p.translationRegistry.GetCanonicalCommandPath(name, p.GetLanguage())
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
func (p *Parser) Parse(args []string, defaults ...string) bool {
	p.ensureInit()
	pruneExecPathFromArgs(&args)

	// Auto-register help flags if enabled
	if err := p.ensureHelpFlags(); err != nil {
		p.addError(err)
		return false
	}

	// Auto-register version flags if enabled
	if err := p.ensureVersionFlags(); err != nil {
		p.addError(err)
		return false
	}

	// Auto-register language flags if enabled
	if err := p.ensureLanguageFlags(); err != nil {
		p.addError(err)
		return false
	}

	// Auto-detect language before showing help
	if p.autoLanguage {
		if lang := p.detectLanguageInArgs(args); lang != language.Und {
			// fmt.Printf("DEBUG: Detected language: %v\n", lang)
			p.SetLanguage(lang)
		}
	}

	// Early check for help request
	if p.autoHelp && p.hasHelpInArgs(args) {
		improvedParser := NewHelpParser(p, p.helpConfig)

		// Filter out language flags before passing to help parser
		helpArgs := p.filterLanguageFlags(args)

		// The improved parser will handle all parsing and error detection
		err := improvedParser.Parse(helpArgs)
		p.helpExecuted = true
		if p.helpEndFunc != nil {
			return p.helpEndFunc() == nil
		}

		// Return true if help was shown successfully
		return err == nil
	}

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

	state := parse.NewState(args, defaults...)
	if g, ok := envFlagsByCommand["global"]; ok && len(g) > 0 {
		state.InsertArgsAt(0, g...)
	}

	for state.Advance() {
		cur := state.CurrentArg()
		if p.isFlag(cur) {
			if p.isGlobalFlag(cur) {
				p.evalFlagWithPath(state, "")
			} else {
				// We now iterate over the stack to handle flags for each command context
				// We need to restore the state position for each command context
				// because the state position is relative to the command context
				flagProcessed := false
				for i := ctxStack.Len() - 1; i >= 0; i-- {
					cmdContext, ok := ctxStack.At(i)
					if ok {
						currentCommandPath = cmdContext
					} else {
						currentCommandPath = ""
					}

					originalPos := state.Pos()

					if p.evalFlagWithPath(state, currentCommandPath) {
						flagProcessed = true
						processedStack = true
						// Continue to process flag for other contexts that might also need it
					}

					// Only allow state advancement for the first command context
					if ctxStack.Len() > 1 && state.Pos() > originalPos {
						// For subsequent command contexts, restore the position
						state.SetPos(originalPos)
					}
				}

				// Check if we should try the fallback
				shouldTryFallback := !flagProcessed && ctxStack.Len() == 0

				if flagProcessed {
					processedStack = true
				}

				if processedStack && ctxStack.Len() > 1 {
					state.Skip()
				}

				// Try fallback for global/POSIX flags, or generate error
				if !flagProcessed {
					if shouldTryFallback {
						// No command context, try as global/POSIX flag
						if !p.evalFlagWithPath(state, "") {
							// Still not found, generate error
							flagName := strings.TrimLeftFunc(cur, p.prefixFunc)
							p.generateFlagError(flagName, currentCommandPath)
						}
					} else {
						// In command context and flag not found anywhere
						flagName := strings.TrimLeftFunc(cur, p.prefixFunc)
						p.generateFlagError(flagName, currentCommandPath)
					}
				}
			}

		} else {
			// Parse the next command
			terminating := p.parseCommand(state, cmdQueue, &commandPathSlice)
			currentCommandPath = strings.Join(commandPathSlice, " ")
			// Inject relevant environment variables for the current command context
			if instanceCount, exists := envInserted[currentCommandPath]; !exists || instanceCount < cmdQueue.Len() {
				if len(envFlagsByCommand[currentCommandPath]) > 0 {
					state.InsertArgsAt(state.Pos()+1, envFlagsByCommand[currentCommandPath]...)
				}
				envInserted[currentCommandPath]++
			}

			if lastCommandPath != "" {
				lastCommandPath = p.evalExecOnParse(lastCommandPath)
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
	if lastCommandPath != "" {
		_ = p.evalExecOnParse(lastCommandPath)
	}

	// Validate all processed options
	p.setPositionalArguments(state)
	p.validateProcessedOptions()

	// Process secure arguments if parsing succeeded
	success := len(p.errors) == 0
	if success {
		for f := p.secureArguments.Front(); f != nil; f = f.Next() {
			p.processSecureFlag(*f.Key, f.Value)
		}
		if p.callbackOnParseComplete && !p.callbackOnParse {
			numErrs := p.ExecuteCommands()
			if numErrs > 0 {
				for _, kv := range p.GetCommandExecutionErrors() {
					p.addError(errs.ErrProcessingCommand.Wrap(kv.Value).WithArgs(kv.Key))
				}
			}
		}
		success = len(p.errors) == 0

	}
	p.secureArguments = nil

	// Check for auto-version after successful parsing
	if p.autoVersion && p.IsVersionRequested() {
		p.PrintVersion(p.stdout)
		p.versionExecuted = true
	}

	// Run validation hook if set and parsing was successful so far
	if success && p.validationHook != nil {
		if err := p.validationHook(p); err != nil {
			p.addError(err)
			success = false
		}
	}

	// fmt.Printf("DEBUG Parse: returning success=%v, errors=%d\n", success, len(p.errors))
	return success
}

// SetHelpBehavior sets the help output behavior
func (p *Parser) SetHelpBehavior(behavior HelpBehavior) {
	p.helpBehavior = behavior
}

// shouldUseStderrForHelp determines if help should use stderr based on context
func (p *Parser) shouldUseStderrForHelp(isError bool) bool {
	switch p.helpBehavior {
	case HelpBehaviorStderr:
		return true
	case HelpBehaviorSmart:
		return isError
	default:
		return false
	}
}

// PrintHelpWithContext prints help to the appropriate output stream
func (p *Parser) PrintHelpWithContext(isError bool) {
	writer := p.stdout
	if p.shouldUseStderrForHelp(isError) {
		writer = p.stderr
	}
	p.PrintHelp(writer)
}

// GetHelpWriter returns the appropriate writer for help output
func (p *Parser) GetHelpWriter(isError bool) io.Writer {
	if p.shouldUseStderrForHelp(isError) {
		return p.stderr
	}
	return p.stdout
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

	defaultArgs := make([]string, 0, len(defaults))
	for key, val := range defaults {
		if _, found := argMap[key]; !found {
			defaultArgs = append(defaultArgs, string(p.prefixes[0])+key)
			defaultArgs = append(defaultArgs, val)
		}
	}

	return p.Parse(args, defaultArgs...)
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
	mainKey := p.flagOrShortFlag(lookup, commandPath...)
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
					p.addError(errs.ErrSettingBoundValue.Wrap(err).WithArgs(flag))
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
		return false, errs.ErrFlagNotFound.WithArgs(flag)
	}

	val, err := strconv.ParseBool(value)

	if err != nil {
		return false, errs.ErrParseBool.Wrap(err).WithArgs(value)
	}

	return val, nil
}

// GetInt attempts to convert the string value of a Flag to an int64.
func (p *Parser) GetInt(flag string, bitSize int, commandPath ...string) (int64, error) {
	value, success := p.Get(flag, commandPath...)
	if !success {
		return 0, errs.ErrFlagNotFound.WithArgs(flag)
	}

	val, err := strconv.ParseInt(value, 10, bitSize)
	if err != nil {
		return 0, errs.ErrParseInt.Wrap(err).WithArgs(value, bitSize)
	}

	return val, nil
}

// GetFloat attempts to convert the string value of a Flag to a float64
func (p *Parser) GetFloat(flag string, bitSize int, commandPath ...string) (float64, error) {
	value, success := p.Get(flag, commandPath...)
	if !success {
		return 0, errs.ErrFlagNotFound.WithArgs(flag)
	}

	val, err := strconv.ParseFloat(value, bitSize)
	if err != nil {
		return 0, errs.ErrParseFloat.Wrap(err).WithArgs(value, bitSize)
	}

	return val, nil
}

// GetList attempts to split the string value of a Chained Flag to a string slice
// by default the value is split on '|', ',' or ' ' delimiters
func (p *Parser) GetList(flag string, commandPath ...string) ([]string, error) {
	arg, err := p.GetArgument(flag, commandPath...)
	if err == nil {
		if arg.TypeOf == types.Chained {
			value, success := p.Get(flag, commandPath...)
			if !success {
				return []string{}, errs.ErrFlagValueNotRetrieved.WithArgs(flag)
			}

			listDelimFunc := p.getListDelimiterFunc()

			return strings.FieldsFunc(value, listDelimFunc), nil
		}

		return []string{}, errs.ErrInvalidArgumentType.WithArgs(flag, types.Chained)
	}

	return []string{}, err
}

// SetListDelimiterFunc sets the value delimiter function for Chained flags
func (p *Parser) SetListDelimiterFunc(delimiterFunc types.ListDelimiterFunc) error {
	if delimiterFunc != nil {
		p.listFunc = delimiterFunc

		return nil
	}

	return errs.ErrInvalidListDelimiterFunc
}

// SetArgumentPrefixes sets the flag argument prefixes
func (p *Parser) SetArgumentPrefixes(prefixes []rune) error {
	prefixesLen := len(prefixes)
	if prefixesLen == 0 {
		return errs.ErrEmptyArgumentPrefixList
	}

	p.prefixes = prefixes

	return nil
}

func (p *Parser) SetUserBundle(bundle *i18n.Bundle) error {
	if bundle == nil {
		return errs.ErrNilPointer.WithArgs("bundle")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.userI18n = bundle
	p.layeredProvider.SetUserBundle(bundle)

	return nil
}

// ReplaceDefaultBundle is deprecated. Use ExtendSystemBundle instead.
// This method now extends the system bundle with translations from the provided bundle.
//
// Deprecated: using ExtendSystemBundle instead
func (p *Parser) ReplaceDefaultBundle(bundle *i18n.Bundle) error {
	if bundle == nil {
		return errs.ErrNilPointer.WithArgs("bundle")
	}

	return p.ExtendSystemBundle(bundle)
}

// ExtendSystemBundle adds translations from the provided bundle to the parser's system bundle.
// This allows parser-specific translations without modifying global state.
func (p *Parser) ExtendSystemBundle(bundle *i18n.Bundle) error {
	if bundle == nil {
		return errs.ErrNilPointer.WithArgs("bundle")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Copy all translations from the provided bundle to the system bundle
	for _, lang := range bundle.Languages() {
		if translations := bundle.GetTranslations(lang); len(translations) > 0 {
			if err := p.systemBundle.AddLanguage(lang, translations); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Parser) GetSystemBundle() *i18n.Bundle {
	return p.systemBundle
}

// GetUserBundle returns the user-defined i18n bundle if available, otherwise returns nil.
func (p *Parser) GetUserBundle() *i18n.Bundle {
	return p.userI18n
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

	// Add naming consistency warnings
	warnings = append(warnings, p.checkNamingConsistency()...)

	return warnings
}

// GetOptions returns a slice of KeyValue pairs which have been supplied on the command-line.
// Note: Short flags are always resolved to their long form in the returned options.
// For example, if "-d" is specified on the command line and maps to "--debug",
// the returned option will use "debug" as the key.
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
		return errs.ErrEmptyFlag
	}

	// Use the helper function to generate the lookup key
	lookupFlag := buildPathFlag(flag, commandPath...)

	// Ensure no duplicate flags for the same command path or globally
	if _, exists := p.acceptedFlags.Get(lookupFlag); exists {
		return errs.ErrFlagAlreadyExists.WithArgs(lookupFlag)
	}

	if lenS := len(argument.Short); lenS > 0 {
		if p.posixCompatible && lenS > 1 {
			return errs.ErrPosixShortForm.WithArgs(flag, argument.Short)
		}

		// NEW: Use context-aware conflict checking
		if conflictingFlag, hasConflict := p.checkShortFlagConflict(argument.Short, lookupFlag, commandPath...); hasConflict {
			// Determine the context of the conflict
			conflictContext := ""
			if strings.Contains(conflictingFlag, "@") {
				parts := strings.Split(conflictingFlag, "@")
				if len(parts) > 1 {
					conflictContext = " in context '" + parts[1] + "'"
				}
			}
			currentContext := ""
			if len(commandPath) > 0 {
				currentContext = " in context '" + strings.Join(commandPath, " ") + "'"
			}
			return errs.ErrShortFlagConflictContext.WithArgs(argument.Short, conflictingFlag, conflictContext, flag, currentContext)
		}

		p.storeShortFlag(argument.Short, lookupFlag, commandPath...)
	}

	p.lookup[argument.uniqueID] = flag

	if argument.TypeOf == types.Empty {
		argument.TypeOf = types.Single
	}

	p.acceptedFlags.Set(lookupFlag, &FlagInfo{
		Argument:    argument,
		CommandPath: strings.Join(commandPath, " "), // Keep track of the command path
	})

	// Always register flag metadata for translation lookup
	p.registerFlagTranslations(flag, argument, commandPath...)

	if argument.Capacity < 0 {
		return errs.ErrNegativeCapacity.WithArgs(flag, argument.Capacity)
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
		return errs.ErrNilPointer
	}

	return s.BindFlag(data, flag, argument, commandPath...)
}

// CustomBindFlagToParser is a helper function to allow passing generics to the Parser.CustomBindFlag method
func CustomBindFlagToParser[T any](s *Parser, data *T, proc ValueSetFunc, flag string, argument *Argument, commandPath ...string) error {
	if s == nil {
		return errs.ErrNilPointer
	}

	return s.CustomBindFlag(data, proc, flag, argument, commandPath...)
}

// BindFlag is used to bind a *pointer* to string, int, uint, bool, float or time.Time scalar or slice variable with a Flag
// which is set when Parse is invoked.
// An error is returned if data cannot be bound - for compile-time safety use BindFlagToParser instead
func (p *Parser) BindFlag(bindPtr interface{}, flag string, argument *Argument, commandPath ...string) error {
	if bindPtr == nil {
		return errs.ErrNilPointer
	}
	if ok, err := util.CanConvert(bindPtr, argument.TypeOf); !ok {
		return err
	}

	v := reflect.ValueOf(bindPtr)
	if v.Kind() != reflect.Ptr {
		return errs.ErrNonPointerVar
	}

	elem := v.Elem()
	if !elem.IsValid() {
		return errs.ErrBindInvalidValue
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
		return errs.ErrPointerExpected
	}

	if !reflect.ValueOf(data).Elem().IsValid() {
		return errs.ErrBindInvalidValue
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
//
// Deprecated. AcceptPattern is deprecated in favor of the more powerful and composable validation system.
// See the validation guide for more details.
func (p *Parser) AcceptPattern(flag string, val types.PatternValue, commandPath ...string) error {
	return p.AcceptPatterns(flag, []types.PatternValue{val}, commandPath...)
}

// AcceptPatterns same as AcceptPattern but acts on a list of patterns and descriptions. When specified, the patterns defined
// in AcceptPatterns represent a set of values, of which one must be supplied on the command-line. The patterns are evaluated
// on Parse, if no command-line options match one of the PatternValue, Parse returns false.
//
// Deprecated. AcceptPatterns is deprecated in favor of the more powerful and composable validation system.
// See the validation guide for more details.
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
			return errs.ErrRegexCompile.WithArgs(acceptVal[i].Pattern, err)
		}
		acceptVal[i].Compiled = re
	}

	return nil
}

// GetAcceptPatterns takes a flag string and returns an error if the flag does not exist, a slice of LiterateRegex otherwise
//
// Deprecated. GetAcceptPatterns is deprecated in favor of the more powerful and composable validation system.
// See the validation guide for more details.
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
		return nil, errs.ErrOptionNotSet.WithArgs(flag)
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

		return "", errs.ErrShortFlagUndefined.WithArgs(flag)
	}

	return "", err
}

// HasFlag returns true when the Flag has been seen on the command line.
func (p *Parser) HasFlag(flag string, commandPath ...string) bool {
	// First try canonical lookup
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

	// If not found, try translation lookup
	if !found {
		if canonical, ok := p.translationRegistry.GetCanonicalFlagName(flag, p.GetLanguage()); ok {
			// Try again with the canonical name
			mainKey = p.flagOrShortFlag(canonical, commandPath...)
			_, found = p.options[mainKey]
			if !found && p.secureArguments != nil {
				// secure arguments are evaluated after all others - if a callback (ex. RequiredIf) relies
				// on HasFlag during Parse then we need to check secureArguments - we only do this
				// if the argument has been passed on the command line
				flagParts := splitPathFlag(mainKey)
				if _, found = p.rawArgs[flagParts[0]]; found {
					_, found = p.secureArguments.Get(mainKey)
				}
			}
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
	// First try canonical lookup
	_, found := p.commandOptions.Get(path)

	// If not found, try translation lookup
	if !found {
		if canonical, ok := p.translationRegistry.GetCanonicalCommandPath(path, p.GetLanguage()); ok {
			// Try again with the canonical path
			_, found = p.commandOptions.Get(canonical)
		}
	}

	return found
}

// ClearErrors removes all errors stored in the parser without affecting other state.
// This is useful when you want to retry operations after handling previous errors.
func (p *Parser) ClearErrors() {
	p.errors = p.errors[:0]
}

// DescribeFlag is used to provide a description of a Flag
func (p *Parser) DescribeFlag(flag, description string, commandPath ...string) error {
	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if found {
		flagInfo.Argument.Description = description

		return nil
	}

	return errs.ErrFlagNotFound.WithArgs(flag)
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
		return errs.ErrCommandNotFound.WithArgs(commandPath)
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
	_, found := p.acceptedFlags.Get(mainKey)
	if found {
		delete(p.options, mainKey)
		p.acceptedFlags.Delete(mainKey)

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
		return errs.ErrDependencyOnEmptyFlag
	}

	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return errs.ErrFlagNotFound.WithArgs(flag)
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
		return errs.ErrDependencyOnEmptyFlag
	}

	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return errs.ErrFlagNotFound.WithArgs(flag)
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
		return errs.ErrDependencyOnEmptyFlag
	}

	mainKey := p.flagOrShortFlag(flag, commandPath...)
	flagInfo, found := p.acceptedFlags.Get(mainKey)
	if !found {
		return errs.ErrFlagNotFound.WithArgs(flag)
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
	out := make([]error, len(p.errors))
	var te i18n.TranslatableError
	for i, e := range p.errors {
		if errors.As(e, &te) {
			out[i] = errs.WithProvider(te, p.layeredProvider)
		} else {
			out[i] = e
		}
	}
	return out
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

		addFlagToCompletionData(&data, cmd, flagName, flagInfo, p.renderer)
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
	// Show version in header if configured
	if p.showVersionInHelp && (p.version != "" || p.versionFunc != nil) {
		fmt.Fprintf(writer, "%s %s\n\n", filepath.Base(os.Args[0]), p.GetVersion())
	}

	_, _ = writer.Write([]byte(p.layeredProvider.GetFormattedMessage(messages.MsgUsageKey, os.Args[0]) + "\n"))
	p.PrintPositionalArgs(writer)
	p.PrintFlags(writer)
	if p.registeredCommands.Count() > 0 {
		_, _ = writer.Write([]byte(fmt.Sprintf("\n%s:\n", p.layeredProvider.GetMessage(messages.MsgCommandsKey))))
		p.PrintCommands(writer)
	}
}

// PrintUsageWithGroups pretty prints accepted Flags and show command-specific Flags grouped by Commands to io.Writer.
func (p *Parser) PrintUsageWithGroups(writer io.Writer, config ...*PrettyPrintConfig) {
	_, _ = writer.Write([]byte(p.layeredProvider.GetFormattedMessage(messages.MsgUsageKey, os.Args[0]) + "\n"))
	var prettyPrintConfig *PrettyPrintConfig
	if len(config) > 0 {
		prettyPrintConfig = config[0]
	} else {
		prettyPrintConfig = &PrettyPrintConfig{
			NewCommandPrefix:     " +  ",
			DefaultPrefix:        " ├─ ",
			TerminalPrefix:       " └─ ",
			InnerLevelBindPrefix: " ** ",
			OuterLevelBindPrefix: " │  ",
		}
	}

	p.PrintPositionalArgs(writer)
	p.PrintGlobalFlags(writer)

	// Print command-specific flags and commands
	if p.registeredCommands.Count() > 0 {
		_, _ = writer.Write([]byte(fmt.Sprintf("\n%s:\n", p.layeredProvider.GetMessage(messages.MsgCommandsKey))))
		p.PrintCommandsWithFlags(writer, prettyPrintConfig)
	}
}

// PrintPositionalArgs prints information about positional arguments
func (p *Parser) PrintPositionalArgs(writer io.Writer) {
	var args []PositionalArgument

	// Collect all flags marked as positional
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.Argument != nil && f.Value.Argument.isPositional() {
			if f.Value.Argument.Position == nil {
				continue
			}

			args = append(args, PositionalArgument{
				Position: *f.Value.Argument.Position,
				Value:    *f.Key,
				Argument: f.Value.Argument,
			})
		}
	}

	// Sort by position
	sort.SliceStable(args, func(i, j int) bool {
		return args[i].Position < args[j].Position
	})

	// Print args with indices
	if len(args) > 0 {
		_, _ = writer.Write([]byte(fmt.Sprintf("\n%s:\n", p.layeredProvider.GetMessage(messages.MsgPositionalArgumentsKey))))
		for _, arg := range args {
			_, _ = writer.Write([]byte(fmt.Sprintf(" %s \"%s\" (%s: %d)\n",
				arg.Value,
				p.renderer.FlagDescription(arg.Argument),
				p.layeredProvider.GetMessage(messages.MsgPositionalKey),
				arg.Position)))
		}
	}

}

// PrintGlobalFlags prints global (non-command-specific) flags
func (p *Parser) PrintGlobalFlags(writer io.Writer) {
	_, _ = writer.Write([]byte(fmt.Sprintf("\n%s:\n\n", p.layeredProvider.GetMessage(messages.MsgGlobalFlagsKey))))

	count := 0
	totalGlobals := 0

	// First count total globals
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.Argument.isPositional() {
			continue
		}
		if f.Value.CommandPath == "" {
			totalGlobals++
		}
	}

	// Print globals up to MaxGlobals limit
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.Argument.isPositional() {
			continue
		}
		if f.Value.CommandPath == "" { // Global flags have no command path
			if p.helpConfig.MaxGlobals > 0 && count >= p.helpConfig.MaxGlobals {
				remaining := totalGlobals - count
				if remaining > 0 {
					_, _ = writer.Write([]byte(fmt.Sprintf(" ... %s %d %s\n",
						p.layeredProvider.GetMessage(messages.MsgAndKey),
						remaining,
						p.layeredProvider.GetMessage(messages.MsgMoreKey))))
				}
				break
			}
			_, _ = writer.Write([]byte(fmt.Sprintf(" %s\n", p.renderer.FlagUsage(f.Value.Argument))))
			count++
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
				switch {
				case level == 0:
					prefix = config.NewCommandPrefix
				case len(cmd.Subcommands) == 0:
					prefix = config.TerminalPrefix
				default:
					prefix = config.DefaultPrefix
				}

				// Print the command itself with proper indentation
				commandStr := cmd.path
				if p.helpConfig.ShowDescription {
					desc := p.renderer.CommandDescription(cmd)
					if desc != "" {
						commandStr += " \"" + desc + "\""
					}
				}
				command := fmt.Sprintf("%s%s%s\n", prefix, strings.Repeat(config.InnerLevelBindPrefix, level), commandStr)
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
	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		if f.Value.CommandPath == commandPath {
			flag := fmt.Sprintf("%s%s\n", strings.Repeat(config.OuterLevelBindPrefix, level+1), p.renderer.FlagUsage(f.Value.Argument))

			_, _ = writer.Write([]byte(flag))
		}
	}
}

// PrintFlags pretty prints accepted command-line switches to io.Writer
func (p *Parser) PrintFlags(writer io.Writer) {
	// Track which flags we've already printed to avoid duplicates
	printedFlags := make(map[string]bool)

	for f := p.acceptedFlags.Front(); f != nil; f = f.Next() {
		// Extract the base flag name (without command path)
		flagKey := *f.Key
		flagParts := splitPathFlag(flagKey)
		baseFlagName := flagParts[0] // Flag name is the first part

		// Skip if we've already printed this flag
		if printedFlags[baseFlagName] {
			continue
		}

		// Skip positional arguments
		if f.Value.Argument.isPositional() {
			continue
		}

		// Mark as printed and output
		printedFlags[baseFlagName] = true
		_, _ = writer.Write([]byte(fmt.Sprintf(" %s\n", p.renderer.FlagUsage(f.Value.Argument))))
	}
}

// SetHelpStyle sets the help output style
func (p *Parser) SetHelpStyle(style HelpStyle) {
	p.helpConfig.Style = style
}

// SetHelpConfig sets the complete help configuration
func (p *Parser) SetHelpConfig(config HelpConfig) {
	p.helpConfig = config
}

// GetHelpConfig returns the current help configuration
func (p *Parser) GetHelpConfig() HelpConfig {
	return p.helpConfig
}

// PrintHelp prints help according to the configured style
func (p *Parser) PrintHelp(writer io.Writer) {
	style := p.helpConfig.Style

	// Auto-detect style if set to Smart
	if style == HelpStyleSmart {
		style = p.detectBestStyle()
	}

	switch style {
	case HelpStyleFlat:
		p.printFlatHelp(writer)
	case HelpStyleGrouped:
		p.printGroupedHelp(writer)
	case HelpStyleCompact:
		p.printCompactHelp(writer)
	case HelpStyleHierarchical:
		p.printHierarchicalHelp(writer)
	default:
		p.printFlatHelp(writer)
	}
}

func (p *Parser) DefaultPrettyPrintConfig() *PrettyPrintConfig {
	return &PrettyPrintConfig{
		NewCommandPrefix:     " +  ",
		DefaultPrefix:        " ├─ ",
		TerminalPrefix:       " └─ ",
		InnerLevelBindPrefix: " ** ",
		OuterLevelBindPrefix: " │  ",
	}
}

// PrintCommands writes the list of accepted Command structs to io.Writer.
func (p *Parser) PrintCommands(writer io.Writer, config ...*PrettyPrintConfig) {
	var prettyPrintConfig *PrettyPrintConfig
	if len(config) > 0 {
		prettyPrintConfig = config[0]
	} else {
		prettyPrintConfig = &PrettyPrintConfig{
			NewCommandPrefix:     " +",
			DefaultPrefix:        " │",
			TerminalPrefix:       " └",
			OuterLevelBindPrefix: "─",
		}
	}
	p.PrintCommandsUsing(writer, prettyPrintConfig)
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
				command := fmt.Sprintf("%s%s %s\n", start, strings.Repeat(config.OuterLevelBindPrefix, level),
					p.renderer.CommandUsage(cmd))
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
func (p *Parser) SetTerminalReader(t input.TerminalReader) input.TerminalReader {
	current := p.terminalReader
	p.terminalReader = t
	return current
}

// GetTerminalReader returns the current terminal reader
// this is a low-level function and should not be used by most users - by default terminal reader is nil and the real terminal is used
func (p *Parser) GetTerminalReader() input.TerminalReader {
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

// SetSuggestionsFormatter sets a custom formatter for displaying suggestions in error messages.
// The formatter receives a slice of suggestions and should return a formatted string.
// If not set, suggestions are displayed as a comma-separated list.
//
// Example:
//
//	parser.SetSuggestionsFormatter(func(suggestions []string) string {
//	    return "\n  • " + strings.Join(suggestions, "\n  • ")
//	})
func (p *Parser) SetSuggestionsFormatter(formatter SuggestionsFormatter) {
	p.suggestionsFormatter = formatter
}

// SetSuggestionThreshold sets the maximum Levenshtein distance for suggestions.
// You can set different thresholds for flags and commands.
// A threshold of 0 disables suggestions for that type.
// Default is 2 for both flags and commands.
//
// Example:
//
//	parser.SetSuggestionThreshold(3, 2) // More lenient for flags, standard for commands
func (p *Parser) SetSuggestionThreshold(flagThreshold, commandThreshold int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if flagThreshold >= 0 {
		p.flagSuggestionThreshold = flagThreshold
	}
	if commandThreshold >= 0 {
		p.cmdSuggestionThreshold = commandThreshold
	}
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

// SetAutoHelp enables or disables automatic help flag registration
func (p *Parser) SetAutoHelp(enabled bool) {
	p.autoHelp = enabled
}

// GetAutoHelp returns whether automatic help is enabled
func (p *Parser) GetAutoHelp() bool {
	return p.autoHelp
}

// SetHelpFlags sets custom help flag names (default: "help" and "h")
func (p *Parser) SetHelpFlags(flags []string) {
	p.helpFlags = flags
}

// GetHelpFlags returns the current help flag names
func (p *Parser) GetHelpFlags() []string {
	return p.helpFlags
}

// SetAutoLanguage enables or disables automatic language detection
func (p *Parser) SetAutoLanguage(enabled bool) {
	p.autoLanguage = enabled
}

// GetAutoLanguage returns whether automatic language detection is enabled
func (p *Parser) GetAutoLanguage() bool {
	return p.autoLanguage
}

// SetLanguageFlags sets custom language flag names (default: "language", "lang", and "l")
func (p *Parser) SetLanguageFlags(flags []string) {
	p.languageFlags = flags
}

// GetLanguageFlags returns the current language flag names
func (p *Parser) GetLanguageFlags() []string {
	return p.languageFlags
}

// SetCheckSystemLocale enables or disables checking system locale environment variables (LC_ALL, LC_MESSAGES, LANG).
// By default, only GOOPT_LANG is checked.
func (p *Parser) SetCheckSystemLocale(check bool) {
	p.checkSystemLocale = check
}

// GetCheckSystemLocale returns whether system locale environment variables are checked
func (p *Parser) GetCheckSystemLocale() bool {
	return p.checkSystemLocale
}

// SetLanguageEnvVar sets the environment variable name to check for language preference.
// Default is "GOOPT_LANG". Set to empty string to disable environment variable checking.
func (p *Parser) SetLanguageEnvVar(envVar string) {
	p.languageEnvVar = envVar
}

// GetLanguageEnvVar returns the configured language environment variable name
func (p *Parser) GetLanguageEnvVar() string {
	return p.languageEnvVar
}

// hasHelpInArgs quickly scans args to check if help is requested without full parsing
func (p *Parser) hasHelpInArgs(args []string) bool {
	if !p.autoHelp || len(p.helpFlags) == 0 {
		return false
	}

	for _, arg := range args {
		if p.isFlag(arg) {
			stripped := strings.TrimLeftFunc(arg, p.prefixFunc)
			for _, helpFlag := range p.helpFlags {
				// Only trigger on help flags that we auto-registered
				if p.autoRegisteredHelp[helpFlag] && stripped == helpFlag {
					return true
				}
			}
		}
	}
	return false
}

// envGetter is a function type for getting environment variables (mockable for testing)
type envGetter func(string) string

// detectLanguageInArgs quickly scans args to detect language preference without full parsing
func (p *Parser) detectLanguageInArgs(args []string) language.Tag {
	return p.detectLanguageInArgsWithEnv(args, os.Getenv)
}

// detectLanguageInArgsWithEnv is the testable version that accepts an environment getter
func (p *Parser) detectLanguageInArgsWithEnv(args []string, getenv envGetter) language.Tag {
	if !p.autoLanguage || len(p.languageFlags) == 0 {
		return language.Und
	}

	// Collect all language flags, last one wins
	var lastLang language.Tag = language.Und

	// Quick scan for language flag
	for i, arg := range args {
		if p.isFlag(arg) {
			stripped := strings.TrimLeftFunc(arg, p.prefixFunc)

			// Check for --language=value format
			if idx := strings.Index(stripped, "="); idx > 0 {
				flagName := stripped[:idx]
				value := stripped[idx+1:]
				for _, langFlag := range p.languageFlags {
					if langFlag == flagName {
						// Normalize underscore to dash for BCP 47 compatibility
						normalizedValue := strings.Replace(value, "_", "-", -1)
						if tag, err := language.Parse(normalizedValue); err == nil {
							lastLang = tag
						}
					}
				}
			} else {
				// Check for --language value format
				for _, langFlag := range p.languageFlags {
					if stripped == langFlag && i+1 < len(args) && !p.isFlag(args[i+1]) {
						// Normalize underscore to dash for BCP 47 compatibility
						normalizedValue := strings.Replace(args[i+1], "_", "-", -1)
						if tag, err := language.Parse(normalizedValue); err == nil {
							lastLang = tag
						}
					}
				}
			}
		}
	}

	// If we found a language flag, use it
	if lastLang != language.Und {
		return lastLang
	}

	// Otherwise, fallback to environment variables
	// Always check the configured language environment variable first
	if p.languageEnvVar != "" {
		if lang := getenv(p.languageEnvVar); lang != "" {
			// Normalize underscore to dash for BCP 47 compatibility
			lang = strings.Replace(lang, "_", "-", -1)
			if tag, err := language.Parse(lang); err == nil {
				return tag
			}
		}
	}

	// Only check system locale if explicitly enabled
	if p.checkSystemLocale {
		// Try LC_ALL, LC_MESSAGES, then LANG in order of precedence
		for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
			if lang := getenv(envVar); lang != "" {
				// Extract language part from locale (e.g., "en_US.UTF-8" -> "en-US")
				if idx := strings.Index(lang, "."); idx > 0 {
					lang = lang[:idx]
				}
				// Handle C and POSIX locales
				if lang == "C" || lang == "POSIX" {
					continue
				}
				lang = strings.Replace(lang, "_", "-", -1)
				if tag, err := language.Parse(lang); err == nil {
					return tag
				}
			}
		}
	}

	return language.Und
}

// filterLanguageFlags removes language flags and their values from args
func (p *Parser) filterLanguageFlags(args []string) []string {
	if !p.autoLanguage || len(p.languageFlags) == 0 {
		return args
	}

	filtered := make([]string, 0, len(args))
	skip := false

	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		shouldFilter := false

		if p.isFlag(arg) {
			stripped := strings.TrimLeftFunc(arg, p.prefixFunc)

			// Check for --language=value format
			if idx := strings.Index(stripped, "="); idx > 0 {
				flagName := stripped[:idx]
				for _, langFlag := range p.languageFlags {
					if langFlag == flagName {
						shouldFilter = true
						break
					}
				}
			} else {
				// Check for --language value format
				for _, langFlag := range p.languageFlags {
					if stripped == langFlag {
						shouldFilter = true
						// Also skip the next arg if it's not a flag
						if i+1 < len(args) && !p.isFlag(args[i+1]) {
							skip = true
						}
						break
					}
				}
			}
		}

		if !shouldFilter {
			filtered = append(filtered, arg)
		}
	}

	return filtered
}

// IsHelpRequested returns true if any help flag was provided on the command line
func (p *Parser) IsHelpRequested() bool {
	if !p.autoHelp {
		return false
	}

	// If help was executed, it was requested
	if p.helpExecuted {
		return true
	}

	// Only check flags that we auto-registered
	for _, flag := range p.helpFlags {
		if p.autoRegisteredHelp[flag] && p.HasFlag(flag) {
			return true
		}
	}
	return false
}

// WasHelpShown returns true if help was automatically displayed during Parse
func (p *Parser) WasHelpShown() bool {
	return p.helpExecuted
}

// ensureHelpFlags automatically registers help flags if not already defined by the user
func (p *Parser) ensureHelpFlags() error {
	if !p.autoHelp || len(p.helpFlags) == 0 {
		return nil
	}

	// Check which help flags are available
	longFlag := ""
	shortFlag := ""

	// Check long flag (first in array)
	if len(p.helpFlags) > 0 {
		if _, err := p.GetArgument(p.helpFlags[0]); err != nil {
			// Long flag is available
			longFlag = p.helpFlags[0]
		}
	}

	// Check short flag (second in array)
	if len(p.helpFlags) > 1 {
		// Check if short flag is already taken by any flag
		if _, conflict := p.checkShortFlagConflict(p.helpFlags[1], longFlag); !conflict {
			// Also check if the short flag exists as a main flag
			if _, err := p.GetArgument(p.helpFlags[1]); err != nil {
				shortFlag = p.helpFlags[1]
			}
		}
	}

	// If no flags are available, user has defined all help flags
	if longFlag == "" && shortFlag == "" {
		return nil
	}

	// Auto-register help flag with available flags
	helpArg := &Argument{
		DescriptionKey: messages.MsgHelpDescriptionKey,
		TypeOf:         types.Standalone,
		DefaultValue:   "false",
	}

	// Use the first available flag as primary
	primaryFlag := longFlag
	if primaryFlag == "" {
		primaryFlag = shortFlag
		shortFlag = "" // Don't use it as short flag
	}

	// Set short flag if available
	if shortFlag != "" {
		helpArg.Short = shortFlag
	}

	var err error
	if !p.autoRegisteredHelp[primaryFlag] {
		err = p.AddFlag(primaryFlag, helpArg)
		if err == nil {
			// Track that we auto-registered these flags
			p.autoRegisteredHelp[primaryFlag] = true
			if shortFlag != "" {
				p.autoRegisteredHelp[shortFlag] = true
			}
		}
	}

	return err
}

// SetVersion sets a static version string
func (p *Parser) SetVersion(version string) {
	p.version = version
}

// GetVersion returns the current version string
func (p *Parser) GetVersion() string {
	if p.versionFunc != nil {
		return p.versionFunc()
	}
	return p.version
}

// SetVersionFunc sets a function to dynamically generate version info
func (p *Parser) SetVersionFunc(f func() string) {
	p.versionFunc = f
}

// SetVersionFormatter sets a custom formatter for version output
func (p *Parser) SetVersionFormatter(f func(string) string) {
	p.versionFormatter = f
}

// SetAutoVersion enables or disables automatic version flag registration
func (p *Parser) SetAutoVersion(enabled bool) {
	p.autoVersion = enabled
}

// GetAutoVersion returns whether automatic version is enabled
func (p *Parser) GetAutoVersion() bool {
	return p.autoVersion
}

// SetVersionFlags sets custom version flag names (default: "version" and "v")
func (p *Parser) SetVersionFlags(flags []string) {
	p.versionFlags = flags
}

// GetVersionFlags returns the current version flag names
func (p *Parser) GetVersionFlags() []string {
	return p.versionFlags
}

// SetShowVersionInHelp controls whether version is shown in help output
func (p *Parser) SetShowVersionInHelp(show bool) {
	p.showVersionInHelp = show
}

// IsVersionRequested returns true if any version flag was provided
func (p *Parser) IsVersionRequested() bool {
	if !p.autoVersion || p.version == "" && p.versionFunc == nil {
		return false
	}

	// Only check flags that we auto-registered
	for _, flag := range p.versionFlags {
		if p.autoRegisteredVersion[flag] && p.HasFlag(flag) {
			return true
		}
	}
	return false
}

// WasVersionShown returns true if version was automatically displayed
func (p *Parser) WasVersionShown() bool {
	return p.versionExecuted
}

// PrintVersion outputs the version information
func (p *Parser) PrintVersion(w io.Writer) {
	version := p.GetVersion()
	if version == "" {
		version = "unknown"
	}

	var output string
	if p.versionFormatter != nil {
		output = p.versionFormatter(version)
	} else {
		// Default format: just the version
		output = version
	}

	fmt.Fprintln(w, output)
}

// ensureVersionFlags automatically registers version flags if not already defined
func (p *Parser) ensureVersionFlags() error {
	if !p.autoVersion || len(p.versionFlags) == 0 {
		return nil
	}

	// Only register if we have a version set
	if p.version == "" && p.versionFunc == nil {
		return nil
	}

	// Check which version flags are available
	longFlag := ""
	shortFlag := ""

	// Check long flag (first in array)
	if len(p.versionFlags) > 0 {
		if _, err := p.GetArgument(p.versionFlags[0]); err != nil {
			// Long flag is available
			longFlag = p.versionFlags[0]
		}
	}

	// Check short flag (second in array)
	if len(p.versionFlags) > 1 {
		if _, conflict := p.checkShortFlagConflict(p.versionFlags[1], p.versionFlags[0]); !conflict {
			// Short flag is available
			shortFlag = p.versionFlags[1]
		}
	}

	// If no flags are available, user has defined all version flags
	if longFlag == "" && shortFlag == "" {
		return nil
	}

	// Auto-register version flag with available flags
	versionArg := &Argument{
		DescriptionKey: messages.MsgVersionDescriptionKey,
		TypeOf:         types.Standalone,
		DefaultValue:   "false",
	}

	// Use the first available flag as primary
	primaryFlag := longFlag
	if primaryFlag == "" {
		primaryFlag = shortFlag
		shortFlag = "" // Don't use it as short flag
	}

	// Set short flag if available
	if shortFlag != "" {
		versionArg.Short = shortFlag
	}

	var err error
	if !p.autoRegisteredVersion[primaryFlag] {
		err = p.AddFlag(primaryFlag, versionArg)
		if err == nil {
			// Track that we auto-registered these flags
			p.autoRegisteredVersion[primaryFlag] = true
			if shortFlag != "" {
				p.autoRegisteredVersion[shortFlag] = true
			}
		}
	}

	return err
}

// ensureLanguageFlags automatically registers language flags if not already defined by the user
func (p *Parser) ensureLanguageFlags() error {
	if !p.autoLanguage || len(p.languageFlags) == 0 {
		return nil
	}

	// Check which language flags are available
	var availableFlags []string

	for _, flag := range p.languageFlags {
		// Check if flag already exists
		if _, err := p.GetArgument(flag); err != nil {
			// For short flags, check for conflicts
			if len(flag) == 1 {
				if _, conflict := p.checkShortFlagConflict(flag, ""); !conflict {
					availableFlags = append(availableFlags, flag)
				}
			} else {
				availableFlags = append(availableFlags, flag)
			}
		}
	}

	// If no flags are available, user has defined all language flags
	if len(availableFlags) == 0 {
		return nil
	}

	// Auto-register language flag with available flags
	langArg := &Argument{
		DescriptionKey: messages.MsgLanguageDescriptionKey,
		TypeOf:         types.Single,
		DefaultValue:   p.GetLanguage().String(),
	}

	// Use the first available long flag as primary
	primaryFlag := ""
	shortFlag := ""

	for _, flag := range availableFlags {
		if len(flag) > 1 && primaryFlag == "" {
			primaryFlag = flag
		} else if len(flag) == 1 && shortFlag == "" {
			shortFlag = flag
		}
	}

	// If no long flag available, use short as primary
	if primaryFlag == "" && shortFlag != "" {
		primaryFlag = shortFlag
		shortFlag = ""
	}

	// Set short flag if available
	if shortFlag != "" {
		langArg.Short = shortFlag
	}

	var err error
	if primaryFlag != "" && !p.autoRegisteredLanguage[primaryFlag] {
		err = p.AddFlag(primaryFlag, langArg)
		if err == nil {
			// Track that we auto-registered these flags
			p.autoRegisteredLanguage[primaryFlag] = true
			if shortFlag != "" {
				p.autoRegisteredLanguage[shortFlag] = true
			}
		}
	}

	return err
}

// AddGlobalPreHook adds a pre-execution hook that runs before any command
func (p *Parser) AddGlobalPreHook(hook PreHookFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.globalPreHooks = append(p.globalPreHooks, hook)
}

// AddGlobalPostHook adds a post-execution hook that runs after any command
func (p *Parser) AddGlobalPostHook(hook PostHookFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.globalPostHooks = append(p.globalPostHooks, hook)
}

// AddCommandPreHook adds a pre-execution hook for a specific command
func (p *Parser) AddCommandPreHook(commandPath string, hook PreHookFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.commandPreHooks[commandPath] == nil {
		p.commandPreHooks[commandPath] = []PreHookFunc{}
	}
	p.commandPreHooks[commandPath] = append(p.commandPreHooks[commandPath], hook)
}

// AddCommandPostHook adds a post-execution hook for a specific command
func (p *Parser) AddCommandPostHook(commandPath string, hook PostHookFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.commandPostHooks[commandPath] == nil {
		p.commandPostHooks[commandPath] = []PostHookFunc{}
	}
	p.commandPostHooks[commandPath] = append(p.commandPostHooks[commandPath], hook)
}

// SetHookOrder sets the order in which hooks are executed
func (p *Parser) SetHookOrder(order HookOrder) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hookOrder = order
}

// GetHookOrder returns the current hook execution order
func (p *Parser) GetHookOrder() HookOrder {
	return p.hookOrder
}

// ClearGlobalHooks removes all global hooks
func (p *Parser) ClearGlobalHooks() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.globalPreHooks = []PreHookFunc{}
	p.globalPostHooks = []PostHookFunc{}
}

// ClearCommandHooks removes all hooks for a specific command
func (p *Parser) ClearCommandHooks(commandPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.commandPreHooks, commandPath)
	delete(p.commandPostHooks, commandPath)
}

// executePreHooks executes all applicable pre-hooks for a command
func (p *Parser) executePreHooks(cmd *Command) error {
	var preHooks []PreHookFunc

	// Get command-specific hooks
	cmdHooks := p.commandPreHooks[cmd.Path()]

	// Determine execution order
	if p.hookOrder == OrderGlobalFirst {
		preHooks = append(preHooks, p.globalPreHooks...)
		preHooks = append(preHooks, cmdHooks...)
	} else {
		preHooks = append(preHooks, cmdHooks...)
		preHooks = append(preHooks, p.globalPreHooks...)
	}

	// Execute all pre-hooks
	for _, hook := range preHooks {
		if err := hook(p, cmd); err != nil {
			return err
		}
	}

	return nil
}

// executePostHooks executes all applicable post-hooks for a command
func (p *Parser) executePostHooks(cmd *Command, cmdErr error) error {
	var postHooks []PostHookFunc

	// Get command-specific hooks
	cmdHooks := p.commandPostHooks[cmd.Path()]

	// Determine execution order (reverse of pre-hooks for cleanup)
	if p.hookOrder == OrderGlobalFirst {
		postHooks = append(postHooks, cmdHooks...)
		postHooks = append(postHooks, p.globalPostHooks...)
	} else {
		postHooks = append(postHooks, p.globalPostHooks...)
		postHooks = append(postHooks, cmdHooks...)
	}

	// Execute all post-hooks
	var lastErr error
	for _, hook := range postHooks {
		if err := hook(p, cmd, cmdErr); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// AddFlagValidators adds multiple validators for a flag
func (p *Parser) AddFlagValidators(flag string, validators ...validation.ValidatorFunc) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if flag exists
	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return errs.ErrFlagDoesNotExist.WithArgs(flag)
	}

	// Add validators to the flag's argument
	flagInfo.Argument.Validators = append(flagInfo.Argument.Validators, validators...)
	return nil
}

// SetFlagValidators replaces all validators for a flag
func (p *Parser) SetFlagValidators(flag string, validators ...validation.ValidatorFunc) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if flag exists
	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return errs.ErrFlagDoesNotExist.WithArgs(flag)
	}

	// Replace validators
	flagInfo.Argument.Validators = validators
	return nil
}

// ClearFlagValidators removes all validators for a flag
func (p *Parser) ClearFlagValidators(flag string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if flag exists
	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return errs.ErrFlagDoesNotExist.WithArgs(flag)
	}

	// Clear validators
	flagInfo.Argument.Validators = nil
	return nil
}
