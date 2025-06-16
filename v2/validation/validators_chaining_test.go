package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function for tests to handle regex creation with error
func mustRegex(v Validator, err error) Validator {
	if err != nil {
		panic(err)
	}
	return v
}

func TestValidatorChaining(t *testing.T) {
	t.Run("Multiple validators via ParseValidators", func(t *testing.T) {
		// Test that we can chain multiple validators
		specs := []string{
			"minlength(5)",
			"maxlength(10)",
			"alphanumeric",
			"nowhitespace",
		}

		validators, err := ParseValidators(specs)
		assert.NoError(t, err)
		assert.Len(t, validators, 4)

		// Create a combined validator using All
		combined := All(validators...)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid", "Hello123", false},
			{"Too short", "Hi12", true},             // fails minlength:5
			{"Too long", "Hello123456", true},       // fails maxlength:10
			{"Has space", "Hello 123", true},        // fails nowhitespace
			{"Has special char", "Hello@123", true}, // fails alphanumeric
			{"Empty", "", true},                     // fails minlength:5
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := combined.Validate(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Multiple regex validators", func(t *testing.T) {
		// Since we can't use commas in regex patterns in struct tags,
		// we need to test programmatically
		validators := []Validator{
			mustRegex(Regex("^[A-Z]", "Must start with uppercase")), // Must start with uppercase letter
			mustRegex(Regex("[0-9]$", "Must end with digit")),       // Must end with digit
			mustRegex(Regex("^.{5,10}$", "Must be 5-10 chars")),     // Must be 5-10 chars long
		}

		combined := All(validators...)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid", "Hello1", false},
			{"Valid longer", "Testing123", false},
			{"No uppercase start", "hello1", true},
			{"No digit end", "HelloX", true},
			{"Too short", "Hi1", true},
			{"Too long", "HelloWorld123", true},
			{"All lowercase", "hello123", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := combined.Validate(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Regex with comma workaround", func(t *testing.T) {
		// To use regex with length requirements in struct tags,
		// combine with dedicated length validators
		specs := []string{
			"regex(^[A-Z][a-z]+[0-9]+$)", // Pattern without commas
			"minlength(5)",
			"maxlength(10)",
		}

		validators, err := ParseValidators(specs)
		assert.NoError(t, err)
		assert.Len(t, validators, 3)

		combined := All(validators...)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid min", "Hello1", false},
			{"Valid max", "Testing123", false},
			{"Too short", "Hi1", true},
			{"Too long", "HelloWorld123", true},
			{"Wrong pattern", "hello123", true},
			{"Wrong pattern 2", "HELLO123", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := combined.Validate(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("OneOf combinator", func(t *testing.T) {
		// Test the OneOf combinator - at least one must pass
		validators := []Validator{
			Email(), // Valid email
			mustRegex(Regex("^[0-9]{10}$", "10-digit phone")),       // OR 10 digit number
			mustRegex(Regex("^[A-Z]{2}-[0-9]{4}$", "Code XX-1234")), // OR pattern like XX-1234
		}

		combined := OneOf(validators...)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid email", "user@example.com", false},
			{"Valid phone", "1234567890", false},
			{"Valid code", "CA-1234", false},
			{"Invalid all", "not-valid", true},
			{"Almost phone", "123456789", true}, // 9 digits
			{"Almost code", "ca-1234", true},    // lowercase
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := combined.Validate(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
