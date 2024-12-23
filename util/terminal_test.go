package util

import (
	"bytes"
	"strings"
	"testing"
)

// MockTerminal for testing
type MockTerminal struct {
	Password         []byte
	IsTerminalResult bool
	Err              error
}

func (m *MockTerminal) ReadPassword(fd int) ([]byte, error) {
	return m.Password, m.Err
}

func (m *MockTerminal) IsTerminal(fd int) bool {
	return m.IsTerminalResult
}

func TestGetSecureString(t *testing.T) {
	tests := []struct {
		name          string
		prompt        string
		mockPassword  []byte
		isTerminal    bool
		mockErr       error
		want          string
		wantErr       bool
		wantErrString string
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
			name:          "empty password",
			prompt:        "Enter password: ",
			mockPassword:  []byte(""),
			isTerminal:    true,
			wantErr:       true,
			wantErrString: "empty password is invalid",
		},
		{
			name:          "not a terminal",
			prompt:        "Enter password: ",
			isTerminal:    false,
			wantErr:       true,
			wantErrString: "not attached to a terminal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			mock := &MockTerminal{
				Password:         tt.mockPassword,
				IsTerminalResult: tt.isTerminal,
				Err:              tt.mockErr,
			}

			got, err := GetSecureString(tt.prompt, &buf, mock)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSecureString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrString != "" && !strings.Contains(err.Error(), tt.wantErrString) {
				t.Errorf("GetSecureString() error = %v, wantErrString %v", err, tt.wantErrString)
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
