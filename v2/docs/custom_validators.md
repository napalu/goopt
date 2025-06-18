# Custom Validators Guide

## Overview

Validators in goopt are simple functions with the signature:
```go
type ValidatorFunc func(value string) error
```

This makes it easy to create custom validators for your specific needs.

## Current Limitations

**Important**: Custom validators can only be used programmatically via `WithValidator()` or `AddFlagValidators()`. They cannot be used in struct tags like `goopt:"validators:custom"` because struct tags only support built-in validators.


## Creating Custom Validators

### Simple Custom Validator

```go

// Custom validator that checks if a string is a valid hex color
func HexColor() validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, "#") {
            return errors.New("hex color must start with #")
        }
        
        // Remove # and check length
        hex := value[1:]
        if len(hex) != 3 && len(hex) != 6 {
            return errors.New("hex color must be 3 or 6 characters")
        }
        
        // Check if all characters are valid hex
        for _, r := range hex {
            if !((r >= '0' && r <= '9') || 
                 (r >= 'a' && r <= 'f') || 
                 (r >= 'A' && r <= 'F')) {
                return errors.New("invalid hex character")
            }
        }
        
        return nil
    }
}
```

### Parameterized Custom Validator

```go


// Custom validator that checks string prefix
func HasPrefix(prefix string) validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, prefix) {
            return fmt.Errorf("value must start with %q", prefix)
        }
        return nil
    }
}

// Custom validator for enum-like values with case sensitivity option
func Enum(caseSensitive bool, allowed ...string) validation.ValidatorFunc {
    return func(value string) error {
        for _, a := range allowed {
            if caseSensitive {
                if value == a {
                    return nil
                }
            } else {
                if strings.EqualFold(value, a) {
                    return nil
                }
            }
        }
        return fmt.Errorf("must be one of: %s", strings.Join(allowed, ", "))
    }
}
```


## Using Custom Validators

### Programmatically

```go
package main

import (
	"errors"
	"fmt"
	"strings"
	"strconv"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/validation"
)

// Custom validator for semantic versions
func SemanticVersion(allowPrerelease bool) validation.ValidatorFunc {
	return func(value string) error {
		// Remove optional 'v' prefix
		v := strings.TrimPrefix(value, "v")

		// Split by dash for prerelease
		parts := strings.Split(v, "-")
		if len(parts) > 2 {
			return errors.New("invalid semantic version format")
		}

		if len(parts) == 2 && !allowPrerelease {
			return errors.New("prerelease versions not allowed")
		}

		// Check main version
		versionParts := strings.Split(parts[0], ".")
		if len(versionParts) != 3 {
			return errors.New("version must have exactly 3 parts (major.minor.patch)")
		}

		for i, part := range versionParts {
			if _, err := strconv.Atoi(part); err != nil {
				return fmt.Errorf("version part %d must be a number", i+1)
			}
		}

		return nil
	}
}

// Custom validator that checks string prefix
func HasPrefix(prefix string) validation.ValidatorFunc {
	return func(value string) error {
		if !strings.HasPrefix(value, prefix) {
			return fmt.Errorf("value must start with %q", prefix)
		}
		return nil
	}
}

// Custom validator that checks if a string is a valid hex color
func HexColor() validation.ValidatorFunc {
	return func(value string) error {
		if !strings.HasPrefix(value, "#") {
			return errors.New("hex color must start with #")
		}

		// Remove # and check length
		hex := value[1:]
		if len(hex) != 3 && len(hex) != 6 {
			return errors.New("hex color must be 3 or 6 characters")
		}

		// Check if all characters are valid hex
		for _, r := range hex {
			if !((r >= '0' && r <= '9') ||
				(r >= 'a' && r <= 'f') ||
				(r >= 'A' && r <= 'F')) {
				return errors.New("invalid hex character")
			}
		}

		return nil
	}
}

func main() {
	parser, err := goopt.NewParserWith(
		goopt.WithFlag("color", goopt.NewArg(
			goopt.WithDescription("Hex color code"),
			goopt.WithValidator(HexColor()),
		)),

		goopt.WithFlag("env", goopt.NewArg(
			goopt.WithDescription("Environment name"),
			goopt.WithValidator(HasPrefix("env-")),
		)),

		goopt.WithFlag("version", goopt.NewArg(
			goopt.WithDescription("Semantic version"),
			goopt.WithValidator(SemanticVersion(true)),
		)),
	)
	
}
```

### Combining with Built-in Validators

```go
package main

import (
	"errors"
	"fmt"
	"strings"
	"strconv"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/validation"
)

// Custom validator that checks string prefix
func HasPrefix(prefix string) validation.ValidatorFunc {
	return func(value string) error {
		if !strings.HasPrefix(value, prefix) {
			return fmt.Errorf("value must start with %q", prefix)
		}
		return nil
	}
}

func main()  {
	parser := goopt.NewParser()
	
	// Combine custom validators with built-in ones
	parser.AddFlag("api-key", goopt.NewArg(
		goopt.WithValidators(
			validation.Length(32),              // Built-in
			HasPrefix("sk_"),                   // Custom
			validation.AlphaNumeric(),          // Built-in
		),
	))

	// Use composition functions
	parser.AddFlag("config", goopt.NewArg(
		goopt.WithValidator(
			validation.All(
				validation.MinLength(5),
				validation.MaxLength(50),
				validation.Not(HasPrefix("test_")),  // Negate custom validator
			),
		),
	)) 
}


```

