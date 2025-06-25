# goopt Locale Packages

This directory contains individual locale packages that can be imported to add language support to goopt.

## Built-in Languages

The following languages are automatically imported by the default system bundle:

- `en` - English (default)
- `de` - German
- `fr` - French

These provide a solid foundation for internationalization out of the box.

## Additional Languages

Import only the additional languages you need:

```go
import (
    "github.com/napalu/goopt/v2"
    esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
    arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"
)

// Add system locale support at parser creation
parser, err := goopt.NewParserFromStruct(cfg,
    goopt.WithSystemLocales(
        goopt.NewSystemLocale(esLocale.Tag, esLocale.SystemTranslations),
        goopt.NewSystemLocale(arLocale.Tag, arLocale.SystemTranslations),
    ),
)
```

## Benefits

- **Zero overhead**: Only imported languages are included in your binary
- **Type-safe**: Import errors caught at compile time
- **Explicit**: Clear which languages your app supports
- **Extensible**: Easy to add new languages
- **Consistent**: All languages (built-in and additional) use the same mechanism

## Available Languages

Built-in (imported by default):
- `en` - English
- `de` - German
- `fr` - French

Additional (import as needed):
- `ar` - Arabic (includes RTL support)
- `es` - Spanish
- `he` - Hebrew (includes RTL support)
- `hi` - Hindi
- `ja` - Japanese
- `zh` - Chinese

## Generating New Locale Packages

Use `goopt-i18n-gen` to generate locale packages from JSON files:

```bash
goopt-i18n-gen generate-locales \
    -i "all_locales/*.json" \
    -o "i18n/locales/"
```