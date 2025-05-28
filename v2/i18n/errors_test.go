package i18n

import (
	"errors"
	"testing"
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
