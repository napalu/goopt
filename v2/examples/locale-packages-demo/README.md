# Locale Packages Demo

This example demonstrates how to use goopt's locale package system to add support for additional languages beyond the built-in English, German, and French.

## The Pay-for-What-You-Use Model

Goopt provides a sophisticated internationalization system with minimal overhead:

- **Built-in languages**: English, German, and French are included by default (~50KB total)
- **Additional languages**: Import only what you need via locale packages
- **Zero overhead**: Unused languages don't bloat your binary
- **Type-safe**: Compile-time verification of locale availability

## Using Locale Packages

```go
import (
    // Import only the languages you need
    arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"
    esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
    heLocale "github.com/napalu/goopt/v2/i18n/locales/he"
    jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
)

// Register them with your parser
parser, err := goopt.NewParserFromStruct(cfg,
    goopt.WithSystemLocales(
        goopt.NewSystemLocale(arLocale.Tag, arLocale.SystemTranslations),
        goopt.NewSystemLocale(esLocale.Tag, esLocale.SystemTranslations),
        goopt.NewSystemLocale(heLocale.Tag, heLocale.SystemTranslations),
        goopt.NewSystemLocale(jaLocale.Tag, jaLocale.SystemTranslations),
    ),
)
```

## Available Languages

Currently available locale packages:
- `ar` - Arabic (RTL)
- `es` - Spanish
- `he` - Hebrew (RTL)
- `ja` - Japanese

## Running the Demo

```bash
# Default (English)
go run main.go --help

# Spanish
go run main.go -l es --help

# Arabic (with RTL support)
go run main.go -l ar --help

# Japanese
go run main.go -l ja --help

# Hebrew (with RTL support)
go run main.go -l he --help

# Process command with validation errors in different languages
go run main.go -l es process -f test.txt --port 99999
```

## Creating New Locale Packages

To add support for a new language:

1. Create a stub locale file with a few translations:
   ```bash
   cat > i18n/all_locales/pt.json << 'EOF'
   {
     "goopt.msg.optional": "opcional",
     "goopt.msg.commands": "Comandos"
   }
   EOF
   ```

2. Sync with the English reference to add all keys:
   ```bash
   goopt-i18n-gen sync \
     -i "i18n/locales/en.json" \
     -t "i18n/all_locales/pt.json" \
     --todo-prefix "[TODO]"
   ```

3. Translate the [TODO] entries

4. Generate the locale package:
   ```bash
   goopt-i18n-gen generate-locales \
     -i "i18n/all_locales/pt.json" \
     -o "i18n/locales/"
   ```

## Benefits

- **Performance**: String constants compile into the binary
- **Type Safety**: Can't reference non-existent locales
- **Modularity**: Each language is a separate package
- **No Runtime Overhead**: No file I/O or parsing at runtime
- **Tree Shaking**: Only imported languages are included

## Note on Translations

The Arabic and Hebrew translations in this demo currently show [TODO] markers as they haven't been professionally translated yet. In a production system, these would be replaced with proper translations by native speakers.