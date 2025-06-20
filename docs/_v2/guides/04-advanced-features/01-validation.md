---
layout: default
title: Validation
parent: Advanced Features
nav_order: 1
version: v2
---

# Validation Guide

`goopt` provides a powerful and flexible validation system that allows you to ensure the correctness of user input. It's built on a foundation of composable validator functions that can be used both programmatically and directly within struct tags.

### Validator Syntax: Parentheses are required for validators with arguments

The validation system uses a consistent, parenthesis-based syntax for validators with arguments. 

```go

// required syntax
`goopt:"validators:minlength(5),maxlength(20)"`
```

**Key Rules:**
1.  Validators with **no arguments** can omit parentheses (e.g., `email` or `email()`).
2.  Validators **with arguments** *must* use parentheses (e.g., `minlength(5)`).
3.  Multiple validators are **comma-separated** (e.g., `validators:email,minlength(10)`).

## Using Built-in Validators

You can apply validators directly in your struct tags or programmatically when defining flags.

#### In Struct Tags
```go
type Config struct {
    // Simple validators
    Email      string `goopt:"name:email;validators:email"`
    Port       int    `goopt:"name:port;validators:port"`
    
    // Multiple validators (all must pass)
    Username   string `goopt:"name:username;validators:minlength(3),maxlength(20),alphanumeric"`
    
    // Compositional validators
    ID         string `goopt:"name:id;validators:oneof(email,regex(^EMP-[0-9]{6}$))"`
}
```

#### Programmatically
```go
import "github.com/napalu/goopt/v2/validation"

// During parser creation
parser, err := goopt.NewParserWith(
    goopt.WithFlag("email", goopt.NewArg(
        goopt.WithValidator(validation.Email()),
    )),
)

// Or add validators after the fact
parser.AddFlagValidators("port", validation.Port())
```

## Available Built-in Validators

Here is a reference of the most common built-in validators.

### Type Validators

| Validator | Struct Tag | Description |
|---|---|---|
| `Integer()` | `integer` or `int` | Validates integer values. |
| `Float()` | `float` or `number` | Validates floating-point numbers. |
| `Boolean()` | `boolean` or `bool` | Validates boolean values. |

### String Validators

| Validator | Struct Tag | Description |
|---|---|---|
| `MinLength(n)` | `minlength(n)` | Minimum number of Unicode characters. |
| `MaxLength(n)` | `maxlength(n)` | Maximum number of Unicode characters. |
| `ByteLength(n)`| `bytelength(n)`| Exact length in bytes (for UTF-8). |
| `AlphaNumeric()`| `alphanumeric`| Contains only letters and numbers. |
| `Identifier()`| `identifier`| A valid Go-style identifier. |
| `NoWhitespace()`| `nowhitespace`| Contains no whitespace characters. |

### Pattern & Network Validators

| Validator | Struct Tag | Description                                                                                                      |
|---|---|------------------------------------------------------------------------------------------------------------------|
| `Regex(pattern, desc)`| `regex(pattern:...,desc:...)` | Matches a regular expression. `desc` when supplied can be a message key (for translato^tion) or a literal string |
| `Email()` | `email` | A valid email address format.                                                                                    |
| `URL(schemes...)` | `url(http,https)` | A valid URL, optionally restricted to schemes.                                                                   |
| `Hostname()` | `hostname` | A valid DNS hostname.                                                                                            |
| `IP()` | `ip` | An IPv4 or IPv6 address.                                                                                         |
| `Port()` | `port` | A valid port number (1-65535).                                                                                   |

### Numeric Validators

| Validator            | Struct Tag          | Description                         |
|----------------------|---------------------|-------------------------------------|
| `Range(min, max)`    | `range(min,max)`    | An inclusive float numeric range.   |
| `IntRange(min, max)` | `intrange(min,max)` | An inclusive integer numeric range. |
| `Min(n)`             | `min(n)`            | A minimum numeric value.            |
| `Max(n)`             | `max(n)`            | A maximum numeric value.            |

### Collection Validators

| Validator | Struct Tag | Description |
|---|---|---|
| `IsOneOf(values...)`| `isoneof(val1,val2)` | Value must be one of the given strings. |
| `FileExtension(exts...)`| `fileext(.txt,.md)` | File path must have one of the extensions. |

### Compositional Validators

| Validator | Struct Tag | Description |
|---|---|---|
| `All(validators...)`| `all(...)` | All nested validators must pass (AND logic). |
| `OneOf(validators...)`| `oneof(...)` | At least one nested validator must pass (OR logic). |
| `Not(validator)`| `not(...)` | Negates a validator. |

---

## Creating Custom Validators

For domain-specific rules, you can easily create your own validators.

**Important:** Custom validators can currently only be used programmatically via `WithValidator()` or `AddFlagValidators()`. They cannot be referenced by name from a struct tag.

### Simple Custom Validator
A validator is just a function that takes a string and returns an error.

```go
import "github.com/napalu/goopt/v2/validation"

// HexColor validates that a string is a valid hex color code (e.g., "#ff0000").
func HexColor() validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, "#") || (len(value) != 4 && len(value) != 7) {
            return errors.New("must be a valid hex color like #rgb or #rrggbb")
        }
        // ... more detailed validation ...
        return nil
    }
}
```

### Using Custom Validators
You can then add your custom validator to any flag.

```go
parser.AddFlag("color", goopt.NewArg(
    goopt.WithDescription("A hex color code for the background"),
    goopt.WithValidator(HexColor()),
))
```

### Combining with Built-in Validators
Custom validators are fully composable with built-in ones.

```go
// A validator that requires a non-reserved username.
notReserved := validation.Not(validation.IsOneOf("admin", "root", "system"))

parser.AddFlag("username", goopt.NewArg(
    goopt.WithValidators(
        validation.MinLength(4),    // Built-in
        validation.AlphaNumeric(),  // Built-in
        notReserved,                // Custom composed
    ),
))
```

### Making Custom Validators Translatable
To support internationalization, your custom validators can return translatable errors from the `errs` package.

```go
import "github.com/napalu/goopt/v2/errs"

// Define your translatable error key.
var ErrInvalidHexColor = i18n.NewError("validation.invalid_hex_color")

func HexColorI18n() validation.ValidatorFunc {
    return func(value string) error {
        if !strings.HasPrefix(value, "#") {
            // Return a translatable error instead of a standard one.
            return ErrInvalidHexColor.WithArgs(value)
        }
        return nil
    }
}
```
Now, you can provide a translation for `validation.invalid_hex_color` in your i18n JSON files, 
see [Internationalization]({{ site.baseurl }}/v2/guides/06-internationalization/index/) for details.