package validation

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

func TestNumericValidators(t *testing.T) {
	t.Run("Min", func(t *testing.T) {
		validator := Min(10.5)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid exact", "10.5", false},
			{"Valid greater", "15.0", false},
			{"Valid integer", "20", false},
			{"Invalid less", "5.5", true},
			{"Invalid negative", "-10", true},
			{"Invalid string", "abc", true},
			{"Empty string", "", true},
			{"Large number", "1000000", false},
			{"Scientific notation", "1e2", false},
			{"Decimal precision", "10.500001", false},
			{"Just below", "10.499999", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Max", func(t *testing.T) {
		validator := Max(100.5)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid exact", "100.5", false},
			{"Valid less", "50.0", false},
			{"Valid zero", "0", false},
			{"Valid negative", "-10", false},
			{"Invalid greater", "101", true},
			{"Invalid much greater", "200.5", true},
			{"Invalid string", "xyz", true},
			{"Empty string", "", true},
			{"Scientific notation", "1e1", false}, // 10
			{"Large scientific", "1e3", true},     // 1000
			{"Decimal precision", "100.499999", false},
			{"Just above", "100.500001", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Range", func(t *testing.T) {
		validator := Range(5.5, 10.5)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid min boundary", "5.5", false},
			{"Valid max boundary", "10.5", false},
			{"Valid middle", "8", false},
			{"Invalid below", "5.4", true},
			{"Invalid above", "10.6", true},
			{"Invalid negative", "-1", true},
			{"Invalid string", "test", true},
			{"Empty string", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("IntRange", func(t *testing.T) {
		validator := IntRange(5, 10)

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid min boundary", "5", false},
			{"Valid max boundary", "10", false},
			{"Valid middle", "8", false},
			{"Invalid below", "4", true},
			{"Invalid above", "11", true},
			{"Invalid negative", "-1", true},
			{"Invalid decimal", "7.5", true},
			{"Invalid string", "test", true},
			{"Empty string", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Float", func(t *testing.T) {
		validator := Float()

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid integer", "42", false},
			{"Valid decimal", "3.14", false},
			{"Valid negative", "-2.5", false},
			{"Valid zero", "0", false},
			{"Valid scientific", "1.23e-4", false},
			{"Valid positive sign", "+10.5", false},
			{"Invalid string", "abc", true},
			{"Invalid mixed", "10.5abc", true},
			{"Empty string", "", true},
			{"Multiple dots", "10.5.3", true},
			{"Infinity", "inf", false},
			{"NaN", "NaN", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Boolean", func(t *testing.T) {
		validator := Boolean()

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			// Valid true values
			{"true lowercase", "true", false},
			{"TRUE uppercase", "TRUE", false},
			{"True mixed", "True", false},
			{"1", "1", false},
			{"t", "t", false},
			{"T", "T", false},

			// Valid false values
			{"false lowercase", "false", false},
			{"FALSE uppercase", "FALSE", false},
			{"False mixed", "False", false},
			{"0", "0", false},
			{"f", "f", false},
			{"F", "F", false},

			// Invalid values
			{"yes", "yes", true},
			{"no", "no", true},
			{"on", "on", true},
			{"off", "off", true},
			{"empty", "", true},
			{"number", "42", true},
			{"string", "hello", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Port", func(t *testing.T) {
		validator := Port()

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid min port", "1", false},
			{"Valid max port", "65535", false},
			{"Valid common HTTP", "80", false},
			{"Valid common HTTPS", "443", false},
			{"Valid common SSH", "22", false},
			{"Valid high port", "8080", false},
			{"Invalid zero", "0", true},
			{"Invalid negative", "-1", true},
			{"Invalid too high", "65536", true},
			{"Invalid much too high", "100000", true},
			{"Invalid string", "http", true},
			{"Invalid decimal", "80.5", true},
			{"Empty string", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestIPValidator(t *testing.T) {
	validator := IP()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// Valid IPv4
		{"Valid IPv4", "192.168.1.1", false},
		{"Valid IPv4 localhost", "127.0.0.1", false},
		{"Valid IPv4 broadcast", "255.255.255.255", false},
		{"Valid IPv4 network", "10.0.0.0", false},
		{"Valid IPv4 zeros", "0.0.0.0", false},

		// Valid IPv6
		{"Valid IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"Valid IPv6 compressed", "2001:db8:85a3::8a2e:370:7334", false},
		{"Valid IPv6 localhost", "::1", false},
		{"Valid IPv6 all zeros", "::", false},
		{"Valid IPv6 mixed", "::ffff:192.168.1.1", false},

		// Invalid
		{"Invalid IPv4 out of range", "256.1.1.1", true},
		{"Invalid IPv4 negative", "-1.0.0.0", true},
		{"Invalid IPv4 missing octet", "192.168.1", true},
		{"Invalid IPv4 extra octet", "192.168.1.1.1", true},
		{"Invalid IPv4 with port", "192.168.1.1:80", true},
		{"Invalid IPv6 too many groups", "2001:db8:85a3::8a2e:370:7334:extra", true},
		{"Invalid hostname", "example.com", true},
		{"Invalid string", "not-an-ip", true},
		{"Empty string", "", true},
		{"Invalid IPv4 with letters", "192.168.1.a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCustomValidator(t *testing.T) {
	// Test a custom validator that checks if string starts with "test_"
	customValidator := Custom(func(value string) error {
		if len(value) >= 5 && value[:5] == "test_" {
			return nil
		}
		return assert.AnError
	})

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"Valid prefix", "test_something", false},
		{"Valid exact", "test_", false},
		{"Invalid no prefix", "something", true},
		{"Invalid partial", "test", true},
		{"Invalid empty", "", true},
		{"Invalid different prefix", "prod_something", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customValidator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Test nil function
	t.Run("Nil function", func(t *testing.T) {
		validator := Custom(nil)
		err := validator("any value")
		assert.NoError(t, err, "nil custom function should not error")
	})

	// Test a real-world example: UUID validator
	t.Run("UUID validator", func(t *testing.T) {
		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		uuidValidator := Custom(func(value string) error {
			if !uuidRegex.MatchString(strings.ToLower(value)) {
				return errors.New("value must be a valid UUID")
			}
			return nil
		})

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"Valid UUID lowercase", "550e8400-e29b-41d4-a716-446655440000", false},
			{"Valid UUID uppercase", "550E8400-E29B-41D4-A716-446655440000", false},
			{"Invalid missing segment", "550e8400-e29b-41d4-a716", true},
			{"Invalid extra segment", "550e8400-e29b-41d4-a716-446655440000-extra", true},
			{"Invalid no dashes", "550e8400e29b41d4a716446655440000", true},
			{"Invalid wrong character", "550e8400-e29b-41d4-a716-44665544000g", true},
			{"Empty string", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := uuidValidator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	// Test custom validator with structured errors
	t.Run("Password strength validator", func(t *testing.T) {
		passwordValidator := Custom(func(value string) error {
			if len(value) < 8 {
				return errors.New("password must be at least 8 characters")
			}

			hasUpper := false
			hasLower := false
			hasDigit := false
			hasSpecial := false

			for _, r := range value {
				switch {
				case unicode.IsUpper(r):
					hasUpper = true
				case unicode.IsLower(r):
					hasLower = true
				case unicode.IsDigit(r):
					hasDigit = true
				case unicode.IsPunct(r) || unicode.IsSymbol(r):
					hasSpecial = true
				}
			}

			if !hasUpper {
				return errors.New("password must contain at least one uppercase letter")
			}
			if !hasLower {
				return errors.New("password must contain at least one lowercase letter")
			}
			if !hasDigit {
				return errors.New("password must contain at least one digit")
			}
			if !hasSpecial {
				return errors.New("password must contain at least one special character")
			}

			return nil
		})

		tests := []struct {
			name    string
			value   string
			wantErr bool
			errMsg  string
		}{
			{"Valid strong password", "Test@123", false, ""},
			{"Valid with multiple specials", "MyP@ssw0rd!", false, ""},
			{"Too short", "Test@1", true, "password must be at least 8 characters"},
			{"No uppercase", "test@123", true, "password must contain at least one uppercase letter"},
			{"No lowercase", "TEST@123", true, "password must contain at least one lowercase letter"},
			{"No digit", "Test@Pass", true, "password must contain at least one digit"},
			{"No special", "TestPass123", true, "password must contain at least one special character"},
			{"Empty password", "", true, "password must be at least 8 characters"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := passwordValidator(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
					if tt.errMsg != "" {
						assert.EqualError(t, err, tt.errMsg)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
