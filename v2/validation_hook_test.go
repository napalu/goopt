package goopt

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/napalu/goopt/v2/validation"

	"github.com/napalu/goopt/v2/types"

	"github.com/stretchr/testify/assert"
)

func TestValidationHook(t *testing.T) {
	t.Run("simple validation hook", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("min", NewArg(
				WithType(types.Single),
				WithValidator(validation.Integer()),
			)),
			WithFlag("max", NewArg(
				WithType(types.Single),
				WithValidator(validation.Integer()),
			)),
			WithValidationHook(func(p *Parser) error {
				minStr, _ := p.Get("min")
				maxStr, _ := p.Get("max")

				if minStr != "" && maxStr != "" {
					minVal, _ := strconv.Atoi(minStr)
					maxVal, _ := strconv.Atoi(maxStr)

					if minVal > maxVal {
						return errors.New("min cannot be greater than max")
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid case
		success := parser.Parse([]string{"cmd", "--min", "1", "--max", "10"})
		assert.True(t, success)

		// Invalid case
		parser2, _ := NewParserWith(
			WithFlag("min", NewArg(WithType(types.Single), WithValidator(validation.Integer()))),
			WithFlag("max", NewArg(WithType(types.Single), WithValidator(validation.Integer()))),
			WithValidationHook(func(p *Parser) error {
				minVal, err := p.GetInt("min", 64)
				assert.NoError(t, err)
				maxVal, err := p.GetInt("max", 64)
				assert.NoError(t, err)
				if minVal > maxVal {
					return errors.New("min cannot be greater than max")
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--min", "10", "--max", "5"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "min cannot be greater than max")
	})

	t.Run("struct-based validation hook", func(t *testing.T) {
		type DateRange struct {
			StartDate string `goopt:"name:start-date"`
			EndDate   string `goopt:"name:end-date"`
			MaxDays   int    `goopt:"name:max-days;default:30"`
		}

		config := &DateRange{}
		parser, err := NewParserFromStruct(config,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*DateRange](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.StartDate != "" && cfg.EndDate != "" {
					start, err1 := time.Parse("2006-01-02", cfg.StartDate)
					end, err2 := time.Parse("2006-01-02", cfg.EndDate)

					if err1 != nil || err2 != nil {
						return errors.New("invalid date format, use YYYY-MM-DD")
					}

					if start.After(end) {
						return errors.New("start date must be before end date")
					}

					days := int(end.Sub(start).Hours() / 24)
					if days > cfg.MaxDays {
						return fmt.Errorf("date range exceeds maximum of %d days", cfg.MaxDays)
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid case
		success := parser.Parse([]string{"cmd", "--start-date", "2024-01-01", "--end-date", "2024-01-15"})
		assert.True(t, success)

		// Invalid: start after end
		config2 := &DateRange{}
		parser2, _ := NewParserFromStruct(config2,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*DateRange](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.StartDate != "" && cfg.EndDate != "" {
					start, _ := time.Parse("2006-01-02", cfg.StartDate)
					end, _ := time.Parse("2006-01-02", cfg.EndDate)

					if start.After(end) {
						return errors.New("start date must be before end date")
					}
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--start-date", "2024-01-15", "--end-date", "2024-01-01"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "start date must be before end date")
	})

	t.Run("conditional validation", func(t *testing.T) {
		type ServerConfig struct {
			Environment string `goopt:"name:env;validators:isoneof(dev,test,prod)"`
			Debug       bool   `goopt:"name:debug"`
			LogLevel    string `goopt:"name:log-level;default:info"`
		}

		config := &ServerConfig{}
		parser, err := NewParserFromStruct(config,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*ServerConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				// Production-specific rules
				if cfg.Environment == "prod" {
					if cfg.Debug {
						return errors.New("debug mode not allowed in production")
					}

					if cfg.LogLevel == "debug" {
						return errors.New("debug log level not allowed in production")
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid: dev with debug
		success := parser.Parse([]string{"cmd", "--env", "dev", "--debug"})
		assert.True(t, success)

		// Invalid: prod with debug
		config2 := &ServerConfig{}
		parser2, _ := NewParserFromStruct(config2,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*ServerConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.Environment == "prod" && cfg.Debug {
					return errors.New("debug mode not allowed in production")
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--env", "prod", "--debug"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "debug mode not allowed in production")
	})

	t.Run("mutex flags validation", func(t *testing.T) {
		parser, err := NewParserWith(
			WithFlag("config-file", NewArg(WithType(types.Single))),
			WithFlag("config-url", NewArg(WithType(types.Single))),
			WithFlag("config-inline", NewArg(WithType(types.Single))),
			WithValidationHook(func(p *Parser) error {
				count := 0
				if _, has := p.Get("config-file"); has {
					count++
				}
				if _, has := p.Get("config-url"); has {
					count++
				}
				if _, has := p.Get("config-inline"); has {
					count++
				}

				if count > 1 {
					return errors.New("only one config source can be specified")
				}

				if count == 0 {
					return errors.New("at least one config source must be specified")
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Valid: one source
		success := parser.Parse([]string{"cmd", "--config-file", "config.yaml"})
		assert.True(t, success)

		// Invalid: multiple sources
		parser2, _ := NewParserWith(
			WithFlag("config-file", NewArg(WithType(types.Single))),
			WithFlag("config-url", NewArg(WithType(types.Single))),
			WithValidationHook(func(p *Parser) error {
				count := 0
				if _, has := p.Get("config-file"); has {
					count++
				}
				if _, has := p.Get("config-url"); has {
					count++
				}

				if count > 1 {
					return errors.New("only one config source can be specified")
				}

				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--config-file", "config.yaml", "--config-url", "http://example.com/config"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "only one config source can be specified")
	})

	t.Run("validation hook runs after field validation", func(t *testing.T) {
		validationHookCalled := false

		parser, err := NewParserWith(
			WithFlag("email", NewArg(
				WithType(types.Single),
				WithValidator(validation.Email()),
			)),
			WithValidationHook(func(p *Parser) error {
				validationHookCalled = true
				return nil
			}),
		)
		assert.NoError(t, err)

		// Invalid email - field validation should fail first
		success := parser.Parse([]string{"cmd", "--email", "not-an-email"})
		assert.False(t, success)
		assert.False(t, validationHookCalled, "validation hook should not be called when field validation fails")

		// Valid email - hook should be called
		validationHookCalled = false
		parser2, _ := NewParserWith(
			WithFlag("email", NewArg(
				WithType(types.Single),
				WithValidator(validation.Email()),
			)),
			WithValidationHook(func(p *Parser) error {
				validationHookCalled = true
				return nil
			}),
		)
		success = parser2.Parse([]string{"cmd", "--email", "test@example.com"})
		assert.True(t, success)
		assert.True(t, validationHookCalled, "validation hook should be called when field validation passes")
	})

	t.Run("mixed struct and dynamic flags", func(t *testing.T) {
		type BaseConfig struct {
			Mode string `goopt:"name:mode;validators:isoneof(dev,test,prod)"`
		}

		base := &BaseConfig{}
		parser, err := NewParserFromStruct(base,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*BaseConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				// Check dynamic flags based on mode
				if cfg.Mode == "prod" {
					apiKey, hasKey := p.Get("api-key")
					if !hasKey || apiKey == "" {
						return errors.New("--api-key required in production mode")
					}
				}

				return nil
			}),
		)
		assert.NoError(t, err)

		// Add dynamic flag
		err = parser.AddFlag("api-key", NewArg(
			WithDescription("API key for production"),
			WithType(types.Single),
		))
		assert.NoError(t, err)

		// Valid: dev mode without api-key
		success := parser.Parse([]string{"cmd", "--mode", "dev"})
		assert.True(t, success)

		// Invalid: prod mode without api-key
		base2 := &BaseConfig{}
		parser2, _ := NewParserFromStruct(base2,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*BaseConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.Mode == "prod" {
					apiKey, hasKey := p.Get("api-key")
					if !hasKey || apiKey == "" {
						return errors.New("--api-key required in production mode")
					}
				}

				return nil
			}),
		)
		_ = parser2.AddFlag("api-key", NewArg(WithType(types.Single)))

		success = parser2.Parse([]string{"cmd", "--mode", "prod"})
		assert.False(t, success)
		assert.Contains(t, parser2.GetErrors()[0].Error(), "--api-key required in production mode")

		// Valid: prod mode with api-key
		base3 := &BaseConfig{}
		parser3, _ := NewParserFromStruct(base3,
			WithValidationHook(func(p *Parser) error {
				cfg, ok := GetStructCtxAs[*BaseConfig](p)
				if !ok {
					return errors.New("invalid config type")
				}

				if cfg.Mode == "prod" {
					apiKey, hasKey := p.Get("api-key")
					if !hasKey || apiKey == "" {
						return errors.New("--api-key required in production mode")
					}
				}

				return nil
			}),
		)
		_ = parser3.AddFlag("api-key", NewArg(WithType(types.Single)))

		success = parser3.Parse([]string{"cmd", "--mode", "prod", "--api-key", "secret123"})
		assert.True(t, success)
	})
}
