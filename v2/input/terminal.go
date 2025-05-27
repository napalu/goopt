package input

import (
	"fmt"
	"io"
	"os"

	"github.com/napalu/goopt/v2/errs"
	"golang.org/x/term"
)

// TerminalReader interface for reading secure input
type TerminalReader interface {
	ReadPassword(fd int) ([]byte, error)
	IsTerminal(fd int) bool
}

// DefaultTerminal implements real terminal operations
type DefaultTerminal struct{}

// ReadPassword reads a password from the terminal
func (t *DefaultTerminal) ReadPassword(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

// IsTerminal checks if we are attached to a real terminal
func (t *DefaultTerminal) IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

// GetSecureString reads a password from the terminal
func GetSecureString(prompt string, w io.Writer, terminal TerminalReader) (string, error) {
	if terminal == nil {
		terminal = &DefaultTerminal{}
	}

	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		_, err := fmt.Fprint(w, prompt)
		if err != nil {
			return "", err
		}
		bytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			_, _ = fmt.Fprintln(w)
			return "", err
		}
		pass := string(bytes)
		if len(pass) == 0 {
			_, _ = fmt.Fprintln(w)
			return "", errs.ErrParseEmptyInput.WithArgs("password")
		}
		_, _ = fmt.Fprintln(w)

		return pass, nil
	}

	return "", errs.ErrNotAttachedToTerminal.WithArgs("stdin")
}
