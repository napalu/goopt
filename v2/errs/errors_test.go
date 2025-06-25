package errs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

func TestUpdateMessageProvider(t *testing.T) {
	originalMsg := ErrEmptyFlag.Error()

	// Create a Finnish bundle
	bundle := i18n.NewEmptyBundle()
	err := bundle.AddLanguage(language.Finnish, map[string]string{
		"goopt.error.empty_flag":          "Tyhjä lippu",
		"goopt.error.flag_already_exists": "Lippu on jo olemassa: %s",
	})
	assert.NoError(t, err)

	// Create a message provider from the bundle
	provider := i18n.NewLayeredMessageProvider(bundle, nil, nil)
	provider.SetDefaultLanguage(language.Finnish)
	// Check that the error message has changed
	newMsg := ErrEmptyFlag.Format(provider)
	assert.NotEqual(t, originalMsg, newMsg, "Expected error message to change after formatting with new provider, but got same: %s", newMsg)
	assert.Equal(t, "Tyhjä lippu", newMsg, "Expected Finnish error message 'Tyhjä lippu', got '%s'", newMsg)

	// Test with arguments
	errWithArgs := ErrFlagAlreadyExists.WithArgs("test-flag")
	assert.Contains(t, errWithArgs.Error(), "test-flag", "Expected error to contain 'test-flag', got: %s", errWithArgs.Error())
}
