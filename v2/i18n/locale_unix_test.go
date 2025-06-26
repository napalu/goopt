//go:build !windows
// +build !windows

package i18n

import (
	"github.com/napalu/goopt/v2/types"
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
			name:     "BCP-47 format unchanged",
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
		{
			name:     "Complex Unix locale",
			input:    "en_US.iso885915",
			expected: "en-US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeLocaleString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSystemLocale(t *testing.T) {
	tests := []struct {
		name        string
		lcAll       string
		lcMessages  string
		lang        string
		expectedTag language.Tag
		expectError bool
	}{
		{
			name:        "LC_ALL takes precedence",
			lcAll:       "fr_FR.UTF-8",
			lcMessages:  "de_DE.UTF-8",
			lang:        "en_US.UTF-8",
			expectedTag: language.French,
		},
		{
			name:        "LC_MESSAGES when no LC_ALL",
			lcAll:       "",
			lcMessages:  "de_DE.UTF-8",
			lang:        "en_US.UTF-8",
			expectedTag: language.German,
		},
		{
			name:        "LANG when no LC_* vars",
			lcAll:       "",
			lcMessages:  "",
			lang:        "es_ES.UTF-8",
			expectedTag: language.Spanish,
		},
		{
			name:        "No locale set",
			lcAll:       "",
			lcMessages:  "",
			lang:        "",
			expectedTag: language.Und,
			expectError: true,
		},
		{
			name:        "Invalid locale format",
			lcAll:       "invalid",
			lcMessages:  "",
			lang:        "",
			expectedTag: language.Und,
			expectError: true,
		},
		{
			name:        "C locale defaults to English",
			lcAll:       "C",
			lcMessages:  "",
			lang:        "",
			expectedTag: language.MustParse("en-US"),
		},
		{
			name:        "POSIX locale defaults to English",
			lcAll:       "",
			lcMessages:  "POSIX",
			lang:        "",
			expectedTag: language.MustParse("en-US"),
		},
		{
			name:        "Complex locale with charset",
			lcAll:       "",
			lcMessages:  "",
			lang:        "ja_JP.eucJP",
			expectedTag: language.Japanese,
		},
		{
			name:        "Locale with modifier",
			lcAll:       "",
			lcMessages:  "",
			lang:        "de_DE@euro",
			expectedTag: language.German,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var eGet types.EnvGetter = func(key string) string {
				switch key {
				case "LC_ALL":
					return tt.lcAll
				case "LC_MESSAGES":
					return tt.lcMessages
				case "LANG":
					return tt.lang
				default:
					return ""
				}
			}

			tag, err := GetSystemLocale(eGet)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Compare language tags by their string representation
			// This handles cases where language.Parse returns equivalent but different tags
			expectedStr := tt.expectedTag.String()
			actualStr := tag.String()

			// Handle base language matching (e.g., "fr" vs "fr-FR")
			if expectedBase, _ := tt.expectedTag.Base(); expectedBase.String() != "und" {
				if actualBase, _ := tag.Base(); actualBase.String() != "und" {
					// If both have valid base languages, compare those
					if expectedBase.String() == actualBase.String() {
						return // Test passes
					}
				}
			}

			// Otherwise compare the full strings
			assert.Equal(t, expectedStr, actualStr, "Expected %s, got %s", expectedStr, actualStr)
		})
	}
}
