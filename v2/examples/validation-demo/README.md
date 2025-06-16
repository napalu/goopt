# Validation Demo

This example demonstrates goopt v2's validation hooks feature for robust input validation.

## Features Demonstrated

1. **Built-in Validators**
   - Email validation
   - URL validation with scheme restrictions
   - Port number validation (1-65535)
   - IP address validation (IPv4 and IPv6)
   - Hostname validation
   - File extension validation
   - String length constraints
   - Numeric range validation
   - Pattern matching with regex

2. **Combining Validators**
   - Multiple validators on a single flag
   - `All()` - all validators must pass
   - `Any()` - at least one validator must pass

3. **Custom Validators**
   - Create your own validation logic
   - Integrate with existing validators

## Running the Demo

### Basic Usage
```bash
go run main.go --email user@example.com --username johndoe
```

### With All Options
```bash
go run main.go \
  --email user@example.com \
  --username johndoe \
  --port 8080 \
  --webhook https://example.com/hook \
  --age 25 \
  --config app.json \
  --host api.example.com \
  --bind 127.0.0.1 \
  --priority high
```

### Validation Examples

#### Email Validation
```bash
# Valid
go run main.go --email user@example.com --username test

# Invalid
go run main.go --email "not-an-email" --username test
# Error: invalid email format: not-an-email
```

#### Port Validation
```bash
# Valid
go run main.go --email test@example.com --username test --port 8080

# Invalid - out of range
go run main.go --email test@example.com --username test --port 70000
# Error: value must be between 1 and 65535

# Invalid - not a number
go run main.go --email test@example.com --username test --port abc
# Error: value must be a number
```

#### Username Validation
```bash
# Valid
go run main.go --email test@example.com --username john123

# Invalid - too short
go run main.go --email test@example.com --username ab
# Error: value must be at least 3 characters long

# Invalid - special characters
go run main.go --email test@example.com --username "john-doe"
# Error: value must contain only letters and numbers
```

#### URL Validation
```bash
# Valid
go run main.go --email test@example.com --username test \
  --webhook https://example.com/webhook

# Invalid - wrong scheme
go run main.go --email test@example.com --username test \
  --webhook ftp://example.com/file
# Error: URL scheme must be one of: http, https
```

#### Complex Password Validation
```bash
# The password flag uses secure input and validates:
# - Minimum 8 characters
# - At least one uppercase letter
# - At least one lowercase letter
# - At least one digit

go run main.go --email test@example.com --username test --password
# Enter password: [hidden input]

# Valid: SecurePass123
# Invalid: weakpass (no uppercase or digits)
# Invalid: SHORT1 (too short)
```

#### File Extension Validation
```bash
# Valid
go run main.go --email test@example.com --username test \
  --config app.json

# Invalid extension
go run main.go --email test@example.com --username test \
  --config app.xml
# Error: file must have one of these extensions: .json, .yaml, .yml, .toml
```

## Validator Types

### Basic Validators
- `Required()` - Ensures non-empty value
- `Email()` - Validates email format
- `URL(schemes...)` - Validates URL with optional scheme restrictions
- `Hostname()` - Validates hostname format
- `IP()` - Validates IPv4 or IPv6 address
- `Port()` - Validates port number (1-65535)

### String Validators
- `MinLength(n)` - Minimum string length
- `MaxLength(n)` - Maximum string length
- `Length(n)` - Exact string length
- `Regex(pattern)` - Match regular expression
- `AlphaNumeric()` - Only letters and numbers
- `Identifier()` - Valid identifier (starts with letter)
- `NoWhitespace()` - No whitespace characters

### Numeric Validators
- `Integer()` - Valid integer
- `Float()` - Valid float
- `Boolean()` - Valid boolean
- `Range(min, max)` - Numeric range
- `Min(n)` - Minimum value
- `Max(n)` - Maximum value

### Choice Validators
- `OneOf(values...)` - Value must be in list
- `NotIn(values...)` - Value must not be in list

### File Validators
- `FileExtension(exts...)` - Valid file extensions

### Combining Validators
- `All(validators...)` - All must pass
- `Any(validators...)` - At least one must pass
- `Custom(func)` - Custom validation function

## Creating Custom Validators

```go
// Custom validator example
evenNumber := validation.Custom(func(value string) error {
    num, err := strconv.Atoi(value)
    if err != nil {
        return errors.New("value must be a number")
    }
    if num%2 != 0 {
        return errors.New("value must be even")
    }
    return nil
})

// Use it
parser.AddFlag("count", goopt.NewArg(
    goopt.WithType(types.Single),
    goopt.WithValidator(evenNumber),
))
```

## Best Practices

1. **Validate Early** - Add validators when defining flags
2. **Combine Validators** - Use `All()` for multiple requirements
3. **User-Friendly Messages** - Validators provide clear error messages
4. **Type Safety** - Validate data types (Integer, Float, Boolean)
5. **Security** - Validate sensitive inputs like passwords thoroughly
6. **Performance** - Validators run after filters, on final values

## Integration with Other Features

Validators work seamlessly with:
- **Filters** - Run after pre/post filters
- **Required Flags** - Complement required validation
- **Secure Flags** - Validate passwords and secrets
- **Default Values** - Defaults bypass validation
- **Positional Args** - Validate positional arguments
- **Struct Tags** - Future: `goopt:"validate:email"`