package i18n

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/text/language"
)

// TranslatableError represents an error that can be translated
type TranslatableError interface {
	error
	Key() string
	Args() []interface{}
	Unwrap() error
	WithArgs(args ...interface{}) TranslatableError
	Wrap(err error) TranslatableError
}

// MessageProvider defines an interface for getting default messages
type MessageProvider interface {
	GetMessage(key string) string
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
	// New field
	messageProvider MessageProvider
}

// DefaultMessageProvider implements MessageProvider using the bundle
type DefaultMessageProvider struct {
	bundle *Bundle
	lang   language.Tag
}

func (p *DefaultMessageProvider) GetMessage(key string) string {
	if msg, ok := p.bundle.translations[p.lang][key]; ok {
		return msg
	}
	return key
}

// NewError creates a new translatable error with a key
func NewError(key string) *TrError {
	defaultMsg := getDefaultProvider().GetMessage(key)
	sentinel := errors.New(defaultMsg)
	return &TrError{
		sentinel:        sentinel,
		key:             key,
		messageProvider: getDefaultProvider(),
	}
}

// Error returns the default message, formatted with args if provided
func (e *TrError) Error() string {
	msg := e.messageProvider.GetMessage(e.key)
	if len(e.args) > 0 {
		msg = fmt.Sprintf(msg, e.args...)
	}

	if e.wrapped != nil {
		return fmt.Sprintf("%s: %v", msg, e.wrapped)
	}
	return msg
}

// WithArgs returns a copy of the error with format arguments
func (e *TrError) WithArgs(args ...interface{}) TranslatableError {
	return &TrError{
		sentinel:        e.sentinel,
		key:             e.key,
		args:            args,
		wrapped:         e.wrapped,
		messageProvider: e.messageProvider,
	}
}

// Wrap returns a new error that wraps another error
func (e *TrError) Wrap(err error) TranslatableError {
	return &TrError{
		sentinel:        e.sentinel,
		key:             e.key,
		args:            e.args,
		wrapped:         err,
		messageProvider: e.messageProvider,
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

// Package-level provider management
var (
	defaultProvider    MessageProvider
	defaultProviderMux sync.RWMutex
)

// SetDefaultMessageProvider allows users to set their own provider
func SetDefaultMessageProvider(p MessageProvider) {
	defaultProviderMux.Lock()
	defer defaultProviderMux.Unlock()
	defaultProvider = p
}

func getDefaultProvider() MessageProvider {
	defaultProviderMux.RLock()
	if defaultProvider != nil {
		defer defaultProviderMux.RUnlock()
		return defaultProvider
	}
	defaultProviderMux.RUnlock()

	// Upgrade to write lock for initialization
	defaultProviderMux.Lock()
	defer defaultProviderMux.Unlock()

	if defaultProvider == nil {
		defaultProvider = &DefaultMessageProvider{
			bundle: Default(),
			lang:   language.English,
		}
	}
	return defaultProvider
}
