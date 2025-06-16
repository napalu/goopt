package goopt

import (
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/validation"
	"golang.org/x/text/language"
)

// NewParserWith allows initialization of Parser using option functions. The caller should always test for error on
// return because Parser will be nil when an error occurs during initialization.
//
// Configuration example:
//
//	 parser, err := NewParserWith(
//			WithFlag("flagWithValue",
//				NewArg(
//					WithShortFlag("fw"),
//					WithType(Single),
//					WithDescription("this flag requires a value"),
//					WithDependentFlags([]string{"flagA", "flagB"}),
//					SetRequired(true))),
//			WithFlag("flagA",
//				NewArg(
//					WithShortFlag("fa"),
//					WithType(Standalone))),
//			WithFlag("flagB",
//				NewArg(
//					WithShortFlag("fb"),
//					WithDescription("This is flag B - flagWithValue depends on it"),
//					WithDefaultValue("db"),
//					WithType(Single))),
//			WithFlag("flagC",
//				NewArg(
//					WithShortFlag("fc"),
//					WithDescription("this is flag C - it's a chained flag which can return a list"),
//					WithType(Chained))))
func NewParserWith(configs ...ConfigureCmdLineFunc) (*Parser, error) {
	cmdLine := NewParser()

	var err error
	for _, config := range configs {
		config(cmdLine, &err)
		if err != nil {
			return nil, err
		}
	}

	return cmdLine, err
}

// WithFlag is a wrapper for AddFlag which is used to define a flag.
// A flag represents a command line option as a "long" and optional "short" form
// which is prefixed by '-', '--' or '/'.
func WithFlag(flag string, argument *Argument) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = cmdLine.AddFlag(flag, argument)
	}
}

// WithBindFlag is a wrapper to BindFlag which is used to bind a pointer to a variable with a flag.
// If `bindVar` is not a pointer, an error is returned
// The following variable types are supported:
//   - *string
//   - *int, *int8, *int16, *int32, *int64
//   - *uint, *uint8, *uint16, *uint32, *uint64
//   - *float32, *float64
//   - *time.Time
//   - *bool
//     For other types use WithCustomBindFlag (wrapper around CustomBindFlag) or CustomBindFlag
func WithBindFlag[T Bindable](flag string, bindVar *T, argument *Argument, commandPath ...string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = BindFlagToParser(cmdLine, bindVar, flag, argument, commandPath...)
	}
}

// WithCustomBindFlag is a wrapper for CustomBindFlag which receives parsed value via the ValueSetFunc callback
// On Parse the callback is called with the following arguments:
//   - the bound flag name as string
//   - the command-line value as string
//   - the custom struct or variable which was bound. The bound structure is passed as reflect.Value
func WithCustomBindFlag[T any](flag string, bindVar *T, proc ValueSetFunc, argument *Argument, commandPath ...string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = CustomBindFlagToParser(cmdLine, bindVar, proc, flag, argument, commandPath...)
	}
}

// WithExecOnParse specifies whether Command callbacks should be executed during Parse as soon as they are encountered.
func WithExecOnParse(value bool) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetExecOnParse(value)
	}
}

// WithExecOnParseComplete specifies whether Command callbacks should be executed after a successful Parse. Note:
// setting this has no effect if WithExecOnParse(true) is set.
func WithExecOnParseComplete(value bool) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetExecOnParseComplete(value)
	}
}

// WithCommand is a wrapper for AddCommand. A Command represents a verb followed by optional sub-commands. A
// sub-command is a Command which is stored in a Command's []Subcommands field. A command which has no children is
// a terminating command which can receive values supplied by the user on the command line. Like flags, commands are
// evaluated on Parse.
// See the Command struct for more details.
func WithCommand(command *Command) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = cmdLine.AddCommand(command)
	}
}

// WithListDelimiterFunc allows providing a custom function for splitting Chained command-line argument values into lists.
func WithListDelimiterFunc(delimiterFunc types.ListDelimiterFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = cmdLine.SetListDelimiterFunc(delimiterFunc)
	}
}

// WithArgumentPrefixes allows providing custom flag prefixes (defaults to '-', '---', and '/').
func WithArgumentPrefixes(prefixes []rune) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = cmdLine.SetArgumentPrefixes(prefixes)
	}
}

// WithPosix for switching on Posix/GNU-like flag compatibility
func WithPosix(usePosix bool) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetPosix(usePosix)
	}
}

// WithCommandNameConverter allows setting a custom name converter for command names
func WithCommandNameConverter(converter NameConversionFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetCommandNameConverter(converter)
	}
}

