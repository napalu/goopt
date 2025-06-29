---
layout: default
title: Internationalization
parent: Guides
nav_order: 9
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
// In your struct, use `desckey` for descriptions and `namekey` for names
type Config struct {
    Verbose bool `goopt:"short:v;desckey:flag.verbose.desc;namekey:flag.verbose.name"`
}

// In your en.json file:
{
  "flag.verbose.desc": "Enable verbose output",
  "flag.verbose.name": "verbose"
}

// In your de.json file:
{
  "flag.verbose.desc": "Ausführliche Ausgabe aktivieren",
  "flag.verbose.name": "ausführlich"
}
```

### 3. Automatic RTL Support
goopt automatically detects right-to-left languages (Arabic, Hebrew, Farsi, Urdu) and adjusts the help output layout accordingly. No additional configuration needed!

### 4. Locale-Aware Formatting
Numbers, dates, and other locale-specific data are automatically formatted according to the user's language settings, including support for regional variants like Swiss German (de-CH) or Canadian French (fr-CA).

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
| `goopt.WithSystemLocales(...)` | Adds language support at parser creation. | Parser-specific | **Recommended** way to add support for additional languages. |

---

## Extended Language Support

### Using Locale Packages (Recommended)

goopt provides pre-built locale packages for additional languages beyond the built-in English, German, and French. This is the recommended way to add language support:

```go
import (
    "github.com/napalu/goopt/v2"
    esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
    jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
)

parser, err := goopt.NewParserFromStruct(cfg,
    goopt.WithSystemLocales(
        goopt.NewSystemLocale(esLocale.Tag, esLocale.SystemTranslations),
        goopt.NewSystemLocale(jaLocale.Tag, jaLocale.SystemTranslations),
    ),
)
```

**Benefits:**
- Only imported languages are included in your binary
- Compile-time safety (typos in imports are caught immediately)
- Zero runtime overhead for unused languages
- Type-safe and IDE-friendly

**Available locale packages:**
- `es` - Spanish
- `ja` - Japanese  
- `ar` - Arabic (with RTL support)
- `he` - Hebrew (with RTL support)
- More coming soon!

---

## Advanced i18n Features

### Complete Runtime Localization: Translating Flag and Command Names

One of goopt's most powerful i18n features is the ability to **completely localize your CLI's interface**, including the actual flag and command names themselves. This goes far beyond just translating descriptions - users can interact with your CLI using commands and flags in their native language, switchable at runtime.

#### Just-In-Time (JIT) Translation Registry

The translation registry uses a JIT (Just-In-Time) approach for building the mappings between canonical names and their translations. This means:

- **Language files are loaded upfront** (not on-demand)
- **The mapping between namekeys and actual flags/commands is built on-demand** when first accessed
- **This optimizes startup time** by deferring the construction of translation mappings until they're actually needed
- **Once built, mappings are cached** for instant subsequent lookups

This JIT approach ensures fast startup times even with large translation files, as the parser only builds the specific mappings it needs for the current execution.

#### How It Works

Using the `namekey` tag, you can provide translations for flag and command names that are recognized and accepted by the parser:

```go
type Config struct {
    // This flag can be used as --output OR its translated equivalent
    Output string `goopt:"namekey:flag.output.name;desckey:flag.output.desc"`
    
    // This command can be invoked as 'list' OR its translated equivalent  
    List struct{} `goopt:"kind:command;namekey:cmd.list.name;desckey:cmd.list.desc"`
}

// In your translation files:
// en.json: { "flag.output.name": "output", "cmd.list.name": "list" }
// de.json: { "flag.output.name": "ausgabe", "cmd.list.name": "auflisten" }
// fr.json: { "flag.output.name": "sortie", "cmd.list.name": "lister" }
```

#### Runtime Switching in Action

The same binary can accept commands in different languages based on the user's locale:

```bash
# English user:
$ myapp list --output results.txt

# German user (with GOOPT_LANG=de or --language de):
$ myapp auflisten --ausgabe ergebnisse.txt

# French user:
$ myapp lister --sortie resultats.txt

