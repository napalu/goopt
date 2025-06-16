# Validation Guide

The goopt v2 validation system provides comprehensive input validation through a flexible validator system that can be used both programmatically and via struct tags.

## Overview

goopt provides a powerful validation system based on composable validator functions. Key features include:

- **Built-in Validators** - Common validation patterns like email, URL, numeric ranges
- **Composable Validators** - Combine validators with AND/OR/NOT logic
- **Custom Validators** - Easy to create your own validation functions
- **Struct Tag Support** - Use validators directly in struct tags (built-in validators only)
- **Programmatic Support** - Add validators dynamically via code
- **Translatable Errors** - All validation errors support internationalization

## Validator Syntax

### New Parentheses Syntax (Required)

As of this version, validators use a consistent parentheses-based syntax. The old colon-separated syntax is no longer supported.

```go
// Old syntax (no longer supported)
`goopt:"validators:minlength:5,maxlength:20"`

// New syntax (required)
`goopt:"validators:minlength(5),maxlength(20)"`
```

#### Rules:
1. **No-argument validators** can omit parentheses:
   - `email` or `email()`
   - `integer` or `integer()`
   
2. **Validators with arguments** MUST use parentheses:
   - `minlength(5)` - NOT `minlength:5`
   - `range(1,100)` - NOT `range:1:100`

3. **Multiple validators** are comma-separated:
   - `validators:email,minlength(10)`
   - `validators:integer,range(1,100)`

For a comprehensive guide on the new syntax, migration instructions, and complete examples, see the [Validator Syntax Guide](/VALIDATOR_SYNTAX_GUIDE.md).

## Using Validators

### In Struct Tags

```go
type Config struct {
    // Simple validators
    Email      string `goopt:"name:email;validators:email"`
    Count      int    `goopt:"name:count;validators:integer,min(0)"`
    
    // Multiple validators
    Username   string `goopt:"name:username;validators:minlength(3),maxlength(20),alphanumeric"`
    
    // Complex validators with arguments
    Password   string `goopt:"name:password;validators:minlength(8),regex([A-Z]),regex([0-9])"`
    Port       int    `goopt:"name:port;validators:range(1024,65535)"`
    
    // Compositional validators
    ID         string `goopt:"name:id;validators:oneof(email,regex(^EMP-[0-9]{6}$))"`
    
    // Regex with description
    License    string `goopt:"name:license;validators:regex(pattern:^[A-Z]{3}-[0-9]{4}$,desc:License format XXX-1234)"`
}
```

### Programmatically

```go
// During flag creation
parser, err := goopt.NewParserWith(
    goopt.WithFlag("email", goopt.NewArg(
        goopt.WithType(types.Single),
        goopt.WithValidator(validation.Email()),
    )),
)

// After flag creation
parser.AddFlagValidators("port", validation.Port())

// Multiple validators
parser.AddFlagValidators("username",
    validation.MinLength(3),
    validation.MaxLength(20),
    validation.AlphaNumeric(),
)
```

## Built-in Validators

### Type Validators

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| Integer | `integer` or `int` | Validates integer values |
| Float | `float` or `number` | Validates floating-point numbers |
| Boolean | `boolean` or `bool` | Validates boolean values |

### String Validators

#### Character Length (Unicode)

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| MinLength(n) | `minlength(n)` or `minlen(n)` | Minimum character count |
| MaxLength(n) | `maxlength(n)` or `maxlen(n)` | Maximum character count |
| Length(n) | `length(n)` or `len(n)` | Exact character count |

**Note:** These count Unicode characters. "café" = 4 characters (not 5 bytes).

#### Byte Length

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| MinByteLength(n) | `minbytelength(n)` or `minbytelen(n)` | Minimum byte count |
| MaxByteLength(n) | `maxbytelength(n)` or `maxbytelen(n)` | Maximum byte count |
| ByteLength(n) | `bytelength(n)` or `bytelen(n)` | Exact byte count |

**Note:** These count UTF-8 bytes. "café" = 5 bytes (not 4 characters).

#### Other String Validators

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| AlphaNumeric | `alphanumeric` or `alnum` | Letters and numbers only |
| Identifier | `identifier` or `id` | Valid identifier format |
| NoWhitespace | `nowhitespace` or `nospace` | No whitespace characters |

### Pattern Validators

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| Regex(pattern) | `regex(pattern)` | Match regex pattern |
| Regex(pattern, desc) | `regex(pattern:xxx,desc:xxx)` | Pattern with description |

### Network Validators

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| Email | `email` | RFC-compliant email |
| URL | `url` | Any valid URL |
| URL(schemes...) | `url(http,https)` | Specific URL schemes |
| Hostname | `hostname` or `host` | Valid DNS hostname |
| IP | `ip` or `ipaddress` | IPv4 or IPv6 address |
| Port | `port` | Port number (1-65535) |

### Numeric Validators

