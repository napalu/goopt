package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseValidatorSpecs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple validators",
			input:    "email,minlength:5,maxlength:20",
			expected: []string{"email", "minlength:5", "maxlength:20"},
		},
		{
			name:     "Regex with escaped comma",
			input:    `regex:^.{5\,10}$,alphanumeric`,
			expected: []string{"regex:^.{5,10}$", "alphanumeric"},
		},
		{
			name:     "Regex with escaped colon",
			input:    `regex:[a-z]\:[0-9],nowhitespace`,
			expected: []string{"regex:[a-z]:[0-9]", "nowhitespace"},
		},
		{
			name:     "Multiple escaped characters",
			input:    `regex:^[A-Z]{2\,4}\:[0-9]{3\,5}$,minlength:6`,
			expected: []string{"regex:^[A-Z]{2,4}:[0-9]{3,5}$", "minlength:6"},
		},
		{
			name:     "Escaped backslash",
			input:    `regex:\\d+\\w+,integer`,
			expected: []string{`regex:\d+\w+`, "integer"},
		},
		{
			name:     "Complex regex patterns",
			input:    `regex:^(?:https?\:\/\/)?[\w\.\-]+\:[0-9]{2\,5}$,url`,
			expected: []string{`regex:^(?:https?:\/\/)?[\w\.\-]+:[0-9]{2,5}$`, "url"},
		},
		{
			name:     "Empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "Single validator",
			input:    "email",
			expected: []string{"email"},
		},
		{
			name:     "Trailing comma",
			input:    "email,minlength:5,",
			expected: []string{"email", "minlength:5"},
		},
		{
			name:     "Spaces around validators",
			input:    " email , minlength:5 , maxlength:20 ",
			expected: []string{"email", "minlength:5", "maxlength:20"},
		},
		{
			name:     "Unknown escape preserved",
			input:    `regex:\n\t\r,alphanumeric`,
			expected: []string{`regex:\n\t\r`, "alphanumeric"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatorSpecs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseValidatorSpecsRealWorld(t *testing.T) {
	// Test real-world regex patterns that would break with simple comma splitting
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Password complexity regex",
			input:    `regex:^(?=.*[a-z])(?=.*[A-Z])(?=.*[0-9])(?=.*[@$!%*?&])[A-Za-z0-9@$!%*?&]{8\,20}$`,
			expected: []string{"regex:^(?=.*[a-z])(?=.*[A-Z])(?=.*[0-9])(?=.*[@$!%*?&])[A-Za-z0-9@$!%*?&]{8,20}$"},
		},
		{
			name:     "Email with length constraints",
			input:    `email,regex:^.{5\,100}$`,
			expected: []string{"email", "regex:^.{5,100}$"},
		},
		{
			name:     "UUID pattern",
			input:    `regex:^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$,nowhitespace`,
			expected: []string{"regex:^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", "nowhitespace"},
		},
		{
			name:     "Version number pattern",
			input:    `regex:^v?[0-9]{1\,3}\.[0-9]{1\,3}\.[0-9]{1\,3}$`,
			expected: []string{"regex:^v?[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatorSpecs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
