# i18n-demo: Extending goopt with New Languages

This example demonstrates how to add support for languages that are not built into goopt v2, showing how to extend the framework's system messages with your own translations.

## Background

goopt comes with:
- **Built-in languages**: English, German, French (always available)
- **Optional language packages**: Spanish, Japanese, Arabic, Hebrew, etc. (in `i18n/locales/`)

But what if you need a language that doesn't exist yet, like Italian, Russian, Polish, or Vietnamese?

## The Solution

This demo shows how to extend goopt's system messages with your own translations using `WithExtendBundle()`.

## Key Concept: Two Types of Translations

1. **System Messages** (goopt's built-in messages):
   - Error messages: "flag not found", "invalid value", etc.
   - Help text: "Usage:", "Commands:", "Required", etc.
   - UI elements: "or", "defaults to", etc.

2. **Application Messages** (your app's custom text):
   - Command descriptions
   - Flag descriptions
   - Your custom output messages

## Project Structure

```
i18n-demo/
├── locales/               # Your application's translations
│   ├── en.json           # English translations for your app
│   ├── es.json           # Spanish translations for your app
│   └── ja.json           # Japanese translations for your app
├── system-locales/        # goopt system message translations
│   ├── es.json           # Spanish translations for goopt's messages
│   └── ja.json           # Japanese translations for goopt's messages
└── messages/             # Generated code from goopt-i18n-gen
    └── keys.go
```

## The Technique

```go
// Step 1: Load your app's translations
userBundle, _ := i18n.NewBundleWithFS(userLocales, "locales")

// Step 2: Load system message translations for new languages
// Note: We're using Spanish/Japanese as examples, but this technique
// works for ANY language not yet supported by goopt
systemBundle, _ := i18n.NewBundleWithFS(systemLocales, "system-locales", language.Spanish)

// Step 3: Create parser with both bundles
parser, _ := goopt.NewParserFromStruct(cfg,
    goopt.WithUserBundle(userBundle),        // Your app's translations
    goopt.WithExtendBundle(systemBundle),    // Extend system messages with new languages
)
```

## Building and Running

```bash
# Build the example
go build -o i18n-demo

# Run with default language (English)
./i18n-demo --help

# Run with Spanish (demonstrating extended system messages)
./i18n-demo --language es --help

# Run with Japanese (demonstrating extended system messages)
./i18n-demo --language ja --help
```

## Adding a Real New Language (e.g., Italian)

To add support for a language not in goopt:

1. **Create a system messages template**:
   ```bash
   # Use goopt-i18n-gen to create a template with all system keys
   goopt-i18n-gen sync -i "../../i18n/all_locales/en.json" -t "system-locales/it.json" --todo-prefix "[TODO]"
   ```

2. **Translate the system messages** in `system-locales/it.json`:
   - Replace ALL `[TODO]` prefixed messages with Italian translations
   - Every key must be translated - missing keys will cause errors

3. **Create your app translations** in `locales/it.json`:
   - Add translations for your app-specific commands and flags

4. **Update your code** to include Italian in the language list

5. **Run with Italian**:
   ```bash
   ./i18n-demo --language it --help
   ```

## Important Notes

1. **Complete Translations Required**: You MUST translate ALL keys in system-locales. Missing keys will cause validation errors when loading the bundle - goopt enforces completeness to prevent partial translations that could confuse users.

2. **For Existing Languages**: If you need Spanish or Japanese in a real app, use the official packages instead:
   ```go
   import (
       esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
       jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
   )
   
   parser, _ := goopt.NewParserFromStruct(cfg,
       goopt.WithSystemLocales(
           goopt.NewSystemLocale(esLocale.Tag, esLocale.SystemTranslations),
           goopt.NewSystemLocale(jaLocale.Tag, jaLocale.SystemTranslations),
       ),
   )
   ```

3. **Bundle Default Language**: When creating the system bundle with `NewBundleWithFS`, specify one of the languages in your files as the default (third parameter). This is important for proper initialization.

## Why This Technique?

- **No waiting**: Don't wait for official goopt language support
- **Full control**: You control the quality and tone of translations
- **Immediate availability**: Add any language you need right now
- **Community contribution**: Your translations can help others

## Example Use Cases

- **Regional software**: Add local languages for specific markets
- **Enterprise requirements**: Add languages required by your organization
- **Specialized domains**: Use domain-specific terminology in your language
- **Testing/Development**: Test i18n features with mock languages

## Contributing Back

If you create translations for a new language, consider contributing them to goopt! Submit a PR with:
1. Your translation file in `i18n/all_locales/`
2. Generated locale package in `i18n/locales/`
3. Tests demonstrating the translation quality

Your contributions help make goopt accessible to more developers worldwide!