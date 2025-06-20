---
layout: default
title: Internationalization
nav_order: 6
has_children: true
---

# Internationalization (i18n) Guide

`goopt` is designed with internationalization as a core feature. It provides a comprehensive system for creating command-line applications that can be easily translated into multiple languages.

All system messages, from help text to error descriptions, are pre-translated into English, German, and French, and the system is fully extensible.

## Core Concepts

### 1. Translation Bundles
Translations are managed through `i18n.Bundle` objects. A bundle stores a collection of message keys (like `"app.description"`) and their translated string values for one or more languages.

### 2. Translation Keys
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

## Basic Usage: Translating Your Application

This is the most common workflow for making your application's specific text (like command and flag descriptions) multilingual.

#### 1. Create Your Translation Files
Organize your application's translations in JSON files, typically in a `locales/` directory.

`locales/en.json`:
```json
{
  "cmd.create.desc": "Create a new resource",
  "flag.output.desc": "Path to the output file"
}
```

#### 2. Load Your User Bundle
Use `go:embed` to bundle your locale files with your application and then set this as the **User Bundle** on your parser.

```go
import (
    "embed"
    "github.com/napalu/goopt/v2"
    "github.com/napalu/goopt/v2/i18n"
)

//go:embed locales/*.json
var localesFS embed.FS

func main() {
    // Load your custom translations.
    userBundle, _ := i18n.NewBundleWithFS(localesFS, "locales")

    // Create a parser and provide your translations as the User Bundle.
    parser, _ := goopt.NewParserFromStruct(&MyConfig{}, 
        goopt.WithUserBundle(userBundle),
    )
    
    // ...
}
```
Now, `goopt` will use your `userBundle` to translate any `desckey` tags it finds.

---

## Advanced: The Layered Bundle Architecture

For more advanced control, it's important to understand `goopt`'s three-tier translation system. When looking for a translation, `goopt` checks each layer in order of precedence:

1.  **User Bundle (Highest Priority):** This is the bundle you provide with `parser.SetUserBundle()`. It contains your application-specific messages.
2.  **System Bundle:** Each `Parser` instance has its own system bundle. This is where you can override built-in `goopt` messages for a *specific parser instance*.
3.  **Default Bundle (Lowest Priority):** A global, singleton bundle that contains all of `goopt`'s built-in translations (errors, help text, etc.).

This architecture allows you to customize translations at different scopes.

### Use Case 1: Overriding a System Message for One Parser

If you want to change the wording of a single built-in error message for just one parser without affecting others, you can `ExtendSystemBundle`.

```go
// Create a parser.
parser := goopt.NewParser()

// Create a small bundle with just your override.
overrideBundle := i18n.NewEmptyBundle()
overrideBundle.AddLanguage(language.English, map[string]string{
    "goopt.error.flag_not_found": "Whoops! The flag '%s' doesn't exist.",
})

// Extend this parser's system bundle with your override.
parser.ExtendSystemBundle(overrideBundle)

// Now, only this parser instance will use the custom error message.```
```

### Use Case 2: Adding a New System-Wide Language

If you want to add support for a new language (e.g., Spanish) to `goopt`'s core system messages so that *all* parsers in your application can use it, you should extend the **global default bundle**.

This is exactly what the [i18n-demo](https://github.com/napalu/goopt/tree/main/v2/examples/i18n-demo/) example does.

1.  **Create JSON files for the system messages** (e.g., `system-locales/es.json`) containing translations for keys like `goopt.error.flag_not_found`.
2.  **Embed and load these files into the global default bundle** at application startup.

```go
import (
    "embed"
    "github.com/napalu/goopt/v2/i18n"
)

//go:embed system-locales/*.json
var systemLocalesFS embed.FS

func init() {
    // Get the global default bundle.
    defaultBundle := i18n.Default()
    
    // Load your new system-wide translations into it.
    err := defaultBundle.LoadFromFS(systemLocalesFS, "system-locales")
    if err != nil {
        panic(err)
    }
}
```
Now, any parser created anywhere in your application can be switched to Spanish (`parser.SetSystemLanguage(language.Spanish)`) and all the built-in error messages will be translated.

### Summary of i18n Methods

| Method | What It Does | Scope | When to Use It |
|---|---|---|---|
| `parser.SetUserBundle(bundle)` | Sets the bundle for your app's `desckey` tags. | Parser-specific | **Always use this** for your application's own text. |
| `parser.ExtendSystemBundle(bundle)` | Overrides built-in `goopt` messages. | Parser-specific | When you want to change a system message for just one parser. |
| `i18n.Default().LoadFromFS(...)` | Adds translations to the global system bundle. | **Global** (all parsers) | When you are adding a new language translation for `goopt`'s core messages. |

---

## Tooling: `goopt-i18n-gen`

Manually managing translation keys is tedious and error-prone. `goopt` provides a powerful code generation tool, `goopt-i18n-gen`, to automate the entire workflow.

**This tool is the recommended way to manage translations in any `goopt` project.**

**[Continue to the `goopt-i18n-gen` Tooling Guide]({{ site.baseurl }}/v2/guides/06-internationalization/01-tooling-goopt-i18n-gen/) →**