package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/napalu/goopt/types"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
)

//go:embed locales/*.json
var defaultLocales embed.FS

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
)

type Bundle struct {
	mu             sync.RWMutex
	defaultLang    language.Tag
	translations   map[language.Tag]map[string]string
	catalog        *catalog.Builder
	printers       map[language.Tag]*message.Printer
	validatedLangs map[language.Tag]struct{}
	matcher        language.Matcher
}

var defaultBundle *Bundle

func init() {
	var err error
	defaultBundle, err = NewBundleWithFS(defaultLocales, "locales")
	if err != nil {
		panic("failed to load embedded locales: " + err.Error())
	}
}

func Default() *Bundle {
	return defaultBundle
}

func NewBundle() (*Bundle, error) {
	return NewBundleWithFS(defaultLocales, "locales")
}

func NewEmptyBundle() *Bundle {
	return &Bundle{
		mu:             sync.RWMutex{},
		defaultLang:    language.English,
		translations:   make(map[language.Tag]map[string]string),
		catalog:        catalog.NewBuilder(),
		printers:       make(map[language.Tag]*message.Printer),
		validatedLangs: make(map[language.Tag]struct{}),
	}
}

func NewBundleWithFS(fs embed.FS, dirPrefix string) (*Bundle, error) {
	b := &Bundle{
		defaultLang:    language.English,
		translations:   make(map[language.Tag]map[string]string),
		catalog:        catalog.NewBuilder(),
		printers:       make(map[language.Tag]*message.Printer),
		validatedLangs: make(map[language.Tag]struct{}),
		mu:             sync.RWMutex{},
	}

	if err := b.loadEmbeddedWithFS(fs, dirPrefix); err != nil {
		return nil, err
	}

	// Build supported languages list
	supported := make([]language.Tag, 0, len(b.translations))
	for lang := range b.translations {
		supported = append(supported, lang)
	}
	b.matcher = language.NewMatcher(supported)

	// Validate default language
	if _, exists := b.translations[b.defaultLang]; !exists {
		return nil, fmt.Errorf("%w: %s", ErrDefaultLanguageTranslationsMissing, b.defaultLang)
	}

	b.validatedLangs[b.defaultLang] = struct{}{}

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
	defer b.mu.RUnlock()
	if p, exists := b.printers[lang]; exists {
		return p.Sprintf(key, args...)
	}

	if p := b.printers[b.defaultLang]; p != nil {
		return p.Sprintf(key, args...)
	}

	return key
}

// Errorf returns an error with a localized message
func (b *Bundle) Errorf(key string, args ...interface{}) error {
	return errors.New(b.T(key, args...))
}

// WrapErrorf wraps an error with a localized message and format control
func (b *Bundle) WrapErrorf(sentinel error, key string, args ...interface{}) error {
	if !b.HasKey(b.defaultLang, key) {
		return fmt.Errorf("i18n: missing translation %q: %w", key, sentinel)
	}

	return NewTranslatableError(sentinel, key, args...)
}

// AddLanguage adds a new language to the bundle or updates existing language if it exists
func (b *Bundle) AddLanguage(lang language.Tag, translations map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Merge with existing translations
	existing := b.translations[lang]
	merged := make(map[string]string)
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range translations {
		merged[k] = v
	}

	// Store original state for rollback
	original := b.translations[lang]
	b.translations[lang] = merged

	// Validate only for new non-default languages
	var errs []error
	if lang != b.defaultLang && original == nil {
		errs = b.validateLanguage(lang)
	}

	if len(errs) > 0 {
		// Rollback
		if original == nil {
			delete(b.translations, lang)
		} else {
			b.translations[lang] = original
		}
		return fmt.Errorf("%w: %s: %v", ErrInvalidTranslations, lang, errs)
	}

	// Update catalog and printer
	for key, value := range translations { // Only new/updated keys
		if err := b.catalog.SetString(lang, key, value); err != nil {
			// Partial rollback for failed key
			delete(merged, key)
			b.translations[lang] = merged
			return fmt.Errorf("%w: %s: %w", ErrFailedToSetString, key, err)
		}
	}

	b.printers[lang] = message.NewPrinter(lang, message.Catalog(b.catalog))
	return nil
}

// Formatter returns a printer for the given language
func (b *Bundle) Formatter(lang language.Tag) *message.Printer {
	b.mu.RLock()
	if p, exists := b.printers[lang]; exists {
		b.mu.RUnlock()
		return p
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	if p, exists := b.printers[lang]; exists {
		return p
	}

	if _, exists := b.translations[lang]; !exists {
		return b.printers[b.defaultLang]
	}

	p := message.NewPrinter(lang, message.Catalog(b.catalog))
	b.printers[lang] = p
	return p
}

// HasLanguage checks if a language is supported
func (b *Bundle) HasLanguage(lang language.Tag) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, exists := b.translations[lang]
	return exists
}

// Languages returns a list of supported languages
func (b *Bundle) Languages() []language.Tag {
	b.mu.RLock()
	defer b.mu.RUnlock()

	langs := make([]language.Tag, 0, len(b.translations))
	for lang := range b.translations {
		langs = append(langs, lang)
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].String() < langs[j].String()
	})

	return langs
}

// HasKey checks if a key exists in a language
func (b *Bundle) HasKey(lang language.Tag, key string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	translations, exists := b.translations[lang]
	if !exists {
		return false
	}

	_, exists = translations[key]
	return exists
}

// SetDefaultLanguage sets the default language
func (b *Bundle) SetDefaultLanguage(lang language.Tag) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.defaultLang = lang
}

func (b *Bundle) loadEmbeddedWithFS(fs embed.FS, dirPrefix string) error {
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
		if parsedLang != b.defaultLang {
			langEntries = append(langEntries, types.KeyValue[language.Tag, string]{
				Key:   parsedLang,
				Value: filepath.Join(dirPrefix, entry.Name()),
			})
		} else {
			if err := b.processLangFile(fs, parsedLang, filepath.Join(dirPrefix, entry.Name())); err != nil {
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
	var errors []error

	translations, exists := b.translations[lang]
	if !exists {
		return []error{fmt.Errorf("%w: %s", ErrLanguageNotFound, lang)}
	}

	if len(translations) == 0 {
		errors = append(errors, fmt.Errorf("%w: %s", ErrEmptyTranslations, lang))
	}

	if lang != b.defaultLang {
		defaultTranslations, exists := b.translations[b.defaultLang]
		if !exists {
			errors = append(errors, fmt.Errorf("%w: %s", ErrDefaultLanguageNotFound, b.defaultLang))
			return errors
		}

		for key := range defaultTranslations {
			if _, exists := translations[key]; !exists {
				errors = append(errors, fmt.Errorf("%w: %s: %q", ErrMissingKey, lang, key))
			}
		}

		for key := range translations {
			if _, exists := defaultTranslations[key]; !exists {
				errors = append(errors, fmt.Errorf("%w: %s: %q", ErrExtraKey, lang, key))
			}
		}
	}

	return errors
}