# All three commands do exactly the same thing!
```

#### Advanced: Aliases and Canonical Names

What makes this system particularly powerful is that **both the canonical (original) and translated names are always accepted**. This means:

1. **Gradual adoption**: Users can transition to localized commands at their own pace
2. **Mixed teams**: International teams can use the same scripts with different locales
3. **Documentation**: English documentation remains valid for all users

```bash
# A German user can use EITHER:
$ myapp --ausgabe datei.txt    # German flag name
$ myapp --output datei.txt      # English flag name still works!

# The help system shows the appropriate version:
$ GOOPT_LANG=de myapp --help
  --ausgabe    Ausgabedatei spezifizieren
  
$ GOOPT_LANG=en myapp --help  
  --output     Specify output file
```

#### Context-Aware Suggestions

When users mistype commands or flags, goopt's suggestion system intelligently detects which language the user was attempting to use:

```bash
# If user types something close to German:
$ myapp --max-verbindung
Fehler: unbekannter Flag: max-verbindung. Meinten Sie vielleicht eines davon?
  --max-verbindungen

# If user types something close to English:
$ myapp --max-connection
Error: unknown flag: max-connection. Did you mean one of these?
  --max-connections
```

#### Implementation Example

Here's a complete example showing runtime-switchable command names:

```go
type App struct {
    Server struct {
        Start struct {
            Port int `goopt:"short:p;default:8080;desckey:flag.port.desc"`
        } `goopt:"kind:command;namekey:cmd.start.name;desckey:cmd.start.desc"`
    } `goopt:"kind:command;namekey:cmd.server.name;desckey:cmd.server.desc"`
}

// Translation files:
// de.json:
{
  "cmd.server.name": "server",    // Same in German
  "cmd.start.name": "starten",    // Different in German
  "flag.port.desc": "Server-Port"
}

// Usage:
// English: myapp server start --port 9000
// German:  myapp server starten --port 9000
```

This feature is particularly valuable for:
- **Domain-specific tools** where native terminology is important
- **Educational software** where using the local language reduces barriers
- **Government and enterprise** applications requiring full localization
- **International teams** working with shared tools

#### Naming Convention Consistency

goopt helps maintain naming consistency across languages with its warning system. If you use name converters (e.g., `ToLowerCamel` for camelCase), the system will warn about translations that don't follow the convention:

```go
// Set your naming convention
parser, _ := goopt.NewParserWith(
    goopt.WithFlagNameConverter(goopt.ToLowerCamel),     // --myFlag
    goopt.WithCommandNameConverter(goopt.ToKebabCase),   // my-command
)

// After parsing, check for consistency warnings:
warnings := parser.GetWarnings()
// Might include:
// "Translation '--max-verbindungen' for flag '--maxConnections' 
//  doesn't follow naming convention (converter would produce '--maxVerbindungen')"
// "Flag '--my-flag' doesn't follow naming convention (converter would produce '--myFlag')"
```

This helps ensure your CLI maintains a consistent style across all languages and makes environment variable mapping predictable.

### Right-to-Left (RTL) Language Support

goopt automatically detects RTL languages and adjusts the help layout:

```go
// Set language to Arabic
parser.SetSystemLanguage(language.Arabic)

// Help output will automatically:
// - Align text to the right
// - Place flag names on the right side
// - Use appropriate separators
```

Supported RTL languages: Arabic (ar), Hebrew (he), Farsi (fa), Urdu (ur)

### Locale-Aware Number and Date Formatting

All numbers and dates in help text and error messages are automatically formatted according to the current locale:

```go
// Default value of 10000 displays as:
// - English: 10,000
// - German: 10.000  
// - Swiss German: 10'000
// - French: 10 000

type Config struct {
    Port int `goopt:"default:8080;desc:Server port"`
}
```

### Regional Language Variants

goopt fully supports regional language variants:

```go
// Set specific regional variant
parser.SetSystemLanguage(language.MustParse("de-CH")) // Swiss German
parser.SetSystemLanguage(language.MustParse("fr-CA")) // Canadian French
parser.SetSystemLanguage(language.MustParse("en-GB")) // British English

