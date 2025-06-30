package errs

import (
	"errors"

	"github.com/napalu/goopt/v2/i18n"
)

// ErrWithProvider wraps a TranslatableError with a specific MessageProvider
type ErrWithProvider struct {
	te       i18n.TranslatableError
	provider i18n.MessageProvider
}

// WithProvider creates a new ErrWithProvider that wraps a TranslatableError with a specific MessageProvider
func WithProvider(te i18n.TranslatableError, provider i18n.MessageProvider) error {
	return &ErrWithProvider{
		te:       te,
		provider: provider,
	}
}

func (e *ErrWithProvider) Error() string {
	return e.te.Format(e.provider)
}

func (e *ErrWithProvider) Unwrap() error {
	return e.te
}

func (e *ErrWithProvider) Is(target error) bool {
	return errors.Is(e.te, target)
}

func (e *ErrWithProvider) As(target interface{}) bool {
	return errors.As(e.te, target)
}
