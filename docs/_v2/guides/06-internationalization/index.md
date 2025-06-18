---
layout: default
title: Internationalization
nav_order: 6
has_children: true
---

# Internationalization (i18n) Guide

`goopt` is designed with internationalization as a core feature, not an afterthought. It provides a comprehensive system for creating command-line applications that can be easily translated into multiple languages.

All system messages, from help text to error descriptions, are pre-translated into English, German, and French, and the system is fully extensible.

## Core Concepts

### 1. Translation Bundles
Translations are managed through `i18n.Bundle` objects. A bundle stores a collection of message keys (like `"app.description"`) and their translated string values for one or more languages.

### 2. Layered Architecture
`goopt` uses a three-tier translation system to provide both robust defaults and high flexibility:
*   **Default Bundle:** Contains all built-in system messages. This is shared by all parsers.
*   **System Bundle:** A parser-specific bundle that can override the default system messages without affecting other parsers.
*   **User Bundle:** Your application-specific bundle for translating command and flag descriptions.

### 3. Translation Keys
Instead of hardcoding strings in your application, you use translation keys. You define these keys in JSON files and reference them in your `goopt` struct tags.

```go
// In your struct, use `desckey` instead of `desc`.
type Config struct {
    Verbose bool `goopt:"short:v;desckey:flag.verbose.desc"`
}

// In your en.json file:
{
  "flag.verbose.desc": "Enable verbose output"
}

// In your de.json file:
{
  "flag.verbose.desc": "Ausführliche Ausgabe aktivieren"
}
```

## Using Internationalization in Your App

Here is the typical workflow for a multi-language `goopt` application.

#### 1. Create Your Translation Files
Organize your translations in JSON files, typically in a `locales` directory.

`locales/en.json`:
```json
{
  "cmd.create.desc": "Create a new resource",
  "flag.output.desc": "Path to the output file"
}
```
`locales/de.json`:
```json
{
  "cmd.create.desc": "Erstellt eine neue Ressource",
  "flag.output.desc": "Pfad zur Ausgabedatei"
}
```

#### 2. Load Your User Bundle
Use `go:embed` to bundle your locale files with your application and load them into a `goopt` bundle.

```go
import (
    "embed"
    "github.com/napalu/goopt/v2"
    "github.com/napalu/goopt/v2/i18n"
    "golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

func main() {
    // Load your custom translations.
    userBundle, _ := i18n.NewBundleWithFS(localesFS, "locales")

    // Create a parser and provide the user bundle.
    parser, _ := goopt.NewParserFromStruct(&MyConfig{}, 
        goopt.WithUserBundle(userBundle),
    )
    
    // ...
}
```

#### 3. Switch Languages Dynamically
You can add a global flag to allow users to switch the language at runtime.

```go
type MyConfig struct {
    Language string `goopt:"short:l;desc:Set the display language;default:en"`
    // ... other flags and commands
}

func main() {
    // ... after parsing ...
    
    // Set the language on both the user and system bundles.
    langTag := parseLanguage(cfg.Language) // Your function to parse "en" -> language.English
    userBundle.SetDefaultLanguage(langTag)
    i18n.Default().SetDefaultLanguage(langTag) // Set on the global system bundle
    
    // Now, all subsequent output from goopt (like help or errors)
    // will be in the selected language.
}
```

## Translatable Errors

All system errors produced by `goopt` are automatically translatable. When you print an error from the parser, it will be rendered in the currently selected language.

```go
if !parser.Parse(os.Args) {
    // These errors will automatically be in French if the language was set to French.
    for _, err := range parser.GetErrors() {
        fmt.Fprintf(os.Stderr, "Error: %s\n", err)
    }
}
```

You can even extend the system bundle with translations for new languages or override existing messages. See `parser.ExtendSystemBundle()` for details.

## The `goopt-i18n-gen` Tool

Manually managing hundreds of translation keys is tedious and error-prone. To solve this, `goopt` provides a powerful code generation tool, `goopt-i18n-gen`, that automates the entire i18n workflow.

It can:
*   Scan your Go code for `goopt` structs.
*   Automatically generate translation keys for your flags and commands.
*   Populate your JSON locale files with stubs from your `desc` tags.
*   Generate a type-safe Go package with constants for all your keys, eliminating typos.

**This tool is the recommended way to manage translations in any `goopt` project.**

**[Continue to the `goopt-i18n-gen` Tooling Guide](01-tooling-goopt-i18n-gen.md) →**