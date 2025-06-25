//go:build !windows
// +build !windows

package i18n

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/text/language"
)

// GetSystemLocale detects the system locale on Unix-like systems
func GetSystemLocale() (language.Tag, error) {
	// Check locale environment variables in order of precedence
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang := os.Getenv(envVar); lang != "" {
			lang = NormalizeLocaleString(lang)
			if tag, err := language.Parse(lang); err == nil {
				return tag, nil
			}
		}
	}

	return language.Und, errors.New("could not detect Unix locale")
}

// NormalizeLocaleString converts various locale formats to BCP-47
func NormalizeLocaleString(locale string) string {
	// Handle Unix format
	// "en_US.UTF-8" -> "en-US"
	if idx := strings.Index(locale, "."); idx > 0 {
		locale = locale[:idx]
	}

	// Handle encoding suffix
	// "en_US@euro" -> "en-US"
	if idx := strings.Index(locale, "@"); idx > 0 {
		locale = locale[:idx]
	}

	// Convert underscore to dash
	locale = strings.Replace(locale, "_", "-", -1)

	// Handle special cases
	switch locale {
	case "C", "POSIX":
		return "en-US" // Default to English
	}

	return locale
}
