package i18n

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
	"testing"
)

func TestNewLayeredMessageProvider(t *testing.T) {
	defaultBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	var userBundle *Bundle = nil

	provider := NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle)
	assert.NotNil(t, provider)
	assert.Equal(t, defaultBundle, provider.defaultBundle)
	assert.Equal(t, systemBundle, provider.systemBundle)
	assert.Nil(t, provider.userBundle)
}

func TestLayeredMessageProvider_GetMessage(t *testing.T) {
	// Create bundles with test messages
	defaultBundle := NewEmptyBundle()
	defaultBundle.translations[defaultBundle.defaultLang] = map[string]string{
		"default_msg": "Default message",
	}

	systemBundle := NewEmptyBundle()
	userBundle := NewEmptyBundle()

	provider := NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle)

	// Test with a known message key
	msg := provider.GetMessage("default_msg")
	assert.Equal(t, "Default message", msg)

	// Test with unknown key - should return the key itself
	msg = provider.GetMessage("unknown_key_12345")
	assert.Equal(t, "unknown_key_12345", msg)
}

func TestLayeredMessageProvider_SetUserBundle(t *testing.T) {
	defaultBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(defaultBundle, systemBundle, nil)

	// Create a user bundle with custom messages
	userBundle := NewEmptyBundle()
	userBundle.translations[userBundle.defaultLang] = map[string]string{
		"custom_message": "This is a custom message",
	}

	provider.SetUserBundle(userBundle)

	// User bundle message should take precedence
	msg := provider.GetMessage("custom_message")
	assert.Equal(t, "This is a custom message", msg)
}

func TestLayeredMessageProvider_SetSystemBundle(t *testing.T) {
	defaultBundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(defaultBundle, nil, nil)

	// Create a custom system bundle
	systemBundle := NewEmptyBundle()
	systemBundle.translations[systemBundle.defaultLang] = map[string]string{
		"system_message": "This is a system message",
	}

	provider.SetSystemBundle(systemBundle)

	// System bundle message should be available
	msg := provider.GetMessage("system_message")
	assert.Equal(t, "This is a system message", msg)
}

func TestLayeredMessageProvider_GetFormattedMessage(t *testing.T) {
	defaultBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(defaultBundle, systemBundle, nil)

	// Create a bundle with a formatted message
	userBundle := NewEmptyBundle()
	userBundle.translations[userBundle.defaultLang] = map[string]string{
		"formatted_msg": "Hello %s, you have %d messages",
	}
	provider.SetUserBundle(userBundle)

	// Test formatted message
	msg := provider.GetFormattedMessage("formatted_msg", "John", 5)
	assert.Equal(t, "Hello John, you have 5 messages", msg)

	// Test with unknown key - no format specifiers, so args are ignored
	msg = provider.GetFormattedMessage("unknown_key", "test")
	assert.Equal(t, "unknown_key", msg)
}

func TestLayeredMessageProvider_GetLanguage(t *testing.T) {
	// Create bundles with different languages
	defaultBundle := NewEmptyBundle()
	defaultBundle.defaultLang = language.French

	provider := NewLayeredMessageProvider(defaultBundle, nil, nil)
	assert.Equal(t, language.French, provider.GetLanguage())
}

func TestLayeredMessageProvider_tryGetMessage(t *testing.T) {
	defaultBundle := NewEmptyBundle()
	defaultBundle.translations[defaultBundle.defaultLang] = map[string]string{
		"default_only": "Default only message",
		"shared_msg":   "Default version",
	}

	// Set both user and system bundles
	userBundle := NewEmptyBundle()
	userBundle.translations[userBundle.defaultLang] = map[string]string{
		"user_msg":   "User message",
		"shared_msg": "User version",
	}

	systemBundle := NewEmptyBundle()
	systemBundle.translations[systemBundle.defaultLang] = map[string]string{
		"system_msg": "System message",
		"shared_msg": "System version",
	}

	provider := NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle)

	// User bundle should take precedence for shared messages
	msg := provider.GetMessage("shared_msg")
	assert.Equal(t, "User version", msg)

	// System-only message
	msg = provider.GetMessage("system_msg")
	assert.Equal(t, "System message", msg)

	// User-only message
	msg = provider.GetMessage("user_msg")
	assert.Equal(t, "User message", msg)

	// Default-only message
	msg = provider.GetMessage("default_only")
	assert.Equal(t, "Default only message", msg)
}
