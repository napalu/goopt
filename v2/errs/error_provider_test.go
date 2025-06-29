package errs

import (
	"errors"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/stretchr/testify/assert"
)

// Mock message provider for testing
type mockProvider struct {
	messages map[string]string
}

func (m *mockProvider) GetMessage(key string) string {
	if msg, ok := m.messages[key]; ok {
		return msg
	}
	return key
}

func TestWithProvider(t *testing.T) {
	// Create a translatable error with formatting
	baseErr := i18n.NewError("test.error").WithArgs("arg1", 123)
	provider := &mockProvider{
		messages: map[string]string{
			"test.error": "Test error: %s and %d",
		},
	}

	err := WithProvider(baseErr, provider)

	assert.NotNil(t, err)
	// The error should format the message through the provider
	assert.Contains(t, err.Error(), "Test error:")
	assert.Contains(t, err.Error(), "arg1")
	assert.Contains(t, err.Error(), "123")

	// Test Unwrap
	var wp *withProvider
	assert.True(t, errors.As(err, &wp))
	assert.Equal(t, baseErr, wp.Unwrap())

	// Test Is
	assert.True(t, errors.Is(err, baseErr))
	assert.False(t, errors.Is(err, errors.New("other error")))
}

func TestWithProvider_SimpleMessage(t *testing.T) {
	// Test without args
	baseErr := i18n.NewError("test.simple")
	provider := &mockProvider{
		messages: map[string]string{
			"test.simple": "Simple error message",
		},
	}

	err := WithProvider(baseErr, provider)

	assert.NotNil(t, err)
	assert.Equal(t, "Simple error message", err.Error())
}

func TestWithProvider_As(t *testing.T) {
	baseErr := i18n.NewError("test.error")
	provider := &mockProvider{
		messages: map[string]string{
			"test.error": "Test error",
		},
	}

	err := WithProvider(baseErr, provider)

	// Test As with withProvider type
	var wp *withProvider
	assert.True(t, errors.As(err, &wp))
	assert.Equal(t, provider, wp.provider)

	// Test that baseErr is still accessible through the wrapper
	assert.Equal(t, "test.error", baseErr.Key())
}
