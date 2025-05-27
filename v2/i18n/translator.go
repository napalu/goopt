package i18n

import "golang.org/x/text/language"

// Translator is an interface for handling internationalization and localization of strings in an application.
// T retrieves a localized string based on a key and optional arguments for formatting.
// TL retrieves a localized string based on language, key and optional arguments for formatting.
// SetDefaultLanguage sets the default language for translation operations.
// GetDefaultLanguage retrieves the default language currently set for translation.
type Translator interface {
	T(key string, args ...interface{}) string
	TL(lang language.Tag, key string, args ...interface{}) string
	SetDefaultLanguage(lang language.Tag)
	GetDefaultLanguage() language.Tag
}
