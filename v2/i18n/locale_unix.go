//go:build !windows
// +build !windows

package i18n

import (
	"errors"
	"github.com/napalu/goopt/v2/types"
	"golang.org/x/text/language"
)

// GetSystemLocale detects the system locale on Unix-like systems
func GetSystemLocale(getter types.EnvGetter) (language.Tag, error) {
	// Check locale environment variables in order of precedence
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang := getter(envVar); lang != "" {
			lang = NormalizeLocaleString(lang)
			if tag, err := language.Parse(lang); err == nil {
				return tag, nil
			}
		}
	}

	return language.Und, errors.New("could not detect Unix locale")
}
