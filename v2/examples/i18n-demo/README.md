# goopt v2 i18n Demo

This example demonstrates the comprehensive internationalization (i18n) features of goopt v2, showcasing how to build a fully localized CLI application that supports multiple languages.

## Key Features Demonstrated

### 1. **Struct Tag Translation Keys (`descKey`)**
- Uses `descKey` instead of `desc` to reference translation keys
- Example: `goopt:"short:u;descKey:i18n.demo.user.create.username_desc;required:true"`
- Enables automatic translation of all descriptions

### 2. **Embedded Translation Files**
- User translations: `//go:embed locales/*.json` for application-specific messages
- System translations: `//go:embed system-locales/*.json` for extending goopt's built-in languages
- Clean separation between system and application translations

### 3. **Dual Bundle System**
- System bundle: `i18n.Default()` handles goopt's internal messages
- User bundle: Custom translations for application-specific content
- Shows how to extend system languages without modifying goopt core

### 4. **CommandFunc Pattern**
- Commands defined with `Exec goopt.CommandFunc` field
- Command execution via `parser.ExecuteCommands()`
- Access to parser context with `goopt.GetStructCtxAs[*Config](parser)`

### 5. **Runtime Language Switching**
- Language flag (`--lang`) for selecting display language
- Dynamic bundle language updates after parsing
- Synchronized language switching for both system and user bundles

### 6. **Translator Integration**
- `i18n.Translator` interface embedded in config struct (with `ignore:"true"`)
- Direct access to translations in command functions via `cfg.TR.T(key, args...)`
- Clean separation of translation logic from business logic

### 7. **Comprehensive Localization**
- Command descriptions and names
- Flag descriptions and validation messages
- Error messages (including required field errors)
- Help text section headers (Usage, Commands, Global Flags)
- Status indicators (required, optional, default value)
- Command output messages

### 8. **Multi-Language Support**
- English (en) - default
- Spanish (es)
- Japanese (ja)
- French (fr)
- German (de)

## Project Structure

```
i18n-demo/
├── i18n_demo.go           # Main application
├── locales/               # User-defined translations
│   ├── en.json           # English (application messages)
│   ├── es.json           # Spanish (application messages)
│   ├── ja.json           # Japanese (application messages)
│   ├── fr.json           # French (application messages)
│   └── de.json           # German (application messages)
├── system-locales/        # Extended system translations
│   ├── es.json           # Spanish (goopt system messages)
│   └── ja.json           # Japanese (goopt system messages)
├── go.mod
└── README.md
```

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