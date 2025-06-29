//go:build windows
// +build windows

package i18n

import (
	"errors"
	"github.com/napalu/goopt/v2/types"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/text/language"
)

const (
	// Windows locale name max length
	localeNameMaxLength = 85
)

// GetSystemLocale detects the system locale on Windows using multiple methods
func GetSystemLocale(getter types.EnvGetter) (language.Tag, error) {
	// Return cached value if available

	// Try multiple methods in order of preference
	// Method 1: Environment variables (for compatibility with Unix/Mac behaviour)
	if tag, err := getLocaleFromEnvironment(getter); err == nil {
		return tag, nil
	}

	// Method 2: Windows API - GetUserDefaultLocaleName (most reliable)
	if tag, err := getLocaleFromWindowsAPI(); err == nil {
		return tag, nil
	}

	// Method 3: Registry (fallback)
	if tag, err := getLocaleFromRegistry(); err == nil {
		return tag, nil
	}

	return language.Und, errors.New("could not detect Windows locale")
}

// getLocaleFromWindowsAPI uses Windows API to get the user's default locale
func getLocaleFromWindowsAPI() (language.Tag, error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")

	// Try GetUserDefaultLocaleName first (Vista+, returns BCP-47 format)
	if getUserDefaultLocaleName := kernel32.NewProc("GetUserDefaultLocaleName"); getUserDefaultLocaleName.Find() == nil {
		buf := make([]uint16, localeNameMaxLength)
		ret, _, _ := getUserDefaultLocaleName.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)

		if ret > 0 {
			localeName := windows.UTF16ToString(buf)
			if tag, err := language.Parse(localeName); err == nil {
				return tag, nil
			}
		}
	}

	// Fallback to GetUserDefaultUILanguage (returns LANGID)
	if getUserDefaultUILanguage := kernel32.NewProc("GetUserDefaultUILanguage"); getUserDefaultUILanguage.Find() == nil {
		ret, _, _ := getUserDefaultUILanguage.Call()
		if ret != 0 {
			langID := uint16(ret)
			// Convert LANGID to locale name
			if localeName := langIDToLocaleName(langID); localeName != "" {
				if tag, err := language.Parse(localeName); err == nil {
					return tag, nil
				}
			}
		}
	}

	return language.Und, errors.New("Windows API locale detection failed")
}

// getLocaleFromRegistry reads locale information from Windows registry
func getLocaleFromRegistry() (language.Tag, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Control Panel\International`, registry.QUERY_VALUE)
	if err != nil {
		return language.Und, err
	}
	defer k.Close()

	// Try LocaleName first (Windows Vista+)
	if localeName, _, err := k.GetStringValue("LocaleName"); err == nil && localeName != "" {
		if tag, err := language.Parse(localeName); err == nil {
			return tag, nil
		}
	}

	// Fallback to building locale from sLanguage and sCountry
	if lang, _, err := k.GetStringValue("sLanguage"); err == nil && lang != "" {
		// sLanguage contains full language name like "English" or "German"
		// Try to map it to a language code
		if tag := LanguageNameToLanguageTag(lang); tag != language.Und {
			return tag, nil
		}
	}

	return language.Und, errors.New("registry locale detection failed")
}

// getLocaleFromEnvironment checks environment variables
func getLocaleFromEnvironment(getter types.EnvGetter) (language.Tag, error) {
	// Check Windows-specific LANGUAGE variable
	if lang := getter("LANGUAGE"); lang != "" {
		lang = NormalizeLocaleString(lang)
		if tag, err := language.Parse(lang); err == nil {
			return tag, nil
		}
	}

	// Check standard Unix-style variables (some Windows apps set these)
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang := os.Getenv(envVar); lang != "" {
			lang = NormalizeLocaleString(lang)
			if tag, err := language.Parse(lang); err == nil {
				return tag, nil
			}
		}
	}

	return language.Und, errors.New("no locale found in environment")
}

// Based on [MS-LCID] Windows Language Code Identifier Reference
// langIDToLocaleName converts a Windows LANGID to a locale name using the
// LCIDToLocaleName Windows API call. This function is designed to work on
// Windows Vista and later. On older systems, it will gracefully fail,
// allowing the system to use other fallback methods for locale detection.
func langIDToLocaleName(langID uint16) string {
	// LCID is a combination of a language identifier and a sort order.
	// We use the default sort order by converting the LANGID to a 32-bit unsigned integer.
	lcid := uint32(langID)

	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel32.NewProc("LCIDToLocaleName")

	// Check if the LCIDToLocaleName function is available on the system.
	if proc.Find() != nil {
		// This API is not available on systems older than Windows Vista.
		// Returning an empty string will cause the calling function to try other fallbacks.
		return ""
	}

	// Create a buffer to hold the locale name.
	buf := make([]uint16, localeNameMaxLength)

	// Call the Windows API function.
	ret, _, _ := proc.Call(
		uintptr(lcid),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0, // Flags (0 for default behavior)
	)

	// If the call fails, return an empty string.
	if ret == 0 {
		return ""
	}

	// Convert the UTF-16 buffer to a Go string and return it.
	return windows.UTF16ToString(buf)
}
