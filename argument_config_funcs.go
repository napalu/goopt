package goopt

import (
	"fmt"
	"regexp"

	"github.com/napalu/goopt/types"
)

// NewArg convenience initialization method to configure flags
func NewArg(configs ...ConfigureArgumentFunc) *Argument {
	argument := &Argument{}
	for _, config := range configs {
		config(argument, nil)
	}

	return argument
}

// Set configures the Argument instance with the provided ConfigureArgumentFunc(s),
// and returns an error if a configuration results in an error.
//
// Usage example:
//
//	arg := &Argument{}
//	err := arg.Set(
//	    WithDescription("example argument"),
//	    WithType(Standalone),
//	    IsRequired,
//	)
//	if err != nil {
//	    // handle error
//	}
func (a *Argument) Set(configs ...ConfigureArgumentFunc) error {
	a.ensureInit()
	var err error
	for _, config := range configs {
		config(a, &err)
		if err != nil {
			return err
		}
	}
	return nil
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
//  4. File - a flag which expects a valid file path whose content is the value
func WithType(typeof types.OptionType) ConfigureArgumentFunc {
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
// Results in a warning being emitted in GetWarnings() when the dependent flags are not specified.
func WithDependentFlags(dependencies []string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		if argument.DependencyMap == nil {
			argument.DependencyMap = make(map[string][]string)
		}
		for _, dep := range dependencies {
			// Empty slice means flag just needs to be present
			argument.DependencyMap[dep] = nil
		}
	}
}

// WithDependencyMap specifies flag dependencies using a map of flag names to accepted values
func WithDependencyMap(dependencies map[string][]string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		if argument.DependencyMap == nil {
			argument.DependencyMap = make(map[string][]string)
		}
		for k, v := range dependencies {
			argument.DependencyMap[k] = v
		}
	}
}

// WithDependentValueFlags specifies flag dependencies with their accepted values
//
// Deprecated: Use WithDependencyMap instead
func WithDependentValueFlags(dependencies, values []string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DependsOn = dependencies
		argument.OfValue = values
		if argument.DependencyMap == nil {
			argument.DependencyMap = make(map[string][]string)
		}
		// Also populate the new format for forward compatibility
		for i, dep := range dependencies {
			if i < len(values) {
				argument.DependencyMap[dep] = []string{values[i]}
			}
		}
	}
}

// SetSecure sets the secure flag to true or false
func SetSecure(secure bool) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Secure.IsSecure = secure
	}
}

// SetSecurePrompt sets the prompt for the secure flag
func SetSecurePrompt(prompt string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Secure.Prompt = prompt
	}
}

// WithDefaultValue sets the default value for the argument
func WithDefaultValue(defaultValue string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DefaultValue = defaultValue
	}
}

// WithPreValidationFilter sets the pre-validation filter for the argument
func WithPreValidationFilter(filter FilterFunc) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.PreFilter = filter
	}
}

// WithPostValidationFilter sets the post-validation filter for the argument
func WithPostValidationFilter(filter FilterFunc) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.PostFilter = filter
	}
}

// WithAcceptedValues sets the accepted values for the argument. The values can be either literal strings or regular expressions.
// Each value can optionally have a description that will be shown in help text.
func WithAcceptedValues(values []types.PatternValue) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.AcceptedValues = values

		for i := 0; i < len(values); i++ {
			re, e := regexp.Compile(argument.AcceptedValues[i].Pattern)
			if e != nil {
				*err = e
				return
			}

			argument.AcceptedValues[i].Compiled = re
		}
	}
}

// WithPosition sets a position requirement for the argument
func WithPosition(pos PositionType) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		// Validate position type
		if pos != AtStart && pos != AtEnd {
			*err = fmt.Errorf("invalid position type: %d", pos)
			return
		}
		argument.Position = &pos
	}
}

// WithRelativeIndex sets the index for ordered positional arguments
func WithRelativeIndex(idx int) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		// Validate index is non-negative
		if idx < 0 {
			*err = fmt.Errorf("positional index must be non-negative, got: %d", idx)
			return
		}
		argument.RelativeIndex = &idx
	}
}
