package i18n

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// LayeredMessageProvider implements MessageProvider with a three-tier lookup system:
// 1. User bundle (highest priority)
// 2. System bundle (parser-specific overrides)
// 3. Default bundle (immutable fallback)
type LayeredMessageProvider struct {
	mu              sync.RWMutex
	userBundle      *Bundle
	systemBundle    *Bundle
	defaultBundle   *Bundle
	formatter       *Formatter
	currentLanguage language.Tag // Store the desired language at provider level
}

// NewLayeredMessageProvider creates a new layered message provider
func NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle *Bundle) *LayeredMessageProvider {
	p := &LayeredMessageProvider{
		defaultBundle:   defaultBundle,
		systemBundle:    systemBundle,
		userBundle:      userBundle,
		currentLanguage: language.English, // Default to English
	}
	// Initialize formatter with current language
	p.updateFormatter()
	return p
}

// GetMessage returns the message for the given key, checking each layer in order
func (p *LayeredMessageProvider) GetMessage(key string) string {
	p.mu.RLock()
	lang := p.currentLanguage
	p.mu.RUnlock()

	// Use TL with the current language
	return p.TL(lang, key)
}

// tryGetMessage attempts to get a message from a bundle
func (p *LayeredMessageProvider) tryGetMessage(bundle *Bundle, lang language.Tag, key string) (string, bool) {
	if bundle == nil {
		return "", false
	}

	// Just try the provided language - it should already be the matched language
	if bundle.HasKey(lang, key) {
		// Get the raw translation without formatting
		bundle.mu.RLock()
		if translations, ok := bundle.translations.Get(lang.String()); ok {
			if msg, ok := translations[key]; ok {
				bundle.mu.RUnlock()
				return msg, true
			}
		}
		bundle.mu.RUnlock()
	}

	// Try English fallback if different
	if lang != language.English && bundle.HasKey(language.English, key) {
		bundle.mu.RLock()
		if translations, ok := bundle.translations.Get(language.English.String()); ok {
			if msg, ok := translations[key]; ok {
				bundle.mu.RUnlock()
				return msg, true
			}
		}
		bundle.mu.RUnlock()
	}

	return "", false
}

// SetUserBundle updates the user bundle
func (p *LayeredMessageProvider) SetUserBundle(bundle *Bundle) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userBundle = bundle
	p.updateFormatter()
}

// SetSystemBundle updates the system bundle
func (p *LayeredMessageProvider) SetSystemBundle(bundle *Bundle) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.systemBundle = bundle
	p.updateFormatter()
}

// updateFormatter updates the formatter based on current language stored at provider level
func (p *LayeredMessageProvider) updateFormatter() {
	p.formatter = NewFormatter(p.currentLanguage)
}

// GetFormattedMessage returns the formatted message for the given key with args
func (p *LayeredMessageProvider) GetFormattedMessage(key string, args ...interface{}) string {
	msg := p.GetMessage(key)
	if len(args) > 0 {
		// Check if the message contains format specifiers
		// Common format specifiers: %s, %d, %v, %f, %t, %x, %b, %q, etc.
		if strings.Contains(msg, "%") && !strings.Contains(msg, "%%") {
			return fmt.Sprintf(msg, args...)
		}
		// If no format specifiers, just return the message as-is
		return msg
	}
	return msg
}

// GetLanguage returns the current language stored at provider level
func (p *LayeredMessageProvider) GetLanguage() language.Tag {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentLanguage
}

// GetFormatter returns the current locale-aware formatter
func (p *LayeredMessageProvider) GetFormatter() *Formatter {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.formatter
}

// FormatInt formats an integer according to current locale
func (p *LayeredMessageProvider) FormatInt(n int) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.formatter != nil {
		return p.formatter.FormatInt(n)
	}
	return fmt.Sprintf("%d", n)
}

// FormatFloat formats a float according to current locale
func (p *LayeredMessageProvider) FormatFloat(n float64, precision int) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.formatter != nil {
		return p.formatter.FormatFloat(n, precision)
	}
	return fmt.Sprintf("%.*f", precision, n)
}

// FormatRange formats a numeric range according to current locale
func (p *LayeredMessageProvider) FormatRange(min, max interface{}) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.formatter != nil {
		return p.formatter.FormatRange(min, max, p)
	}
	return fmt.Sprintf("%v to %v", min, max)
}

// GetPrinter returns a locale-aware printer for the current language
func (p *LayeredMessageProvider) GetPrinter() *message.Printer {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.formatter != nil {
		return p.formatter.printer
	}

	// Fallback to English printer
	return NewFormatter(language.English).printer
}

// T returns the translation for the given key in the current language
func (p *LayeredMessageProvider) T(key string, args ...interface{}) string {
	p.mu.RLock()
	lang := p.currentLanguage
	p.mu.RUnlock()
	return p.TL(lang, key, args...)
}

