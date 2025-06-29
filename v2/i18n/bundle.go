// Package i18n provides internationalization support.
//
// There are two ways to manage translations:
//
//  1. System-wide through i18n.Default() and i18n.SetDefault():
//     This affects all new components that use i18n.Default()
//
//  2. Instance-level through parser.GetSystemBundle() or parser.ReplaceDefaultBundle():
//     This affects only a specific parser instance
//
// Note that changing the system bundle via i18n.SetDefault() will not affect
// existing parser instances that have already been created.
package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/napalu/goopt/v2/i18n/locales/de"
	"github.com/napalu/goopt/v2/i18n/locales/en"
	"github.com/napalu/goopt/v2/i18n/locales/fr"
	"strings"
	"sync"

	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/types/orderedmap"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
)

// Note: defaultLocales embed.FS has been replaced with DefaultSystemBundle()
// which imports the generated locale packages for en, de, and fr

var (
	ErrInvalidLanguage                    = errors.New("invalid language in filename")
	ErrDefaultLanguageTranslationsMissing = errors.New("default language translations missing")
	ErrInvalidTranslations                = errors.New("invalid translations")
	ErrEmptyTranslations                  = errors.New("empty translations")
	ErrFailedToSetString                  = errors.New("failed to set string")
	ErrLanguageNotFound                   = errors.New("language not found")
	ErrDefaultLanguageNotFound            = errors.New("default " + ErrLanguageNotFound.Error())
	ErrExtraKey                           = errors.New("extra key")
	ErrMissingKey                         = errors.New("missing key")
	ErrBundleImmutable                    = errors.New("bundle is immutable and cannot be modified")
)

type Bundle struct {
	mu                 sync.RWMutex
	defaultLang        language.Tag
	translations       *orderedmap.OrderedMap[string, map[string]string] // key is language.Tag.String()
	catalog            *catalog.Builder
	printers           map[language.Tag]*message.Printer
	validatedLanguages map[language.Tag]struct{}
	matcher            language.Matcher
	isImmutable        bool // Prevents modification when true
}

// Translator is an interface for handling internationalization and localization of strings in an application.
// T retrieves a localized string based on a key and optional arguments for formatting.
// TL retrieves a localized string based on language, key and optional arguments for formatting.
// SetDefaultLanguage sets the default language for translation operations.
// GetDefaultLanguage retrieves the default language currently set for translation.
// GetPrinter returns a locale-aware printer for standard Go formatting patterns (%d, %f, %s, etc.)
type Translator interface {
	T(key string, args ...interface{}) string
	TL(lang language.Tag, key string, args ...interface{}) string
	SetDefaultLanguage(lang language.Tag)
	GetDefaultLanguage() language.Tag
	GetPrinter() *message.Printer
}

var (
	defaultBundleOnce sync.Once
	defaultBundle     *Bundle
	defaultBundleMu   sync.RWMutex
)

// DefaultSystemBundle creates a new bundle with the built-in translations for en, de, and fr
func DefaultSystemBundle() (*Bundle, error) {
	bundle := NewEmptyBundle()

	// Add English translations
	if err := bundle.LoadFromString(en.Tag, en.SystemTranslations); err != nil {
		return nil, err
	}

	// Add German translations
	if err := bundle.LoadFromString(de.Tag, de.SystemTranslations); err != nil {
		return nil, err
	}

	// Add French translations
	if err := bundle.LoadFromString(fr.Tag, fr.SystemTranslations); err != nil {
		return nil, err
	}

	// Set default language
	bundle.SetDefaultLanguage(language.English)

	return bundle, nil
}

func Default() *Bundle {
	defaultBundleMu.RLock()
	bundle := defaultBundle
	defaultBundleMu.RUnlock()

	if bundle != nil {
		return bundle
	}

	defaultBundleOnce.Do(func() {
		var err error
		bundle, err = DefaultSystemBundle()
		if err != nil {
			panic("failed to load default locales: " + err.Error())
		}

		// Mark as immutable to prevent tests from modifying the shared default bundle
		// This prevents non-deterministic test failures
		bundle.isImmutable = true

		defaultBundleMu.Lock()
		defaultBundle = bundle
		defaultBundleMu.Unlock()
	})

	defaultBundleMu.RLock()
	bundle = defaultBundle
	defaultBundleMu.RUnlock()

	return bundle
}

func SetDefault(bundle *Bundle) {
	defaultBundleMu.Lock()
	defaultBundle = bundle
	defaultBundleMu.Unlock()
}

