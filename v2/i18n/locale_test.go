package i18n

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
	"strings"
	"testing"
)

func TestNewSystemLocale(t *testing.T) {
	tag := language.Spanish
	translations := `{"key": "value"}`

	locale := NewLocale(tag, translations)

	assert.Equal(t, tag, locale.Tag)
	assert.Equal(t, translations, locale.Translations)
}

func TestLanguageNameToTag(t *testing.T) {
	cases := []struct {
		name     string
		expected language.Tag
	}{
		{"english", language.English},
		{"Deutsch", language.German},
		{"français", language.French},
		{"español", language.Spanish},
		{"日本語", language.Japanese},
		{"العربية", language.Arabic},
		{"עברית", language.Hebrew},
		{"bahasa indonesia", language.Indonesian},
		{"Tiếng Việt", language.Vietnamese},
		{"Ελληνικά", language.Greek},
		{"русский", language.Russian},
		{"čeština", language.Czech},
		{"Polski", language.Polish},
		{"slovenščina", language.Slovenian},
		{"Română", language.Romanian},
	}

	for _, c := range cases {
		got := LanguageNameToLanguageTag(c.name)
		if got != c.expected {
			t.Errorf("LanguageNameToLanguageTag(%q) = %v; want %v", c.name, got, c.expected)
		}
	}
}

