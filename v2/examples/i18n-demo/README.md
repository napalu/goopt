# goopt v2 i18n Demo

This example demonstrates the comprehensive internationalization (i18n) features of goopt v2, including:

- User-defined translations for command and flag descriptions
- Dynamic language switching via command-line flag
- Localized error messages and output
- Support for multiple languages (English, Spanish, Japanese, French, German)

## Features Demonstrated

1. **Translation Key Usage**: All command and flag descriptions use translation keys (e.g., `i18n.demo.user_desc`)
2. **Custom Translation Bundles**: Loading external JSON translation files from the `locales/` directory
3. **Language Switching**: The `--lang` flag allows runtime language selection
4. **Command Structure**: Hierarchical commands with localized descriptions and help text
5. **Localized Output**: All command output respects the selected language

## Building and Running

```bash
# Build the example
go build -o i18n-demo

# Run with default language (English)
./i18n-demo --help

# Run with Spanish
./i18n-demo --lang es --help

# Run with Japanese
./i18n-demo --lang ja user list

# Run with French
./i18n-demo --lang fr database backup -o backup.sql

# Run with German
./i18n-demo --lang de user create -u hans -e hans@example.com
```

## Example Commands

### User Management

```bash
# List users (with verbose output in Spanish)
./i18n-demo --lang es -v user list --all

# Create a user (in Japanese)
./i18n-demo --lang ja user create -u tanaka -e tanaka@example.com --admin

# Delete a user (in French, with force flag)
./i18n-demo --lang fr user delete -u alice --force
```

### Database Management

```bash
# Create a compressed backup (in German)
./i18n-demo --lang de database backup -o backup.sql.gz --compress

# Restore from backup (in Spanish)
./i18n-demo --lang es db restore -i backup.sql.gz --drop-first
```

## Translation Files

The `locales/` directory contains JSON files for each supported language:

- `en.json` - English (default)
- `es.json` - Spanish
- `ja.json` - Japanese
- `fr.json` - French
- `de.json` - German

Each file contains translations for:
- Command descriptions
- Flag descriptions
- Output messages
- Status indicators

## How It Works

1. **Parser Creation**: The parser is created with a default language (English)
2. **Translation Loading**: Custom translations are loaded from JSON files using `i18n.Bundle`
3. **Language Detection**: After parsing, if a language flag is provided, the parser language is updated
4. **Command Execution**: Commands use `bundle.TL(lang, key, args...)` for localized output
5. **Help Generation**: goopt automatically uses the correct translations for help text

## Key goopt v2 i18n Features Used

- `goopt.WithLanguage()` - Set the parser's language
- `parser.SetUserBundle()` - Add custom translations
- `i18n.Bundle.LoadLanguageFile()` - Load translations from JSON
- `bundle.TL()` - Translate with specific language
- Translation keys in struct tags - Automatic help text localization

## Adding New Languages

To add a new language:

1. Create a new JSON file in `locales/` (e.g., `pt.json` for Portuguese)
2. Add all translation keys with translated values
3. Update the `languages` slice in `loadCustomTranslations()`
4. Add a case in `parseLanguageTag()` for the new language code

## Notes

- Error messages from goopt itself are also localized (using built-in translations)
- The example uses goopt's command execution pattern with `CommandFunc`
- Language switching happens before command execution for proper localization
- All user-facing strings use translation keys for complete localization