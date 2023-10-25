package goopt

// NewArg convenience initialization method to fluently configure flags
func NewArg(configs ...ConfigureArgumentFunc) *Argument {
	argument := &Argument{}
	for _, config := range configs {
		config(argument, nil)
	}

	return argument
}

// WithShortFlag represents the short form of a flag. Since by default and design, no max length is enforced,
// the "short" flag can be looked at as an alternative to using the long name. I use it as a moniker. The short flag
// can be used in all methods which take a flag argument. By default, there is no support for "POSIX/GNU-like" chaining
// of boolean flags such as :
//
//	-vvvv
//
// Instead of specifying a flag 4 times, the "goopt" way would be specifying `-v 4`.
//
// If POSIX/GNU compatibility is desired use the SetPosix or WithPosix functions on CmdLineOption (not implemented yet).
func WithShortFlag(shortFlag string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Short = shortFlag
	}
}

// WithDescription the description will be used in usage output presented to the user
func WithDescription(description string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Description = description
	}
}

// WithType - one of three types:
//  1. Single - a flag which expects a value
//  2. Chained - a flag which expects a delimited value representing elements in a list (and is evaluated as a list)
//  3. Standalone - a boolean flag which by default takes no value (defaults to true) but may accept a value which evaluates to true or false
func WithType(typeof OptionType) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.TypeOf = typeof
	}
}

// SetRequired when true, the flag must be supplied on the command-line
func SetRequired(required bool) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Required = required
	}
}

// SetRequiredIf allows to set a function to evaluate if a flag is required
func SetRequiredIf(requiredIf RequiredIfFunc) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.RequiredIf = requiredIf
	}
}

// WithDependentFlags accepts an array of string denoting flags on which an argument depends.
// Results in a warning being emitted in GetWarnings() when the dependent flags are not specified on the command-line.
func WithDependentFlags(dependencies []string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DependsOn = dependencies
	}
}

// WithDependentValueFlags accepts an array of string denoting flags and flag values  on which an argument depends.
// Results in a warning being emitted in GetWarnings() when the dependent flags are not specified on the command-line.
// For example - to specify a dependency on flagA with values 'b' or 'c':
//
//	WithDependentValueFlags([]string{"flagA", "flagA"}, []string{"b", "c"})
func WithDependentValueFlags(dependencies, values []string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DependsOn = dependencies
		argument.OfValue = values
	}
}

func SetSecure(secure bool) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Secure.IsSecure = secure
	}
}

func SetSecurePrompt(prompt string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Secure.Prompt = prompt
	}
}

func WithDefaultValue(defaultValue string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DefaultValue = defaultValue
	}
}

func WithPreValidationFilter(filter FilterFunc) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.PreFilter = filter
	}
}

func WithPostValidationFilter(filter FilterFunc) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.PostFilter = filter
	}
}

func WithAcceptedValues(values []PatternValue) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		if argument.AcceptedValues == nil {
			argument.AcceptedValues = make([]LiterateRegex, 0, len(values))
		}

		for i := 0; i < len(values); i++ {
			err = argument.accept(values[i])
			if err != nil {
				return
			}
		}
	}
}
