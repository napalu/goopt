package validation_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/validation"
	"github.com/stretchr/testify/assert"
)

func TestCustomValidatorIntegration(t *testing.T) {
	t.Run("UUID validator in parser", func(t *testing.T) {
		p := goopt.NewParser()

		// Create a UUID validator
		uuidValidator := validation.Custom(func(value string) error {
			uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
			if !uuidRegex.MatchString(strings.ToLower(value)) {
				return errors.New("value must be a valid UUID")
			}
			return nil
		})

		err := p.AddFlag("id", &goopt.Argument{
			Description: "Unique identifier",
			Validators:  []validation.ValidatorFunc{uuidValidator},
		})
		assert.NoError(t, err)

		// Valid UUID
		args := []string{"--id", "550e8400-e29b-41d4-a716-446655440000"}
		success := p.Parse(args)
		assert.True(t, success)
		assert.Len(t, p.GetErrors(), 0)

		value, found := p.Get("id")
		assert.True(t, found)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", value)

		// Invalid UUID
		p.ClearErrors()
		args = []string{"--id", "not-a-uuid"}
		success = p.Parse(args)
		assert.False(t, success)
		errs := p.GetErrors()
		t.Logf("Got %d errors: %v", len(errs), errs)
		assert.GreaterOrEqual(t, len(errs), 1)
		assert.Contains(t, errs[0].Error(), "value must be a valid UUID")
	})

	t.Run("Password validator with custom errors", func(t *testing.T) {
		p := goopt.NewParser()

		// Create a password strength validator
		passwordValidator := validation.Custom(func(value string) error {
			if len(value) < 8 {
				return errors.New("password must be at least 8 characters")
			}

			var hasUpper, hasLower, hasDigit, hasSpecial bool
			for _, r := range value {
				switch {
				case r >= 'A' && r <= 'Z':
					hasUpper = true
				case r >= 'a' && r <= 'z':
					hasLower = true
				case r >= '0' && r <= '9':
					hasDigit = true
				case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", r):
					hasSpecial = true
				}
			}

			var errs []string
			if !hasUpper {
				errs = append(errs, "uppercase letter")
			}
			if !hasLower {
				errs = append(errs, "lowercase letter")
			}
			if !hasDigit {
				errs = append(errs, "digit")
			}
			if !hasSpecial {
				errs = append(errs, "special character")
			}

			if len(errs) > 0 {
				return errors.New("password must contain: " + strings.Join(errs, ", "))
			}

			return nil
		})

		err := p.AddFlag("password", &goopt.Argument{
			Description: "User password",
			Validators:  []validation.ValidatorFunc{passwordValidator},
		})
		assert.NoError(t, err)

		// Strong password
		args := []string{"--password", "MyStr0ng!Pass"}
		success := p.Parse(args)
		assert.True(t, success)

		// Weak password
		p.ClearErrors()
		args = []string{"--password", "weak"}
		success = p.Parse(args)
		assert.False(t, success)
		assert.Contains(t, p.GetErrors()[0].Error(), "must be at least 8 characters")

		// Missing components
		p.ClearErrors()
		args = []string{"--password", "weakpassword"}
		success = p.Parse(args)
		assert.False(t, success)
		assert.Contains(t, p.GetErrors()[0].Error(), "must contain:")
	})

	t.Run("Combining custom with built-in validators", func(t *testing.T) {
		p := goopt.NewParser()

		// Create a custom validator that checks for reserved usernames
		reservedUsernames := []string{"admin", "root", "system", "operator"}
		notReservedValidator := validation.Custom(func(value string) error {
			lower := strings.ToLower(value)
			for _, reserved := range reservedUsernames {
				if lower == reserved {
					return errors.New("username is reserved")
				}
			}
			return nil
		})

		err := p.AddFlag("username", &goopt.Argument{
			Description: "Username",
			Validators: []validation.ValidatorFunc{
				validation.MinLength(3),   // Built-in validator
				validation.MaxLength(20),  // Built-in validator
				validation.AlphaNumeric(), // Built-in validator
				notReservedValidator,      // Custom validator
			},
		})
		assert.NoError(t, err)

		// Valid username
		args := []string{"--username", "john123"}
		success := p.Parse(args)
		assert.True(t, success)

		// Too short
		p.ClearErrors()
		args = []string{"--username", "jo"}
		success = p.Parse(args)
		assert.False(t, success)

		// Reserved username
		p.ClearErrors()
		args = []string{"--username", "admin"}
		success = p.Parse(args)
		assert.False(t, success)
		assert.Contains(t, p.GetErrors()[0].Error(), "reserved")

		// Non-alphanumeric
		p.ClearErrors()
		args = []string{"--username", "john@123"}
		success = p.Parse(args)
		assert.False(t, success)
	})

	t.Run("Custom validator via struct tags", func(t *testing.T) {
		// Create an email domain validator
		emailDomainValidator := validation.Custom(func(value string) error {
			if !strings.Contains(value, "@") {
				return errors.New("invalid email format")
			}

			parts := strings.Split(value, "@")
			if len(parts) != 2 {
				return errors.New("invalid email format")
			}

			domain := parts[1]
			allowedDomains := []string{"company.com", "company.org"}

			for _, allowed := range allowedDomains {
				if domain == allowed {
					return nil
				}
			}

			return errors.New("email must be from company.com or company.org domain")
		})

		type Config struct {
			Email string `goopt:"validators:email"`
		}

		config := &Config{}
		p, err := goopt.NewParserFromStruct(config)
		assert.NoError(t, err)

		// Add custom validator to the email field
		err = p.AddFlagValidators("email", emailDomainValidator)
		assert.NoError(t, err)

		// Valid company email
		args := []string{"--email", "john@company.com"}
		success := p.Parse(args)
		assert.True(t, success)
		assert.Equal(t, "john@company.com", config.Email)

		// Invalid domain
		p.ClearErrors()
		args = []string{"--email", "john@gmail.com"}
		success = p.Parse(args)
		assert.False(t, success)
		assert.Contains(t, p.GetErrors()[0].Error(), "company.com or company.org")
	})
}