// Each variant can have its own:
// - Number formatting (1'000 vs 1.000)
// - Date formatting (DD/MM/YYYY vs MM/DD/YYYY)
// - Specific translations
```

### Smart Error Message Formatting

Validation error messages intelligently format numbers based on the format specifier:

```go
// Error messages with %d keep raw numbers:
"must be at least %d characters" → "must be at least 1000 characters"

// Error messages with %s apply locale formatting:
"must be at least %s characters" → "must be at least 1,000 characters"
```

This allows translators to control whether numbers should be locale-formatted in each message.

---

## Adding New Language Support

While goopt comes with built-in support for English, German, and French, plus additional locale packages for Spanish, Japanese, Arabic, and Hebrew, you can easily add support for any language.

### Creating a New Language Translation

#### Step 1: Create Initial Translations

Start by creating a JSON file with a few key translations for your language:

```json
{
  "goopt.msg.optional": "opcional",
  "goopt.msg.commands": "Comandos", 
  "goopt.msg.help_description": "Mostrar información de ayuda"
}
```

#### Step 2: Generate Complete Template

Use `goopt-i18n-gen sync` to create a complete translation template with all required keys:

```bash
goopt-i18n-gen sync \
  -i "i18n/locales/en.json" \
  -t "i18n/all_locales/pt.json" \
  --todo-prefix "[TODO]"
```

This creates a file with all system message keys, marking untranslated ones with `[TODO]`.

#### Step 3: Complete the Translation

Replace all `[TODO]` markers with proper translations. Consider:
- Cultural appropriateness and conventions
- Technical terminology standards in your language
- Consistent tone and formality level
- Format specifiers must be preserved (`%[1]s`, `%[2]d`, etc.)

#### Step 4: Generate a Locale Package

Convert your JSON file into a high-performance locale package:

```bash
goopt-i18n-gen generate-locales \
  -i "i18n/all_locales/pt.json" \
  -o "i18n/locales/"
```

This creates a Go package (e.g., `i18n/locales/pt/pt_gen.go`) that can be imported and used like the built-in locale packages.

### Alternative: Runtime Loading

For dynamic scenarios, you can load translations at runtime:

```go
bundle := i18n.NewEmptyBundle()
err := bundle.LoadFromString(language.Portuguese, `{
    "goopt.msg.optional": "opcional",
    "goopt.msg.required": "obrigatório",
    // ... complete translations
}`)

// Extend the default bundle
i18n.Default().Merge(bundle)
```

**Note**: Runtime loading requires complete translations to avoid mixed-language interfaces.

### Translation Guidelines

When translating goopt system messages:

1. **Preserve Format Specifiers**: Keep `%[1]s`, `%[2]d` exactly as they appear
2. **Handle Pluralization**: Some messages may need different forms for singular/plural
3. **Consider Context**: Messages appear in help output, errors, and validation - ensure they work everywhere
4. **Test Formatting**: Verify that help output aligns correctly, especially for RTL languages
5. **Validate Completeness**: Use `goopt-i18n-gen validate` to ensure no messages are missing

### Contributing Your Translation

We welcome contributions of new language translations! To contribute:

1. Follow the workflow above to create a complete translation
2. Test thoroughly in a real application
3. Ensure all messages are translated (no `[TODO]` markers)
4. Submit a pull request with:
   - The JSON file in `i18n/all_locales/`
   - The generated locale package in `i18n/locales/`
   - An example demonstrating the language

Your contribution will help make goopt accessible to more developers worldwide!

## Tooling: `goopt-i18n-gen`

Manually managing translation keys is tedious and error-prone. `goopt` provides a powerful code generation tool, `goopt-i18n-gen`, to automate the entire workflow.

**This tool is the recommended way to manage translations in any `goopt` project.**

**[Continue to the `goopt-i18n-gen` Tooling Guide]({{ site.baseurl }}/v2/guides/06-internationalization/01-tooling-goopt-i18n-gen/) →**