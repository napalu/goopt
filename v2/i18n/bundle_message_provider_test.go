package i18n

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
	"testing"
)

func TestNewBundleMessageProvider(t *testing.T) {
	bundle := NewEmptyBundle()
	provider := NewBundleMessageProvider(bundle)

	assert.NotNil(t, provider)
	assert.Equal(t, bundle, provider.bundle)
}

func TestBundleMessageProvider_GetMessage(t *testing.T) {
	t.Run("with nil bundle", func(t *testing.T) {
		provider := NewBundleMessageProvider(nil)
		msg := provider.GetMessage("test_key")
		assert.Equal(t, "test_key", msg)
	})

	t.Run("with bundle and translation", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.translations[bundle.defaultLang] = map[string]string{
			"greeting": "Hello",
			"farewell": "Goodbye",
		}

		provider := NewBundleMessageProvider(bundle)

		assert.Equal(t, "Hello", provider.GetMessage("greeting"))
		assert.Equal(t, "Goodbye", provider.GetMessage("farewell"))
	})

	t.Run("with missing key returns key", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.translations[bundle.defaultLang] = map[string]string{
			"greeting": "Hello",
		}

		provider := NewBundleMessageProvider(bundle)
		assert.Equal(t, "unknown_key", provider.GetMessage("unknown_key"))
	})

	t.Run("fallback to English", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.defaultLang = language.French

		// Add English translation but not French
		bundle.translations[language.English] = map[string]string{
			"greeting": "Hello",
		}

		provider := NewBundleMessageProvider(bundle)
		// Should fallback to English when French not available
		assert.Equal(t, "Hello", provider.GetMessage("greeting"))
	})

	t.Run("no translation in any language", func(t *testing.T) {
		bundle := NewEmptyBundle()
		bundle.defaultLang = language.Spanish

		provider := NewBundleMessageProvider(bundle)
		// Should return the key when no translation exists
		assert.Equal(t, "no_translation", provider.GetMessage("no_translation"))
	})
}
