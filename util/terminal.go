package util

import (
	"fmt"
	"golang.org/x/term"
	"os"
)

func GetSecureString(prompt string) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print(prompt)
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Println()
			return "", err
		}
		pass := string(bytes)
		if len(pass) == 0 {
			fmt.Println()
			return "", fmt.Errorf("empty password is invalid")
		}
		fmt.Println()

		return pass, nil
	}

	return "", fmt.Errorf("not attached to a terminal. don't know how to get input from stdin")
}
