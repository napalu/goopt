package parse

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:    "simple command",
			input:   "ls -l",
			want:    []string{"ls", "-l"},
			wantErr: false,
		},
		{
			name:    "quoted arguments",
			input:   `echo "hello world"`,
			want:    []string{"echo", "hello world"},
			wantErr: false,
		},
		{
			name:    "multiple quotes",
			input:   `echo "first quote" 'second quote'`,
			want:    []string{"echo", "first quote", "second quote"},
			wantErr: false,
		},
		{
			name:    "escaped quotes",
			input:   `echo \"hello\"`,
			want:    []string{"echo", `"hello"`},
			wantErr: false,
		},
		{
			name:    "multiple spaces",
			input:   "cmd   arg1    arg2",
			want:    []string{"cmd", "arg1", "arg2"},
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "only spaces",
			input:   "   ",
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "with operators",
			input:   "cmd1 | cmd2 > output.txt",
			want:    []string{"cmd1", "|", "cmd2", ">", "output.txt"},
			wantErr: false,
		},
		{
			name:    "with environment variables",
			input:   "echo $HOME",
			want:    []string{"echo", "$HOME"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
