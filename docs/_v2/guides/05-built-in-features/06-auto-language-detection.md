---
layout: default
title: Auto-Language Detection
parent: Built-in Features
nav_order: 6
version: v2
---

# Auto-Language Detection

`goopt` features sophisticated automatic language detection that seamlessly adapts your CLI to the user's preferred language. This built-in feature requires zero configuration and works out of the box.

## How It Works

When your application starts, goopt automatically detects the user's language preference from multiple sources in order of precedence:

1. **Command-line flags** (`--language` or custom flags)
2. **Environment variables** (`GOOPT_LANG` by default)
3. **System locale** (`LC_ALL`, `LC_MESSAGES`, `LANG` - **opt-in only**)

The detected language is then used for all help text, error messages, and even flag/command names if you've configured translations.

## Default Behavior

By default, goopt:
- Automatically registers `--language`, `--lang`, and `-l` flags
- Checks the `GOOPT_LANG` environment variable
- **Does NOT check system locale variables** (this is opt-in for predictability)

This design choice ensures that:
- Your CLI behaves predictably across different environments
- Users have explicit control via `GOOPT_LANG`
- System locale is only used when you explicitly enable it

```bash
# Set language via flag
$ myapp --language de --help
Verwendung: myapp [Optionen]

# Set language via environment
$ GOOPT_LANG=fr myapp --help
Utilisation: myapp [options]

# System locale is IGNORED by default
$ LANG=es_ES.UTF-8 myapp --help
Usage: myapp [options]  # Still in English!
```

## Enabling System Locale Detection

To make your CLI respect system locale settings, you must explicitly opt in:

```go
// Option 1: Enable during parser creation
parser, _ := goopt.NewParserWith(
    goopt.WithCheckSystemLocale(),
)

// Option 2: Enable after creation
parser.SetCheckSystemLocale(true)

// Now the detection order is:
// 1. Command-line flags (--language)
// 2. GOOPT_LANG environment variable
// 3. LC_ALL
// 4. LC_MESSAGES  
// 5. LANG
```

With system locale enabled:

```bash
# Now system locale is respected
$ LANG=es_ES.UTF-8 myapp --help
Uso: myapp [opciones]

# But GOOPT_LANG still takes precedence
$ LANG=es_ES.UTF-8 GOOPT_LANG=de myapp --help
Verwendung: myapp [Optionen]

# And command-line flags take highest precedence
$ LANG=es_ES.UTF-8 GOOPT_LANG=de myapp --language fr --help
Utilisation: myapp [options]
```

## Language Flag Processing

The language flag is special - it's processed during the pre-parse phase:

```bash
# Language is set before help is displayed
$ myapp --help --language de
# Shows help in German

# Order doesn't matter
$ myapp --language de --help
# Also shows help in German

# Works with equals syntax
$ myapp --language=de --help
$ myapp --lang=de --help
$ myapp -l de --help
```

## Configuration Options

### Customizing Language Flags

```go
// Change the language flag names
parser.SetLanguageFlags("locale", "loc", "L")

// Now users can use:
// --locale de
// --loc de  
// -L de

// Disable auto-language detection entirely
parser.SetAutoLanguage(false)
```

### Customizing Environment Variable

```go
// Use a custom environment variable
parser.SetLanguageEnvVar("MY_APP_LANG")

// Now detects from MY_APP_LANG instead of GOOPT_LANG

// Disable environment variable checking
parser.SetLanguageEnvVar("")
```

## Language Format Support

goopt accepts languages in multiple formats and normalizes them:

```bash
# All of these work:
--language en
--language en-US      # Regional variant
--language en_US      # Underscore format (normalized to en-US)
--language english    # Full language names (if parseable)

# Extracted from locale format:
LANG=en_US.UTF-8      # Extracts "en-US"
LC_ALL=de_DE@euro     # Extracts "de-DE"
```

## Integration with i18n System

Auto-language detection is the entry point to goopt's comprehensive internationalization features:

- **Automatic translation** of all help text and error messages
- **Runtime-switchable** command and flag names via namekey
- **RTL support** for Arabic, Hebrew, and other RTL languages
- **Locale-aware formatting** of numbers and dates

See the [Internationalization Guide]({{ site.baseurl }}/v2/guides/06-internationalization/) for complete details on leveraging these features.

## Common Patterns

### Application with Fixed Language

```go
// Always use German, ignore user preferences
parser.SetLanguage(language.German)
parser.SetAutoLanguage(false)
```

### Respecting System Settings

```go
// Check all possible sources including system locale
parser, _ := goopt.NewParserWith(
    goopt.WithCheckSystemLocale(),
    goopt.WithLanguageEnvVar("APP_LOCALE"),
)
```

### Custom Language Persistence

```go
// Save user's language preference
if parser.HasFlag("language") {
    lang := parser.MustGet("language") 
    saveUserPreference("language", lang)
}

// Load on next run
if savedLang := loadUserPreference("language"); savedLang != "" {
    parser.SetLanguage(language.MustParse(savedLang))
}
```

## Adding Language Support

To add support for additional languages beyond the built-in English, German, and French:

```go
import (
    "github.com/napalu/goopt/v2"
    esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
    jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
)

parser, _ := goopt.NewParserWith(
    goopt.WithSystemLocales(
        goopt.NewSystemLocale(esLocale.Tag, esLocale.SystemTranslations),
        goopt.NewSystemLocale(jaLocale.Tag, jaLocale.SystemTranslations),
    ),
)

// Now Spanish and Japanese are auto-detected:
// $ myapp --language es --help
// $ GOOPT_LANG=ja myapp --help
```

## Best Practices

1. **Consider your users** - Enable system locale detection if your users expect it
2. **Document the behavior** - Be clear about whether system locale is checked
3. **Provide `GOOPT_LANG`** - Always document this environment variable
4. **Test both modes** - Ensure your app works with and without system locale
5. **Set precedence** - Make sure users understand the precedence order

## Debugging

To see which language was detected and from where:

```go
detectedLang := parser.GetLanguage()
fmt.Printf("Using language: %s\n", detectedLang)

// Check if system locale is enabled
if parser.IsCheckSystemLocale() {
    fmt.Println("System locale detection is ENABLED")
}

// Check supported languages
supported := parser.GetSupportedLanguages()
fmt.Printf("Supported languages: %v\n", supported)
```

## Security Considerations

The opt-in nature of system locale detection provides security benefits:
- Prevents locale-based attacks in server environments
- Ensures consistent behavior in containers/CI
- Allows explicit control in production systems

Enable system locale detection only when:
- Building end-user CLI tools
- You trust the environment
- Users expect locale-aware behavior

Auto-language detection makes your CLI internationally friendly while maintaining security and predictability!