func NewBundle() (*Bundle, error) {
	return DefaultSystemBundle()
}

func NewEmptyBundle() *Bundle {
	b := &Bundle{
		mu:                 sync.RWMutex{},
		defaultLang:        language.English,
		translations:       orderedmap.NewOrderedMap[string, map[string]string](),
		catalog:            catalog.NewBuilder(),
		printers:           make(map[language.Tag]*message.Printer),
		validatedLanguages: make(map[language.Tag]struct{}),
	}
	// Initialize with empty matcher - will be updated when languages are added
	b.matcher = language.NewMatcher([]language.Tag{})
	return b
}

func NewBundleWithFS(fs embed.FS, dirPrefix string, lang ...language.Tag) (*Bundle, error) {
	b := &Bundle{
		translations:       orderedmap.NewOrderedMap[string, map[string]string](),
		catalog:            catalog.NewBuilder(),
		printers:           make(map[language.Tag]*message.Printer),
		validatedLanguages: make(map[language.Tag]struct{}),
		mu:                 sync.RWMutex{},
	}
	if len(lang) > 0 {
		b.defaultLang = lang[0]
	} else {
		b.defaultLang = language.English
	}
	if err := b.LoadFromFS(fs, dirPrefix); err != nil {
		return nil, err
	}

	// Build supported languages list
	supported := make([]language.Tag, 0, b.translations.Len())
	for iter := b.translations.Front(); iter != nil; iter = iter.Next() {
		// Parse the string key back to language.Tag
		if tag, err := language.Parse(*iter.Key); err == nil {
			supported = append(supported, tag)
		}
	}
	b.matcher = language.NewMatcher(supported)

	// Validate default language
	if _, exists := b.translations.Get(b.defaultLang.String()); !exists {
		return nil, fmt.Errorf("%w: %s", ErrDefaultLanguageTranslationsMissing, b.defaultLang)
	}

	b.validatedLanguages[b.defaultLang] = struct{}{}

	return b, nil
}

// T returns the translation for the given key in the default language
func (b *Bundle) T(key string, args ...interface{}) string {
	b.mu.RLock()
	defaultLang := b.defaultLang
	b.mu.RUnlock()

	return b.TL(defaultLang, key, args...)
}

// TL returns the translation for the given language and key
func (b *Bundle) TL(lang language.Tag, key string, args ...interface{}) string {
	b.mu.RLock()

	// Try exact match first
	if p, exists := b.printers[lang]; exists {
		b.mu.RUnlock()
		return p.Sprintf(key, args...)
	}

	// Use language matching to find best match
	if b.matcher != nil {
		matched, _, _ := b.matcher.Match(lang)
		if p, exists := b.printers[matched]; exists {
			b.mu.RUnlock()
			return p.Sprintf(key, args...)
		}
	}

	// Fallback to default language
	if p := b.printers[b.defaultLang]; p != nil {
		b.mu.RUnlock()
		return p.Sprintf(key, args...)
	}

	b.mu.RUnlock()
	return key
}

