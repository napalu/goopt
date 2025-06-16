package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlphaNumericUnicode(t *testing.T) {
	validator := AlphaNumeric()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// ASCII
		{"ASCII letters", "abcABC", false},
		{"ASCII digits", "123", false},
		{"ASCII mixed", "abc123", false},

		// German
		{"German umlauts", "äöüÄÖÜß", false},
		{"German mixed", "Straße123", false},

		// French
		{"French accents", "àâçèéêëîïôùûü", false},
		{"French mixed", "café123", false},

		// Greek
		{"Greek letters", "αβγδΑΒΓΔ", false},
		{"Greek mixed", "αβγ123", false},

		// Cyrillic
		{"Cyrillic letters", "абвгдАБВГД", false},
		{"Cyrillic mixed", "Москва123", false},

		// Arabic
		{"Arabic letters", "أبجد", false},
		{"Arabic digits", "٠١٢٣", false}, // Arabic-Indic digits
		{"Arabic mixed", "أبجد123", false},

		// Chinese
		{"Chinese characters", "中文字符", false},
		{"Chinese mixed", "中文123", false},

		// Japanese
		{"Hiragana", "あいうえお", false},
		{"Katakana", "アイウエオ", false},
		{"Kanji", "日本語", false},
		{"Japanese mixed", "日本123", false},

		// Korean
		{"Korean Hangul", "한글", false},
		{"Korean mixed", "한글123", false},

		// Hebrew
		{"Hebrew letters", "אבגד", false},
		{"Hebrew mixed", "עברית123", false},

		// Devanagari (Hindi)
		{"Devanagari letters", "अआइई", false},
		{"Devanagari digits", "०१२३", false},
		{"Devanagari mixed", "हिन्दी123", false},

		// Invalid cases
		{"With space", "abc 123", true},
		{"With punctuation", "abc,123", true},
		{"With symbols", "abc@123", true},
		{"With dash", "abc-123", true},
		{"With underscore", "abc_123", true},
		{"Empty string", "", true},
		{"Only spaces", "   ", true},
		{"Emoji", "😀", true},
		{"Currency symbols", "$€¥", true},
		{"Mathematical symbols", "±×÷", true},
		{"Zero-width space", "abc\u200b123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.value)
			if tt.wantErr {
				assert.Error(t, err, "Expected error for value: %s", tt.value)
			} else {
				assert.NoError(t, err, "Expected no error for value: %s", tt.value)
			}
		})
	}
}

func TestIdentifierUnicode(t *testing.T) {
	validator := Identifier()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// ASCII
		{"ASCII simple", "variable", false},
		{"ASCII with underscore", "my_var", false},
		{"ASCII with digits", "var123", false},
		{"ASCII full", "my_var_123", false},

		// Must start with letter
		{"Starts with digit", "123var", true},
		{"Starts with underscore", "_var", true},

		// Unicode identifiers
		{"German identifier", "überprüfung", false},
		{"French identifier", "données", false},
		{"Greek identifier", "μεταβλητή", false},
		{"Cyrillic identifier", "переменная", false},
		{"Arabic identifier", "متغير", false},
		{"Chinese identifier", "变量", false},
		{"Japanese identifier", "変数", false},
		{"Korean identifier", "변수", false},
		{"Hebrew identifier", "משתנה", false},

		// Mixed scripts with underscore
		{"Unicode with underscore", "über_prüfung", false},
		{"Unicode with digits", "données123", false},
		{"Mixed scripts", "hello世界", false},
		{"Complex identifier", "user_データ_123", false},

		// Invalid cases
		{"With space", "my var", true},
		{"With dash", "my-var", true},
		{"With dot", "my.var", true},
		{"Empty string", "", true},
		{"Only underscore", "_", true},
		{"Only digits", "123", true},
		{"Special characters", "var$", true},
		{"Emoji", "😀var", true},
		{"Starts with emoji", "😀", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.value)
			if tt.wantErr {
				assert.Error(t, err, "Expected error for value: %s", tt.value)
			} else {
				assert.NoError(t, err, "Expected no error for value: %s", tt.value)
			}
		})
	}
}

func TestNoWhitespaceUnicode(t *testing.T) {
	validator := NoWhitespace()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// Valid cases
		{"Simple text", "hello", false},
		{"With digits", "hello123", false},
		{"With underscore", "hello_world", false},
		{"With dash", "hello-world", false},
		{"Unicode text", "你好世界", false},
		{"Mixed scripts", "helloмир世界", false},

		// Various whitespace characters
		{"Regular space", "hello world", true},
		{"Tab", "hello\tworld", true},
		{"Newline", "hello\nworld", true},
		{"Carriage return", "hello\rworld", true},
		{"Form feed", "hello\fworld", true},
		{"Vertical tab", "hello\vworld", true},

		// Unicode whitespace
		{"Non-breaking space", "hello\u00A0world", true},
		{"En space", "hello\u2002world", true},
		{"Em space", "hello\u2003world", true},
		{"Thin space", "hello\u2009world", true},
		{"Hair space", "hello\u200Aworld", true},
		{"Line separator", "hello\u2028world", true},
		{"Paragraph separator", "hello\u2029world", true},

		// Edge cases
		{"Empty string", "", false},
		{"Only spaces", "   ", true},
		{"Leading space", " hello", true},
		{"Trailing space", "hello ", true},
		{"Multiple spaces", "hello  world", true},

		// Note: Zero-width space is NOT considered whitespace by unicode.IsSpace
		{"Zero-width space", "hello\u200Bworld", false},
		{"Zero-width non-joiner", "hello\u200Cworld", false},
		{"Zero-width joiner", "hello\u200Dworld", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.value)
			if tt.wantErr {
				assert.Error(t, err, "Expected error for value: %q", tt.value)
			} else {
				assert.NoError(t, err, "Expected no error for value: %q", tt.value)
			}
		})
	}
}
