package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestNewLayeredMessageProvider(t *testing.T) {
	defBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	var userBundle *Bundle = nil

	provider := NewLayeredMessageProvider(defBundle, systemBundle, userBundle)
	assert.NotNil(t, provider)
	assert.Equal(t, defBundle, provider.defaultBundle)
	assert.Equal(t, systemBundle, provider.systemBundle)
	assert.Nil(t, provider.userBundle)
}

func TestLayeredMessageProviderBase_GetMessage(t *testing.T) {
	// Create bundles with test messages
	defBundle := NewEmptyBundle()
	defBundle.translations[defBundle.defaultLang] = map[string]string{
		"default_msg": "Default message",
	}

	systemBundle := NewEmptyBundle()
	userBundle := NewEmptyBundle()

	provider := NewLayeredMessageProvider(defBundle, systemBundle, userBundle)

	// Test with a known message key
	msg := provider.GetMessage("default_msg")
	assert.Equal(t, "Default message", msg)

	// Test with unknown key - should return the key itself
	msg = provider.GetMessage("unknown_key_12345")
	assert.Equal(t, "unknown_key_12345", msg)
}

func TestLayeredProvider_GetMessage(t *testing.T) {
	t.Run("with nil bundle", func(t *testing.T) {
		provider := NewLayeredMessageProvider(nil, nil, nil)
		msg := provider.GetMessage("test_key")
		assert.Equal(t, "test_key", msg)
	})

	t.Run("with bundle and translation", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.translations[bundle.defaultLang] = map[string]string{
			"greeting": "Hello",
			"farewell": "Goodbye",
		}

		provider := NewLayeredMessageProvider(bundle, nil, nil)

		assert.Equal(t, "Hello", provider.GetMessage("greeting"))
		assert.Equal(t, "Goodbye", provider.GetMessage("farewell"))
	})

	t.Run("with missing key returns key", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.translations[bundle.defaultLang] = map[string]string{
			"greeting": "Hello",
		}

		provider := NewLayeredMessageProvider(bundle, nil, nil)
		assert.Equal(t, "unknown_key", provider.GetMessage("unknown_key"))
	})

	t.Run("fallback to English", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.defaultLang = language.French

		// Add English translation but not French
		bundle.translations[language.English] = map[string]string{
			"greeting": "Hello",
		}

		provider := NewLayeredMessageProvider(bundle, nil, nil)
		// Should fallback to English when French not available
		assert.Equal(t, "Hello", provider.GetMessage("greeting"))
	})

	t.Run("no translation in any language", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.defaultLang = language.Spanish

		provider := NewLayeredMessageProvider(bundle, nil, nil)
		// Should return the key when no translation exists
		assert.Equal(t, "no_translation", provider.GetMessage("no_translation"))
	})
}

func TestLayeredMessageProvider_SetUserBundle(t *testing.T) {
	defBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(defBundle, systemBundle, nil)

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
	defBundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(defBundle, nil, nil)

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
	defBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(defBundle, systemBundle, nil)

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
	defBundle := NewEmptyBundle()
	defBundle.defaultLang = language.French

	provider := NewLayeredMessageProvider(defBundle, nil, nil)
	assert.Equal(t, language.French, provider.GetLanguage())
}

func TestLayeredMessageProvider_tryGetMessage(t *testing.T) {
	defBundle := NewEmptyBundle()
	defBundle.translations[defBundle.defaultLang] = map[string]string{
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

	provider := NewLayeredMessageProvider(defBundle, systemBundle, userBundle)

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
