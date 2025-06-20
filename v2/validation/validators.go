package validation

import (
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/napalu/goopt/v2/errs"
)

// ValidatorFunc validates a string value and returns an error if invalid
type ValidatorFunc func(value string) error

// All combines multiple validators - all must pass
func All(validators ...ValidatorFunc) ValidatorFunc {
	return func(value string) error {
		for _, validator := range validators {
			if err := validator(value); err != nil {
				return err
			}
		}
		return nil
	}
}

// Any combines multiple validators - at least one must pass
func Any(validators ...ValidatorFunc) ValidatorFunc {
	return func(value string) error {
		var errors []string
		for _, validator := range validators {
			if err := validator(value); err == nil {
				return nil
			} else {
				errors = append(errors, err.Error())
			}
		}
		return errs.ErrValidationCombinedFailed.WithArgs(strings.Join(errors, "; "))
	}
}

// Email validates email format
func Email() ValidatorFunc {
	return func(value string) error {
		_, err := mail.ParseAddress(value)
		if err != nil {
			return errs.ErrInvalidEmailFormat.WithArgs(value).Wrap(err)
		}
		return nil
	}
}

// URL validates URL format
func URL(schemes ...string) ValidatorFunc {
	return func(value string) error {
		u, err := url.Parse(value)
		if err != nil {
			return errs.ErrInvalidURL.WithArgs(err)
		}

		// Check scheme if specified
		if len(schemes) > 0 {
			validScheme := false
			for _, scheme := range schemes {
				if u.Scheme == scheme {
					validScheme = true
					break
				}
			}
			if !validScheme {
				return errs.ErrURLSchemeMustBeOneOf.WithArgs(strings.Join(schemes, ", "))
			}
		}

		// Basic validation
		if u.Host == "" {
			return errs.ErrURLMustHaveHost
		}

		return nil
	}
}

// MinLength validates minimum string length in Unicode characters (not bytes)
// For example, "café" has length 4, not 5 (even though é is 2 bytes in UTF-8)
func MinLength(min int) ValidatorFunc {
	return func(value string) error {
		if utf8.RuneCountInString(value) < min {
			return errs.ErrMinLength.WithArgs(min, value)
		}
		return nil
	}
}

// MaxLength validates maximum string length in Unicode characters (not bytes)
func MaxLength(max int) ValidatorFunc {
	return func(value string) error {
		if utf8.RuneCountInString(value) > max {
			return errs.ErrMaxLength.WithArgs(max, value)
		}
		return nil
	}
}

// Length validates exact string length in Unicode characters (not bytes)
func Length(exact int) ValidatorFunc {
	return func(value string) error {
		if utf8.RuneCountInString(value) != exact {
			return errs.ErrExactLength.WithArgs(exact, value)
		}
		return nil
	}
}

// ByteLength validates exact string length in bytes (not Unicode characters)
// For example, "café" has byte length 5 (not 4) because 'é' is 2 bytes in UTF-8
func ByteLength(exact int) ValidatorFunc {
	return func(value string) error {
		if len(value) != exact {
			return errs.ErrExactByteLength.WithArgs(exact, value)
		}
		return nil
	}
}

// MinByteLength validates minimum string length in bytes (not Unicode characters)
func MinByteLength(min int) ValidatorFunc {
	return func(value string) error {
		if len(value) < min {
			return errs.ErrMinByteLength.WithArgs(min, value)
		}
		return nil
	}
}

// MaxByteLength validates maximum string length in bytes (not Unicode characters)
func MaxByteLength(max int) ValidatorFunc {
	return func(value string) error {
		if len(value) > max {
			return errs.ErrMaxByteLength.WithArgs(max, value)
		}
		return nil
	}
}

// Range validates numeric values are within range (inclusive)
func Range(min, max float64) ValidatorFunc {
	return func(value string) error {
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return errs.ErrValueMustBeNumber.WithArgs(value)
		}
		if num < min || num > max {
			return errs.ErrValueBetween.WithArgs(min, max, value)
		}
		return nil
	}
}

// IntRange validates integer values are within range (inclusive)
func IntRange(min, max int) ValidatorFunc {
	return func(value string) error {
		num, err := strconv.Atoi(value)
		if err != nil {
			return errs.ErrValueMustBeInteger.WithArgs(value)
		}
		if num < min || num > max {
			return errs.ErrValueBetween.WithArgs(min, max, value)
		}
		return nil
	}
}

// Min validates numeric minimum
func Min(min float64) ValidatorFunc {
	return func(value string) error {
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return errs.ErrValueMustBeNumber.WithArgs(value)
		}
		if num < min {
			return errs.ErrValueAtLeast.WithArgs(min, value)
		}
		return nil
	}
}

