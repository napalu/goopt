---
layout: default
title: Internationalization
parent: Guides
nav_order: 5
---

# Internationalization (i18n) Guide

## Overview

goopt v2 provides comprehensive internationalization (i18n) support, allowing you to create command-line applications that can adapt to different languages. The i18n system is based on translation bundles that store message keys and their translations.

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
    
    "github.com/napalu/goopt/v2"
    "github.com/napalu/goopt/v2/i18n"
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

## Type-Safe Translation Keys with goopt-i18n-gen

goopt provides a code generation tool that creates compile-time safe constants from your JSON translation files, eliminating the need for error-prone string keys.

### Benefits

- **Type Safety**: No more typos in translation keys
- **IDE Support**: Full autocomplete for all translation keys
- **Refactoring Safety**: Rename keys and all usages update automatically
- **Compile-Time Validation**: Invalid keys are caught at build time

### Installation

Install the generator tool:

```bash
go install github.com/napalu/goopt/v2/cmd/goopt-i18n-gen@latest
```

### Multi-Language Support

goopt-i18n-gen itself is fully internationalized! The `-l` flag controls the language of the tool's output messages (not the generated content):

```bash
# English output (default)
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go
# Tool output: "Generated messages/keys.go"

# German output
goopt-i18n-gen -l de -i "locales/*.json" generate -o messages/keys.go
# Tool output: "messages/keys.go generiert"

# French output
goopt-i18n-gen -l fr -i "locales/*.json" generate -o messages/keys.go
# Tool output: "messages/keys.go généré"
```

**Note**: The `-l` flag only changes the tool's messages, not the generated file content. The generated Go file will always contain ALL keys from ALL input JSON files specified.

### Usage

1. Create your translation file (e.g., `locales/en.json`):

```json
{
  "app.name": "My Application",
  "app.description": "A powerful CLI tool",
  "user.create.success": "User '%s' created successfully",
  "user.delete.confirm": "Delete user '%s'? This action cannot be undone."
}
```

2. Generate the constants:

```bash
# For single locale
goopt-i18n-gen -i locales/en.json generate -o messages/keys.go -p messages

# For multiple locales (recommended - ensures all keys are included)
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages
```

3. Use the generated constants in your code:

```go
package main

import (
    "fmt"
    "github.com/napalu/goopt/v2/i18n"
    "myapp/messages"
)

//go:generate goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages

func main() {
    bundle := i18n.NewBundleWithFS(localesFS, "locales")
    
    // Before: error-prone string keys
    // message := bundle.T("user.create.success", username)
    
    // After: compile-time safe constants
    message := bundle.T(messages.Keys.UserCreate.Success, username)
    fmt.Println(message)
}
```

### Generated Code Structure

The generator creates a nested structure that mirrors your JSON hierarchy:

```go
// Code generated by goopt-i18n-gen. DO NOT EDIT.
package messages

var Keys = struct {
    App struct {
        Name        string
        Description string
    }
    UserCreate struct {
        Success string
    }
    UserDelete struct {
        Confirm string
    }
}{
    App: struct {
        Name        string
        Description string
    }{
        Name:        "app.name",
        Description: "app.description",
    },
    UserCreate: struct {
        Success string
    }{
        Success: "user.create.success",
    },
    UserDelete: struct {
        Confirm string
    }{
        Confirm: "user.delete.confirm",
    },
}

// K is a shorthand alias for Keys
var K = Keys
```

### Commands and Options

goopt-i18n-gen uses a command-based structure:

**Commands:**
- `init`: Initialize empty translation files
- `generate`: Generate Go constants from JSON files
- `audit`: Audit goopt fields for missing descKey tags
- `validate`: Check that all descKey references have translations
- `add`: Add translation keys to locale files programmatically

**Global options:**
- `-i, --input`: Input JSON files (comma-separated or wildcards, required)
  - Multiple files: `-i "en.json,de.json,fr.json"`
  - Wildcards: `-i "locales/*.json"` 
  - **Note**: goopt uses comma-separated values for multiple inputs, not repeated flags
- `-v, --verbose`: Enable verbose output
- `-l, --language`: Language for output (en, de, fr)