// TL returns the translation for the given language and key
func (p *LayeredMessageProvider) TL(lang language.Tag, key string, args ...interface{}) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try to get message from each layer in priority order, checking for the requested language first
	bundles := []*Bundle{p.userBundle, p.systemBundle, p.defaultBundle}

	for _, bundle := range bundles {
		if bundle != nil && bundle.HasKey(lang, key) {
			if msg, ok := p.tryGetMessage(bundle, lang, key); ok {
				if len(args) > 0 {
					// Use locale-aware printer for formatting
					if p.formatter != nil && p.formatter.printer != nil {
						return p.formatter.printer.Sprintf(msg, args...)
					}
					// Fallback to regular formatting
					return fmt.Sprintf(msg, args...)
				}
				return msg
			}
		}
	}

	// If requested language not found, fallback to English
	if lang != language.English {
		for _, bundle := range bundles {
			if bundle != nil && bundle.HasKey(language.English, key) {
				if msg, ok := p.tryGetMessage(bundle, language.English, key); ok {
					if len(args) > 0 {
						// Use locale-aware printer for formatting
						if p.formatter != nil && p.formatter.printer != nil {
							return p.formatter.printer.Sprintf(msg, args...)
						}
						// Fallback to regular formatting
						return fmt.Sprintf(msg, args...)
					}
					return msg
				}
			}
		}
	}

	// Return key if no translation found
	if len(args) > 0 {
		// Use locale-aware printer even for missing keys
		if p.formatter != nil && p.formatter.printer != nil {
			return p.formatter.printer.Sprintf(key, args...)
		}
		return fmt.Sprintf(key, args...)
	}
	return key
}

// SetDefaultLanguage sets the current language for the provider,
// using language matching to find the best available match
func (p *LayeredMessageProvider) SetDefaultLanguage(lang language.Tag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Strategy: Prioritize matches in user bundle over exact matches in system/default bundles
	// This ensures user messages are found even when system has generic language

	// First: Check if user bundle has exact match
	if p.userBundle != nil && p.userBundle.HasLanguage(lang) {
		p.currentLanguage = lang
		p.updateFormatter()
		return
	}

	// Second: Check if user bundle has a good match via language matching
	if p.userBundle != nil {
		matched := p.userBundle.MatchLanguage(lang)
		// Accept the match if it's not just falling back to the bundle's default
		if matched != p.userBundle.GetDefaultLanguage() || p.userBundle.HasLanguage(matched) {
			p.currentLanguage = p.normalizeLanguageTag(matched)
			p.updateFormatter()
			return
		}
	}

	// Third: Check system and default bundles for exact match
	for _, bundle := range []*Bundle{p.systemBundle, p.defaultBundle} {
		if bundle != nil && bundle.HasLanguage(lang) {
			p.currentLanguage = lang
			p.updateFormatter()
			return
		}
	}

	// Fourth: Use language matching on system and default bundles
	var bestMatch language.Tag
	for _, bundle := range []*Bundle{p.systemBundle, p.defaultBundle} {
		if bundle != nil && bundle.HasTranslations() {
			matched := bundle.MatchLanguage(lang)
			// DEBUG
			// fmt.Printf("DEBUG: SetDefaultLanguage matching %v -> %v (bundle default: %v)\n", lang, matched, bundle.GetDefaultLanguage())
			if matched != bundle.GetDefaultLanguage() || bundle.HasLanguage(matched) {
				bestMatch = matched
				break
			}
		}
	}

	// Use the best match we found, or the requested language as last resort
	if bestMatch != (language.Tag{}) {
		p.currentLanguage = p.normalizeLanguageTag(bestMatch)
	} else {
		p.currentLanguage = lang
	}

	p.updateFormatter()
}

// GetDefaultLanguage returns the current language stored at provider level
func (p *LayeredMessageProvider) GetDefaultLanguage() language.Tag {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentLanguage
}

// normalizeLanguageTag handles the special -u-rg-xxzzzz tags that the language matcher
// returns for exact language matches with different regions. This ensures we store
// the actual language available in our bundles.
func (p *LayeredMessageProvider) normalizeLanguageTag(tag language.Tag) language.Tag {
	// First, check if we have an exact match in any bundle
	for _, bundle := range []*Bundle{p.userBundle, p.systemBundle, p.defaultBundle} {
		if bundle != nil && bundle.HasLanguage(tag) {
			return tag
		}
	}

	// If not, check if this is the special -u-rg-xxzzzz format or extract base language
	base, _ := tag.Base()
	if base.String() != "und" {
		baseTag := language.Make(base.String())
		// Check if base language is available
		for _, bundle := range []*Bundle{p.userBundle, p.systemBundle, p.defaultBundle} {
			if bundle != nil && bundle.HasLanguage(baseTag) {
				return baseTag
			}
		}
	}

	// If nothing matches, return the original tag
	return tag
}
