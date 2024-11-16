package util

import (
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
)

func GetSecureString(prompt string, w io.Writer) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		_, err := fmt.Fprint(w, prompt)
		if err != nil {
			return "", err
		}
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			_, _ = fmt.Fprintln(w)
			return "", err
		}
		pass := string(bytes)
		if len(pass) == 0 {
			_, _ = fmt.Fprintln(w)
			return "", fmt.Errorf("empty password is invalid")
		}
		_, _ = fmt.Fprintln(w)

		return pass, nil
	}

	return "", fmt.Errorf("not attached to a terminal. don't know how to get input from stdin")
}
