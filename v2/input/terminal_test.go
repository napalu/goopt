package input

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/errs"
)

// MockTerminal for testing
type MockTerminal struct {
	Password    []byte
	IsATerminal bool
	Err         error
}

func (m *MockTerminal) ReadPassword(fd int) ([]byte, error) {
	return m.Password, m.Err
}

func (m *MockTerminal) IsTerminal(fd int) bool {
	return m.IsATerminal
}

// DefaultTerminal methods are not tested because they are simple pass-through
// wrappers around golang.org/x/term functions. Testing them would provide no
// value and would be environment-dependent.

func TestGetSecureString(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		mockPassword []byte
		isTerminal   bool
		mockErr      error
		want         string
		wantErr      bool
		errWanted    error
	}{
		{
			name:         "successful password input",
			prompt:       "Enter password: ",
			mockPassword: []byte("secretpass"),
			isTerminal:   true,
			want:         "secretpass",
			wantErr:      false,
		},
		{
			name:         "empty password",
			prompt:       "Enter password: ",
			mockPassword: []byte(""),
			isTerminal:   true,
			wantErr:      true,
			errWanted:    errs.ErrParseEmptyInput.WithArgs("password"),
		},
		{
			name:       "not a terminal",
			prompt:     "Enter password: ",
			isTerminal: false,
			wantErr:    true,
			errWanted:  errs.ErrNotAttachedToTerminal.WithArgs("stdin"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			mock := &MockTerminal{
				Password:    tt.mockPassword,
				IsATerminal: tt.isTerminal,
				Err:         tt.mockErr,
			}

			got, err := GetSecureString(tt.prompt, &buf, mock)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSecureString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errWanted != nil && !errors.Is(err, tt.errWanted) {
				t.Errorf("GetSecureString() error = %v, wantErrString %v", err, tt.errWanted)
				return
			}
			if got != tt.want {
				t.Errorf("GetSecureString() = %v, want %v", got, tt.want)
			}
			if !tt.wantErr {
				promptOutput := strings.TrimSpace(buf.String())
				expectedPrompt := strings.TrimSpace(tt.prompt)
				if promptOutput != expectedPrompt {
					t.Errorf("Prompt not written correctly, got %q, want %q", promptOutput, expectedPrompt)
				}
			}
		})
	}
}

func TestGetSecureString_NilTerminal(t *testing.T) {
	// When terminal is nil, it should default to DefaultTerminal
	// Since we're not in a real terminal during tests, this should fail
	var buf bytes.Buffer
	_, err := GetSecureString("Password: ", &buf, nil)

	// Should get the "not attached to terminal" error
	if !errors.Is(err, errs.ErrNotAttachedToTerminal.WithArgs("stdin")) {
		t.Errorf("Expected ErrNotAttachedToTerminal, got %v", err)
	}
}
