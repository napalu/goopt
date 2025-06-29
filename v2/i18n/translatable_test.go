package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestNewTranslatable(t *testing.T) {
	trans := NewTranslatable("test.key")

	assert.Equal(t, "test.key", trans.MsgOrKey)
}

func TestTranslatableMessage_T(t *testing.T) {
	// Create a test bundle
	bundle := NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"test.greeting": "Hello, World!",
		"test.count":    "You have items",
	})
	bundle.AddLanguage(language.French, map[string]string{
		"test.greeting": "Bonjour, Monde!",
		"test.count":    "Vous avez des articles",
	})

	// Create provider
	provider := NewLayeredMessageProvider(bundle, nil, nil)

	// Test translations
	trans1 := NewTranslatable("test.greeting")
	assert.Equal(t, "Hello, World!", trans1.T(provider))

	trans2 := NewTranslatable("test.count")
	assert.Equal(t, "You have items", trans2.T(provider))

	// Change language at provider level
	provider.SetDefaultLanguage(language.French)

	assert.Equal(t, "Bonjour, Monde!", trans1.T(provider))
	assert.Equal(t, "Vous avez des articles", trans2.T(provider))

	// Test missing key
	trans3 := NewTranslatable("missing.key")
	assert.Equal(t, "missing.key", trans3.T(provider))
}
