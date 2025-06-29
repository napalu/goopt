# Comprehensive i18n Demo

This demo showcases the full internationalization (i18n) capabilities of `goopt`, including:

- Localized help messages, flag names, and descriptions
- Translated command and flag names (not just descriptions!)
- Locale-aware number formatting
- Suggestion system for mistyped flags and commands
- Support for multiple languages (English, German, French)
- Translated positional argument metadata

## Usage

Simply run the demo:

```bash
go run ./main.go
```

It will simulate a series of CLI invocations across different languages and scenarios — valid, invalid, and help requests — printing localized output.

No setup required.

## Supported Languages

- English (`en`)
- German (`de`)
- French (`fr`)