| Validator | Struct Tag          | Description           |
|-----------|---------------------|-----------------------|
| Range(min, max) | `range(min,max)`    | Inclusive float range |
| IntRange(min, max) | `intrange(min,max)` | Inclusive integer range |
| Min(n) | `min(n,m)`          | Minimum value         |
| Max(n) | `max(n,m)`          | Maximum value         |

### Collection Validators

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| IsOneOf(values...) | `isoneof(val1,val2,val3)` | Must be one of values |
| IsNotOneOf(values...) | `isnotoneof(val1,val2,val3)` | Must not be one of values |
| FileExtension(exts...) | `fileext(.txt,.md)` or `extension(.txt,.md)` | Allowed file extensions |

### Compositional Validators

| Validator | Struct Tag | Description |
|-----------|------------|-------------|
| All(validators...) | `all(...)` | All validators must pass (AND) |
| OneOf(validators...) | `oneof(...)` | At least one must pass (OR) |
| Not(validator) | `not(...)` | Negates a validator |

## Regex Validator Formats

The regex validator supports multiple formats:

```go
// 1. Simple pattern (pattern used as description)
`validators:regex(^[A-Z]+$)`

// 2. Pattern with explicit description
`validators:regex(pattern:^[A-Z]+$,desc:Uppercase letters only)`

// 3. JSON-like format (backward compatibility)
`validators:regex({pattern:^[A-Z]+$,desc:Uppercase letters only})`
```

**Note:** Go's regex engine doesn't support lookahead assertions `(?=...)`. Use multiple simple regex validators with `all()` instead.

## Compositional Validators

### All (AND Logic)

All validators must pass:

```go
// Struct tag
`validators:all(minlength(8),maxlength(20),regex([A-Z]),regex([0-9]),regex([!@#$%]))`

// Programmatic
validation.All(
    validation.MinLength(8),
    validation.MaxLength(20),
    validation.Regex("[A-Z]", "Must have uppercase"),
    validation.Regex("[0-9]", "Must have digit"),
    validation.Regex("[!@#$%]", "Must have special char"),
)
```

### OneOf (OR Logic)

At least one validator must pass:

```go
// Struct tag
`validators:oneof(email,regex(^[0-9]{10}$))`

// Programmatic
validation.OneOf(
    validation.Email(),
    validation.Regex(`^\d{10}$`, "10-digit phone"),
)
```

### Not (Negation)

Inverts a validator:

```go
// Struct tag
`validators:not(isoneof(admin,root,system))`

// Programmatic
validation.Not(validation.IsOneOf("admin", "root", "system"))
```

## Custom Validators

Custom validators can only be used programmatically (not in struct tags):

```go
// Simple custom validator
func HexColor() validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, "#") {
            return errors.New("hex color must start with #")
        }
        // ... validation logic
        return nil
    }
}

// Parameterized custom validator
func StartsWith(prefix string) validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, prefix) {
            return fmt.Errorf("must start with %q", prefix)
        }
        return nil
    }
}

// Use custom validators
parser.AddFlag("color", goopt.NewArg(
    goopt.WithValidator(HexColor()),
))

parser.AddFlag("env", goopt.NewArg(
    goopt.WithValidator(StartsWith("prod-")),
))
```

### Making Custom Validators Translatable

```go
import "github.com/napalu/goopt/v2/errs"

// Define error keys
var ErrInvalidHexColor = errs.New("validation.invalid_hex_color")

// Use in validator
func HexColorI18n() validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, "#") {
            return ErrInvalidHexColor.WithArgs(value)
        }
        return nil
    }
}
```

## Validation with Filters

Combine filters and validators for preprocessing:

```go
// Struct tag
type Config struct {
    Name string `goopt:"name:name;filters:trim,lower;validators:minlength(3)"`
}

// Programmatic
parser.AddFlag("name", goopt.NewArg(
    goopt.WithPreValidationFilter(strings.TrimSpace),
    goopt.WithPostValidationFilter(strings.ToLower),
    goopt.WithValidator(validation.MinLength(3)),
))

// Flow: "  JOHN  " → "JOHN" → "john" → validated
```

## Complex Examples

### Password Validation

```go
// Struct tag (using multiple validators)
type Config struct {
    Password string `goopt:"validators:all(minlength(12),maxlength(128),regex([a-z]),regex([A-Z]),regex([0-9]),regex([@$!%*?&]))"`
}

