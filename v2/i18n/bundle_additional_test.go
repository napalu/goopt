package i18n

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"testing"
)

func TestNewBundle_LoadsDefaultTranslations(t *testing.T) {
	bundle, err := NewBundle()
	require.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.Equal(t, language.English, bundle.defaultLang)
	assert.NotNil(t, bundle.translations)
	assert.NotNil(t, bundle.catalog)
	assert.NotNil(t, bundle.printers)

	// Should have some default translations loaded
	assert.NotEmpty(t, bundle.translations)
}

func TestBundle_SetDefaultLanguage(t *testing.T) {
	bundle := NewEmptyBundle()

	// Test setting default language
	err := bundle.SetDefaultLanguage(language.Spanish)
	assert.NoError(t, err)
	assert.Equal(t, language.Spanish, bundle.GetDefaultLanguage())

	// Test with immutable bundle
	bundle.isImmutable = true
	err = bundle.SetDefaultLanguage(language.French)
	assert.Error(t, err)
	assert.Equal(t, ErrBundleImmutable, err)
	assert.Equal(t, language.Spanish, bundle.GetDefaultLanguage()) // Should not change
}

func TestBundle_GetSupportedLanguages(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add some test translations
	bundle.translations[language.English] = map[string]string{"test": "test"}
	bundle.translations[language.French] = map[string]string{"test": "test"}
	bundle.translations[language.German] = map[string]string{"test": "test"}

	languages := bundle.GetSupportedLanguages()
	assert.Len(t, languages, 3)

	// Check that all languages are present (order not guaranteed)
	langMap := make(map[language.Tag]bool)
	for _, lang := range languages {
		langMap[lang] = true
	}
	assert.True(t, langMap[language.English])
	assert.True(t, langMap[language.French])
	assert.True(t, langMap[language.German])
}

func TestBundle_GetTranslations(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add test translations for English
	testTranslations := map[string]string{
		"hello":   "Hello",
		"goodbye": "Goodbye",
		"welcome": "Welcome",
	}
	bundle.translations[language.English] = testTranslations

	// Get translations
	translations := bundle.GetTranslations(language.English)
	assert.Len(t, translations, 3)
	assert.Equal(t, "Hello", translations["hello"])
	assert.Equal(t, "Goodbye", translations["goodbye"])
	assert.Equal(t, "Welcome", translations["welcome"])

	// Verify it's a copy, not the original map
	translations["new"] = "New message"
	assert.Len(t, bundle.translations[language.English], 3) // Original should still have 3

	// Test getting translations for non-existent language
	translations = bundle.GetTranslations(language.Japanese)
	assert.Nil(t, translations)
}
