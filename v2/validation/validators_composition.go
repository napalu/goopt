package validation

import (
	"fmt"
	"strings"

	"github.com/napalu/goopt/v2/errs"
)

// OneOf creates a validator where at least one of the provided validators must pass
// This is a composition operator that accepts any validators, not just string values
func OneOf(validators ...Validator) Validator {
	return NewAnyValidator(validators...)
}

// Not creates a validator that negates another validator
// The validation passes only if the inner validator fails
func Not(validator Validator) Validator {
	return NewFuncValidator(
		fmt.Sprintf("not[%s]", validator.Name()),
		"logic",
		fmt.Sprintf("Not %s", validator.Description()),
		func(value string) error {
			if err := validator.Validate(value); err == nil {
				// Inner validator passed, so NOT should fail
				return errs.ErrValueCannotBe.WithArgs(value, value)
			}
			// Inner validator failed, so NOT should pass
			return nil
		},
	)
}

// Convenience functions for common string matching cases
// These create validators from string lists for backward compatibility

// IsOneOf creates a validator that checks if value is one of the allowed strings
// This is a convenience function equivalent to OneOf(Equals("a"), Equals("b"), ...)
func IsOneOf(allowed ...string) Validator {
	allowedSet := make(map[string]bool)
	for _, v := range allowed {
		allowedSet[v] = true
	}

	return NewFuncValidator(
		fmt.Sprintf("is-one-of[%s]", strings.Join(allowed, ",")),
		"choice",
		fmt.Sprintf("One of: %s", strings.Join(allowed, ", ")),
		func(value string) error {
			if !allowedSet[value] {
				return errs.ErrValueMustBeOneOf.WithArgs(strings.Join(allowed, ", "), value)
			}
			return nil
		},
	)
}

// IsNotOneOf creates a validator that checks if value is NOT one of the forbidden strings
// This is a convenience function equivalent to Not(IsOneOf(...))
func IsNotOneOf(forbidden ...string) Validator {
	return Not(IsOneOf(forbidden...))
}

// Equals creates a validator that checks for exact string match
// Useful for composition with OneOf
func Equals(expected string) Validator {
	return NewFuncValidator(
		fmt.Sprintf("equals[%s]", expected),
		"match",
		fmt.Sprintf("Exactly: %s", expected),
		func(value string) error {
			if value != expected {
				return errs.ErrValueMustBeOneOf.WithArgs(expected, value)
			}
			return nil
		},
	)
}

// Contains creates a validator that checks if value contains a substring
func Contains(substring string) Validator {
	return NewFuncValidator(
		fmt.Sprintf("contains[%s]", substring),
		"substring",
		fmt.Sprintf("Contains: %s", substring),
		func(value string) error {
			if !strings.Contains(value, substring) {
				return errs.ErrPatternMatch.WithArgs(fmt.Sprintf("must contain '%s'", substring), value)
			}
			return nil
		},
	)
}

// HasPrefix creates a validator that checks if value starts with prefix
func HasPrefix(prefix string) Validator {
	return NewFuncValidator(
		fmt.Sprintf("has-prefix[%s]", prefix),
		"prefix",
		fmt.Sprintf("Starts with: %s", prefix),
		func(value string) error {
			if !strings.HasPrefix(value, prefix) {
				return errs.ErrPatternMatch.WithArgs(fmt.Sprintf("must start with '%s'", prefix), value)
			}
			return nil
		},
	)
}

// HasSuffix creates a validator that checks if value ends with suffix
func HasSuffix(suffix string) Validator {
	return NewFuncValidator(
		fmt.Sprintf("has-suffix[%s]", suffix),
		"suffix",
		fmt.Sprintf("Ends with: %s", suffix),
		func(value string) error {
			if !strings.HasSuffix(value, suffix) {
				return errs.ErrPatternMatch.WithArgs(fmt.Sprintf("must end with '%s'", suffix), value)
			}
			return nil
		},
	)
}

// Example compositions:
//
// // Accept multiple ID formats
// OneOf(
//     Email(),
//     Integer(),
//     HasPrefix("EMP-"),
// )
//
// // Username requirements
// All(
//     AlphaNumeric(),
//     MinLength(3),
//     MaxLength(20),
//     Not(IsOneOf("admin", "root", "system")),
//     Not(Integer()), // Not purely numeric
// )
//
// // Complex password
// All(
//     MinLength(12),
//     Not(Contains("password")), // No variation of "password"
//     Not(Contains("123456")),   // No common sequences
//     OneOf(                     // At least one of these character types
//         Contains("!@#$%^&*"),  // Special chars
//         Contains("0123456789"), // Digits
//     ),
// )
