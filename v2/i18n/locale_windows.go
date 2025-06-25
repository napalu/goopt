//go:build windows
// +build windows

package i18n

import (
	"errors"
	"os"
	"st
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/text/language"
)

const (
	// Windows locale name max length
	localeNameMaxLength = 85
)

var (
	// Cache for locale detection to avoid repeated API calls
	cachedWindowsLocale *language.Tag
)

// GetSystemLocale detects the system locale on Windows using multiple methods
func GetSystemLocale() (language.Tag, error) {
	// Return cached value if available
	if cachedWindowsLocale != nil {
		return *cachedWindowsLocale, nil
	}


	
	// Method 1: Windows API - GetUserDefaultLocaleName (most reliable)
	if tag, err := getLocaleFromWindowsAPI(); err == nil {
		cachedWindowsLocale = &tag
		return tag, nil
	}

	// Method 2: Registry (fallback)
	if tag, err := getLocaleFromRegistry(); err == nil {
		cachedWindowsLocale = &tag
		return tag, nil
	}

	// Method 3: Environment variables (last resort)
	if tag, err := getLocaleFromEnvironment(); err == nil {
		cachedWindowsLocale = &tag
		return tag, nil
	}

	return language.Und, errors.New("could not detect Windows locale")
}

// getLocaleFromWindowsAPI uses Windows API to get the user's default locale
func getLocaleFromWindowsAPI() (language.Tag, error) {

	
	// Try GetUserDefaultLocaleName first (Vista+, returns BCP-47 format)
	if getUserDefaultLocaleName := kernel32.NewProc("GetUserDefaultLocaleName"); getUserDefaultLocaleName.Find() == nil {
		buf := make([]uint16, localeNameMaxLength)
		ret, _, _ := getUserDefaultLocaleName.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),

		
		if ret > 0 {
			localeName := windows.UTF16ToString(buf)
			if tag, err := language.Parse(localeName); err == nil {
				return tag, nil
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

	
	return language.Und, errors.New("Windows API locale detection failed")
}

// getLocaleFromRegistry reads locale information from Windows registry
	k, err := registry.OpenKey(registry.CURRENT_USER,
	k, err := registry.OpenKey(registry.CURRENT_USER, 
		`Control Panel\International`, registry.QUERY_VALUE)
	if err != nil {
		return language.Und, err
	}

	
	// Try LocaleName first (Windows Vista+)
	if localeName, _, err := k.GetStringValue("LocaleName"); err == nil && localeName != "" {
		if tag, err := language.Parse(localeName); err == nil {
			return tag, nil
		}

	
	// Fallback to building locale from sLanguage and sCountry
	if lang, _, err := k.GetStringValue("sLanguage"); err == nil && lang != "" {
		// sLanguage contains full language name like "English" or "German"
		// Try to map it to a language code
		if tag := mapLanguageNameToTag(lang); tag != language.Und {
			return tag, nil
		}

	
	return language.Und, errors.New("registry locale detection failed")
}

// getLocaleFromEnvironment checks environment variables as a last resort
func getLocaleFromEnvironment() (language.Tag, error) {
	if lang := os.Getenv("LANGUAGE"); lang != "" {
	if lang := syscall.Getenv("LANGUAGE"); lang != "" {
		lang = NormalizeLocaleString(lang)
		if tag, err := language.Parse(lang); err == nil {
			return tag, nil
		}

	
	// Check standard Unix-style variables (some Windows apps set these)
		if lang := os.Getenv(envVar); lang != "" {
		if lang := syscall.Getenv(envVar); lang != "" {
			lang = NormalizeLocaleString(lang)
			if tag, err := language.Parse(lang); err == nil {
				return tag, nil
			}
		}

	
	return language.Und, errors.New("no locale found in environment")
}

// NormalizeLocaleString converts various locale formats to BCP-47
func NormalizeLocaleString(locale string) string {
	// Handle Windows format (already BCP-47 compatible)

	
	// Handle Unix format
	// "en_US.UTF-8" -> "en-US"
	if idx := strings.Index(locale, "."); idx > 0 {
		locale = locale[:idx]

	
	// Handle encoding suffix
	// "en_US@euro" -> "en-US"
	if idx := strings.Index(locale, "@"); idx > 0 {
		locale = locale[:idx]

	
	// Convert underscore to dash

	
	// Handle special cases
	switch locale {
	case "C", "POSIX":
		return "en-US" // Default to English

	
	return locale
}

// langIDToLocaleName converts a Windows LANGID to a locale name
// This is a simplified mapping - a full implementation would use LCIDToLocaleName API
func langIDToLocaleName(langID uint16) string {
	// Extract primary language ID (lower 10 bits)
	primaryLangID := langID & 0x3FF
	// Extract sublanguage ID (upper 6 bits)

	
	// Common language mappings
	langMap := map[uint16]string{
		0x09: "en", // English
		0x07: "de", // German
		0x0A: "es", // Spanish
		0x0C: "fr", // French
		0x10: "it", // Italian
		0x11: "ja", // Japanese
		0x12: "ko", // Korean
		0x13: "nl", // Dutch
		0x16: "pt", // Portuguese
		0x19: "ru", // Russian
		0x04: "zh", // Chinese
		0x01: "ar", // Arabic
		0x0D: "he", // Hebrew
		0x39: "hi", // Hindi

	
	base, ok := langMap[primaryLangID]
	if !ok {
		return ""

	
	// Handle sublanguages for common cases
	if primaryLangID == 0x09 { // English
		switch subLangID {
		case 0x01:
			return "en-US"
		case 0x02:
			return "en-GB"
		case 0x03:
			return "en-AU"
		case 0x04:
			return "en-CA"
		default:
			return "en"
		}

	
	if primaryLangID == 0x04 { // Chinese
		switch subLangID {
		case 0x01:
			return "zh-CN" // Simplified
		case 0x02:
			return "zh-TW" // Traditional
		default:
			return "zh"
		}

	
	// For other languages, just return the base
	return base
}

// mapLanguageNameToTag maps Windows language display names to language tags
func mapLanguageNameToTag(langName string) language.Tag {
	// Normalize the language name

	
	// Common language name mappings
	langMap := map[string]language.Tag{
		"english":    language.English,
		"german":     language.German,
		"deutsch":    language.German,
		"spanish":    language.Spanish,
		"español":    language.Spanish,
		"french":     language.French,
		"français":   language.French,
		"italian":    language.Italian,
		"italiano":   language.Italian,
		"日本語":        language.Japanese,
		"日本語":       language.Japanese,
		"中文":         language.Chinese,
		"中文":        language.Chinese,
		"portuguese": language.Portuguese,
		"português":  language.Portuguese,
		"russian":    language.Russian,
		"русский":    language.Russian,
		"arabic":     language.Arabic,
		"العربية":    language.Arabic,
		"hebrew":     language.Hebrew,
		"עברית":      language.Hebrew,
		"हिन्दी":     language.Hindi,
		"हिन्दी":      language.Hindi,

	
	if tag, ok := langMap[langName]; ok {
		return tag

	
	return language.Und
}

// ClearWindowsLocaleCache clears the cached Windows locale
// This can be useful if the system locale changes during runtime
func ClearWindowsLocaleCache() {
	cachedWindowsLocale = nil
g
}