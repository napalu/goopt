package validation

import (
	"strings"

	"github.com/napalu/goopt/v2/errs"
)

// OneOf creates a validator where at least one of the provided validators must pass
// This is a composition operator that accepts any validators, not just string values
func OneOf(validators ...ValidatorFunc) ValidatorFunc {
	if len(validators) == 0 {
		return func(value string) error {
			return nil // No validators means always pass
		}
	}

	return func(value string) error {
		var errors []string
		for _, validator := range validators {
			if err := validator(value); err == nil {
				return nil // At least one passed
			} else {
				errors = append(errors, err.Error())
			}
		}
		// None passed - return combined error
		return errs.ErrValidationCombinedFailed.WithArgs(strings.Join(errors, " OR "))
	}
}

// Not creates a validator that negates another validator
// The validation passes only if the inner validator fails
func Not(validator ValidatorFunc) ValidatorFunc {
	return func(value string) error {
		if err := validator(value); err == nil {
			// Inner validator passed, so NOT should fail
			return errs.ErrValueCannotBe.WithArgs(value, value)
		}
		// Inner validator failed, so NOT should pass
		return nil
	}
}

// Convenience functions for common string matching cases
// These create validators from string lists for backward compatibility

// IsOneOf creates a validator that checks if value is one of the allowed strings
// This is a convenience function equivalent to OneOf(Equals("a"), Equals("b"), ...)
func IsOneOf(allowed ...string) ValidatorFunc {
	allowedSet := make(map[string]bool)
	for _, v := range allowed {
		allowedSet[v] = true
	}

	return func(value string) error {
		if !allowedSet[value] {
			return errs.ErrValueMustBeOneOf.WithArgs(value, strings.Join(allowed, ", "))
		}
		return nil
	}
}

// IsNotOneOf creates a validator that checks if value is NOT one of the forbidden strings
// This is a convenience function equivalent to Not(IsOneOf(...))
func IsNotOneOf(forbidden ...string) ValidatorFunc {
	return Not(IsOneOf(forbidden...))
}

// Equals creates a validator that checks for exact string match
// Useful for composition with OneOf
func Equals(expected string) ValidatorFunc {
	return func(value string) error {
		if value != expected {
			return errs.ErrValueMustBeOneOf.WithArgs(value, expected)
		}
		return nil
	}
}

// Contains creates a validator that checks if value contains a substring
func Contains(substring string) ValidatorFunc {
	return func(value string) error {
		if !strings.Contains(value, substring) {
			return errs.ErrPatternMatch.WithArgs(substring, value)
		}
		return nil
	}
}

// HasPrefix creates a validator that checks if value starts with prefix
func HasPrefix(prefix string) ValidatorFunc {
	return func(value string) error {
		if !strings.HasPrefix(value, prefix) {
			return errs.ErrPatternMatch.WithArgs(prefix, value)
		}
		return nil
	}
}

// HasSuffix creates a validator that checks if value ends with suffix
func HasSuffix(suffix string) ValidatorFunc {
	return func(value string) error {
		if !strings.HasSuffix(value, suffix) {
			return errs.ErrPatternMatch.WithArgs("must end with '" + suffix + "'")
		}
		return nil
	}
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