// Max validates numeric maximum
func Max(max float64) ValidatorFunc {
	return func(value string) error {
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return errs.ErrValueMustBeNumber.WithArgs(value)
		}
		if num > max {
			return errs.ErrValueAtMost.WithArgs(max, value)
		}
		return nil
	}
}

// Custom allows for custom validation logic
func Custom(validator ValidatorFunc) ValidatorFunc {
	if validator == nil {
		return func(value string) error {
			return nil
		}
	}
	return validator
}

// Integer validates the value is a valid integer
func Integer() ValidatorFunc {
	return func(value string) error {
		if _, err := strconv.Atoi(value); err != nil {
			return errs.ErrValueMustBeInteger.WithArgs(value)
		}
		return nil
	}
}

// Float validates the value is a valid float
func Float() ValidatorFunc {
	return func(value string) error {
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errs.ErrValueMustBeNumber.WithArgs(value)
		}
		return nil
	}
}

// Boolean validates the value is a valid boolean
func Boolean() ValidatorFunc {
	return func(value string) error {
		if _, err := strconv.ParseBool(value); err != nil {
			return errs.ErrValueMustBeBoolean.WithArgs(value)
		}
		return nil
	}
}

// AlphaNumeric validates the value contains only letters and numbers
// This is locale-aware and accepts letters/digits from any Unicode script
// It also accepts combining marks (diacritics) that are essential in many languages
func AlphaNumeric() ValidatorFunc {
	return func(value string) error {
		if value == "" {
			return errs.ErrValueMustBeAlphanumeric.WithArgs(value)
		}
		for _, r := range value {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) {
				return errs.ErrValueMustBeAlphanumeric.WithArgs(value)
			}
		}
		return nil
	}
}

// Identifier validates the value is a valid identifier (starts with letter, contains letters, numbers, underscore)
// This is locale-aware and accepts letters from any Unicode script
// It also accepts combining marks (diacritics) that are essential in many languages
func Identifier() ValidatorFunc {
	return func(value string) error {
		if value == "" {
			return errs.ErrValueMustBeIdentifier.WithArgs(value)
		}

		// Check first character must be a letter
		runes := []rune(value)
		if !unicode.IsLetter(runes[0]) {
			return errs.ErrValueMustBeIdentifier.WithArgs(value)
		}

		// Rest can be letters, digits, underscore, or combining marks
		for _, r := range runes[1:] {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && !unicode.IsMark(r) {
				return errs.ErrValueMustBeIdentifier.WithArgs(value)
			}
		}
		return nil
	}
}

// NoWhitespace ensures value has no whitespace
// This is locale-aware and checks for all Unicode whitespace characters
func NoWhitespace() ValidatorFunc {
	return func(value string) error {
		for _, r := range value {
			if unicode.IsSpace(r) {
				return errs.ErrValueMustNotContainWhitespace.WithArgs(value)
			}
		}
		return nil
	}
}

// FileExtension validates file has one of the allowed extensions
// This uses case-insensitive comparison that works correctly for all Unicode characters
func FileExtension(extensions ...string) ValidatorFunc {
	return func(value string) error {
		hasValidExt := false
		for _, ext := range extensions {
			// Use EqualFold for proper Unicode case-insensitive comparison
			if len(value) >= len(ext) && strings.EqualFold(value[len(value)-len(ext):], ext) {
				hasValidExt = true
				break
			}
		}
		if !hasValidExt {
			return errs.ErrFileMustHaveExtension.WithArgs(strings.Join(extensions, ", "))
		}
		return nil
	}
}

// Hostname validates the value is a valid hostname according to RFC 1123
// Note: This validator only accepts ASCII hostnames. For internationalized domain names (IDN),
// the hostname must be converted to Punycode before validation (e.g., "münchen.de" -> "xn--mnchen-3ya.de")
func Hostname() ValidatorFunc {
	// RFC 1123 hostname validation (ASCII only)
	re := regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))*$`)
	return func(value string) error {
		if len(value) > 253 {
			return errs.ErrHostnameTooLong
		}
		if !re.MatchString(value) {
			return errs.ErrInvalidHostnameFormat
		}
		return nil
	}
}

// IP validates the value is a valid IP address (v4 or v6)
func IP() ValidatorFunc {
	return func(value string) error {
		if net.ParseIP(value) == nil {
			return errs.ErrValueMustBeValidIP.WithArgs(value)
		}
		return nil
	}
}

// Port validates the value is a valid port number
func Port() ValidatorFunc {
	return IntRange(1, 65535)
}
