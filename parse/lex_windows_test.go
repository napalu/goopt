package parse

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSplitWindows(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
		envVars map[string]string
	}{
		{
			name:    "simple command",
			input:   "dir /b",
			want:    []string{"dir", "/b"},
			wantErr: false,
		},
		{
			name:    "double quotes",
			input:   `echo "hello world"`,
			want:    []string{"echo", "hello world"},
			wantErr: false,
		},
		{
			name:    "single quotes converted to double",
			input:   "echo 'hello world'",
			want:    []string{"echo", "hello world"},
			wantErr: false,
		},
		{
			name:    "caret escape",
			input:   "echo ^| pipe",
			want:    []string{"echo", "|", "pipe"},
			wantErr: false,
		},
		{
			name:    "multiple carets",
			input:   "echo ^^ caret",
			want:    []string{"echo", "^", "caret"},
			wantErr: false,
		},
		{
			name:    "backslash escape in quotes",
			input:   `echo "hello\"world"`,
			want:    []string{"echo", `hello"world`},
			wantErr: false,
		},
		{
			name:    "environment variable",
			input:   "echo %PATH%",
			want:    []string{"echo", "test_path"},
			wantErr: false,
			envVars: map[string]string{"PATH": "test_path"},
		},
		{
			name:    "multiple environment variables",
			input:   "echo %VAR1% %VAR2%",
			want:    []string{"echo", "value1", "value2"},
			wantErr: false,
			envVars: map[string]string{"VAR1": "value1", "VAR2": "value2"},
		},
		{
			name:    "operators",
			input:   "cmd1 && cmd2 || cmd3",
			want:    []string{"cmd1", "&&", "cmd2", "||", "cmd3"},
			wantErr: false,
		},
		{
			name:    "redirection operators",
			input:   "cmd > out.txt 2>> err.txt",
			want:    []string{"cmd", ">", "out.txt", "2>>", "err.txt"},
			wantErr: false,
		},
		{
			name:    "mixed quotes and operators",
			input:   `echo "hello | world" && type "file name.txt"`,
			want:    []string{"echo", "hello | world", "&&", "type", "file name.txt"},
			wantErr: false,
		},
		{
			name:    "newline and carriage return",
			input:   "cmd1\r\ncmd2\n",
			want:    []string{"cmd1", "cmd2"},
			wantErr: false,
		},
		{
			name:    "multiple spaces and tabs",
			input:   "cmd1\t  cmd2    cmd3",
			want:    []string{"cmd1", "cmd2", "cmd3"},
			wantErr: false,
		},
		{
			name:    "empty environment variable",
			input:   "echo %NONEXISTENT%",
			want:    []string{"echo", ""},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			if tt.envVars != nil {
				for k, v := range tt.envVars {
					oldValue := os.Getenv(k)
					os.Setenv(k, v)
					defer os.Setenv(k, oldValue)
				}
			}

			got, err := Split(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Split() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Split() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleOperators(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		index     int
		operators []string
		want      bool
		wantIndex int
		tokens    []string
	}{
		{
			name:      "simple pipe",
			input:     "cmd1 | cmd2",
			tokens:    []string{},
			index:     5,
			operators: []string{"|", "&&", "||"},
			want:      true,
			wantIndex: 6,
		},
		{
			name:      "double operator",
			input:     "cmd1 && cmd2",
			index:     5,
			operators: []string{"&&", "||", "|"},
			want:      true,
			wantIndex: 7,
			tokens:    []string{"cmd1", "&&"},
		},
		{
			name:      "no operator",
			input:     "cmd1 cmd2",
			index:     5,
			operators: []string{"|", "&&", "||"},
			want:      false,
			wantIndex: 5,
			tokens:    []string{"cmd1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := []string{}
			if len(tt.input) > 0 && tt.index > 0 {
				tokens = append(tokens, tt.input[:tt.index])
			}

			builder := &strings.Builder{}
			index := tt.index

			got := handleOperators(tt.input, &tokens, builder, tt.operators, len(tt.input), &index)

			if got != tt.want {
				t.Errorf("handleOperators() = %v, want %v", got, tt.want)
			}
			if index != tt.wantIndex {
				t.Errorf("index = %v, want %v", index, tt.wantIndex)
			}
			if !reflect.DeepEqual(tokens, tt.tokens) {
				t.Errorf("tokens = %v, want %v", tokens, tt.tokens)
			}
		})
	}
}

func TestHandleBackslashes(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		startIndex int
		inQuotes   bool
		want       string
		wantIndex  int
		wantQuotes bool
	}{
		{
			name:       "escape quote in quotes",
			input:      `\"hello`,
			startIndex: 0,
			inQuotes:   true,
			want:       `"`,
			wantIndex:  2,
			wantQuotes: true,
		},
		{
			name:       "escape backslash",
			input:      `\\test`,
			startIndex: 0,
			inQuotes:   false,
			want:       `\`,
			wantIndex:  2,
			wantQuotes: false,
		},
		{
			name:       "single backslash at end",
			input:      `\`,
			startIndex: 0,
			inQuotes:   false,
			want:       `\`,
			wantIndex:  1,
			wantQuotes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &strings.Builder{}
			runes := []rune(tt.input)
			inQuotes := tt.inQuotes

			newIndex := handleBackslashes(runes, builder, &inQuotes, tt.startIndex)

			if builder.String() != tt.want {
				t.Errorf("builder = %v, want %v", builder.String(), tt.want)
			}
			if newIndex != tt.wantIndex {
				t.Errorf("index = %v, want %v", newIndex, tt.wantIndex)
			}
			if inQuotes != tt.wantQuotes {
				t.Errorf("inQuotes = %v, want %v", inQuotes, tt.wantQuotes)
			}
		})
	}
}

func TestHandleEnvVar(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		envVars map[string]string
		index   int
		want    string
		wantErr bool
	}{
		{
			name:    "simple env var",
			input:   "%TEST_VAR%",
			envVars: map[string]string{"TEST_VAR": "value"},
			index:   0,
			want:    "value",
			wantErr: false,
		},
		{
			name:    "missing closing %",
			input:   "%TEST_VAR",
			envVars: map[string]string{"TEST_VAR": "value"},
			index:   0,
			want:    "%TEST_VAR",
			wantErr: false,
		},
		{
			name:    "empty var name",
			input:   "%%",
			envVars: map[string]string{},
			index:   0,
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			if tt.envVars != nil {
				for k, v := range tt.envVars {
					oldValue := os.Getenv(k)
					os.Setenv(k, v)
					defer os.Setenv(k, oldValue)
				}
			}

			builder := &strings.Builder{}
			_, err := handleEnvVar(tt.input, builder, tt.index)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleEnvVar() error = %v, wantErr %v", err, tt.wantErr)
			}
			if builder.String() != tt.want {
				t.Errorf("builder = %v, want %v", builder.String(), tt.want)
			}
		})
	}
}
