package i18n

import (
	"errors"
	"fmt"
)

type TranslatableError interface {
	error
	Key() string
	Args() []interface{}
	Unwrap() error
}

type translatableError struct {
	key     string
	args    []interface{}
	wrapped error
}

func NewTranslatableError(cause error, key string, args ...interface{}) TranslatableError {
	return &translatableError{
		wrapped: cause,
		key:     key,
		args:    args,
	}
}

func (e *translatableError) Error() string {
	return fmt.Sprintf("%s: %v", e.key, e.wrapped)
}

func (e *translatableError) Key() string {
	return e.key
}

func (e *translatableError) Args() []interface{} {
	return e.args
}

func (e *translatableError) Unwrap() error {
	return e.wrapped
}

func (e *translatableError) Is(target error) bool {
	return errors.Is(e.wrapped, target)
}
