package validation

import (
	"regexp"

	"github.com/napalu/goopt/v2/i18n"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/internal/parse"
)

// Regex creates a validator from a regex pattern and description
// This matches the functionality of AcceptedValues but is composable
//
// Examples:
//
//	Regex("^[A-Z]+$", "Uppercase letters only")
//	Regex("^\\d{5}$", "5-digit ZIP code")
//	Regex("^[A-Z]{2},\\d{4}$", "Format: XX,1234")
func Regex(pattern, msgOrKey string) ValidatorFunc {
	// Compile the regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return func(value string) error {
			return errs.ErrInvalidValidator.WithArgs(pattern, err)
		}
	}

	// Return validator that uses the compiled pattern
	return func(value string) error {
		if !re.MatchString(value) {
			return errs.ErrPatternMatch.WithArgs(i18n.NewTranslatable(msgOrKey), value)
		}
		return nil
	}
}

// RegexSpec creates a validator from a regex specification using the same syntax as AcceptedValues
// This is primarily for internal use when converting AcceptedValues to validators
//
// Examples:
//
//	RegexSpec("{pattern:^[A-Z]+$,desc:Uppercase letters only}")
//	RegexSpec("{pattern:^\\d{5}$,desc:5-digit ZIP code}")
func RegexSpec(spec string) ValidatorFunc {
	// Parse using the same parser as AcceptedValues
	pv, err := parse.PatternValue(spec)
	if err != nil {
		// Return a validator that always fails with parse error
		return func(value string) error {
			return errs.ErrInvalidValidator.WithArgs(spec, err)
		}
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
//
// Example:
//
//	Regexes(
//	  RegexPattern{"^\\d{5}$", "5-digit ZIP"},
//	  RegexPattern{"^\\d{5}-\\d{4}$", "ZIP+4 format"},
//	  RegexPattern{"^[A-Z]\\d[A-Z] \\d[A-Z]\\d$", "Canadian postal code"},
//	)
func Regexes(patterns ...RegexPattern) ValidatorFunc {
	var validators []ValidatorFunc
	for _, p := range patterns {
		validators = append(validators, Regex(p.Pattern, p.Description))
	}
	return OneOf(validators...)
}

// MustMatch is an alias for Regex to make intent clearer in some contexts
//
// Example:
//
//	All(
//	  MustMatch("^[A-Z]", "Must start with uppercase"),
//	  MinLength(5),
//	)
func MustMatch(pattern, description string) ValidatorFunc {
	return Regex(pattern, description)
}

// MustNotMatch creates a validator that ensures a value does NOT match a pattern
//
// Example:
//
//	MustNotMatch("^test", "Must not start with 'test'")
func MustNotMatch(pattern, description string) ValidatorFunc {
	return Not(Regex(pattern, description))
}

// Examples of using Regex validators with composition:
//
// // Email from specific domain with length constraints
// All(
//   Regex("{pattern:.*@company\\.com$,desc:Company email required}"),
//   MinLength(5),
//   MaxLength(100),
// )
//
// // ID that could be in multiple formats
// OneOf(
//   Regex("{pattern:^EMP-\\d{6}$,desc:Employee ID (EMP-123456)}"),
//   Regex("{pattern:^USR-\\d{8}$,desc:User ID (USR-12345678)}"),
//   Regex("{pattern:^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$,desc:UUID}"),
// )
//
// // Complex validation with patterns and logic
// All(
//   OneOf(
//     Regex("{pattern:^\\+\\d{1,3}-\\d{10}$,desc:International (+1-1234567890)}"),
//     Regex("{pattern:^\\d{10}$,desc:US domestic (1234567890)}"),
//   ),
//   Not(Regex("{pattern:^\\+0,desc:Invalid country code +0}")),
//   Not(IsOneOf("+1-0000000000", "+1-9999999999")), // Reject test numbers
// )
