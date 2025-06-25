# RTL and New Language Support Demo

This example demonstrates how to add full support for new languages (Arabic and Hebrew) including:
- Complete system message translations
- RTL (Right-to-Left) layout support
- Locale-aware number formatting
- Translated flag and command names

## Key Points

1. **Complete System Translations Required**: When adding a new language that's not built into goopt (currently en, de, fr), you MUST provide translations for ALL system messages. Partial translations would result in a mixed-language interface.

2. **System Bundle Extension**: Use `i18n.Default()` to get the global system bundle and add your translations before creating parsers.

3. **RTL Support**: goopt automatically detects RTL languages (Arabic, Hebrew, Farsi, Urdu) and adjusts the help layout.

## How to Add a New Language

1. Create a JSON file with ALL system message translations (see system-locales/ar.json)
2. Load it into the default system bundle using `i18n.Default().LoadFromFS()`
3. Create your user bundle with application-specific translations
4. The parser will now support the new language fully

## Running the Example

```bash
# Default (English)
./i18n-rtl-demo

# Arabic (RTL)
./i18n-rtl-demo -l ar

# Hebrew (RTL)  
./i18n-rtl-demo -l he

# Show help in Arabic
./i18n-rtl-demo -l ar --help
```