**Generate command options:**
- `-o, --output`: Output Go file (required)
- `-p, --package`: Package name (default: "messages")
- `--prefix`: Optional prefix to strip from keys

**Audit command options:**
- `--files`: Go source files to scan (default: *.go)
- `-d, --generate-desc-keys`: Generate descKey tags
- `-g, --generate-missing`: Generate stub entries for missing keys
- `-u, --auto-update`: Automatically update source files
- `--key-prefix`: Prefix for generated keys (default: app)
- `--backup-dir`: Directory for backup files

**Validate command options:**
- `-s, --scan`: Go source files to scan for descKey references
- `--strict`: Exit with error if validation fails (for CI/CD)
- `-g, --generate-missing`: Generate stub entries for missing keys

**Add command options:**
- `-k, --key`: Single key to add
- `-V, --value`: Value for the key (defaults to key name if not provided)
- `-F, --from-file`: JSON file containing key-value pairs to add
- `-m, --mode`: How to handle existing keys (skip, replace, error) - default: skip
- `-n, --dry-run`: Show what would be added without modifying files

**Extract command options:**
- `-s, --files`: Go files to scan (default: **/*.go)
- `-m, --match-only`: Regex to match strings for inclusion
- `-S, --skip-match`: Regex to match strings for exclusion
- `-P, --key-prefix`: Prefix for generated keys (default: app.extracted)
- `-l, --min-length`: Minimum string length (default: 2)
- `-n, --dry-run`: Preview what would be extracted
- `-u, --auto-update`: Update source files (add comments or replace strings)
- `--tr-pattern`: Translator pattern for replacements (e.g. tr.T)
- `--keep-comments`: Keep i18n comments after replacement
- `--clean-comments`: Remove all i18n-* comments
- `--backup-dir`: Directory for backup files (default: .goopt-i18n-backup)
- `--transform-mode`: What strings to transform: user-facing, with-comments, all-marked, all (default: user-facing)
- `--user-facing-regex`: Regex patterns to identify custom user-facing functions (can be specified multiple times)
- `--format-function-regex`: Regex pattern and format arg index for custom format functions (pattern:index, can be specified multiple times)

### Validation Workflow

The validate command scans your Go source files to find all `descKey` references and ensures they have translations:

1. **Development Workflow** - Automatically generate missing translations:
   ```bash
   goopt-i18n-gen -i locales/en.json validate -s "*.go" -g
   ```
   This will:
   - Scan all Go files for `descKey` references
   - Identify missing translations
   - Generate stub entries (e.g., "Enable verbose output" from field name)
   - Update your JSON file

2. **CI/CD Workflow** - Strict validation:
   ```bash
   goopt-i18n-gen -i locales/en.json validate -s "*.go" --strict
   ```
   This will fail the build if any `descKey` references lack translations.

3. **Multi-locale validation**:
   ```bash
   goopt-i18n-gen -i "locales/*.json" validate -s "*.go"
   ```
   This validates each locale file separately and reports specific missing keys.

4. **Example Output**:
   ```
   Found 15 descKey references in 3 files
   
   locales/en.json: Missing 2 translations:
     app.new_feature_desc (used in config.go:42, field: NewFeature)
     app.experimental_desc (used in config.go:45, field: Experimental)
   
   Generating stub translations for locales/en.json:
     "app.new_feature_desc": "New feature desc"
     "app.experimental_desc": "Experimental desc"
   
   ✓ Updated locales/en.json with missing translations
   ```

### Best Practices

1. **Use go:generate**: Add generate directives for your workflow:
   ```go
   // For basic generation
   //go:generate goopt-i18n-gen -i locales/en.json generate -o messages/keys.go -p messages
   
   // For CI/CD validation
   //go:generate goopt-i18n-gen -i locales/en.json validate -s "*.go" --strict
   ```

2. **Consistent Naming**: Use a hierarchical naming convention for your keys:
   - `app.*` for application-level messages
   - `cmd.*` for command descriptions
   - `flag.*` for flag descriptions
   - `error.*` for error messages

3. **Version Control**: Commit both your JSON files and generated Go files

4. **CI/CD Integration**: 
   - Use `--strict` mode in CI to catch missing translations
   - Run `go generate` to ensure generated files are up-to-date
   - Consider separate translation files per language

5. **Type-Safe References**: Always use the generated constants instead of string literals:
   ```go
   // Avoid
   bundle.T("user.create.success", username)
   
   // Prefer
   bundle.T(messages.Keys.UserCreate.Success, username)
   ```

### Complete Example

See the [i18n-demo example](https://github.com/napalu/goopt/tree/v2/examples/i18n-demo) for a complete working example that demonstrates:
- Multiple language support
- Type-safe translation keys
- Command and flag descriptions
- Dynamic language switching

## The 360° i18n Workflow: Adding i18n to Existing Applications

When you have an existing goopt application without internationalization, the "360° workflow" helps you systematically add i18n support without manually creating all the descKey tags and translation entries.

### Overview

The 360° workflow takes you full circle:
1. **Start** with existing goopt structs (desc tags only, no descKey tags)
2. **Analyze** your code to find all fields needing descKeys
3. **Generate** appropriate descKey values and translation stubs
4. **Apply** the suggested descKeys to your structs
5. **Complete** the circle with type-safe generated constants

### Multi-Language Strategy

When supporting multiple languages, you typically:
1. Start with one base language (e.g., English) for the 360° workflow
2. Generate descKeys and initial translations
3. Copy the base file to create other language files
4. Use the generate command with ALL locale files to create constants that include all keys

### Step-by-Step Guide

#### 1. Starting Point: Plain goopt Structs

You have an existing application with goopt tags but no internationalization:

```go
type AppConfig struct {
    Verbose  bool   `goopt:"short:v;desc:Enable verbose output"`
    Output   string `goopt:"short:o;desc:Output file path"`
    Workers  int    `goopt:"short:w;desc:Number of worker threads;default:4"`
    
    Process struct {
        InputFile  string `goopt:"short:i;desc:Input file to process;required:true"`
        Format     string `goopt:"short:f;desc:Output format (json, xml, csv);default:json"`
        Compress   bool   `goopt:"short:c;desc:Compress output"`
        Exec       goopt.CommandFunc
    } `goopt:"kind:command;name:process;desc:Process input files"`
}
```

#### 2. Initialize and Analyze

Use goopt-i18n-gen to initialize and analyze your code:

```bash
# Initialize empty translation file
goopt-i18n-gen -i locales/en.json init

# Run the 360° analysis
goopt-i18n-gen -i locales/en.json audit -d -g --key-prefix app
```

#### 3. Review Generated Suggestions

The tool analyzes your struct and provides:

```
Found 4 fields without descKey tags:
  AppConfig.Verbose (main.go:10) - flag Verbose [desc: Enable verbose output]
  AppConfig.Output (main.go:11) - flag Output [desc: Output file path]
  AppConfig.Workers (main.go:12) - flag Workers [desc: Number of worker threads]
  AppConfig.Process.InputFile (main.go:15) - flag InputFile [desc: Input file to process]

Generated descKeys and translations:
  AppConfig.Verbose -> descKey:app.app_config.verbose_desc
    Translation: "Enable verbose output"
  AppConfig.Output -> descKey:app.app_config.output_desc
    Translation: "Output file path"
  ...

✓ Updated locales/en.json with generated translations

📝 To complete the setup, add these descKey tags to your structs:

  In main.go:10, update the goopt tag to include:
    descKey:app.app_config.verbose_desc

  In main.go:11, update the goopt tag to include:
    descKey:app.app_config.output_desc
```

#### 4. Apply the Suggested descKeys

You have two options:

**Option A: Manual Update**
Update your struct with the suggested descKey tags as shown in the output.

**Option B: Automatic Update (Recommended)**
Use the `-u` flag to automatically update your source files:

```bash
goopt-i18n-gen -i locales/en.json audit -d -g -u --key-prefix app
```

This will:
- Create timestamped backups of your source files
- Automatically insert the descKey tags into your struct fields
- Update both field tags and command struct tags
- Preserve all existing tag attributes and code formatting

Your struct will be automatically updated to:

```go
type AppConfig struct {
    Verbose  bool   `goopt:"short:v;desc:Enable verbose output;descKey:app.app_config.verbose_desc"`
    Output   string `goopt:"short:o;desc:Output file path;descKey:app.app_config.output_desc"`
    Workers  int    `goopt:"short:w;desc:Number of worker threads;default:4;descKey:app.app_config.workers_desc"`
    
    Process struct {
        InputFile  string `goopt:"short:i;desc:Input file to process;required:true;descKey:app.app_config.process.input_file_desc"`
        // ... rest of the fields with descKeys added
    } `goopt:"kind:command;name:process;desc:Process input files;descKey:app.process_desc"`
}
```

#### 5. Generate Type-Safe Constants

Now generate the final constants file:

```bash
# Add go:generate directive to your main file
//go:generate goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages

# Or run directly
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages
```

#### 6. Use in Your Application

```go
import (
    "github.com/napalu/goopt/v2"
    "github.com/napalu/goopt/v2/i18n"
    "myapp/messages"
)

func main() {
    // Load translations assuming your json files are in locales folder
	bundle, err := i18n.NewBundleWithFS(userLocales, "locales")
    
    // Create parser with translations
    cfg := AppConfig{}
    parser, _ := goopt.NewParserFromStruct(&cfg, goopt.WithUserBundle(bundle))
    
    // Your flags and commands now support multiple languages!
}
```

### Key Benefits of the 360° Workflow

1. **Zero Manual Work**: No need to manually create descKey values or translation entries
2. **Consistent Naming**: Automatically generates hierarchical, consistent key names
3. **Preserves Descriptions**: Uses your existing desc values as initial translations
4. **Works Without desc**: Fields don't need desc attributes - sensible defaults are generated
5. **Incremental Adoption**: Can be applied to parts of your application gradually
6. **CI/CD Ready**: Validation mode ensures all keys have translations

### Special Features

**No desc Required**: The tool works with fields that don't have desc attributes:
```go
// This works fine - tool generates default translation
Verbose bool `goopt:"short:v"`  // Generates: "Verbose"

// With desc - uses the description
Verbose bool `goopt:"short:v;desc:Enable verbose output"`  // Uses: "Enable verbose output"
```

**Nested Structures**: The tool handles complex nested structures:
```go
type App struct {
    Global GlobalFlags                    // Nested flag container
    Process ProcessCmd `goopt:"kind:command"`  // Command with nested flags
}
```


### Complete Multi-Language Workflow

Here's the recommended workflow for applications supporting multiple languages:

```bash
# Step 1: Initialize base language file
goopt-i18n-gen -i locales/en.json init

# Step 2: Generate translations FIRST (before adding descKeys!)
goopt-i18n-gen -i locales/en.json audit -d -g --key-prefix myapp

# Step 3: NOW add descKeys to source (translations exist, no broken state)
goopt-i18n-gen -i locales/en.json audit -d -u --key-prefix myapp

# Step 4: Create additional language files
cp locales/en.json locales/de.json
cp locales/en.json locales/fr.json
cp locales/en.json locales/es.json

# Step 5: Generate constants from ALL locale files
# This ensures every key from every locale is included
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages

# Step 6: Validate all locales have required translations
goopt-i18n-gen -i "locales/*.json" validate -s "*.go"

# Step 7: For CI/CD - strict validation
goopt-i18n-gen -i "locales/*.json" validate -s "*.go" --strict
```

**Important: Why generate translations before adding descKeys?**
- If you add descKeys first, goopt will try to use them immediately
- Without translations, it displays raw keys (e.g., "app.global.help_desc")
- This creates a broken intermediate state
- Always: audit -g (generate translations) THEN audit -u (update source)

**Why process all locale files for generation?**
- Ensures constants include keys that might only exist in some locales
- Prevents runtime errors from missing keys
- Makes it easy to add locale-specific features

### Example: Complete 360° Workflow

See the [i18n-codegen-demo example](https://github.com/napalu/goopt/tree/main/v2/examples/i18n-codegen-demo) for a complete demonstration of:
- Starting with a plain goopt application
- Running the 360° analysis
- Applying generated descKeys
- Building a fully internationalized CLI

The example includes a step-by-step README showing the entire workflow in action.

### Extracting Hardcoded Strings with the extract Command

The `extract` command is a powerful tool for migrating existing Go codebases to internationalization. It can automatically find hardcoded strings, generate translation keys, and even transform your source code to use i18n.

#### Key Features

- **AST-based extraction**: Analyzes Go code structure, not just regex patterns
- **Smart filtering**: Automatically skips constants, comments, and non-user-facing strings
- **Format function handling**: Intelligently transforms Printf, Sprintf, Errorf, etc.
- **String concatenation detection**: Extracts concatenated strings in format functions as single entries
- **Auto-update modes**: Can add TODO comments or directly replace strings with translation calls
- **Safe operation**: Creates backups and supports dry-run mode

#### Basic Extraction Workflow

```bash
# 1. Preview what will be extracted
goopt-i18n-gen -v -i "locales/*.json" extract -n

# 2. Extract strings and update locale files
goopt-i18n-gen -i "locales/*.json" extract

# 3. Generate constants for the new keys
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go
```

#### Comment-Based Migration (Recommended for Large Codebases)

This approach adds comments next to strings, allowing manual review:

```bash
# Step 1: Add TODO comments to all extractable strings
goopt-i18n-gen -i "locales/*.json" extract -u

# Your code now has comments like:
# fmt.Println("Starting server...") // i18n-todo: app.extracted.starting_server

# Step 2: Review and manually update high-priority strings
# Change some to:
# fmt.Println(tr.T(messages.Keys.App.Server.Starting)) // i18n-done

# Step 3: Mark strings to skip
# fmt.Println("DEBUG: raw data") // i18n-skip

# Step 4: Auto-transform remaining TODOs
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T"

# Step 5: Clean up comments
goopt-i18n-gen -i "locales/*.json" extract --clean-comments
```

#### Direct Transformation Workflow

For smaller codebases or when you're confident about the changes:

```bash
# Preview transformations
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T" -n

# Apply transformations
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T"
```

#### Format Function Transformations

The extract command intelligently handles format functions:

```go
// Before extraction
fmt.Printf("User %s logged in at %v", username, time.Now())
fmt.Sprintf("Welcome %s!", name)
fmt.Errorf("failed to process: %v", err)
fmt.Fprintf(os.Stderr, "Error: %s", msg)

// After extraction with --tr-pattern "tr.T"
fmt.Print(tr.T(messages.Keys.AppExtracted.UserSLoggedInAtV, username, time.Now()))
tr.T(messages.Keys.AppExtracted.WelcomeS, name)
errors.New(tr.T(messages.Keys.AppExtracted.FailedToProcessV, err))
fmt.Fprint(os.Stderr, tr.T(messages.Keys.AppExtracted.ErrorS, msg))
```

#### Custom Function Detection

The extract command can identify custom logging and display functions in your codebase:

```bash
# Identify custom user-facing functions
goopt-i18n-gen -i "locales/*.json" extract \
  --user-facing-regex ".*\.Log$" \
  --user-facing-regex ".*\.Display$" \
  -u --tr-pattern "tr.T"

# Identify custom format functions with argument positions
goopt-i18n-gen -i "locales/*.json" extract \
  --format-function-regex ".*\.Logf$:0" \
  --format-function-regex ".*\.MsgAll$:1" \
  -u --tr-pattern "tr.T"
```

Example transformation of custom functions:
```go
// Before: Custom logger with format at position 1
logger.MsgAll(ctx, "User %s logged in", username)

// After: Format string and args replaced
logger.MsgAll(ctx, tr.T(messages.Keys.App.UserLoggedIn, username))
```

For structured logging with Go's slog package, see the [Structured Logging Guide](https://github.com/napalu/goopt/tree/main/v2/cmd/goopt-i18n-gen/SLOG_USAGE.md) which covers recommended patterns for internationalizing slog-based applications.

#### Advanced Filtering

```bash
# Extract only user-visible strings (containing spaces)
goopt-i18n-gen -i "locales/*.json" extract -m ".*\\s+.*"

# Extract only error messages
goopt-i18n-gen -i "locales/*.json" extract -m "(?i)error|fail|unable|cannot" -P app.errors

# Exclude debug/test strings
goopt-i18n-gen -i "locales/*.json" extract -S "(?i)debug|test|todo"

# Extract from specific packages
goopt-i18n-gen -i "locales/*.json" extract -s "internal/api/**/*.go" -P app.api
```

#### Complete Migration Example

```bash
# 1. Initial scan to understand scope
goopt-i18n-gen -v -i "locales/*.json" extract -m ".*\\s+.*" -l 3 -n > strings-review.txt

# 2. Add TODO comments for review
goopt-i18n-gen -i "locales/*.json" extract -m ".*\\s+.*" -l 3 -u
git add -A && git commit -m "Add i18n TODO comments"

# 3. Package-by-package migration
goopt-i18n-gen -i "locales/*.json" extract -s "internal/api/**/*.go" -u --tr-pattern "tr.T"
go test ./internal/api/...

# 4. Complete remaining packages
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T"

# 5. Generate constants and clean up
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go
goopt-i18n-gen -i "locales/*.json" extract --clean-comments
```

For detailed workflow guides and advanced features, see:
- [Extract Workflow Guide](https://github.com/napalu/goopt/tree/main/v2/cmd/goopt-i18n-gen/EXTRACT_WORKFLOW.md) - Step-by-step extraction patterns
- [Structured Logging (slog) Guide](https://github.com/napalu/goopt/tree/main/v2/cmd/goopt-i18n-gen/SLOG_USAGE.md) - Best practices for i18n with Go's slog
- [goopt-i18n-gen README](https://github.com/napalu/goopt/tree/main/v2/cmd/goopt-i18n-gen/README.md) - Complete tool documentation

### Adding Keys with the add Command

The `add` command provides a programmatic way to add translation keys to your locale files, particularly useful when:
- Adding new features that require multiple translation keys
- Synchronizing keys across multiple locale files
- Automating translation key management in build scripts

#### Example: Adding Keys for a New Feature

```bash
# 1. Create a JSON file with all keys for your new feature
cat > search-feature.json <<EOF
{
  "app.search.title": "Search",
  "app.search.placeholder": "Enter search terms...",
  "app.search.button": "Search",
  "app.search.no_results": "No results found for '%s'",
  "app.search.error": "Search failed: %v"
}
EOF

# 2. Add these keys to all locale files
goopt-i18n-gen -i "locales/*.json" add -F search-feature.json

# Result:
# - en.json: Gets the exact values from search-feature.json
# - de.json: Gets "[TODO] Search" for "app.search.title", etc.
# - fr.json: Gets "[TODO] Search" for "app.search.title", etc.
```

#### Smart Language Detection

The add command automatically detects the language from the filename and prefixes non-English values with `[TODO]`:

```bash
# Adding a single key
goopt-i18n-gen -i "locales/*.json" add -k "app.welcome" -V "Welcome to our app"

# Results:
# en.json: "app.welcome": "Welcome to our app"
# de.json: "app.welcome": "[TODO] Welcome to our app"
# fr.json: "app.welcome": "[TODO] Welcome to our app"
```

#### Conflict Resolution Modes

Control how the add command handles existing keys:

```bash
# Skip existing keys (default)
goopt-i18n-gen -i "locales/*.json" add -k "app.title" -V "New Title"

# Replace existing keys
goopt-i18n-gen -i "locales/*.json" add -k "app.title" -V "New Title" -m replace

# Error on existing keys
goopt-i18n-gen -i "locales/*.json" add -k "app.title" -V "New Title" -m error
```

#### Dry Run for Safety

Always preview changes before applying them:

```bash
# See what would be changed without modifying files
goopt-i18n-gen -i "locales/*.json" add -F new-keys.json -n
```

### Tips

1. **Start Small**: Begin with one package or module
2. **Review Generated Keys**: Ensure the generated key names match your naming conventions
3. **Customize Prefixes**: Use `--key-prefix` to match your project structure
4. **Add to CI/CD**: Use `--strict` mode to ensure new code includes descKeys
5. **Document Your Process**: Add the goopt-i18n-gen commands to your project documentation
6. **Use add for Bulk Updates**: Collect new feature keys in JSON files and add them all at once
7. **Preview with Dry Run**: Always use `-n` flag first to review changes

## Related Topics

- [Error Handling]({{ site.baseurl }}/v2/guides/advanced-features/#error-handling) - Learn about structured error handling in goopt
- [Configuration]({{ site.baseurl }}/v2/configuration/index/) - External configuration and environment variables