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

// A comprehensive map of language names (including endonyms) to language tags.
var langMap = map[string]language.Tag{
	"english":          language.English,
	"deutsch":          language.German,
	"german":           language.German,
	"français":         language.French,
	"french":           language.French,
	"español":          language.Spanish,
	"spanish":          language.Spanish,
	"italiano":         language.Italian,
	"italian":          language.Italian,
	"português":        language.Portuguese,
	"portuguese":       language.Portuguese,
	"nederlands":       language.Dutch,
	"dutch":            language.Dutch,
	"polski":           language.Polish,
	"polish":           language.Polish,
	"čeština":          language.Czech,
	"czech":            language.Czech,
	"magyar":           language.Hungarian,
	"hungarian":        language.Hungarian,
	"slovenčina":       language.Slovak,
	"slovak":           language.Slovak,
	"hrvatski":         language.Croatian,
	"croatian":         language.Croatian,
	"slovenščina":      language.Slovenian,
	"slovenian":        language.Slovenian,
	"română":           language.Romanian,
	"romanian":         language.Romanian,
	"български":        language.Bulgarian,
	"bulgarian":        language.Bulgarian,
	"українська":       language.Ukrainian,
	"ukrainian":        language.Ukrainian,
	"русский":          language.Russian,
	"russian":          language.Russian,
	"العربية":          language.Arabic,
	"arabic":           language.Arabic,
	"עברית":            language.Hebrew,
	"hebrew":           language.Hebrew,
	"हिन्दी":           language.Hindi,
	"hindi":            language.Hindi,
	"ไทย":              language.Thai,
	"thai":             language.Thai,
	"日本語":              language.Japanese,
	"japanese":         language.Japanese,
	"한국어":              language.Korean,
	"korean":           language.Korean,
	"中文":               language.Chinese,
	"chinese":          language.Chinese,
	"tiếng việt":       language.Vietnamese,
	"vietnamese":       language.Vietnamese,
	"bahasa indonesia": language.Indonesian,
	"indonesian":       language.Indonesian,
	"suomi":            language.Finnish,
	"finnish":          language.Finnish,
	"svenska":          language.Swedish,
	"swedish":          language.Swedish,
	"dansk":            language.Danish,
	"danish":           language.Danish,
	"norsk":            language.Norwegian,
	"norwegian":        language.Norwegian,
	"ελληνικά":         language.Greek,
	"greek":            language.Greek,
	"català":           language.Catalan,
	"catalan":          language.Catalan,
	"eesti":            language.Estonian,
	"estonian":         language.Estonian,
	"latviešu":         language.Latvian,
	"latvian":          language.Latvian,
	"lietuvių":         language.Lithuanian,
	"lithuanian":       language.Lithuanian,
}

// LanguageNameToLanguageTag maps language display names to language tags.
// It includes a comprehensive list of languages and their native names,
// and it can handle region-specific names like "French (Canada)" by stripping
// the regional information before mapping.
func LanguageNameToLanguageTag(langName string) language.Tag {
	// Normalize the language name to lower case and trim whitespace.
	langName = strings.ToLower(strings.TrimSpace(langName))

	// Handle region-specific names like "English (United Kingdom)" by
	// stripping the part in parentheses.
	if idx := strings.Index(langName, " ("); idx != -1 {
		langName = langName[:idx]
	}

	if tag, ok := langMap[langName]; ok {
		return tag
	}

	// Return an undefined tag if no match is found.
	return language.Und
}

// languageNamesMap provides a reverse lookup to get all known language names
// (both English and native) from a language.Tag.
var languageNamesMap = map[language.Tag][]string{
	language.English:    {"English", "english"},
	language.German:     {"Deutsch", "german"},
	language.French:     {"Français", "french"},
	language.Spanish:    {"Español", "spanish"},
	language.Italian:    {"Italiano", "italian"},
	language.Portuguese: {"Português", "portuguese"},
	language.Dutch:      {"Nederlands", "dutch"},
	language.Polish:     {"Polski", "polish"},
	language.Czech:      {"Čeština", "czech"},
	language.Hungarian:  {"Magyar", "hungarian"},
	language.Slovak:     {"Slovenčina", "slovak"},
	language.Croatian:   {"Hrvatski", "croatian"},
	language.Slovenian:  {"Slovenščina", "slovenian"},
	language.Romanian:   {"Română", "romanian"},
	language.Bulgarian:  {"Български", "bulgarian"},
	language.Ukrainian:  {"Українська", "ukrainian"},
	language.Russian:    {"Русский", "russian"},
	language.Arabic:     {"العربية", "arabic"},
	language.Hebrew:     {"עברית", "hebrew"},
	language.Hindi:      {"हिन्दी", "hindi"},
	language.Thai:       {"ไทย", "thai"},
	language.Japanese:   {"日本語", "japanese"},
	language.Korean:     {"한국어", "korean"},
	language.Chinese:    {"中文", "chinese"},
	language.Vietnamese: {"Tiếng Việt", "vietnamese"},
	language.Indonesian: {"Bahasa Indonesia", "indonesian"},
	language.Finnish:    {"Suomi", "finnish"},
	language.Swedish:    {"Svenska", "swedish"},
	language.Danish:     {"Dansk", "danish"},
	language.Norwegian:  {"Norsk", "norwegian"},
	language.Greek:      {"Ελληνικά", "greek"},
	language.Catalan:    {"Català", "catalan"},
	language.Estonian:   {"Eesti", "estonian"},
	language.Latvian:    {"Latviešu", "latvian"},
	language.Lithuanian: {"Lietuvių", "lithuanian"},
}

// LanguageTagToLanguageNames returns all known names for a given language tag,
// including English and native names.
// It correctly handles regional tags by looking up the base language.
func LanguageTagToLanguageNames(tag language.Tag) []string {
	if tag == language.Und {
		return nil
	}

	if names, ok := languageNamesMap[tag]; ok {
		return names
	}

	base, _ := tag.Base()
	if base == (language.Base{}) { // base is undefined ("und")
		return nil
	}

	baseLang, err := language.Parse(base.String())
	if err != nil {
		return nil
	}

	if names, ok := languageNamesMap[baseLang]; ok {
		return names
	}

	return nil
}

// GetNativeLanguageName returns the native (or first known) name for the given language tag.
// If no name is found, it returns the BCP 47 string form (e.g., "en").
func GetNativeLanguageName(tag language.Tag) string {
	if names := LanguageTagToLanguageNames(tag); len(names) > 0 {
		return names[0] // Native name is first
	}

	return tag.String()
}

var rtlLanguages = map[language.Tag]bool{
	language.Arabic:             true,
	language.Hebrew:             true,
	language.Persian:            true,
	language.Urdu:               true,
	language.MustParse("ps"):    true, // Pashto
	language.MustParse("ku"):    true, // Kurdish (assumes Sorani)
	language.MustParse("sd"):    true, // Sindhi
	language.MustParse("dv"):    true, // Divehi
	language.MustParse("rhg"):   true, // Rohingya
	language.MustParse("yi"):    true, // Yiddish
	language.MustParse("iw"):    true, // old Hebrew
	language.MustParse("ji"):    true, // old Yiddish
	language.MustParse("fa-AF"): true, // Dari (variant of Persian)
}

func IsRTL(tag language.Tag) bool {
	if base, confidence := tag.Base(); confidence > language.Low {
		return rtlLanguages[language.Make(base.String())]
	}

	return false
}