// WithFlagNameConverter allows setting a custom name converter for flag names
func WithFlagNameConverter(converter NameConversionFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetFlagNameConverter(converter)
	}
}

// WithEnvNameConverter allows setting a custom name converter for environment variable names
func WithEnvNameConverter(converter NameConversionFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetEnvNameConverter(converter)
	}
}

// WithLanguage allows setting the language for the parser.
func WithLanguage(lang language.Tag) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		if setErr := cmdLine.SetSystemLanguage(lang); setErr != nil && err != nil {
			*err = setErr
		}
	}
}

// WithUserBundle allows setting the user-defined i18n bundle for the parser.
func WithUserBundle(bundle *i18n.Bundle) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = cmdLine.SetUserBundle(bundle)
	}
}

// WithReplaceBundle allows setting a user-defined i18n bundle which will replace the default i18n bundle and be used to
// for translations of struct field tags, error messages and so on.
//
// Deprecated: use WithExtendBundle instead
func WithReplaceBundle(rbundle *i18n.Bundle) ConfigureCmdLineFunc {
	return WithExtendBundle(rbundle)
}

func WithExtendBundle(eBundle *i18n.Bundle) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		*err = cmdLine.ExtendSystemBundle(eBundle)
	}
}

// WithHelpStyle sets the help output style
func WithHelpStyle(style HelpStyle) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetHelpStyle(style)
	}
}

// WithHelpConfig sets the complete help configuration
func WithHelpConfig(config HelpConfig) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetHelpConfig(config)
	}
}

// WithAutoHelp enables or disables automatic help flag registration (default: true)
func WithAutoHelp(enabled bool) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetAutoHelp(enabled)
	}
}

// WithHelpFlags sets custom help flag names (default: "help", "h")
func WithHelpFlags(flags ...string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetHelpFlags(flags)
	}
}

// WithVersion sets a static version string and enables auto-version
func WithVersion(version string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetVersion(version)
	}
}

// WithVersionFunc sets a function to dynamically generate version info
func WithVersionFunc(f func() string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetVersionFunc(f)
	}
}

// WithVersionFormatter sets a custom formatter for version output
func WithVersionFormatter(f func(string) string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetVersionFormatter(f)
	}
}

// WithAutoVersion enables or disables automatic version flag registration
func WithAutoVersion(enabled bool) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetAutoVersion(enabled)
	}
}

// WithVersionFlags sets custom version flag names (default: "version", "v")
func WithVersionFlags(flags ...string) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetVersionFlags(flags)
	}
}

// WithShowVersionInHelp controls whether version is shown in help output
func WithShowVersionInHelp(show bool) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetShowVersionInHelp(show)
	}
}

// WithGlobalPreHook adds a global pre-execution hook
func WithGlobalPreHook(hook PreHookFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.AddGlobalPreHook(hook)
	}
}

// WithGlobalPostHook adds a global post-execution hook
func WithGlobalPostHook(hook PostHookFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.AddGlobalPostHook(hook)
	}
}

// WithCommandPreHook adds a pre-execution hook for a specific command
func WithCommandPreHook(commandPath string, hook PreHookFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.AddCommandPreHook(commandPath, hook)
	}
}

// WithCommandPostHook adds a post-execution hook for a specific command
func WithCommandPostHook(commandPath string, hook PostHookFunc) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.AddCommandPostHook(commandPath, hook)
	}
}

// WithHookOrder sets the order in which hooks are executed
func WithHookOrder(order HookOrder) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetHookOrder(order)
	}
}

// WithFlagValidators adds one or more validators for a flag (including positional arguments)
func WithFlagValidators(flag string, validators ...validation.Validator) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		if e := cmdLine.AddFlagValidators(flag, validators...); e != nil && err != nil {
			*err = e
		}
	}
}

// WithValidationHook sets a validation hook that runs after all field parsing and validation
// but before Parse returns success. This allows for cross-field validation and conditional logic.
// The hook receives the parser instance and can access parsed values via parser.Get() or
// GetStructCtxAs[T](parser) for struct-based parsers.
// If the hook returns an error, it will be added to parser errors and Parse will return false.
func WithValidationHook(hook func(*Parser) error) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.validationHook = hook
	}
}

// WithHelpBehavior sets the help output behavior
func WithHelpBehavior(behavior HelpBehavior) ConfigureCmdLineFunc {
	return func(cmdLine *Parser, err *error) {
		cmdLine.SetHelpBehavior(behavior)
	}
}
