package i18n

import (
	"golang.org/x/text/language"
	"strings"
)

// Locale represents a system locale with its translations
type Locale struct {
	Tag          language.Tag
	Translations string
}

// NewLocale creates a new system locale
func NewLocale(tag language.Tag, translations string) Locale {
	return Locale{
		Tag:          tag,
		Translations: translations,
	}
}

// NormalizeLocaleString converts various locale string formats into a
// well-formed BCP-47 language tag. It is designed to clean up locale strings
// commonly found in environment variables (e.g., "en_US.UTF-8") and produce
// a canonical representation (e.g., "en-US").
func NormalizeLocaleString(locale string) string {
	// Handle special, language-agnostic identifiers first. We map them to a
	// sensible default, as "C" and "POSIX" do not represent a real language.
	switch locale {
	case "C", "POSIX":
		return "en-US"
	}

	// Clean the string by removing common suffixes that are not part of the
	// BCP-47 standard, such as character encoding or currency modifiers.
	// For example, "en_US.UTF-8" becomes "en_US".
	cleanLocale := locale
	if idx := strings.Index(cleanLocale, "."); idx != -1 {
		cleanLocale = cleanLocale[:idx]
	}
	if idx := strings.Index(cleanLocale, "@"); idx != -1 {
		cleanLocale = cleanLocale[:idx]
	}

	// Replace the common Unix separator "_" with the BCP-47 standard separator "-".
	// For example, "en_US" becomes "en-US".
	bcp47Locale := strings.ReplaceAll(cleanLocale, "_", "-")

	// Use the robust golang.org/x/text/language parser to produce a canonical
	// tag. This correctly handles casing (e.g., "en-us" -> "en-US") and
	// more complex tags with scripts (e.g., "zh-hans-cn" -> "zh-Hans-CN").
	tag, err := language.Parse(bcp47Locale)
	if err != nil {
		// If parsing fails, it could be a custom or non-standard locale format.
		// We return the cleaned-up string as a best-effort fallback.
		return bcp47Locale
	}

	// Return the canonical string representation of the parsed tag.
	return tag.String()
}
