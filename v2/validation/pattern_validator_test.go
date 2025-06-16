package validation

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			validator, err := RegexSpec(tt.spec)
			if err != nil {
				// If spec parsing failed, test should expect error
				assert.True(t, tt.wantErr, "unexpected error parsing spec: %v", err)
				return
			}
			err = validator.Validate(tt.value)
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
			validator, err := Regex(tt.pattern, tt.desc)
			if err != nil {
				t.Fatalf("unexpected error creating validator: %v", err)
			}
			err = validator.Validate(tt.value)
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

	validator, err := Regexes(patterns...)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Test value that matches first pattern
	err = validator.Validate("12345")
	assert.NoError(t, err)

	// Test value that matches second pattern
	err = validator.Validate("HELLO")
	assert.NoError(t, err)

	// Test value that matches third pattern
	err = validator.Validate("hello")
	assert.NoError(t, err)

	// Test value that matches none
	err = validator.Validate("Hello123!")
	assert.Error(t, err)
}

func TestMustMatch(t *testing.T) {
	validator, err := MustMatch(`^[A-Z]{3}$`, "three uppercase letters")
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Valid match
	err = validator.Validate("ABC")
	assert.NoError(t, err)

	// Invalid match
	err = validator.Validate("abc")
	assert.Error(t, err)

	// Too long
	err = validator.Validate("ABCD")
	assert.Error(t, err)
}

func TestMustNotMatch(t *testing.T) {
	// Must not contain numbers
	validator, err := MustNotMatch(`\d`, "no digits allowed")
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Valid - no numbers
	err = validator.Validate("HelloWorld")
	assert.NoError(t, err)

	// Invalid - contains numbers
	err = validator.Validate("Hello123")
	assert.Error(t, err)
}

func TestRegexFromValues(t *testing.T) {
	// Test creating regex from accepted values
	values := []string{"red", "green", "blue"}

	validator := RegexFromValues(values, "color choices")

	// Test valid values
	assert.NoError(t, validator.Validate("red"))
	assert.NoError(t, validator.Validate("green"))
	assert.NoError(t, validator.Validate("blue"))

	// Test invalid value
	err := validator.Validate("yellow")
	assert.Error(t, err)

	// Test case sensitivity
	err = validator.Validate("RED")
	assert.Error(t, err)
}

func TestRegexFromValues_Empty(t *testing.T) {
	// Test with empty values
	validator := RegexFromValues([]string{}, "no choices")

	// Should reject everything
	err := validator.Validate("anything")
	assert.Error(t, err)
}