// Programmatic (more readable)
passwordValidator := validation.All(
    validation.MinLength(12),
    validation.MaxLength(128),
    validation.Regex("[a-z]", "Must have lowercase"),
    validation.Regex("[A-Z]", "Must have uppercase"),
    validation.Regex("[0-9]", "Must have digit"),
    validation.Regex("[@$!%*?&]", "Must have special character"),
)
```

### Multiple ID Formats

```go
// Accept employee ID, user ID, or UUID
type Config struct {
    ID string `goopt:"validators:oneof(regex(^EMP-[0-9]{6}$),regex(^USR-[0-9]{8}$),regex(^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$))"`
}
```

### Excluded Values

```go
// Username that's not a reserved word
type Config struct {
    Username string `goopt:"validators:all(alphanumeric,minlength(3),maxlength(20),not(isoneof(admin,root,system,guest)))"`
}
```

## Error Handling

Validation errors:
- Are added to the parser's error list
- Prevent successful parsing
- Include descriptive messages
- Support internationalization

```go
if !parser.Parse(os.Args) {
    // Validation errors are included
    for _, err := range parser.GetErrors() {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    }
    os.Exit(1)
}
```

## Best Practices

### 1. Use Built-in Validators When Possible

Built-in validators are optimized and provide consistent error messages.

### 2. Order Matters

Place more specific validators first:
```go
`validators:integer,range(1,100)` // Check type first, then range
```

### 3. Provide Clear Descriptions

For regex validators, always provide descriptions:
```go
`validators:regex(pattern:^[A-Z]{2}-[0-9]{4}$,desc:Format: XX-1234)`
```

### 4. Combine Related Validations

Group related validators for maintainability:
```go
// Define once, use many times
var usernameValidation = validation.All(
    validation.MinLength(3),
    validation.MaxLength(20),
    validation.AlphaNumeric(),
    validation.Not(validation.IsOneOf("admin", "root")),
)
```

### 5. Test Your Validators

Always test custom validators thoroughly:
```go
func TestHexColor(t *testing.T) {
    validator := HexColor()
    
    // Test valid cases
    assert.NoError(t, validator("#ffffff"))
    assert.NoError(t, validator("#000"))
    
    // Test invalid cases
    assert.Error(t, validator("ffffff"))  // Missing #
    assert.Error(t, validator("#gggggg")) // Invalid hex
}
```

### Character vs Byte Length Example

Here's when to use character vs byte validators:

```go
type DatabaseConfig struct {
    // Username: human-readable, limit by characters
    Username string `goopt:"name:username;validators:minlength(3),maxlength(20)"`
    
    // Password: may need specific byte limit for storage/encryption
    Password string `goopt:"name:password;validators:minbytelength(8),maxbytelength(72)"` // bcrypt limit
    
    // Display name: UI constraint, use characters
    DisplayName string `goopt:"name:display-name;validators:maxlength(50)"`
    
    // API key: exact byte requirement for format
    APIKey string `goopt:"name:api-key;validators:bytelength(32)"` // 32-byte requirement
    
    // Bio: database storage limit in bytes
    Bio string `goopt:"name:bio;validators:maxbytelength(65535)"` // MySQL TEXT limit
}
```

**Use character validators when:**
- Displaying in UI with character limits
- User-facing constraints
- Readability is the concern

**Use byte validators when:**
- Database storage limits (VARCHAR byte limits)
- API requirements (exact byte lengths)
- Encryption/hashing constraints (bcrypt's 72-byte limit)
- Network protocols with byte limits

## Complete Example

```go
package main

import (
    "fmt"
    "os"
    "strings"
    
    "github.com/napalu/goopt/v2"
    "github.com/napalu/goopt/v2/types"
    "github.com/napalu/goopt/v2/validation"
)

type ServerConfig struct {
    Host       string `goopt:"name:host;desc:Server hostname;validators:hostname;default:localhost"`
    Port       int    `goopt:"name:port;short:p;desc:Server port;validators:range(1024,65535);default:8080"`
    AdminEmail string `goopt:"name:admin-email;desc:Admin email;validators:email;required:true"`
    LogLevel   string `goopt:"name:log-level;desc:Log level;validators:isoneof(debug,info,warn,error);default:info"`
}

func main() {
    config := &ServerConfig{}
    parser, err := goopt.NewParserFromStruct(config)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    // Add custom validator for API key
    parser.AddFlag("api-key", goopt.NewArg(
        goopt.WithDescription("API key (32 hex characters)"),
        goopt.WithRequired(true),
        goopt.WithValidator(validation.All(
            validation.Length(32),
            validation.Regex("^[a-fA-F0-9]+$", "Must be hexadecimal"),
        )),
    ))
    
    // Parse and validate
    if !parser.Parse(os.Args) {
        parser.PrintErrors(os.Stderr)
        parser.PrintHelp(os.Stderr)
        os.Exit(1)
    }
    
    fmt.Printf("Server configured: %s:%d\n", config.Host, config.Port)
    fmt.Printf("Admin email: %s\n", config.AdminEmail)
    fmt.Printf("Log level: %s\n", config.LogLevel)
}
```

## Limitations and Future Enhancements

### Current Limitations

1. **Custom validators cannot be used in struct tags** - Only built-in validators are available in struct tags
2. **No validator registry** - Custom validators must be added programmatically
3. **No conditional validation** - Cannot validate based on other flag values

### Potential Future Enhancements

1. **Validator Registry** - Register custom validators for use in struct tags
2. **Conditional Validation** - Validate based on other flag values
3. **Async Validators** - Support for validators that need to make network calls
4. **More Built-in Validators** - UUID, JWT, credit card, etc.