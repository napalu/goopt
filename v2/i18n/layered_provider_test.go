package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestLayeredMessageProvider_FormatMethods(t *testing.T) {
	// Create test bundle
	bundle := NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"goopt.msg.range_to": "to",
	})
	bundle.AddLanguage(language.French, map[string]string{
		"goopt.msg.range_to": "à",
	})
	bundle.AddLanguage(language.German, map[string]string{
		"goopt.msg.range_to": "bis",
	})

	provider := NewLayeredMessageProvider(bundle, nil, nil)

	t.Run("FormatInt", func(t *testing.T) {
		// Default should be English
		result := provider.FormatInt(1234567)
		assert.Equal(t, "1,234,567", result)

		// Change language to French at provider level
		provider.SetDefaultLanguage(language.French)

		result = provider.FormatInt(1234567)
		assert.Equal(t, "1\u00a0234\u00a0567", result) // non-breaking space
	})

	t.Run("FormatFloat", func(t *testing.T) {
		// Reset to English
		provider.SetDefaultLanguage(language.English)

		result := provider.FormatFloat(1234.56, 2)
		assert.Equal(t, "1,234.56", result)

		// Change language to German at provider level
		provider.SetDefaultLanguage(language.German)

		result = provider.FormatFloat(1234.56, 2)
		assert.Equal(t, "1.234,56", result)
	})

	t.Run("FormatRange", func(t *testing.T) {
		// Reset to English
		provider.SetDefaultLanguage(language.English)

		result := provider.FormatRange(10, 100)
		assert.Equal(t, "10 to 100", result)

		// Change language to French at provider level
		provider.SetDefaultLanguage(language.French)

		result = provider.FormatRange(10, 100)
		assert.Equal(t, "10 à 100", result)
	})
}
