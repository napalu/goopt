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
//   -vvvv
// Instead of specifying a flag 4 times, the "goopt" way would be specifying `-v 4`.
//
// If POSIX/GNU compatibility is desired use the SetPosix or WithPosix functions on CmdLineOption (not implemented yet).
func WithShortFlag(shortFlag string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Short = shortFlag
	}
}

func WithDescription(description string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Description = description
	}
}

func WithType(typeof OptionType) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.TypeOf = typeof
	}
}

func SetRequired(required bool) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Required = required
	}
}

func SetRequiredIf(requiredIf RequiredIfFunc) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.RequiredIf = requiredIf
	}
}

func WithDependentFlags(dependencies []string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DependsOn = dependencies
	}
}

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
