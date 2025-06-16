package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompositionalValidators(t *testing.T) {
	t.Run("OneOf with multiple validator types", func(t *testing.T) {
		// Accept email, URL, or integer
		validator := OneOf(
			Email(),
			URL("http", "https"),
			Integer(),
		)

		tests := []struct {
			value string
			valid bool
		}{
			{"user@example.com", true},   // Valid email
			{"http://example.com", true}, // Valid URL
			{"12345", true},              // Valid integer
			{"not-any-format", false},    // None match
			{"user@", false},             // Invalid email
			{"ftp://example.com", false}, // Wrong URL scheme
			{"12.34", false},             // Not integer
		}

		for _, tt := range tests {
			err := validator.Validate(tt.value)
			if tt.valid {
				assert.NoError(t, err, "Expected %s to be valid", tt.value)
			} else {
				assert.Error(t, err, "Expected %s to be invalid", tt.value)
			}
		}
	})

	t.Run("Not validator negation", func(t *testing.T) {
		tests := []struct {
			name      string
			validator Validator
			value     string
			valid     bool
		}{
			{
				name:      "Not integer",
				validator: Not(Integer()),
				value:     "abc",
				valid:     true,
			},
			{
				name:      "Not integer fails",
				validator: Not(Integer()),
				value:     "123",
				valid:     false,
			},
			{
				name:      "Not email",
				validator: Not(Email()),
				value:     "just-text",
				valid:     true,
			},
			{
				name:      "Not email fails",
				validator: Not(Email()),
				value:     "user@example.com",
				valid:     false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.validator.Validate(tt.value)
				if tt.valid {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("Complex composition", func(t *testing.T) {
		// Username: alphanumeric, 3-20 chars, not a reserved word, not purely numeric
		validator := All(
			AlphaNumeric(),
			MinLength(3),
			MaxLength(20),
			Not(IsOneOf("admin", "root", "system", "user")),
			Not(Integer()), // Not purely numeric
		)

		tests := []struct {
			value  string
			valid  bool
			reason string
		}{
			{"john123", true, "valid username"},
			{"abc", true, "minimum length"},
			{"admin", false, "reserved word"},
			{"12345", false, "purely numeric"},
			{"ab", false, "too short"},
			{"verylongusernamethatexceedslimit", false, "too long"},
			{"user-name", false, "not alphanumeric"},
			{"user", false, "reserved word"},
		}

		for _, tt := range tests {
			t.Run(tt.reason, func(t *testing.T) {
				err := validator.Validate(tt.value)
				if tt.valid {
					assert.NoError(t, err, "Expected %s to be valid: %s", tt.value, tt.reason)
				} else {
					assert.Error(t, err, "Expected %s to be invalid: %s", tt.value, tt.reason)
				}
			})
		}
	})

	t.Run("IsOneOf and IsNotOneOf convenience functions", func(t *testing.T) {
		colors := IsOneOf("red", "green", "blue")
		notReserved := IsNotOneOf("admin", "root", "system")

		// Test IsOneOf
		assert.NoError(t, colors.Validate("red"))
		assert.NoError(t, colors.Validate("green"))
		assert.NoError(t, colors.Validate("blue"))
		assert.Error(t, colors.Validate("yellow"))
		assert.Error(t, colors.Validate(""))

		// Test IsNotOneOf
		assert.NoError(t, notReserved.Validate("user"))
		assert.NoError(t, notReserved.Validate("john"))
		assert.Error(t, notReserved.Validate("admin"))
		assert.Error(t, notReserved.Validate("root"))
		assert.Error(t, notReserved.Validate("system"))
	})

	t.Run("String helper validators", func(t *testing.T) {
		t.Run("Contains", func(t *testing.T) {
			validator := Contains("@example.com")
			assert.NoError(t, validator.Validate("user@example.com"))
			assert.NoError(t, validator.Validate("admin@example.com"))
			assert.Error(t, validator.Validate("user@other.com"))
		})

		t.Run("HasPrefix", func(t *testing.T) {
			validator := HasPrefix("EMP-")
			assert.NoError(t, validator.Validate("EMP-12345"))
			assert.NoError(t, validator.Validate("EMP-ABC"))
			assert.Error(t, validator.Validate("USR-12345"))
			assert.Error(t, validator.Validate("12345"))
		})

		t.Run("HasSuffix", func(t *testing.T) {
			validator := HasSuffix(".pdf")
			assert.NoError(t, validator.Validate("document.pdf"))
			assert.NoError(t, validator.Validate("report.pdf"))
			assert.Error(t, validator.Validate("image.jpg"))
			assert.Error(t, validator.Validate("pdf"))
		})

		t.Run("Equals", func(t *testing.T) {
			validator := Equals("exact-match")
			assert.NoError(t, validator.Validate("exact-match"))
			assert.Error(t, validator.Validate("EXACT-MATCH"))
			assert.Error(t, validator.Validate("exact"))
			assert.Error(t, validator.Validate(""))
		})
	})

	t.Run("Nested composition", func(t *testing.T) {
		// Accept various ID formats but not test IDs
		validator := All(
			OneOf(
				Email(),
				HasPrefix("EMP-"),
				HasPrefix("USR-"),
				Integer(),
			),
			Not(HasPrefix("test-")),
			Not(Equals("0")),
		)

		tests := []struct {
			value  string
			valid  bool
			reason string
		}{
			{"user@example.com", true, "valid email"},
			{"EMP-12345", true, "valid employee ID"},
			{"USR-ABC", true, "valid user ID"},
			{"98765", true, "valid numeric ID"},
			{"test-user@example.com", false, "test email"},
			{"test-123", false, "test ID"},
			{"0", false, "zero ID"},
			{"random-string", false, "no valid format"},
		}

		for _, tt := range tests {
			t.Run(tt.reason, func(t *testing.T) {
				err := validator.Validate(tt.value)
				if tt.valid {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("Empty validators", func(t *testing.T) {
		// OneOf with no validators should always pass
		emptyOneOf := OneOf()
		assert.NoError(t, emptyOneOf.Validate("anything"))
		assert.NoError(t, emptyOneOf.Validate(""))

		// All with no validators should always pass
		emptyAll := All()
		assert.NoError(t, emptyAll.Validate("anything"))
		assert.NoError(t, emptyAll.Validate(""))
	})
}
