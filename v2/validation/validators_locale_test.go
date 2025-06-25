package validation

import (
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

// TestLocaleAwareValidationErrors tests that validation errors format numbers according to locale
func TestLocaleAwareValidationErrors(t *testing.T) {
	tests := []struct {
		name           string
		lang           language.Tag
		validator      ValidatorFunc
		value          string
		expectedNumber string // The formatted number in the error
	}{
		{
			name:           "English MinLength",
			lang:           language.English,
			validator:      MinLength(1000),
			value:          "short",
			expectedNumber: "1,000", // English thousands separator
		},
		{
			name:           "French MinLength",
			lang:           language.French,
			validator:      MinLength(1000),
			value:          "court",
			expectedNumber: "1\u00a0000", // French non-breaking space
		},
		{
			name:           "German MinLength",
			lang:           language.German,
			validator:      MinLength(1000),
			value:          "kurz",
			expectedNumber: "1.000", // German dot separator
		},
		{
			name:           "English Range",
			lang:           language.English,
			validator:      Range(1000, 10000),
			value:          "500",
			expectedNumber: "1,000", // Check first number is formatted
		},
		{
			name:           "French Range max",
			lang:           language.French,
			validator:      Range(100, 10000),
			value:          "50000",
			expectedNumber: "10\u00a0000", // Check max is formatted
		},
		{
			name:           "German IntRange",
			lang:           language.German,
			validator:      IntRange(1000000, 2000000),
			value:          "500",
			expectedNumber: "1.000.000", // Million with German formatting
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create bundle with locale
			bundle := i18n.NewEmptyBundle()

			// Add English messages first (required as base)
			// Override the default messages to use %s format for locale-aware formatting
			bundle.AddLanguage(language.English, map[string]string{
				"goopt.error.validation.min_length":            "value '%s' must be at least %s characters long",
				"goopt.error.validation.value_between":         "value '%s' must be between %s and %s",
				"goopt.error.validation.value_must_be_integer": "value '%s' must be an integer",
			})

			// Add locale-specific messages
			switch tt.lang {
			case language.French:
				bundle.AddLanguage(language.French, map[string]string{
					"goopt.error.validation.min_length":            "la valeur '%s' doit contenir au moins %s caractères",
					"goopt.error.validation.value_between":         "la valeur '%s' doit être entre %s et %s",
					"goopt.error.validation.value_must_be_integer": "la valeur '%s' doit être un entier",
				})
			case language.German:
				bundle.AddLanguage(language.German, map[string]string{
					"goopt.error.validation.min_length":            "Wert '%s' muss mindestens %s Zeichen lang sein",
					"goopt.error.validation.value_between":         "Wert '%s' muss zwischen %s und %s liegen",
					"goopt.error.validation.value_must_be_integer": "Wert '%s' muss eine ganze Zahl sein",
				})
			}

			// Create a layered provider to test error formatting
			defaultBundle, _ := i18n.NewBundle()
			layered := i18n.NewLayeredMessageProvider(defaultBundle, nil, bundle)
			layered.SetDefaultLanguage(tt.lang)

			// Test the validator error
			err := tt.validator(tt.value)
			assert.NotNil(t, err)

			// Format the error using the layered provider
			var te i18n.TranslatableError
			if assert.ErrorAs(t, err, &te) {
				formatted := te.Format(layered)
				assert.Contains(t, formatted, tt.expectedNumber)
			}
		})
	}
}

// TestComplexNumberFormattingInErrors tests various numeric types in error messages
func TestComplexNumberFormattingInErrors(t *testing.T) {
	// Swiss German uses apostrophe as thousands separator
	bundle := i18n.NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"goopt.error.validation.max_byte_length": "value '%s' must not exceed %s bytes",
		"goopt.error.validation.exact_length":    "must be exactly %s characters",
	})

	// Set Swiss German
	swissGerman := language.MustParse("de-CH")
	bundle.AddLanguage(swissGerman, map[string]string{
		"goopt.error.validation.max_byte_length": "Wert '%[2]s' darf %[1]d Bytes nicht überschreiten",
		"goopt.error.validation.exact_length":    "muss genau %s Zeichen lang sein",
	})

	// Create a layered provider
	defaultBundle, _ := i18n.NewBundle()
	layered := i18n.NewLayeredMessageProvider(defaultBundle, nil, bundle)
	layered.SetDefaultLanguage(swissGerman)
	// Test with reasonable numbers that still demonstrate formatting
	validator := MaxByteLength(1024) // 1,024 bytes - enough to show formatting

	// Create a string that's just over the limit
	longString := strings.Repeat("x", 1025) // Just 1 byte over the limit
	err := validator(longString)
	assert.NotNil(t, err)

	// Format the error
	var te i18n.TranslatableError
	if assert.ErrorAs(t, err, &te) {
		errMsg := te.Format(layered)
		t.Logf("Swiss German error: %s", errMsg)

		// The error message uses %[1]d for max bytes and %[2]s for value
		// Since we use %d, the number should NOT be formatted
		// The value is truncated in the error message, so just check it exists
		assert.Contains(t, errMsg, "1024 Bytes", "Error should contain raw number for %d format specifier")
	}
}

// TestFloatFormattingInValidationErrors tests float formatting in error messages
func TestFloatFormattingInValidationErrors(t *testing.T) {
	bundle := i18n.NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"goopt.error.validation.value_at_most": "value '%s' must be at most %s",
	})
	bundle.AddLanguage(language.French, map[string]string{
		"goopt.error.validation.value_at_most": "la valeur '%s' doit être au maximum %s",
	})

	// Create a layered provider
	defaultBundle, _ := i18n.NewBundle()
	layered := i18n.NewLayeredMessageProvider(defaultBundle, nil, bundle)
	layered.SetDefaultLanguage(language.French)

	validator := Max(99.99)
	err := validator("150.5")
	assert.NotNil(t, err)

	// Format the error
	var te i18n.TranslatableError
	if assert.ErrorAs(t, err, &te) {
		errMsg := te.Format(layered)
		// French uses comma as decimal separator
		assert.Contains(t, errMsg, "99,99")
	}
}
