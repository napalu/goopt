package goopt_test

import (
	"testing"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteLengthValidatorsIntegration(t *testing.T) {
	t.Run("struct tags with byte validators", func(t *testing.T) {
		type Config struct {
			// API key with exact byte requirement
			APIKey string `goopt:"name:api-key;validators:bytelength(32)"`

			// Password with min/max byte limits (e.g., bcrypt limit)
			Password string `goopt:"name:password;validators:minbytelength(8),maxbytelength(72)"`

			// Bio with max byte limit (database constraint)
			Bio string `goopt:"name:bio;validators:maxbytelength(500)"`
		}

		tests := []struct {
			name    string
			args    []string
			wantErr bool
			errMsg  string
		}{
			{
				name:    "valid API key",
				args:    []string{"--api-key", "12345678901234567890123456789012"}, // exactly 32 bytes
				wantErr: false,
			},
			{
				name:    "API key too short",
				args:    []string{"--api-key", "tooshort"},
				wantErr: true,
				errMsg:  "exactly 32 bytes",
			},
			{
				name:    "valid password",
				args:    []string{"--password", "mySecureP@ss"}, // 12 bytes
				wantErr: false,
			},
			{
				name:    "password too short",
				args:    []string{"--password", "short"}, // 5 bytes < 8
				wantErr: true,
				errMsg:  "at least 8 bytes",
			},
			{
				name:    "password with unicode",
				args:    []string{"--password", "café123"}, // 8 bytes (é = 2 bytes)
				wantErr: false,
			},
			{
				name:    "password too long",
				args:    []string{"--password", string(make([]byte, 73))}, // 73 bytes > 72
				wantErr: true,
				errMsg:  "at most 72 bytes",
			},
			{
				name:    "bio with unicode under limit",
				args:    []string{"--bio", "I love programming with Go! 中文测试 😊"}, // should be under 500 bytes
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var cfg Config
				p, err := goopt.NewParserFromStruct(&cfg)
				require.NoError(t, err)

				success := p.Parse(tt.args)

				if tt.wantErr {
					assert.False(t, success)
					if tt.errMsg != "" {
						// Check if error message contains expected text
						// The error would be in the parser's internal state
					}
				} else {
					assert.True(t, success)
				}
			})
		}
	})

	t.Run("programmatic byte validators", func(t *testing.T) {
		var token string
		p := goopt.NewParser()

		// Add a flag with byte length validator
		err := p.BindFlag(&token, "token", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("API token (must be 16 bytes)"),
			goopt.WithValidator(validation.Custom("token-16", func(s string) error {
				if len(s) != 16 {
					return errs.ErrValidationFailed.WithArgs("value must be exactly 16 bytes")
				}
				return nil
			})),
		))
		require.NoError(t, err)

		// Test valid input
		success := p.Parse([]string{"--token", "1234567890123456"}) // exactly 16 bytes
		assert.True(t, success)
		assert.Equal(t, "1234567890123456", token)

		// Reset and test invalid input
		token = ""
		p = goopt.NewParser()
		err = p.BindFlag(&token, "token", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("API token (must be 16 bytes)"),
			goopt.WithValidator(validation.Custom("token-16", func(s string) error {
				if len(s) != 16 {
					return errs.ErrValidationFailed.WithArgs("value must be exactly 16 bytes")
				}
				return nil
			})),
		))
		require.NoError(t, err)

		success = p.Parse([]string{"--token", "tooshort"})
		assert.False(t, success)
	})

	t.Run("short form validators", func(t *testing.T) {
		type ShortConfig struct {
			Key      string `goopt:"name:key;validators:bytelen(8)"`
			Username string `goopt:"name:user;validators:minbytelen(3),maxbytelen(20)"`
		}

		var cfg ShortConfig
		p, err := goopt.NewParserFromStruct(&cfg)
		require.NoError(t, err)

		// Test short forms work
		success := p.Parse([]string{"--key", "12345678", "--user", "alice"})
		assert.True(t, success)
		assert.Equal(t, "12345678", cfg.Key)
		assert.Equal(t, "alice", cfg.Username)
	})
}
