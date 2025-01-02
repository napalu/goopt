//go:build linux || darwin

package parse

import "github.com/google/shlex"

// Split splits a command string into arguments
func Split(s string) ([]string, error) {
	args, err := shlex.Split(s)
	if err != nil {
		return nil, err
	}

	return args, nil
}