## Making Custom Validators Translatable

For i18n support, use translatable errors and add the keys

```go
import "github.com/napalu/goopt/v2/errs"

// Define your error keys
var (
    ErrInvalidHexColor = errs.New("validation.invalid_hex_color")
    ErrMustStartWith   = errs.New("validation.must_start_with")
)

// Use in validator
func HexColorI18n() validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, "#") {
            return ErrInvalidHexColor.WithArgs(value)
        }
        // ... rest of validation
        return nil
    }
}
```

## Validator Factories

Create factories for common patterns:

```go
// Factory for database ID validators
func DatabaseID(table string) validation.ValidatorFunc {
    return validation.All(
        validation.Integer(),
        validation.Min(1),
        // Could add actual DB lookup here
        func(value string) error {
            // Simulate DB check
            id, _ := strconv.Atoi(value)
            if id > 1000000 {
                return fmt.Errorf("no %s with ID %s exists", table, value)
            }
            return nil
        },
    )
}

// Usage
parser.AddFlag("user-id", goopt.NewArg(
    goopt.WithValidator(DatabaseID("users")),
))
```

## Testing Custom Validators

```go
func TestHexColor(t *testing.T) {
    validator := HexColor()
    
    tests := []struct {
        input    string
        wantErr  bool
    }{
        {"#fff", false},
        {"#ffffff", false},
        {"#ABCDEF", false},
        {"ffffff", true},      // missing #
        {"#gggggg", true},     // invalid hex
        {"#ffff", true},       // wrong length
    }
    
    for _, tt := range tests {
        err := validator(tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("HexColor(%q) error = %v, wantErr %v", 
                tt.input, err, tt.wantErr)
        }
    }
}
```

## Advanced Patterns

### Async Validation

```go
// Validator that checks URL availability
func URLReachable(timeout time.Duration) validation.ValidatorFunc {
    return func(value string) error {
        client := &http.Client{Timeout: timeout}
        resp, err := client.Head(value)
        if err != nil {
            return fmt.Errorf("URL not reachable: %w", err)
        }
        if resp.StatusCode >= 400 {
            return fmt.Errorf("URL returned status %d", resp.StatusCode)
        }
        return nil
    }
}
```

### Stateful Validators

```go
// Validator that ensures uniqueness across invocations
func Unique() validation.ValidatorFunc {
    seen := make(map[string]bool)
    mu := &sync.Mutex{}
    
    return func(value string) error {
        mu.Lock()
        defer mu.Unlock()
        
        if seen[value] {
            return fmt.Errorf("duplicate value: %s", value)
        }
        seen[value] = true
        return nil
    }
}
```

### Context-Aware Validators

```go
// Create validators that depend on other flag values
type ContextValidator struct {
    parser *goopt.Parser
}

func (cv *ContextValidator) MatchesOtherFlag(flagName string) validation.ValidatorFunc {
    return func(value string) error {
        otherValue := cv.parser.GetOption(flagName)
        if otherValue == nil || value != otherValue.(string) {
            return fmt.Errorf("must match %s", flagName)
        }
        return nil
    }
}
```

## Best Practices

1. **Return Clear Error Messages**: Help users understand what went wrong
2. **Make Validators Reusable**: Design validators to be used across projects
3. **Use Composition**: Combine simple validators to create complex ones
4. **Test Thoroughly**: Validators are critical for data integrity
5. **Consider Performance**: Validators run on every parse
6. **Use Type-Safe Errors**: Leverage goopt's error system for i18n support

## Integration with Struct Tags

While custom validators can't be used directly in struct tags (only built-in validators are available there), you can add them programmatically after creating the parser:

```go

package main

import (
	"errors"
	"fmt"
	"strings"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/validation"

)

type Config struct {
    Color string `goopt:"name:color"`
    APIKey string `goopt:"name:api-key"`
}

// Custom validator that checks if a string is a valid hex color
func HexColor() validation.ValidatorFunc {
	return func(value string) error {
		if !strings.HasPrefix(value, "#") {
			return errors.New("hex color must start with #")
		}

		// Remove # and check length
		hex := value[1:]
		if len(hex) != 3 && len(hex) != 6 {
			return errors.New("hex color must be 3 or 6 characters")
		}

		// Check if all characters are valid hex
		for _, r := range hex {
			if !((r >= '0' && r <= '9') ||
				(r >= 'a' && r <= 'f') ||
				(r >= 'A' && r <= 'F')) {
				return errors.New("invalid hex character")
			}
		}

		return nil
	}
}

// Custom validator that checks string prefix
func HasPrefix(prefix string) validation.ValidatorFunc {
	return func(value string) error {
		if !strings.HasPrefix(value, prefix) {
			return fmt.Errorf("value must start with %q", prefix)
		}
		return nil
	}
}

func main() {
	config := &Config{}
	parser, _ := goopt.NewParserFromStruct(config)

	// Add custom validators after parser creation
	parser.AddFlagValidators("color", HexColor())
	parser.AddFlagValidators("api-key", HasPrefix("sk_"))
}

```

This approach gives you the convenience of struct tags with the flexibility of custom validators.