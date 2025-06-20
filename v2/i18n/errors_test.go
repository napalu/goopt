package i18n

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslatableErrors(t *testing.T) {
	err := NewError("test.error")

	// Test Error()
	if err.Error() == "" {
		t.Error("Error() should return message")
	}

	// Test WithArgs()
	err2 := err.WithArgs("arg1", "arg2")
	if len(err2.Args()) != 2 {
		t.Error("WithArgs() failed")
	}

	// Test Wrap()
	wrapped := err.Wrap(errors.New("inner"))
	if wrapped.Unwrap() == nil {
		t.Error("Wrap() failed")
	}

	// Test Is()
	if !errors.Is(wrapped, err) {
		t.Error("Is() failed")
	}
}

func TestNewErrorWithProvider(t *testing.T) {
	bundle := NewEmptyBundle()
	bundle.translations[bundle.defaultLang] = map[string]string{
		"test_error_key": "Test error message",
	}
	provider := NewLayeredMessageProvider(bundle, nil, nil)

	// Create an error with provider
	err := NewErrorWithProvider("test_error_key", provider)
	assert.NotNil(t, err)

	assert.Equal(t, "test_error_key", err.key)
}

func TestError_Key(t *testing.T) {
	bundle := NewEmptyBundle()
	provider := NewLayeredMessageProvider(bundle, nil, nil)
	err := NewErrorWithProvider("error_key_123", provider)

	assert.Equal(t, "error_key_123", err.Key())
}

func TestSetDefaultMessageProvider(t *testing.T) {
	// Create and set a new default provider
	bundle := NewEmptyBundle()
	newProvider := NewLayeredMessageProvider(bundle, nil, nil)
	SetDefaultMessageProvider(newProvider)

	assert.Equal(t, newProvider, getDefaultProvider())
}
