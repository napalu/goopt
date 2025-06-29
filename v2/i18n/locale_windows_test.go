//go:build windows
// +build windows

package i18n

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestNormalizeLocaleString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Windows format unchanged",
			input:    "en-US",
			expected: "en-US",
		},
		{
			name:     "Unix format with encoding",
			input:    "en_US.UTF-8",
			expected: "en-US",
		},
		{
			name:     "Unix format with modifier",
			input:    "en_US@euro",
			expected: "en-US",
		},
		{
			name:     "Unix format with both",
			input:    "de_DE.UTF-8@euro",
			expected: "de-DE",
		},
		{
			name:     "C locale",
			input:    "C",
			expected: "en-US",
		},
		{
			name:     "POSIX locale",
			input:    "POSIX",
			expected: "en-US",
		},
		{
			name:     "Simple language code",
			input:    "fr",
			expected: "fr",
		},
		{
			name:     "Language with region underscore",
			input:    "zh_CN",
			expected: "zh-CN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeLocaleString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLangIDToLocaleName(t *testing.T) {
	tests := []struct {
		name     string
		langID   uint16
		expected string
	}{
		{
			name:     "English US",
			langID:   0x0409, // en-US
			expected: "en-US",
		},
		{
			name:     "English GB",
			langID:   0x0809, // en-GB
			expected: "en-GB",
		},
		{
			name:     "German",
			langID:   0x0407,
			expected: "de-DE",
		},
		{
			name:     "French",
			langID:   0x040C,
			expected: "fr-FR",
		},
		{
			name:     "Chinese Simplified",
			langID:   0x0804, // zh-CN
			expected: "zh-CN",
		},
		{
			name:     "Chinese Traditional",
			langID:   0x0404, // zh-TW
			expected: "zh-TW",
		},
		{
			name:     "Japanese",
			langID:   0x0411,
			expected: "ja-JP",
		},
		{
			name:     "Spanish",
			langID:   0x040A,
			expected: "es-ES_tradnl",
		},
		{
			name:     "Unknown language",
			langID:   0xFFFF,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := langIDToLocaleName(tt.langID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapLanguageNameToTag(t *testing.T) {
	tests := []struct {
		name     string
		langName string
		expected language.Tag
	}{
		{
			name:     "English",
			langName: "English",
			expected: language.English,
		},
		{
			name:     "English lowercase",
			langName: "english",
			expected: language.English,
		},
		{
			name:     "German",
			langName: "German",
			expected: language.German,
		},
		{
			name:     "German native",
			langName: "Deutsch",
			expected: language.German,
		},
		{
			name:     "Spanish",
			langName: "Spanish",
			expected: language.Spanish,
		},
		{
			name:     "Spanish native",
			langName: "Español",
			expected: language.Spanish,
		},
		{
			name:     "French",
			langName: "French",
			expected: language.French,
		},
		{
			name:     "French native",
			langName: "Français",
			expected: language.French,
		},
		{
			name:     "Japanese",
			langName: "Japanese",
			expected: language.Japanese,
		},
		{
			name:     "Japanese native",
			langName: "日本語",
			expected: language.Japanese,
		},
		{
			name:     "Chinese",
			langName: "Chinese",
			expected: language.Chinese,
		},
		{
			name:     "Chinese native",
			langName: "中文",
			expected: language.Chinese,
		},
		{
			name:     "Arabic",
			langName: "Arabic",
			expected: language.Arabic,
		},
		{
			name:     "Arabic native",
			langName: "العربية",
			expected: language.Arabic,
		},
		{
			name:     "Hebrew",
			langName: "Hebrew",
			expected: language.Hebrew,
		},
		{
			name:     "Hebrew native",
			langName: "עברית",
			expected: language.Hebrew,
		},
		{
			name:     "Hindi",
			langName: "Hindi",
			expected: language.Hindi,
		},
		{
			name:     "Hindi native",
			langName: "हिन्दी",
			expected: language.Hindi,
		},
		{
			name:     "Unknown language",
			langName: "Klingon",
			expected: language.Und,
		},
		{
			name:     "With spaces",
			langName: "  English  ",
			expected: language.English,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LanguageNameToLanguageTag(tt.langName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSystemLocale_Integration(t *testing.T) {
	// This is an integration test that will actually call Windows APIs
	// It may produce different results on different systems
	t.Run("can detect locale", func(t *testing.T) {
		tag, err := GetSystemLocale(os.Getenv)

		// We should get some locale, even if it's just English
		assert.NoError(t, err)
		assert.NotEqual(t, language.Und, tag)

		tag2, err2 := GetSystemLocale(os.Getenv)
		assert.NoError(t, err2)
		assert.Equal(t, tag, tag2)
	})
}
