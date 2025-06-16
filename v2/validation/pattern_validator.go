package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/internal/parse"
	"github.com/napalu/goopt/v2/types"
)

// Regex creates a validator from a regex pattern and description
// This matches the functionality of AcceptedValues but is composable
//
// Examples:
//
//	v, err := Regex("^[A-Z]+$", "Uppercase letters only")
//	v, err := Regex("^\\d{5}$", "5-digit ZIP code")
//	v, err := Regex("^[A-Z]{2},\\d{4}$", "Format: XX,1234")
func Regex(pattern, description string) (Validator, error) {
	// Compile the regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	return NewFuncValidator(
		fmt.Sprintf("regex[%s]", pattern),
		"pattern",
		description,
		func(value string) error {
			if !re.MatchString(value) {
				// Use the description in the error message with value
				return errs.ErrPatternMatch.WithArgs(description, value)
			}
			return nil
		},
	), nil
}

// RegexSpec creates a validator from a regex specification using the same syntax as AcceptedValues
// This is primarily for internal use when converting AcceptedValues to validators
//
// Examples:
//
//	v, err := RegexSpec("{pattern:^[A-Z]+$,desc:Uppercase letters only}")
//	v, err := RegexSpec("{pattern:^\\d{5}$,desc:5-digit ZIP code}")
func RegexSpec(spec string) (Validator, error) {
	// Parse using the same parser as AcceptedValues
	pv, err := parse.PatternValue(spec)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern specification %q: %w", spec, err)
	}

	return Regex(pv.Pattern, pv.Description)
}

// RegexPattern represents a pattern with its description
type RegexPattern struct {
	Pattern     string
	Description string
}

// Regexes creates a validator that matches any of the provided patterns (OR logic)
// Each pattern can have its own description
// Returns an error if any pattern fails to compile
//
// Example:
//
//	v, err := Regexes(
//	  RegexPattern{"^\\d{5}$", "5-digit ZIP"},
//	  RegexPattern{"^\\d{5}-\\d{4}$", "ZIP+4 format"},
//	  RegexPattern{"^[A-Z]\\d[A-Z] \\d[A-Z]\\d$", "Canadian postal code"},
//	)
func Regexes(patterns ...RegexPattern) (Validator, error) {
	var validators []Validator
	for _, p := range patterns {
		v, err := Regex(p.Pattern, p.Description)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p.Pattern, err)
		}
		validators = append(validators, v)
	}
	return OneOf(validators...), nil
}

// MustMatch is an alias for Regex to make intent clearer in some contexts
//
// Example:
//
//	v, err := MustMatch("^[A-Z]", "Must start with uppercase")
//	All(v, MinLength(5))
func MustMatch(pattern, description string) (Validator, error) {
	return Regex(pattern, description)
}

// MustNotMatch creates a validator that ensures a value does NOT match a pattern
//
// Example:
//
//	v, err := MustNotMatch("^test", "Must not start with 'test'")
func MustNotMatch(pattern, description string) (Validator, error) {
	v, err := Regex(pattern, description)
	if err != nil {
		return nil, err
	}
	return Not(v), nil
}

// RegexFromValues creates a regex validator from a simple list of allowed values
// This is more efficient than using regex alternation for simple string matching
//
// Example:
//
//	v := RegexFromValues([]string{"red", "green", "blue"}, "Primary colors")
func RegexFromValues(values []string, description string) Validator {
	if len(values) == 0 {
		return NewFuncValidator(
			"regex-from-values[]",
			"pattern",
			description,
			func(value string) error {
				return errs.ErrPatternMatch.WithArgs(description, value)
			},
		)
	}

	// For efficiency, use a map for exact matching rather than regex
	valueSet := make(map[string]bool)
	for _, v := range values {
		valueSet[v] = true
	}

	return NewFuncValidator(
		fmt.Sprintf("regex-from-values[%s]", strings.Join(values, ",")),
		"pattern",
		description,
		func(value string) error {
			if !valueSet[value] {
				return errs.ErrPatternMatch.WithArgs(description, value)
			}
			return nil
		},
	)
}

// TranslatableRegex creates a validator that supports translatable descriptions
// The description can be either a literal string or a translation key
func TranslatableRegex(pattern, descriptionOrKey string, provider i18n.MessageProvider) (Validator, error) {
	return NewRegexValidator(pattern, descriptionOrKey, provider)
}

// TranslatablePatternError is an error that supports translation of pattern descriptions
type TranslatablePatternError struct {
	DescriptionOrKey string
	Value            string
	Pattern          string
}

func (e *TranslatablePatternError) Error() string {
	// Default error message when no translation is available
	desc := e.DescriptionOrKey
	if desc == "" {
		desc = e.Pattern
	}
	return fmt.Sprintf("value '%s' must match pattern: %s", e.Value, desc)
}

// AcceptedValuesWithProvider creates a validator from a slice of PatternValue with translation support
func AcceptedValuesWithProvider(patterns []types.PatternValue, provider i18n.MessageProvider) Validator {
	if len(patterns) == 0 {
		return NewFuncValidator(
			"accepted-values[]",
			"pattern",
			"No patterns defined",
			func(value string) error {
				return nil
			},
		)
	}

	// Convert each PatternValue to a RegexValidator
	var validators []Validator
	for _, pv := range patterns {
		// Use NewRegexValidator which already supports translation
		v, err := NewRegexValidator(pv.Pattern, pv.Description, provider)
		if err != nil {
			// If pattern compilation fails, create a failing validator
			v = NewFuncValidator(
				"invalid-pattern",
				"error",
				fmt.Sprintf("Invalid pattern: %s", pv.Pattern),
				func(value string) error {
					return err
				},
			)
		}
		// Mark as converted from AcceptedValues
		if vm, ok := v.(*RegexValidator); ok {
			vm.ValidatorMetadata.isConverted = true
		}
		validators = append(validators, v)
	}

	// Use OneOf to match any of the patterns
	if len(validators) == 1 {
		return validators[0]
	}
	return OneOf(validators...)
}