// AddLanguage adds a new language to the bundle or updates existing language if it exists
func (b *Bundle) AddLanguage(lang language.Tag, translations map[string]string) error {
	if b.isImmutable {
		return ErrBundleImmutable
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Merge with existing translations
	existing, _ := b.translations.Get(lang.String())
	merged := make(map[string]string)
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range translations {
		merged[k] = v
	}

	// Store original state for rollback
	original, hadOriginal := b.translations.Get(lang.String())
	b.translations.Set(lang.String(), merged)

	// Validate only for new non-default languages
	var errs []error
	if lang != b.defaultLang && original == nil {
		errs = b.validateLanguage(lang)
	}

	if len(errs) > 0 {
		// Rollback
		if !hadOriginal {
			b.translations.Delete(lang.String())
		} else {
			b.translations.Set(lang.String(), original)
		}
		return fmt.Errorf("%w: %s: %v", ErrInvalidTranslations, lang, errs)
	}

	// Update catalog and printer
	for key, value := range translations { // Only new/updated keys
		if err := b.catalog.SetString(lang, key, value); err != nil {
			// Partial rollback for failed key
			delete(merged, key)
			b.translations.Set(lang.String(), merged)
			return fmt.Errorf("%w: %s: %v", ErrFailedToSetString, key, err)
		}
	}

	b.printers[lang] = message.NewPrinter(lang, message.Catalog(b.catalog))

	// Update matcher with new language list
	b.updateMatcher()

	return nil
}

// HasTranslations returns true if the bundle has any translations
func (b *Bundle) HasTranslations() bool {
	if b == nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.translations.Len() > 0
}

// LanguagesWithKey returns languages that have a specific translation key
func (b *Bundle) LanguagesWithKey(key string) []language.Tag {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	var langs []language.Tag
	for iter := b.translations.Front(); iter != nil; iter = iter.Next() {
		if messages, ok := b.translations.Get(*iter.Key); ok {
			if _, hasKey := messages[key]; hasKey {
				// Parse the string key back to language.Tag
				if tag, err := language.Parse(*iter.Key); err == nil {
					langs = append(langs, tag)
				}
			}
		}
	}
	return langs
}

// updateMatcher updates the language matcher with current supported languages
func (b *Bundle) updateMatcher() {
	supported := make([]language.Tag, 0, b.translations.Len())
	// OrderedMap maintains insertion order, so English will always be first
	for iter := b.translations.Front(); iter != nil; iter = iter.Next() {
		// Parse the string key back to language.Tag
		if tag, err := language.Parse(*iter.Key); err == nil {
			supported = append(supported, tag)
		}
	}
	if len(supported) > 0 {
		b.matcher = language.NewMatcher(supported)
	}
}

// Formatter returns a printer for the given language
func (b *Bundle) Formatter(lang language.Tag) *message.Printer {
	b.mu.RLock()
	// Try exact match first
	if p, exists := b.printers[lang]; exists {
		b.mu.RUnlock()
		return p
	}

	// Use language matching to find best match
	var matched language.Tag
	if b.matcher != nil {
		matched, _, _ = b.matcher.Match(lang)
		if p, exists := b.printers[matched]; exists && matched != lang {
			b.mu.RUnlock()
			return p
		}
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Double-check under write lock
	if p, exists := b.printers[lang]; exists {
		return p
	}

	// Check if we have translations for the exact language
	if _, exists := b.translations.Get(lang.String()); exists {
		p := message.NewPrinter(lang, message.Catalog(b.catalog))
		b.printers[lang] = p
		return p
	}

	// Use matched language if available
	if matched != (language.Tag{}) && matched != lang {
		if p, exists := b.printers[matched]; exists {
			return p
		}
		if _, exists := b.translations.Get(matched.String()); exists {
			// Create printer for matched language
			p := message.NewPrinter(matched, message.Catalog(b.catalog))
			b.printers[matched] = p
			return p
		}
	}

	// Fallback to default language
	return b.printers[b.defaultLang]
}

// HasLanguage checks if a language is supported
func (b *Bundle) HasLanguage(lang language.Tag) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, exists := b.translations.Get(lang.String())
	return exists
}

// MatchLanguage finds the best matching language from available translations
// using RFC 4647 language matching. For example:
// - If user requests "en-CA" and we have "en-US" and "en-GB", it returns one of them
// - If user requests "en" and we have "en-US", it returns "en-US"
// - If user requests "de-AT" and we have "de", it returns "de"
func (b *Bundle) MatchLanguage(requested language.Tag) language.Tag {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// First check for exact match
	if _, exists := b.translations.Get(requested.String()); exists {
		return requested
	}

	// Use matcher to find best match if available
	if b.matcher != nil {
		matched, _, _ := b.matcher.Match(requested)
		return matched
	}

	// Fallback to default language
	return b.defaultLang
}

// Languages returns a list of supported languages
func (b *Bundle) Languages() []language.Tag {
	b.mu.RLock()
	defer b.mu.RUnlock()

	langs := make([]language.Tag, 0, b.translations.Len())
	for iter := b.translations.Front(); iter != nil; iter = iter.Next() {
		// Parse the string key back to language.Tag
		if tag, err := language.Parse(*iter.Key); err == nil {
			langs = append(langs, tag)
		}
	}

	// Return languages in insertion order (English first due to OrderedMap)
	return langs
}

// HasKey checks if a key exists in a language
func (b *Bundle) HasKey(lang language.Tag, key string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	translations, exists := b.translations.Get(lang.String())
	if !exists {
		return false
	}

	_, hasKey := translations[key]
	return hasKey
}

// SetDefaultLanguage sets the default language, using language matching to find
// the best available match if the exact language is not available
func (b *Bundle) SetDefaultLanguage(lang language.Tag) {
	if b.isImmutable {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Use language matching to find best available language
	if b.matcher != nil && b.translations.Len() > 0 {
		// Only use matcher if we have translations to match against
		matched, _, _ := b.matcher.Match(lang)
		b.defaultLang = matched
	} else if _, exists := b.translations.Get(lang.String()); exists {
		b.defaultLang = lang
	} else {
		// If no match found and bundle is not empty, keep current default
		if b.translations.Len() > 0 {
			return
		}
		// Allow setting any language on empty bundle
		b.defaultLang = lang
	}

}

func (b *Bundle) GetDefaultLanguage() language.Tag {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.defaultLang
}

// GetPrinter returns a locale-aware printer for the current default language
func (b *Bundle) GetPrinter() *message.Printer {
	b.mu.RLock()
	defaultLang := b.defaultLang
	b.mu.RUnlock()

	return b.Formatter(defaultLang)
}

// GetTranslations returns all translations for a specific language
func (b *Bundle) GetTranslations(lang language.Tag) map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if translations, ok := b.translations.Get(lang.String()); ok {
		// Return a copy to prevent external modification
		result := make(map[string]string, len(translations))
		for k, v := range translations {
			result[k] = v
		}
		return result
	}
	return nil
}

// LoadFromString loads translations from a JSON string for a specific language
func (b *Bundle) LoadFromString(lang language.Tag, jsonStr string) error {
	if b.isImmutable {
		return ErrBundleImmutable
	}

	var translations map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &translations); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return b.AddLanguage(lang, translations)
}

