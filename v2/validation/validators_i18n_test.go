package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLengthValidatorsUnicode(t *testing.T) {
	t.Run("MinLength with Unicode", func(t *testing.T) {
		validator := MinLength(4)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// ASCII
			{"ASCII exact", "test", false},
			{"ASCII longer", "testing", false},
			{"ASCII shorter", "abc", true},

			// Unicode characters
			{"French café", "café", false},      // 4 characters, not 5 bytes
			{"German Grüße", "Grüße", false},    // 5 characters
			{"Japanese こんにちは", "こんにちは", false},  // 5 characters
			{"Emoji 👍👍👍👍", "👍👍👍👍", false},       // 4 emoji
			{"Mixed hello世界", "hello世界", false}, // 7 characters

			// Edge cases
			{"Empty string", "", true},
			{"One emoji", "😀", true},     // 1 character < 4
			{"Three chars", "中文字", true}, // 3 < 4
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("MaxLength with Unicode", func(t *testing.T) {
		validator := MaxLength(5)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// ASCII
			{"ASCII exact", "hello", false},
			{"ASCII shorter", "hi", false},
			{"ASCII longer", "hello!", true},

			// Unicode
			{"French exact", "café!", false},  // 5 characters
			{"German longer", "Grüßen", true}, // 6 characters
			{"Emoji five", "😀😁😂😃😄", false},    // 5 emoji
			{"Emoji six", "😀😁😂😃😄😅", true},     // 6 emoji

			// Combined characters
			{"Hindi", "हिन्दी", true}, // 6 runes (including combining marks) > 5
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Length exact with Unicode", func(t *testing.T) {
		validator := Length(6)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// ASCII
			{"ASCII exact", "hello!", false},
			{"ASCII shorter", "hello", true},
			{"ASCII longer", "hello!!", true},

			// Unicode
			{"Russian exact", "Привет", false},  // 6 Cyrillic characters
			{"Mixed shorter", "Hi世界!", true},    // 2+2+1 = 5 characters, not 6
			{"Japanese exact", "こんにちは!", false}, // 5 hiragana + 1 punctuation = 6
			{"Emoji exact", "👨‍👩‍👧‍👦ab", true},  // Family emoji is 1 grapheme cluster but multiple runes
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestFileExtensionUnicode(t *testing.T) {
	validator := FileExtension(".txt", ".TXT", ".Txt")

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// Case insensitive
		{"lowercase", "file.txt", false},
		{"uppercase", "file.TXT", false},
		{"mixed case", "file.Txt", false},
		{"mixed case 2", "file.tXt", false},

		// Turkish I problem - strings.EqualFold handles this correctly
		{"Turkish lowercase i", "file.txt", false}, // regular i
		{"Extension with İ", ".Tİxt", true},        // Turkish capital İ won't match

		// Wrong extension
		{"Wrong extension", "file.doc", true},
		{"No extension", "file", true},
		{"Multiple dots", "file.backup.txt", false},

		// Unicode filenames
		{"Chinese filename", "文档.txt", false},
		{"Arabic filename", "ملف.TXT", false},
		{"Mixed scripts", "файл文件.Txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParserCaseInsensitive(t *testing.T) {
	tests := []struct {
		name    string
		specs   []string
		wantErr bool
	}{
		// Case variations
		{"lowercase email", []string{"email"}, false},
		{"uppercase EMAIL", []string{"EMAIL"}, false},
		{"mixed case Email", []string{"Email"}, false},

		// Validator aliases
		{"minlen lowercase", []string{"minlen(5)"}, false},
		{"MINLEN uppercase", []string{"MINLEN(5)"}, false},
		{"MinLength mixed", []string{"MinLength(5)"}, false},

		// Multiple validators
		{"mixed case multiple", []string{"EMAIL", "minLENGTH(5)", "ALPHANUMERIC"}, false},

		// Unicode in validator names (though not typical)
		{"accented char", []string{"émAIL"}, true}, // Should fail as unknown
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validators, err := ParseValidators(tt.specs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, validators)
			}
		})
	}
}

func TestHostnameASCIIOnly(t *testing.T) {
	validator := Hostname()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// Valid ASCII hostnames
		{"simple", "example.com", false},
		{"with dash", "my-site.com", false},
		{"subdomain", "www.example.com", false},
		{"multiple subdomains", "api.v2.example.com", false},

		// Invalid - contains Unicode
		{"German umlaut", "münchen.de", true},
		{"Chinese", "中国.cn", true},
		{"Cyrillic", "россия.рф", true},
		{"Arabic", "مصر.eg", true},

		// Valid - Punycode versions
		{"Punycode German", "xn--mnchen-3ya.de", false},
		{"Punycode Chinese", "xn--fiqs8s.cn", false},

		// Invalid format
		{"starts with dash", "-example.com", true},
		{"ends with dash", "example-.com", true},
		{"double dash", "ex--ample.com", false}, // Actually valid
		{"too long", string(make([]byte, 254)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
