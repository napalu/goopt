package goopt

import (
	"regexp"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/types"
)

// WithShortFlag represents the short form of a flag. Since by default and design, no max length is enforced,
// the "short" flag can be looked at as an alternative to using the long name. I use it as a moniker. The short flag
// can be used in all methods which take a flag argument. By default, there is no support for "POSIX/GNU-like" chaining
// of boolean flags such as :
//
//	-vvvv
//
// Instead of specifying a flag 4 times, the "goopt" way would be specifying `-v 4`.
//
// If POSIX/GNU compatibility is desired, use the SetPosix or WithPosix functions on CmdLineOption (not implemented yet).
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

// WithRequired allows setting a function to evaluate if a flag is required
func WithRequired(required bool) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Required = required
	}
}

// WithRequiredIf allows setting a function to evaluate if a flag is required
func WithRequiredIf(requiredIf RequiredIfFunc) ConfigureArgumentFunc {
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

// WithSecurePrompt sets the prompt for the secure flag
func WithSecurePrompt(prompt string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.Secure.IsSecure = true
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
// Each value can optionally have a description that will be shown in the help text.
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
func WithPosition(idx int) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		if idx < 0 {
			*err = errs.ErrPositionMustBeNonNegative.WithArgs(idx)
			return
		}
		argument.Position = util.NewOfType(idx)
	}
}

// WithDescriptionKey sets a translation key for the argument
func WithDescriptionKey(key string) ConfigureArgumentFunc {
	return func(argument *Argument, err *error) {
		argument.DescriptionKey = key
	}
}
