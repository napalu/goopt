package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegexSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid spec with uppercase pattern",
			spec:    "{pattern:^[A-Z]+$,desc:Uppercase letters only}",
			value:   "HELLO",
			wantErr: false,
		},
		{
			name:    "valid spec with digit pattern",
			spec:    "{pattern:^\\d{5}$,desc:5-digit ZIP code}",
			value:   "12345",
			wantErr: false,
		},
		{
			name:    "invalid value for pattern",
			spec:    "{pattern:^[A-Z]+$,desc:Uppercase letters only}",
			value:   "hello",
			wantErr: true,
		},
		{
			name:    "malformed spec",
			spec:    "not a valid spec",
			value:   "anything",
			wantErr: true, // Will always fail due to parse error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := RegexSpec(tt.spec)
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		desc    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid email pattern",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			desc:    "email address",
			value:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "invalid email",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			desc:    "email address",
			value:   "not-an-email",
			wantErr: true,
		},
		{
			name:    "phone number pattern",
			pattern: `^\+?1?\d{10}$`,
			desc:    "phone number",
			value:   "+1234567890",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := Regex(tt.pattern, tt.desc)
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegexes(t *testing.T) {
	patterns := []RegexPattern{
		{`^\d+$`, "digits only"},
		{`^[A-Z]+$`, "uppercase letters"},
		{`^[a-z]+$`, "lowercase letters"},
	}

	validator := Regexes(patterns...)

	// Test value that matches first pattern
	err := validator("12345")
	assert.NoError(t, err)

	// Test value that matches second pattern
	err = validator("HELLO")
	assert.NoError(t, err)

	// Test value that matches third pattern
	err = validator("hello")
	assert.NoError(t, err)

	// Test value that matches none
	err = validator("Hello123!")
	assert.Error(t, err)
}

func TestMustMatch(t *testing.T) {
	validator := MustMatch(`^[A-Z]{3}$`, "three uppercase letters")

	// Valid match
	err := validator("ABC")
	assert.NoError(t, err)

	// Invalid match
	err = validator("abc")
	assert.Error(t, err)

	// Too long
	err = validator("ABCD")
	assert.Error(t, err)
}

func TestMustNotMatch(t *testing.T) {
	// Must not contain numbers
	validator := MustNotMatch(`\d`, "no digits allowed")

	// Valid - no numbers
	err := validator("HelloWorld")
	assert.NoError(t, err)

	// Invalid - contains numbers
	err = validator("Hello123")
	assert.Error(t, err)
}
