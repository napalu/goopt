package i18n

import (
	"errors"
	"fmt"
)

// TranslatableError represents an error that can be translated
type TranslatableError interface {
	error
	Key() string
	Args() []interface{}
	Unwrap() error
	WithArgs(args ...interface{}) TranslatableError
	Wrap(err error) TranslatableError
	Is(target error) bool
	Format(provider MessageProvider) string
}

// TrError represents a translatable error with optional formatting arguments
// and error wrapping support. It implements both the TranslatableError interface
// and the standard error interface.
//
// Example usage:
//
//	err := NewError("validation.error")
//	err = err.WithArgs("field", "value")
//	err = err.Wrap(originalError)
type TrError struct {
	// The sentinel error value for comparison with errors.Is
	sentinel error
	// The translation key
	key string
	// Optional format arguments
	args []interface{}
	// Optional wrapped error
	wrapped error
}

// NewError creates a new translatable error with a key
func NewError(key string) *TrError {
	sentinel := errors.New(key)
	return &TrError{
		sentinel: sentinel,
		key:      key,
	}
}

// NewErrorWithProvider creates a new translatable error with a key and specific provider
func NewErrorWithProvider(key string, provider MessageProvider) *TrError {
	defaultMsg := provider.GetMessage(key)
	sentinel := errors.New(defaultMsg)
	return &TrError{
		sentinel: sentinel,
		key:      key,
	}
}

// Error returns the default message, formatted with args if provided
func (e *TrError) Error() string {
	return e.Format(getDefaultProvider())
}

// WithArgs returns a copy of the error with format arguments
func (e *TrError) WithArgs(args ...interface{}) TranslatableError {
	return &TrError{
		sentinel: e.sentinel,
		key:      e.key,
		args:     args,
		wrapped:  e.wrapped,
	}
}

// Wrap returns a new error that wraps another error
func (e *TrError) Wrap(err error) TranslatableError {
	return &TrError{
		sentinel: e.sentinel,
		key:      e.key,
		args:     e.args,
		wrapped:  err,
	}
}

// Is implements errors.Is for comparison with the sentinel error
func (e *TrError) Is(target error) bool {
	// Check if target is the same sentinel error
	if t, ok := target.(*TrError); ok {
		return e.sentinel == t.sentinel
	}
	// Check if target is the sentinel error directly
	return target == e.sentinel || target == e
}

// Key returns the translation key
func (e *TrError) Key() string {
	return e.key
}

// Args returns the format arguments
func (e *TrError) Args() []interface{} {
	return e.args
}

// Unwrap returns the wrapped error
func (e *TrError) Unwrap() error {
	return e.wrapped
}

func (e *TrError) Format(provider MessageProvider) string {
	msg := provider.GetMessage(e.key)
	if len(e.args) > 0 {
		translatedArgs := make([]interface{}, len(e.args))
		for i, arg := range e.args {
			if t, ok := arg.(Translatable); ok {
				translatedArgs[i] = t.T(provider)
			} else {
				translatedArgs[i] = arg
			}
		}
		msg = fmt.Sprintf(msg, translatedArgs...)
	}

	if e.wrapped != nil {
		return fmt.Sprintf("%s: %v", msg, e.wrapped)
	}
	return msg
}