func TestTagToNames(t *testing.T) {
	tests := []struct {
		tag         language.Tag
		wantFirst   string // Expect native name to come first
		wantInclude string // Also ensure the English name is included
	}{
		{language.English, "English", "english"},
		{language.French, "Français", "french"},
		{language.German, "Deutsch", "german"},
		{language.Japanese, "日本語", "japanese"},
		{language.Arabic, "العربية", "arabic"},
		{language.Spanish, "Español", "spanish"},
		{language.Korean, "한국어", "korean"},
		{language.Make("zz"), "", ""}, // Undefined
	}

	for _, tt := range tests {
		names := LanguageTagToLanguageNames(tt.tag)
		if len(names) == 0 && tt.wantFirst == "" {
			continue // ok for undefined language
		}

		if got := names[0]; got != tt.wantFirst {
			t.Errorf("LanguageTagToLanguageNames(%v)[0] = %q; want native name %q", tt.tag, got, tt.wantFirst)
		}

		found := false
		for _, name := range names {
			if name == tt.wantInclude {
				found = true
				break
			}
		}
		if tt.wantInclude != "" && !found {
			t.Errorf("LanguageTagToLanguageNames(%v) = %v; want to include %q", tt.tag, names, tt.wantInclude)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	// Pick canonical names from your languageNamesMap values
	names := []string{
		"Français", "العربية", "עברית", "日本語", "Bahasa Indonesia",
		"русский", "čeština", "Polski", "Español", "Deutsch",
	}

	for _, name := range names {
		tag := LanguageNameToLanguageTag(name)
		backNames := LanguageTagToLanguageNames(tag)

		matched := false
		for _, n := range backNames {
			if strings.EqualFold(name, n) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("Roundtrip failed: %q → %v → %v", name, tag, backNames)
		}
	}
}

func TestGetNativeLanguageName(t *testing.T) {
	cases := []struct {
		tag      language.Tag
		expected string
	}{
		{language.English, "English"},
		{language.French, "Français"},
		{language.German, "Deutsch"},
		{language.Japanese, "日本語"},
		{language.Arabic, "العربية"},
		{language.Russian, "Русский"},
		{language.Hindi, "हिन्दी"},
		{language.Korean, "한국어"},
		{language.Chinese, "中文"},
		{language.Make("zz"), "und"}, // fallback
	}

	for _, c := range cases {
		got := GetNativeLanguageName(c.tag)
		if got != c.expected {
			t.Errorf("GetNativeLanguageName(%v [%s]) = %q; want %q", c.tag, c.tag.String(), got, c.expected)
		}
	}
}

func TestIsRTL(t *testing.T) {
	tests := []struct {
		tag      language.Tag
		expected bool
	}{
		{language.Arabic, true},
		{language.Hebrew, true},
		{language.MustParse("fa"), true},  // Persian
		{language.MustParse("ur"), true},  // Urdu
		{language.MustParse("sd"), true},  // Sindhi
		{language.MustParse("dv"), true},  // Divehi
		{language.MustParse("ps"), true},  // Pashto
		{language.MustParse("ku"), true},  // Kurdish (assumed Sorani)
		{language.MustParse("rhg"), true}, // Rohingya
		{language.MustParse("yi"), true},  // Yiddish

		{language.English, false},
		{language.German, false},
		{language.Chinese, false},
		{language.MustParse("ja"), false}, // Japanese
		{language.MustParse("fr"), false},
		{language.Und, false},
	}

	for _, test := range tests {
		if got := IsRTL(test.tag); got != test.expected {
			t.Errorf("IsRTL(%q) = %v; want %v", test.tag, got, test.expected)
		}
	}
}

func TestParseFormatSpecifiers(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected []rune
	}{
		{
			name:     "Simple format",
			format:   "value %d must be at least %d",
			expected: []rune{'d', 'd'},
		},
		{
			name:     "Positional arguments",
			format:   "value '%[2]s' must be at least %[1]d characters",
			expected: []rune{'d', 's'},
		},
		{
			name:     "Mixed types",
			format:   "%s %d %f %v",
			expected: []rune{'s', 'd', 'f', 'v'},
		},
		{
			name:     "With flags and precision",
			format:   "%-10s %+d %.2f %#v",
			expected: []rune{'s', 'd', 'f', 'v'},
		},
		{
			name:     "Escaped percent",
			format:   "100%% complete: %d items",
			expected: []rune{'d'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFormatSpecifiers(tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorLocaleFormatting(t *testing.T) {
	// Create bundles with different format specifiers
	bundle := NewEmptyBundle()

	// Messages with different format verbs
	bundle.AddLanguage(language.English, map[string]string{
		"test.int_format":     "value must be at least %d",
		"test.string_format":  "value must be at least %s",
		"test.mixed_format":   "value '%[2]s' must be at least %[1]d characters",
		"test.generic_format": "value must be between %v and %v",
	})

	bundle.AddLanguage(language.French, map[string]string{
		"test.int_format":     "la valeur doit être au moins %d",
		"test.string_format":  "la valeur doit être au moins %s",
		"test.mixed_format":   "la valeur '%[2]s' doit contenir au moins %[1]d caractères",
		"test.generic_format": "la valeur doit être entre %v et %v",
	})

	// Create layered provider
	defaultBundle, _ := NewBundle()
	layered := NewLayeredMessageProvider(defaultBundle, nil, bundle)
	layered.SetDefaultLanguage(language.French)
	tests := []struct {
		name           string
		errorKey       string
		args           []interface{}
		expectedSubstr string
		description    string
	}{
		{
			name:           "Integer with %d format",
			errorKey:       "test.int_format",
			args:           []interface{}{1000},
			expectedSubstr: "1000", // Should NOT be formatted
			description:    "%d should preserve raw integer",
		},
		{
			name:           "Integer with %s format",
			errorKey:       "test.string_format",
			args:           []interface{}{1000},
			expectedSubstr: "1\u00a0000", // Should be formatted (French)
			description:    "%s should apply locale formatting",
		},
		{
			name:           "Mixed format specifiers",
			errorKey:       "test.mixed_format",
			args:           []interface{}{1000, "test"},
			expectedSubstr: "1000", // %d should stay raw
			description:    "Mixed specifiers should handle each appropriately",
		},
		{
			name:           "Generic format with %v",
			errorKey:       "test.generic_format",
			args:           []interface{}{1000, 2000},
			expectedSubstr: "1\u00a0000", // %v should be formatted
			description:    "%v should apply locale formatting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.errorKey).WithArgs(tt.args...)
			formatted := err.Format(layered)

			assert.Contains(t, formatted, tt.expectedSubstr, tt.description)
			t.Logf("Formatted error: %s", formatted)
		})
	}
}

func TestSmartLocaleFormattingWithRealErrors(t *testing.T) {
	// Test with Swiss German which uses apostrophe separators
	bundle := NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"goopt.error.validation.max_byte_length": "value '%[2]s' must be at most %[1]d bytes long",
		"goopt.error.validation.value_between":   "value '%[3]s' must be between %[1]v and %[2]v",
	})

	swissGerman := language.MustParse("de-CH")
	bundle.AddLanguage(swissGerman, map[string]string{
		"goopt.error.validation.max_byte_length": "Wert '%[2]s' darf höchstens %[1]d Bytes lang sein",
		"goopt.error.validation.value_between":   "Wert '%[3]s' muss zwischen %[1]v und %[2]v liegen",
	})

	defaultBundle, _ := NewBundle()
	layered := NewLayeredMessageProvider(defaultBundle, nil, bundle)
	layered.SetDefaultLanguage(swissGerman)

	// Test %d format (should not be locale formatted)
	err1 := NewError("goopt.error.validation.max_byte_length").WithArgs(1234567, "data")
	formatted1 := err1.Format(layered)
	assert.Contains(t, formatted1, "1234567")      // Raw number
	assert.NotContains(t, formatted1, "1'234'567") // No formatted number in the numeric part

	// Test %v format (should be locale formatted)
	err2 := NewError("goopt.error.validation.value_between").WithArgs(1000, 10000, "500")
	formatted2 := err2.Format(layered)
	// Swiss German formatting - x/text uses different apostrophe character
	t.Logf("Formatted error: %s", formatted2)
	// Check that numbers are formatted (not checking exact separator due to Unicode variations)
	assert.Contains(t, formatted2, "zwischen", "Should contain German text")
	assert.NotContains(t, formatted2, "zwischen 1000 und 10000", "Numbers should be formatted, not raw")
}
