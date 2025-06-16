package validation

import (
	"fmt"
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

// URL validates URL format
func URL(schemes ...string) Validator {
	schemesStr := "any"
	if len(schemes) > 0 {
		schemesStr = strings.Join(schemes, ",")
	}

	return NewFuncValidator(
		fmt.Sprintf("url[%s]", schemesStr),
		"format",
		fmt.Sprintf("Valid URL with schemes: %s", schemesStr),
		func(value string) error {
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
		},
	)
}

// MinLength validates minimum string length in Unicode characters (not bytes)
func MinLength(min int) Validator {
	return NewFuncValidator(
		fmt.Sprintf("min-length[%d]", min),
		"length",
		fmt.Sprintf("Minimum length %d characters", min),
		func(value string) error {
			if utf8.RuneCountInString(value) < min {
				return errs.ErrMinLength.WithArgs(min, value)
			}
			return nil
		},
	)
}

// MaxLength validates maximum string length in Unicode characters (not bytes)
func MaxLength(max int) Validator {
	return NewFuncValidator(
		fmt.Sprintf("max-length[%d]", max),
		"length",
		fmt.Sprintf("Maximum length %d characters", max),
		func(value string) error {
			if utf8.RuneCountInString(value) > max {
				return errs.ErrMaxLength.WithArgs(max, value)
			}
			return nil
		},
	)
}

// Length validates exact string length in Unicode characters (not bytes)
func Length(exact int) Validator {
	return NewFuncValidator(
		fmt.Sprintf("length[%d]", exact),
		"length",
		fmt.Sprintf("Exactly %d characters", exact),
		func(value string) error {
			if utf8.RuneCountInString(value) != exact {
				return errs.ErrExactLength.WithArgs(exact, value)
			}
			return nil
		},
	)
}

// ByteLength validates exact string length in bytes (not Unicode characters)
func ByteLength(exact int) Validator {
	return NewFuncValidator(
		fmt.Sprintf("byte-length[%d]", exact),
		"byte-length",
		fmt.Sprintf("Exactly %d bytes", exact),
		func(value string) error {
			if len(value) != exact {
				return errs.ErrExactByteLength.WithArgs(exact, value)
			}
			return nil
		},
	)
}

// MinByteLength validates minimum string length in bytes (not Unicode characters)
func MinByteLength(min int) Validator {
	return NewFuncValidator(
		fmt.Sprintf("min-byte-length[%d]", min),
		"byte-length",
		fmt.Sprintf("Minimum %d bytes", min),
		func(value string) error {
			if len(value) < min {
				return errs.ErrMinByteLength.WithArgs(min, value)
			}
			return nil
		},
	)
}

// MaxByteLength validates maximum string length in bytes (not Unicode characters)
func MaxByteLength(max int) Validator {
	return NewFuncValidator(
		fmt.Sprintf("max-byte-length[%d]", max),
		"byte-length",
		fmt.Sprintf("Maximum %d bytes", max),
		func(value string) error {
			if len(value) > max {
				return errs.ErrMaxByteLength.WithArgs(max, value)
			}
			return nil
		},
	)
}

// Range validates numeric values are within range (inclusive)
func Range(min, max float64) Validator {
	return NewFuncValidator(
		fmt.Sprintf("range[%g,%g]", min, max),
		"range",
		fmt.Sprintf("Number between %g and %g", min, max),
		func(value string) error {
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errs.ErrValueMustBeNumber.WithArgs(value)
			}
			if num < min || num > max {
				return errs.ErrValueBetween.WithArgs(min, max, value)
			}
			return nil
		},
	)
}

// IntRange validates integer values are within range (inclusive)
func IntRange(min, max int) Validator {
	return IntRangeV(min, max)
}

// Min validates numeric minimum
func Min(min float64) Validator {
	return NewFuncValidator(
		fmt.Sprintf("min[%g]", min),
		"range",
		fmt.Sprintf("Minimum value %g", min),
		func(value string) error {
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errs.ErrValueMustBeNumber.WithArgs(value)
			}
			if num < min {
				return errs.ErrValueAtLeast.WithArgs(min, value)
			}
			return nil
		},
	)
}

// Max validates numeric maximum
func Max(max float64) Validator {
	return NewFuncValidator(
		fmt.Sprintf("max[%g]", max),
		"range",
		fmt.Sprintf("Maximum value %g", max),
		func(value string) error {
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errs.ErrValueMustBeNumber.WithArgs(value)
			}
			if num > max {
				return errs.ErrValueAtMost.WithArgs(max, value)
			}
			return nil
		},
	)
}

