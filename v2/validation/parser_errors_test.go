package validation

import (
	"testing"
)

func TestParserErrors(t *testing.T) {
	tests := []struct {
		name      string
		validator string
		expectErr bool
	}{
		{
			name:      "unknown validator",
			validator: "foobar",
			expectErr: true,
		},
		{
			name:      "minlength missing argument",
			validator: "minlength",
			expectErr: true,
		},
		{
			name:      "minlength invalid argument",
			validator: "minlength(abc)",
			expectErr: true,
		},
		{
			name:      "range missing arguments",
			validator: "range(1)",
			expectErr: true,
		},
		{
			name:      "range invalid min",
			validator: "range(abc,10)",
			expectErr: true,
		},
		{
			name:      "range invalid max",
			validator: "range(1,xyz)",
			expectErr: true,
		},
		{
			name:      "oneof missing arguments",
			validator: "oneof",
			expectErr: true,
		},
		{
			name:      "fileext missing arguments",
			validator: "fileext",
			expectErr: true,
		},
		{
			name:      "valid email validator",
			validator: "email",
			expectErr: false,
		},
		{
			name:      "valid minlength validator",
			validator: "minlength(5)",
			expectErr: false,
		},
		{
			name:      "valid range validator",
			validator: "range(1,100)",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseValidators([]string{tt.validator})
			if tt.expectErr && err == nil {
				t.Errorf("expected error for validator %q", tt.validator)
			} else if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for validator %q: %v", tt.validator, err)
			}
		})
	}
}
