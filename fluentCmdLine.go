package goopt

// NewCmdLine allows fluent initialization of CmdLineOption. The caller should always test for error on
// return because CmdLineOption will be nil when an error occurs during initialization.
//
// Examples of fluent configuration:
//
//  cmdLine, err := NewCmdLine(
//		WithFlag("flagWithValue",
//			NewArg(
//				WithShortFlag("fw"),
//				WithType(Single),
//				WithDescription("this flag requires a value"),
//				WithDependentFlags([]string{"flagA", "flagB"}),
//				SetRequired(true))),
//		WithFlag("flagA",
//			NewArg(
//				WithShortFlag("fa"),
//				WithType(Standalone))),
//		WithFlag("flagB",
//			NewArg(
//				WithShortFlag("fb"),
//				WithDescription("This is flag B - flagWithValue depends on it"),
//				WithDefaultValue("db"),
//				WithType(Single))),
//		WithFlag("flagC",
//			NewArg(
//				WithShortFlag("fc"),
//				WithDescription("this is flag C - it's a chained flag which can return a list"),
//				WithType(Chained))))
//
func NewCmdLine(configs ...ConfigureCmdLineFunc) (*CmdLineOption, error) {
	cmdLine := NewCmdLineOption()

	var err error
	for _, config := range configs {
		config(cmdLine, &err)
		if err != nil {
			return nil, err
		}
	}

	return cmdLine, err
}

// WithFlag is a fluent wrapper for AddFlag which is used to define a flag.
// A flag represents a command line option as a "long" and optional "short" form
// which is prefixed by '-', '--' or '/'.
func WithFlag(flag string, argument *Argument) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		*err = cmdLine.AddFlag(flag, argument)
	}
}

// WithBindFlag is a fluent wrapper to BindFlag which is used to bind a pointer to a variable with a flag.
// If `bindVar` is not a pointer, an error is returned
// The following variable types are supported:
//  - *string
//  - *int, *int8, *int16, *int32, *int64
//  - *uint, *uint8, *uint16, *uint32, *uint64
//  - *float32, *float64
//  - *time.Time
//  - *bool
//  For other types use WithCustomBindFlag (fluent wrapper around CustomBindFlag) or CustomBindFlag
func WithBindFlag[T Bindable](flag string, bindVar *T, argument *Argument) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		*err = BindFlagToCmdLine(cmdLine, bindVar, flag, argument)
	}
}

// WithCustomBindFlag is a fluent wrapper for CustomBindFlag which receives parsed value via the ValueSetFunc callback
// On Parse the callback is called with the following arguments:
//  - the bound flag name as string
//  - the command-line value as string
//  - the custom struct or variable which was bound. The bound structure is passed as reflect.Value
func WithCustomBindFlag[T any](flag string, bindVar *T, proc ValueSetFunc, argument *Argument) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		*err = CustomBindFlagToCmdLine(cmdLine, bindVar, proc, flag, argument)
	}
}

// WithCommand is a fluent wrapper for AddCommand. A Command represents a verb followed by optional sub-commands. A
// sub-command is a Command which is stored in a Command's []Subcommands field. A command which has no children is
// a terminating command which can receive values supplied by the user on the command line. Like flags, commands are
// evaluated on Parse.
// See the Command struct for more details.
func WithCommand(command *Command) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		*err = cmdLine.AddCommand(command)
	}
}

// WithListDelimiterFunc allows providing a custom function for splitting Chained command-line argument values into lists.
func WithListDelimiterFunc(delimiterFunc ListDelimiterFunc) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		*err = cmdLine.SetListDelimiterFunc(delimiterFunc)
	}
}

// WithArgumentPrefixes allows providing custom flag prefixes (defaults to '-', '---', and '/').
func WithArgumentPrefixes(prefixes []rune) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		*err = cmdLine.SetArgumentPrefixes(prefixes)
	}
}

// WithPosix stub for switching on Posix/GNU-like flag compatibility - not implemented yet
// TODO implement
func WithPosix(usePosix bool) ConfigureCmdLineFunc {
	return func(cmdLine *CmdLineOption, err *error) {
		cmdLine.SetPosix(usePosix)
	}
}