// Custom allows for custom validation logic
func Custom(name string, fn func(string) error) Validator {
	if fn == nil {
		return NewFuncValidator(
			name+"-noop",
			"custom",
			"No-op validator",
			func(value string) error {
				return nil
			},
		)
	}
	return NewFuncValidator(
		name,
		"custom",
		"Custom validation",
		fn,
	)
}

// Integer validates the value is a valid integer
func Integer() Validator {
	return NewFuncValidator(
		"integer",
		"type",
		"Valid integer",
		func(value string) error {
			if _, err := strconv.Atoi(value); err != nil {
				return errs.ErrValueMustBeInteger.WithArgs(value)
			}
			return nil
		},
	)
}

// Float validates the value is a valid float
func Float() Validator {
	return NewFuncValidator(
		"float",
		"type",
		"Valid floating-point number",
		func(value string) error {
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return errs.ErrValueMustBeNumber.WithArgs(value)
			}
			return nil
		},
	)
}

// Boolean validates the value is a valid boolean
func Boolean() Validator {
	return NewFuncValidator(
		"boolean",
		"type",
		"Valid boolean (true/false)",
		func(value string) error {
			if _, err := strconv.ParseBool(value); err != nil {
				return errs.ErrValueMustBeBoolean.WithArgs(value)
			}
			return nil
		},
	)
}

// AlphaNumeric validates the value contains only letters and numbers
func AlphaNumeric() Validator {
	return NewFuncValidator(
		"alphanumeric",
		"format",
		"Letters and numbers only",
		func(value string) error {
			if value == "" {
				return errs.ErrValueMustBeAlphanumeric.WithArgs(value)
			}
			for _, r := range value {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) {
					return errs.ErrValueMustBeAlphanumeric.WithArgs(value)
				}
			}
			return nil
		},
	)
}

// Identifier validates the value is a valid identifier
func Identifier() Validator {
	return NewFuncValidator(
		"identifier",
		"format",
		"Valid identifier (starts with letter, contains letters/numbers/underscore)",
		func(value string) error {
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
		},
	)
}

// NoWhitespace ensures value has no whitespace
func NoWhitespace() Validator {
	return NewFuncValidator(
		"no-whitespace",
		"format",
		"No whitespace allowed",
		func(value string) error {
			for _, r := range value {
				if unicode.IsSpace(r) {
					return errs.ErrValueMustNotContainWhitespace.WithArgs(value)
				}
			}
			return nil
		},
	)
}

// FileExtension validates file has one of the allowed extensions
func FileExtension(extensions ...string) Validator {
	extsStr := strings.Join(extensions, ",")
	return NewFuncValidator(
		fmt.Sprintf("file-ext[%s]", extsStr),
		"format",
		fmt.Sprintf("File extension must be one of: %s", extsStr),
		func(value string) error {
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
		},
	)
}

// Hostname validates the value is a valid hostname according to RFC 1123
func Hostname() Validator {
	// RFC 1123 hostname validation (ASCII only)
	re := regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))*$`)

	return NewFuncValidator(
		"hostname",
		"format",
		"Valid hostname (RFC 1123)",
		func(value string) error {
			if len(value) > 253 {
				return errs.ErrHostnameTooLong
			}
			if !re.MatchString(value) {
				return errs.ErrInvalidHostnameFormat
			}
			return nil
		},
	)
}

// IP validates the value is a valid IP address (v4 or v6)
func IP() Validator {
	return NewFuncValidator(
		"ip",
		"format",
		"Valid IP address (v4 or v6)",
		func(value string) error {
			if net.ParseIP(value) == nil {
				return errs.ErrValueMustBeValidIP.WithArgs(value)
			}
			return nil
		},
	)
}

// Port validates the value is a valid port number
func Port() Validator {
	return IntRange(1, 65535)
}

// Email validates the value is a valid email address
func Email() Validator {
	return NewFuncValidator(
		"email",
		"format",
		"Valid email address",
		func(value string) error {
			_, err := mail.ParseAddress(value)
			if err != nil {
				return errs.ErrInvalidEmailFormat.WithArgs(value)
			}
			return nil
		},
	)
}

// All creates a validator where all validators must pass
func All(validators ...Validator) Validator {
	return NewAllValidator(validators...)
}
