package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByteLengthValidators(t *testing.T) {
	t.Run("MinByteLength", func(t *testing.T) {
		validator := MinByteLength(5)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// ASCII
			{"ASCII exact", "hello", false},
			{"ASCII longer", "hello world", false},
			{"ASCII shorter", "hi", true},

			// Unicode - different byte counts
			{"French café", "café", false},   // 5 bytes (é = 2 bytes)
			{"French caf", "caf", true},      // 3 bytes < 5
			{"German Grüße", "Grüße", false}, // 7 bytes (ü = 2 bytes, ß = 2 bytes)
			{"German Grün", "Grün", false},   // 5 bytes (ü = 2 bytes)
			{"German Gru", "Gru", true},      // 3 bytes < 5

			// Multi-byte characters
			{"Japanese こ", "こ", true},    // 3 bytes < 5
			{"Japanese こん", "こん", false}, // 6 bytes > 5
			{"Chinese 中", "中", true},     // 3 bytes < 5
			{"Chinese 中文", "中文", false},  // 6 bytes > 5

			// Emoji (4 bytes each)
			{"Single emoji", "😀", true}, // 4 bytes < 5
			{"Two emoji", "😀😁", false},  // 8 bytes > 5

			// Mixed
			{"Mixed a中", "a中", true},    // 1 + 3 = 4 bytes < 5
			{"Mixed ab中", "ab中", false}, // 2 + 3 = 5 bytes

			// Edge cases
			{"Empty string", "", true},
			{"Exactly 5 bytes", "12345", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "at least 5 bytes")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("MaxByteLength", func(t *testing.T) {
		validator := MaxByteLength(10)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// ASCII
			{"ASCII exact", "1234567890", false},
			{"ASCII shorter", "hello", false},
			{"ASCII longer", "hello world!", true}, // 12 bytes > 10

			// Unicode
			{"French café!", "café!", false},          // 6 bytes < 10
			{"French café test", "café test", false},  // 10 bytes
			{"French café tests", "café tests", true}, // 11 bytes > 10

			// Multi-byte characters
			{"Japanese こんにちは", "こんにちは", true}, // 15 bytes > 10
			{"Japanese こんに", "こんに", false},    // 9 bytes < 10
			{"Chinese 中文测试", "中文测试", true},    // 12 bytes > 10
			{"Chinese 中文测", "中文测", false},     // 9 bytes < 10

			// Emoji
			{"Three emoji", "😀😁😂", true}, // 12 bytes > 10
			{"Two emoji", "😀😁", false},   // 8 bytes < 10

			// Edge cases
			{"Empty string", "", false},
			{"Exactly 10 bytes", "1234567890", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "at most 10 bytes")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("ByteLength exact", func(t *testing.T) {
		validator := ByteLength(8)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// ASCII
			{"ASCII exact", "12345678", false},
			{"ASCII shorter", "1234567", true},
			{"ASCII longer", "123456789", true},

			// Unicode - must be exactly 8 bytes
			{"French café123", "café123", false},  // 5 + 3 = 8 bytes
			{"French café12", "café12", true},     // 5 + 2 = 7 bytes
			{"French café1234", "café1234", true}, // 5 + 4 = 9 bytes

			// Multi-byte characters
			{"Mixed a中b中", "a中b中", false}, // 1 + 3 + 1 + 3 = 8 bytes
			{"Mixed 中文a", "中文a", true},    // 3 + 3 + 1 = 7 bytes

			// Emoji
			{"Two emoji", "😀😁", false},           // 4 + 4 = 8 bytes
			{"One emoji + text", "😀test", false}, // 4 + 4 = 8 bytes

			// Edge cases
			{"Empty string", "", true},
			{"Exactly 8 bytes ASCII", "abcdefgh", false},
			{"Exactly 7 bytes mixed", "ab中cd", true},             // 2 + 3 + 2 = 7
			{"Exactly 8 bytes mixed corrected", "ab中cde", false}, // 2 + 3 + 3 = 8 bytes
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "exactly 8 bytes")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Byte counting examples", func(t *testing.T) {
		// Test to verify our understanding of byte counts
		tests := []struct {
			text     string
			expected int
		}{
			{"café", 5},     // c(1) + a(1) + f(1) + é(2) = 5
			{"Grüße", 7},    // G(1) + r(1) + ü(2) + ß(2) + e(1) = 7
			{"こんにちは", 15},   // Each hiragana character is 3 bytes
			{"中文", 6},       // Each Chinese character is 3 bytes
			{"😀", 4},        // Emoji is 4 bytes
			{"👨‍👩‍👧‍👦", 25}, // Family emoji with zero-width joiners
			{"naïve", 6},    // n(1) + a(1) + ï(2) + v(1) + e(1) = 6
			{"Москва", 12},  // Each Cyrillic character is 2 bytes
			{"مرحبا", 10},   // Each Arabic character is 2 bytes
		}

		for _, tt := range tests {
			t.Run(tt.text, func(t *testing.T) {
				assert.Equal(t, tt.expected, len(tt.text))
			})
		}
	})
}

func TestByteLengthValidatorsViaParser(t *testing.T) {
	t.Run("struct tags with byte length validators", func(t *testing.T) {
		specs := []string{
			"minbytelength(5)",
			"maxbytelength(10)",
			"bytelength(8)",
		}

		validators, err := ParseValidators(specs)
		assert.NoError(t, err)
		assert.Len(t, validators, 3)

		// Test minbytelength
		assert.NoError(t, validators[0].Validate("hello")) // 5 bytes
		assert.NoError(t, validators[0].Validate("café!")) // 6 bytes
		assert.Error(t, validators[0].Validate("hi"))      // 2 bytes

		// Test maxbytelength
		assert.NoError(t, validators[1].Validate("hello"))      // 5 bytes
		assert.NoError(t, validators[1].Validate("1234567890")) // 10 bytes
		assert.Error(t, validators[1].Validate("hello world!")) // 12 bytes

		// Test bytelength
		assert.NoError(t, validators[2].Validate("12345678")) // 8 bytes
		assert.NoError(t, validators[2].Validate("café123"))  // 8 bytes
		assert.Error(t, validators[2].Validate("1234567"))    // 7 bytes
	})

	t.Run("short forms", func(t *testing.T) {
		specs := []string{
			"minbytelen(5)",
			"maxbytelen(10)",
			"bytelen(8)",
		}

		validators, err := ParseValidators(specs)
		assert.NoError(t, err)
		assert.Len(t, validators, 3)

		// Should work the same as long forms
		assert.NoError(t, validators[0].Validate("hello")) // 5 bytes
		assert.Error(t, validators[0].Validate("hi"))      // 2 bytes
	})

	t.Run("negative values", func(t *testing.T) {
		_, err := ParseValidators([]string{"minbytelength(-5)"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be negative")

		_, err = ParseValidators([]string{"maxbytelength(-10)"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be negative")

		_, err = ParseValidators([]string{"bytelength(-8)"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be negative")
	})

	t.Run("missing arguments", func(t *testing.T) {
		_, err := ParseValidators([]string{"minbytelength()"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires 1 argument")

		_, err = ParseValidators([]string{"maxbytelength"})
		assert.Error(t, err)
		// Without parentheses, it's treated as a no-arg validator
		// which will fail with "requires 1 argument"

		_, err = ParseValidators([]string{"bytelength(5,10)"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires 1 argument")
	})
}
