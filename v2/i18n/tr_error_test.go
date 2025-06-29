package i18n

import (
	"errors"
	"fmt"
	"golang.org/x/text/language"
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
	bundle.translations.Set(bundle.defaultLang.String(), map[string]string{
		"test_error_key": "Test error message",
	})
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

func TestTranslatableError_Wrapping(t *testing.T) {
	// Create test bundles
	bundle := NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"error.outer":     "outer error",
		"error.middle":    "middle error",
		"error.inner":     "inner error",
		"error.with_args": "error with %s",
		"error.wrapper":   "wrapper error",
	})
	bundle.AddLanguage(language.German, map[string]string{
		"error.outer":     "äußerer Fehler",
		"error.middle":    "mittlerer Fehler",
		"error.inner":     "innerer Fehler",
		"error.with_args": "Fehler mit %s",
		"error.wrapper":   "Wrapper-Fehler",
	})

	// Create default bundle and layered providers
	defaultBundle, _ := NewBundle()
	provider := NewLayeredMessageProvider(defaultBundle, nil, bundle)

	germanProvider := NewLayeredMessageProvider(defaultBundle, nil, bundle)
	germanProvider.SetDefaultLanguage(language.German)

	t.Run("Simple wrapped translatable error", func(t *testing.T) {
		inner := NewError("error.inner")
		outer := NewError("error.outer").Wrap(inner)

		// English
		got := outer.Format(provider)
		want := "outer error: inner error"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}

		// German
		got = outer.Format(germanProvider)
		want = "äußerer Fehler: innerer Fehler"
		if got != want {
			t.Errorf("Format() German = %q, want %q", got, want)
		}
	})

	t.Run("Multiple levels of wrapping", func(t *testing.T) {
		inner := NewError("error.inner")
		middle := NewError("error.middle").Wrap(inner)
		outer := NewError("error.outer").Wrap(middle)

		// English
		got := outer.Format(provider)
		want := "outer error: middle error: inner error"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}

		// German
		got = outer.Format(germanProvider)
		want = "äußerer Fehler: mittlerer Fehler: innerer Fehler"
		if got != want {
			t.Errorf("Format() German = %q, want %q", got, want)
		}
	})

	t.Run("Wrapped translatable error with args", func(t *testing.T) {
		inner := NewError("error.with_args").WithArgs("test")
		outer := NewError("error.outer").Wrap(inner)

		// English
		got := outer.Format(provider)
		want := "outer error: error with test"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}

		// German
		got = outer.Format(germanProvider)
		want = "äußerer Fehler: Fehler mit test"
		if got != want {
			t.Errorf("Format() German = %q, want %q", got, want)
		}
	})

	t.Run("Wrapped non-translatable error", func(t *testing.T) {
		regularErr := errors.New("regular error")
		outer := NewError("error.outer").Wrap(regularErr)

		// English
		got := outer.Format(provider)
		want := "outer error: regular error"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}

		// German (regular error stays in original language)
		got = outer.Format(germanProvider)
		want = "äußerer Fehler: regular error"
		if got != want {
			t.Errorf("Format() German = %q, want %q", got, want)
		}
	})

	t.Run("Mixed translatable and non-translatable errors", func(t *testing.T) {
		inner := NewError("error.inner")
		regularErr := fmt.Errorf("regular error: %w", inner)
		outer := NewError("error.outer").Wrap(regularErr)

		// The outer error wraps a regular error, which itself wraps a translatable error
		// Our recursive unwrapping should find the inner translatable error

		// English
		got := outer.Format(provider)
		want := "outer error: inner error"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}

		// German
		got = outer.Format(germanProvider)
		want = "äußerer Fehler: innerer Fehler"
		if got != want {
			t.Errorf("Format() German = %q, want %q", got, want)
		}
	})

	t.Run("Error() method uses default provider", func(t *testing.T) {
		// This test verifies that Error() uses the default provider
		// Since the test keys don't exist in the default bundle,
		// it will return the key itself
		inner := NewError("error.inner")
		outer := NewError("error.outer").Wrap(inner)

		// Error() should return the keys since they don't exist in default bundle
		got := outer.Error()
		// The wrapped error's Error() is called which also returns the key
		want := "error.outer: error.inner"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Complex chain with fmt.Errorf wrapping", func(t *testing.T) {
		// Simulate a validation error wrapped multiple times
		validationErr := NewError("error.with_args").WithArgs("value")
		wrappedOnce := fmt.Errorf("processing: %w", validationErr)
		wrappedTwice := fmt.Errorf("validation failed: %w", wrappedOnce)
		finalErr := NewError("error.wrapper").Wrap(wrappedTwice)

		// English
		got := finalErr.Format(provider)
		want := "wrapper error: error with value"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}

		// German
		got = finalErr.Format(germanProvider)
		want = "Wrapper-Fehler: Fehler mit value"
		if got != want {
			t.Errorf("Format() German = %q, want %q", got, want)
		}
	})

	t.Run("Nil wrapped error", func(t *testing.T) {
		err := NewError("error.outer")
		// No wrapped error

		got := err.Format(provider)
		want := "outer error"
		if got != want {
			t.Errorf("Format() = %q, want %q", got, want)
		}
	})

	t.Run("errors.Is works through wrapping", func(t *testing.T) {
		sentinel := NewError("error.inner")
		wrapped := NewError("error.outer").Wrap(sentinel)

		if !errors.Is(wrapped, sentinel) {
			t.Error("errors.Is(wrapped, sentinel) should be true")
		}
	})

	t.Run("errors.As works through wrapping", func(t *testing.T) {
		inner := NewError("error.inner").WithArgs("test")
		outer := NewError("error.outer").Wrap(inner)

		var te TranslatableError
		if !errors.As(outer, &te) {
			t.Error("errors.As(outer, &te) should be true")
		}
	})
}