// LoadFromFS loads language files from the specified embedded file system directory and updates the bundle's translations.
// It processes each JSON file to extract translations and associates them with the respective language in the bundle.
func (b *Bundle) LoadFromFS(fs embed.FS, dirPrefix string) error {
	entries, err := fs.ReadDir(dirPrefix)
	if err != nil {
		return err
	}

	langEntries := make([]types.KeyValue[language.Tag, string], 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		lang := strings.TrimSuffix(entry.Name(), ".json")
		parsedLang, err := language.Parse(lang)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidLanguage, entry.Name())
		}
		filePath := dirPrefix + "/" + entry.Name() // Always use forward slashes
		if parsedLang != b.defaultLang {
			langEntries = append(langEntries, types.KeyValue[language.Tag, string]{
				Key:   parsedLang,
				Value: filePath,
			})
		} else {
			if err := b.processLangFile(fs, parsedLang, filePath); err != nil {
				return err
			}
		}
	}

	for _, langEntry := range langEntries {
		if err := b.processLangFile(fs, langEntry.Key, langEntry.Value); err != nil {
			return err
		}
	}

	return nil
}

// ParseLanguageTag parses a BCP 47 language tag and returns the corresponding language.Tag object or an error if invalid.
func ParseLanguageTag(l string) (language.Tag, error) {
	return language.Parse(l)
}

func (b *Bundle) processLangFile(fs embed.FS, lang language.Tag, path string) error {
	data, err := fs.ReadFile(path)
	if err != nil {
		return err
	}

	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return err
	}

	if err := b.AddLanguage(lang, translations); err != nil {
		return err
	}

	return nil
}

func (b *Bundle) validateLanguage(lang language.Tag) []error {
	var e []error

	translations, exists := b.translations.Get(lang.String())
	if !exists {
		return []error{fmt.Errorf("%w: %s", ErrLanguageNotFound, lang)}
	}

	if len(translations) == 0 {
		e = append(e, fmt.Errorf("%w: %s", ErrEmptyTranslations, lang))
	}

	// Check if bundle has any existing languages to validate against
	existingLanguages := 0
	var referenceTranslations map[string]string
	for iter := b.translations.Front(); iter != nil; iter = iter.Next() {
		if *iter.Key != lang.String() {
			if trans, ok := b.translations.Get(*iter.Key); ok && len(trans) > 0 {
				existingLanguages++
				if referenceTranslations == nil {
					referenceTranslations = trans
				}
			}
		}
	}

	// If bundle is empty (no other languages), allow adding without validation
	if existingLanguages == 0 {
		return e
	}

	// Validate against existing translations
	for key := range referenceTranslations {
		if _, exists := translations[key]; !exists {
			e = append(e, fmt.Errorf("%w: %s: %q", ErrMissingKey, lang, key))
		}
	}

	for key := range translations {
		if _, exists := referenceTranslations[key]; !exists {
			e = append(e, fmt.Errorf("%w: %s: %q", ErrExtraKey, lang, key))
		}
	}

	return e
}
