---
layout: default
title: Internationalization
parent: Guides
nav_order: 5
---

# Internationalization (i18n) Guide

## Overview

goopt provides comprehensive internationalization (i18n) support, allowing you to create command-line applications that can adapt to different languages. The i18n system is based on translation bundles that store message keys and their translations.

## Built-in Language Support

goopt comes with translations for the following languages out of the box:

- English (default)
- French
- German

The built-in translations cover all system messages, error descriptions, and usage-related text.

## Core Concepts

### Translation Bundles

Translations in goopt are managed through bundles:

- A **Bundle** is a collection of translations for multiple languages
- Each bundle maps message keys to translated strings
- goopt uses two types of bundles:
    - **Default Bundle**: For system messages and error descriptions
    - **User Bundle**: For application-specific messages (optional)

## Creating Translation Bundles

goopt provides three main ways to create translation bundles:

### 1. Using the Default Bundle

The default bundle is created automatically and includes all system translations:

```go
// Get the default bundle (already populated with system translations)
bundle := i18n.Default()
```

### 2. Creating an Empty Bundle

You can create an empty bundle to add your own translations:

```go
// Create a new empty bundle with English as the default language
bundle := i18n.NewEmptyBundle()

// Add translations to the bundle
bundle.AddLanguage(language.German, map[string]string{
    "my.command.description": "Befehlsbeschreibung",
    "my.flag.description": "Flaggenbeschreibung"
})
```

### 3. Loading from Embedded Files

For larger applications, you can organize translations in JSON files:

```go
//go:embed translations/*.json
var translationsFS embed.FS

// Create a bundle from embedded files
bundle, err := i18n.NewBundleWithFS(translationsFS, "translations")
if err != nil {
    log.Fatal(err)
}
```

Example JSON file (`translations/de.json`):
```json
{
  "my.command.description": "Eine neue Datei erstellen",
  "my.flag.description": "Ausführliche Ausgabe aktivieren"
}
```

## Using Bundles with Parser

Once you have created bundles, you can use them with your parser:

### Setting the Default Bundle

You can replace the default bundle with your own:

```go
parser := goopt.NewParser()

// Create a custom bundle
customBundle, err := i18n.NewBundle()
if err != nil {
    log.Fatal(err)
}

// Replace the default bundle
parser.ReplaceDefaultBundle(customBundle)
```

### Setting a User Bundle

You can add a separate bundle for your application-specific messages:

```go
// Create a user bundle
userBundle := i18n.NewEmptyBundle()
userBundle.AddLanguage(language.German, germanTranslations)

// Set it as the user bundle
parser.SetUserBundle(userBundle)
```

## Working with Translations

### Adding Translations to a Bundle

Add translations for a specific language:

```go
// Add German translations
bundle.AddLanguage(language.German, map[string]string{
    "my.command.description": "Befehlsbeschreibung",
    "my.flag.description": "Flaggenbeschreibung"
})
```

### Using Translation Keys

Use translation keys when creating commands and flags:

```go
// Create a command with a translatable description
parser.AddCommand(
    goopt.NewCommand(
        goopt.WithName("create"),
        goopt.WithCommandDescriptionKey("my.command.description"),
    ),
)

// Add a flag with a translatable description
parser.AddFlag("verbose", goopt.NewArg(
    goopt.WithShort("v"),
    goopt.WithDescriptionKey("my.flag.description"),
))
```

### Translation Key Organization

Recommended patterns for organizing translation keys:

- System keys (built-in): `goopt.error.{error_type}`, `goopt.msg.{message_type}`
- Command descriptions: `my.commands.{command_name}`
- Flag descriptions: `my.flags.{flag_name}`
- Custom error messages: `my.errors.{error_name}`

## Translatable Errors

All system errors in goopt are automatically translatable:

```go
if !parser.Parse(os.Args) {
    // These errors will be in the current language
    for _, err := range parser.GetErrors() {
        fmt.Fprintf(os.Stderr, "Error: %s\n", err)
    }
}
```

## Complete Example

```go
package main

import (
    "embed"
    "fmt"
    "log"
    "os"
    
    "github.com/napalu/goopt"
    "github.com/napalu/goopt/i18n"
    "golang.org/x/text/language"
)

//go:embed translations/*.json
var translationsFS embed.FS

func main() {
    // Create a parser
    parser := goopt.NewParser()
    
    // Method 1: Create a user bundle from embedded files
    userBundle, err := i18n.NewBundleWithFS(translationsFS, "translations")
    if err != nil {
        log.Fatalf("Failed to load translations: %v", err)
    }
    
    // Method 2: Create an empty bundle and add translations programmatically
    // (This would be an alternative to Method 1)
    /*
    userBundle := i18n.NewEmptyBundle()
    userBundle.AddLanguage(language.German, map[string]string{
        "app.description": "Dateiverwaltungswerkzeug",
        "cmd.create": "Erstellt eine neue Datei",
        "flag.verbose": "Ausführliche Ausgabe aktivieren",
    })
    */
    
    // Set the user bundle
    parser.SetUserBundle(userBundle)
    
    // Create commands with translatable descriptions
    parser.AddCommand(
        goopt.NewCommand(
            goopt.WithName("create"),
            goopt.WithCommandDescriptionKey("cmd.create"),
        ),
    )
    
    // Add flags with translatable descriptions
    parser.AddFlag("verbose", goopt.NewArg(
        goopt.WithShortFlag("v"),
        goopt.WithDescriptionKey("flag.verbose"),
    ))
    
    // Parse arguments
    if !parser.Parse(os.Args) {
        // Errors will be in the current language
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, "Error: %s\n", err)
        }
        parser.PrintUsageWithGroups(os.Stdout)
        return
    }
    
    // Continue with application logic...
}
```

Example translation file (`translations/de.json`):
```json
{
  "app.description": "Dateiverwaltungswerkzeug",
  "cmd.create": "Erstellt eine neue Datei",
  "flag.verbose": "Ausführliche Ausgabe aktivieren",
  "flag.output": "Ausgabedatei"
}
```

## Related Topics

- [Error Handling]({{ site.baseurl }}/v1/guides/advanced-features/#error-handling) - Learn about structured error handling in goopt
- [Configuration]({{ site.baseurl }}/v1/configuration/index/) - External configuration and environment variables