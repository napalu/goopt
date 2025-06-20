package errs

import (
	"errors"

	"github.com/napalu/goopt/v2/i18n"
)

type withProvider struct {
	te       i18n.TranslatableError
	provider i18n.MessageProvider
}

func WithProvider(te i18n.TranslatableError, provider i18n.MessageProvider) error {
	return &withProvider{
		te:       te,
		provider: provider,
	}
}

func (e *withProvider) Error() string {
	return e.te.Format(e.provider)
}

func (e *withProvider) Unwrap() error {
	return e.te
}

func (e *withProvider) Is(target error) bool {
	return errors.Is(e.te, target)
}

func (e *withProvider) As(target interface{}) bool {
	return errors.As(e.te, target)
}
