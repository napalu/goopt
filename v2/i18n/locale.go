package i18n

import "golang.org/x/text/language"

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
