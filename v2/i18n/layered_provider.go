package i18n

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/text/language"
)

// LayeredMessageProvider implements MessageProvider with a three-tier lookup system:
// 1. User bundle (highest priority)
// 2. System bundle (parser-specific overrides)
// 3. Default bundle (immutable fallback)
type LayeredMessageProvider struct {
	mu            sync.RWMutex
	userBundle    *Bundle
	systemBundle  *Bundle
	defaultBundle *Bundle
}

// NewLayeredMessageProvider creates a new layered message provider
func NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle *Bundle) *LayeredMessageProvider {
	return &LayeredMessageProvider{
		defaultBundle: defaultBundle,
		systemBundle:  systemBundle,
		userBundle:    userBundle,
	}
}

// GetMessage returns the message for the given key, checking each layer in order
func (p *LayeredMessageProvider) GetMessage(key string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Determine the language to use
	lang := language.English
	switch {
	case p.userBundle != nil:
		lang = p.userBundle.GetDefaultLanguage()
	case p.systemBundle != nil:
		lang = p.systemBundle.GetDefaultLanguage()
	case p.defaultBundle != nil:
		lang = p.defaultBundle.GetDefaultLanguage()
	}

	if p.userBundle != nil {
		if msg, ok := p.tryGetMessage(p.userBundle, lang, key); ok {
			return msg
		}
	}

	if p.systemBundle != nil {
		if msg, ok := p.tryGetMessage(p.systemBundle, lang, key); ok {
			return msg
		}
	}

	if p.defaultBundle != nil {
		if msg, ok := p.tryGetMessage(p.defaultBundle, lang, key); ok {
			return msg
		}
	}

	return key
}

// tryGetMessage attempts to get a message from a bundle
func (p *LayeredMessageProvider) tryGetMessage(bundle *Bundle, lang language.Tag, key string) (string, bool) {
	if bundle == nil {
		return "", false
	}

	if bundle.HasKey(lang, key) {
		// Get the raw translation without formatting
		bundle.mu.RLock()
		if translations, ok := bundle.translations[lang]; ok {
			if msg, ok := translations[key]; ok {
				bundle.mu.RUnlock()
				return msg, true
			}
		}
		bundle.mu.RUnlock()
	}

	if lang != language.English && bundle.HasKey(language.English, key) {
		bundle.mu.RLock()
		if translations, ok := bundle.translations[language.English]; ok {
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
}

// SetSystemBundle updates the system bundle
func (p *LayeredMessageProvider) SetSystemBundle(bundle *Bundle) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.systemBundle = bundle
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

// GetLanguage returns the current language, checking each layer
func (p *LayeredMessageProvider) GetLanguage() language.Tag {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.userBundle != nil {
		return p.userBundle.GetDefaultLanguage()
	}
	if p.systemBundle != nil {
		return p.systemBundle.GetDefaultLanguage()
	}
	if p.defaultBundle != nil {
		return p.defaultBundle.GetDefaultLanguage()
	}
	return language.English
}
