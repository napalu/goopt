package errs

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

func TestUpdateMessageProvider(t *testing.T) {
	originalMsg := ErrEmptyFlag.Error()

	// Create a bundle with Finnish as default language
	bundle := i18n.NewEmptyBundle()
	bundle.SetDefaultLanguage(language.Finnish)
	err := bundle.AddLanguage(language.Finnish, map[string]string{
		"goopt.error.empty_flag":          "Tyhjä lippu",
		"goopt.error.flag_already_exists": "Lippu on jo olemassa: %s",
	})
	assert.NoError(t, err)

	// Create a message provider from the bundle
	provider := i18n.NewBundleMessageProvider(bundle)

	UpdateMessageProvider(provider)

	// Check that the error message has changed
	newMsg := ErrEmptyFlag.Error()
	assert.NotEqual(t, originalMsg, newMsg, "Expected error message to change after UpdateMessageProvider, but got same: %s", newMsg)
	assert.Equal(t, "Tyhjä lippu", newMsg, "Expected Finnish error message 'Tyhjä lippu', got '%s'", newMsg)

	// Test with arguments
	errWithArgs := ErrFlagAlreadyExists.WithArgs("test-flag")
	assert.Contains(t, errWithArgs.Error(), "test-flag", "Expected error to contain 'test-flag', got: %s", errWithArgs.Error())
}
