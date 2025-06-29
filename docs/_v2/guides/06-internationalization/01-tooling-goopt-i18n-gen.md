---
layout: default
title: Tooling (goopt-i18n-gen)
parent: Internationalization
nav_order: 2
version: v2
---

# Tooling: `goopt-i18n-gen`

`goopt-i18n-gen` is a powerful command-line tool designed to automate and streamline the entire internationalization (i18n) workflow for `goopt` applications. It helps you add, manage, and validate translations with minimal effort.

### Key Features
- **Type-Safe Key Generation:** Converts your JSON translation files into type-safe Go constants, preventing typos and enabling autocompletion.
- **Source Code Auditing:** Scans your Go source files for `goopt` structs and automatically generate `descKey` tags.
- **Translation Stubbing:** Uses your existing `desc:"..."` tags to create initial translations in your JSON files.
- **String Extraction:** Analyzes your code to find hardcoded user-facing strings and helps you replace them with translation calls.
- **Validation:** Ensures that every `descKey` in your code has a corresponding entry in your translation files.

### Installation
```bash
go install github.com/napalu/goopt/v2/cmd/goopt-i18n-gen@latest
```
For a complete, runnable demonstration of the workflows described below, see the [i18n-codegen-demo example](https://github.com/napalu/goopt/tree/main/v2/examples/i18n-codegen-demo).

---

## Workflow 1: Adding i18n to an Existing `goopt` App

This is the recommended "360° workflow" for retrofitting an existing application that already uses `goopt` with `desc` tags.

### The Chicken-and-Egg Problem
If you manually add `descKey` tags to your code before the translations exist in your JSON files, `goopt` will display the raw keys (e.g., `app.flag.verbose_desc`) to the user. This workflow avoids that broken state.

**The Golden Rule: Translations first, `descKey` tags second!**

### Step-by-Step Guide

1.  **Initial State:** Start with a standard `goopt` struct using `desc` tags.
    ```go
    type AppConfig struct {
        Verbose bool `goopt:"short:v;desc:Enable verbose output"`
    }
    ```

2.  **Initialize and Generate Translations:** Use the `audit` command to find fields needing i18n and populate your locale file. **This step does not modify your Go code.**
    ```bash
    # Create an empty locales/en.json if it doesn't exist
    goopt-i18n-gen -i locales/en.json init

    # Analyze code and generate translations into the JSON file
    goopt-i18n-gen -i locales/en.json audit -d -g --key-prefix app
    ```
    Your `locales/en.json` file will now contain the generated translation:
    ```json
    {
      "app.app_config.verbose_desc": "Enable verbose output"
    }
    ```

3.  **Update Source Code with `descKey` Tags:** Now that the translations exist, it's safe to add the `descKey` tags. Use the `-u` (auto-update) flag to do this automatically.
    ```bash
    # This command modifies your Go source files.
    goopt-i18n-gen -i locales/en.json audit -d -u --key-prefix app
    ```
    Your struct is now safely updated:
    ```go
    type AppConfig struct {
        Verbose bool `goopt:"short:v;desc:Enable verbose output;descKey:app.app_config.verbose_desc"`
    }
    ```

4.  **Generate Type-Safe Constants:** Finally, generate the type-safe Go constants from your JSON file(s).
    ```bash
    goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages
    ```

---

## Workflow 2: Extracting Hardcoded Strings

For applications with hardcoded strings outside of `goopt` tags (e.g., in `fmt.Println` calls), use the `extract` command.

### Comment-Based Migration (Recommended)
This safe, iterative workflow adds `// i18n-todo:` comments to your code, allowing for manual review.

1.  **Extract and Comment:**
    ```bash
    goopt-i18n-gen -i "locales/*.json" extract
    ```
    Your code will be updated:
    ```go
    // Before:
    fmt.Println("Starting server...")

    // After:
    fmt.Println("Starting server...") // i18n-todo: app.extracted.starting_server
    ```

2.  **Review and Update:** Manually replace the commented lines with translation calls. You can mark them as `// i18n-done` or `// i18n-skip` to track progress.

3.  **Auto-Transform (Optional):** After your review, you can automatically transform all remaining `i18n-todo` comments into translation calls.
    ```bash
    goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T"
    ```

### Handling Structured Logging (slog)
When using structured loggers like `slog`, you need to prevent the extraction of logging keys (e.g., `"user_id"`, `"request_id"`). The most effective way is to skip all strings that don't contain whitespace.

```bash
# This command correctly extracts "User logged in" but skips "user_id".
goopt-i18n-gen -i "locales/*.json" extract -S "^[^\s]+$"
```
This pattern is highly recommended for any project using structured logging.

---

## Full Command Reference

### Global Options
*   `-i, --input`: Input JSON files (e.g., `"locales/*.json"`). **Required.**
*   `-v, --verbose`: Enable verbose output.
*   `-l, --language`: Set the tool's own output language (`en`, `de`, `fr`).

### `init`
Initializes empty translation files.
`goopt-i18n-gen -i locales/en.json init`

### `audit` 
Scans Go source files for `goopt` fields and helps generate `descKey` tags and translations.
*   `--files`: Go source files to scan (default: `*.go`).
*   `-d, --generate-desc-keys`: Generate suggestions for `descKey` tags.
*   `-g, --generate-missing`: Generate stub entries for missing keys in JSON files.
*   `-u, --auto-update`: **Modifies source files** to automatically add `descKey` tags.
*   `-n, --dry-run`: Preview changes without modifying files.
*   `--key-prefix`: Prefix for generated keys (e.g., `app`).
*   `--backup-dir`: Directory for backup files (default: `.goopt-i18n-backup`).

### `extract` Command
Extracts string literals from Go source files.
*   `-s, --files`: Go files to scan (default: `**/*.go`).
*   `-m, --match-only`: Regex to include only matching strings.
*   `-S, --skip-match`: Regex to exclude matching strings.
*   `-P, --key-prefix`: Prefix for generated keys (default: `app.extracted`).
*   `-l, --min-length`: Minimum string length (default: `2`).
*   `-u, --auto-update`: Update source files (adds comments or replaces strings).
*   `-n, --dry-run`: Preview changes without modifying files.
*   `--tr-pattern`: The translator call pattern to use for replacement (e.g., `tr.T`).
*   `--transform-mode`: What to transform: `user-facing` (default), `with-comments`, `all-marked`, `all`.
*   `--keep-comments`: Keep `i18n-*` comments after replacement.
*   `--clean-comments`: Remove all `i18n-*` comments from source files.
*   `--backup-dir`: Directory for backup files.
*   `--user-facing-regex`: Regex to identify custom user-facing functions.
*   `--format-function-regex`: Regex and argument index for custom format functions (e.g., `"MyLogf:1"`)

### `generate`
Generates a type-safe Go constants file.
*   `-o, --output`: Output Go file path. **Required.**
*   `-p, --package`: Go package name for the generated file.

### `validate`
Checks that all `descKey` tags in your code have translations.
*   `-s, --scan`: Go source files to scan.
*   `--strict`: Exit with a non-zero status code on failure (for CI/CD).

### `add`
Programmatically adds new keys to locale files.
*   `-k, --key`: The key to add.
*   `-V, --value`: The value for the key.
*   `-F, --from-file`: A JSON file containing key-value pairs to add.
*   `-m, --mode`: How to handle existing keys (`skip`, `replace`, `error`).

### `sync`
Ensures all locale files contain the same set of keys.
*   `-t, --target`: Target files to sync against the `-i` reference files.
*   `-r, --remove-extra`: Remove keys from target files that don't exist in the reference.