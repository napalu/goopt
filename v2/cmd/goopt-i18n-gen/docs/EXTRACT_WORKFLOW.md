# Extract Command Workflow Guide

The `extract` command is a powerful tool for migrating existing Go codebases to internationalization. It provides multiple approaches to gradually transform hardcoded strings into translatable content.

## Overview

The extract command can:
1. **Scan** Go files to find string literals
2. **Filter** strings using regex patterns and length requirements  
3. **Extract** strings to locale JSON files with generated keys
4. **Transform** source code by adding comments or replacing strings
5. **Handle** format functions intelligently (Printf, Sprintf, Errorf, etc.)

## Basic Usage

### 1. Discovery Phase

First, understand what strings exist in your codebase:

```bash
# See all strings (dry run)
goopt-i18n-gen -i "locales/*.json" extract -n

# See all strings with locations (verbose)
goopt-i18n-gen -v -i "locales/*.json" extract -n

# Count total strings
goopt-i18n-gen -i "locales/*.json" extract -n | grep "occurrences" | wc -l
```

### 2. Filtering Strategies

```bash
# Only user-visible strings (containing spaces)
goopt-i18n-gen -i "locales/*.json" extract -m ".*\\s+.*"

# Only error messages
goopt-i18n-gen -i "locales/*.json" extract -m "(?i)error|fail|unable|cannot"

# Exclude debug/test strings
goopt-i18n-gen -i "locales/*.json" extract -S "(?i)debug|test|todo"

# Exclude slog/structured logging field names (recommended for slog users)
goopt-i18n-gen -i "locales/*.json" extract -S "^[^\s]+$"

# Minimum length to avoid single characters
goopt-i18n-gen -i "locales/*.json" extract -l 5

# Combine filters
goopt-i18n-gen -i "locales/*.json" extract -m ".*\\s+.*" -S "^TEST_" -l 3
```

**Note for slog users**: See [SLOG_USAGE.md](SLOG_USAGE.md) for detailed patterns to exclude structured logging field names.

### 3. Extraction and Key Generation

```bash
# Extract with default prefix (app.extracted)
goopt-i18n-gen -i "locales/*.json" extract

# Extract with custom prefix
goopt-i18n-gen -i "locales/*.json" extract -P app.ui.labels

# Extract from specific files/packages
goopt-i18n-gen -i "locales/*.json" extract -s "cmd/**/*.go" -P app.cli
goopt-i18n-gen -i "locales/*.json" extract -s "internal/api/**/*.go" -P app.api
```

## Transform Modes and User-Facing Functions

The extract command supports different modes for determining which strings to transform:

### Transform Modes

- **`user-facing`** (default): Only transforms strings in known user-facing functions
- **`with-comments`**: Only transforms strings marked with `i18n-todo` comments  
- **`all-marked`**: Transforms both user-facing functions AND strings with `i18n-todo` comments
- **`all`**: Transforms all strings that have translation keys

### Custom User-Facing Functions

By default, the tool recognizes standard user-facing functions like `fmt.Print*`, `log.*`, and common logging methods. You can extend this with regex patterns:

```bash
# Mark custom logger methods as user-facing (multiple patterns allowed)
goopt-i18n-gen -i "locales/*.json" extract -u \
  --user-facing-regex ".*\.MsgAll$" \
  --user-facing-regex ".*\.LogUser$"

# Multiple patterns for custom loggers
goopt-i18n-gen -i "locales/*.json" extract -u \
  --user-facing-regex ".*\.(Log|Print|Display|Show|Render).*" \
  --user-facing-regex ".*\.(Info|Warn|Error)$"
```

### Custom Format Functions

For functions that take format strings (like `Printf`), use `--format-function-regex` with the pattern and argument index:

```bash
# Pattern:index format (0-based index)
goopt-i18n-gen -i "locales/*.json" extract -u \
  --format-function-regex ".*\.MsgAll$:1" \
  --format-function-regex ".*\.Logf$:0"
```

#### Complete Example

```go
// Custom audit logger
type AuditLogger struct {}

// MsgAll takes: fields (index 0), format string (index 1), args...
func (l *AuditLogger) MsgAll(fields map[string]interface{}, format string, args ...interface{}) {
    // Custom logging implementation
}

// Usage
s.Log.MsgAll(s.auditFields, "disabled user %s during sync", user.Name)
```

To properly handle this custom format function:

```bash
goopt-i18n-gen -i "locales/*.json" extract -u \
  --user-facing-regex ".*\.MsgAll$" \
  --format-function-regex ".*\.MsgAll$:1"
```

This tells the tool:
- `--user-facing-regex ".*\.MsgAll$"` - Functions matching `MsgAll` are user-facing
- `--format-function-regex ".*\.MsgAll$:1"` - MsgAll is a format function with format string at index 1

The transformation will change:
```go
s.Log.MsgAll(fields, "User %s disabled", username)
// becomes:
s.Log.MsgAll(fields, tr.T(messages.Keys.UserSDisabled, username))
```

## Auto-Update Workflows

### Comment-Based Workflow (Recommended for Large Codebases)

This approach adds comments next to strings, allowing manual review before transformation.

```bash
# Step 1: Add TODO comments (NO -u flag!)
goopt-i18n-gen -i "locales/*.json" extract

# Your code now looks like:
# fmt.Println("Starting server...") // i18n-todo: app.extracted.starting_server

# Step 2: Review and manually update high-priority strings
# Change some to:
# fmt.Println(tr.T(messages.Keys.App.Server.Starting)) // i18n-done

# Step 3: Mark strings to skip
# fmt.Println("DEBUG: raw data") // i18n-skip

# Step 4: Auto-transform source code (WITH -u flag!)
goopt-i18n-gen -i "locales/*.json" extract -u

# Step 5: Clean up any remaining comments
goopt-i18n-gen -i "locales/*.json" extract --clean-comments
```

### Direct Transformation Workflow (For Smaller Codebases)

Replace strings directly with translation calls:

```bash
# Preview what will be changed
goopt-i18n-gen -i "locales/*.json" extract -u -n

# Apply transformations (uses tr.T by default)
goopt-i18n-gen -i "locales/*.json" extract -u

# Use custom translator pattern if needed
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "myApp.Translate"

# Keep i18n comments for documentation after transformation
goopt-i18n-gen -i "locales/*.json" extract -u --keep-comments
```

## Format Function Handling

The extract command intelligently transforms format functions:

### Transformations Applied

```go
// Printf-style → Print with tr.T
fmt.Printf("User %s logged in", name)
// becomes:
fmt.Print(tr.T(messages.Keys.App.Extracted.UserSLoggedIn, name))

// Sprintf → Direct tr.T call
msg := fmt.Sprintf("Welcome %s!", name)
// becomes:
msg := tr.T(messages.Keys.App.Extracted.WelcomeS, name)

// Errorf → errors.New with tr.T
return fmt.Errorf("failed to connect: %v", err)
// becomes:
return errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnectV, err))

// Errorf with %w → fmt.Errorf with tr.T (preserves error wrapping)
return fmt.Errorf("connection failed: %w", err)
// becomes:
return fmt.Errorf(tr.T(messages.Keys.App.Extracted.ConnectionFailedW), err)
```

### String Concatenation

The extractor detects concatenated strings in format functions:

```go
// Concatenated format strings are extracted as one
fmt.Printf("User: %s" + " Status: %s", user, status)
// Extracted as: "User: %s Status: %s"
// Transformed to:
fmt.Print(tr.T(messages.Keys.App.Extracted.UserSStatusS, user, status))
```

## Package-by-Package Migration

For large codebases, migrate one package at a time:

```bash
# 1. Start with the UI package
goopt-i18n-gen -i "locales/*.json" extract -s "internal/ui/**/*.go" -P app.ui -u

# 2. Move to API responses
goopt-i18n-gen -i "locales/*.json" extract -s "internal/api/**/*.go" -P app.api -u

# 3. Handle CLI messages
goopt-i18n-gen -i "locales/*.json" extract -s "cmd/**/*.go" -P app.cli -u

# 4. Generate constants after each package
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go
```

## Best Practices

### 1. Use Meaningful Prefixes

```bash
# Group by feature
extract -P app.auth         # Authentication strings
extract -P app.ui.forms     # Form labels and validation
extract -P app.errors       # Error messages
extract -P app.cli.help     # CLI help text
```

### 2. Review Before Transformation

```bash
# Always dry-run first
goopt-i18n-gen -v -i "locales/*.json" extract -u --tr-pattern "tr.T" -n

# Check the diff
git diff
```

### 3. Handle Special Cases with i18n-skip

Some strings shouldn't be translated. Use the `// i18n-skip` directive to exclude them:

#### Basic Usage

```go
// Inline skip - affects only the current line
query := "SELECT * FROM users WHERE id = ?" // i18n-skip
msg := "User found"  // This will be extracted

// Comment before - affects the next line
// i18n-skip
apiKey := "sk-1234567890abcdef"
msg := "API initialized"  // This will be extracted
```

#### Common Use Cases

```go
// SQL queries
// i18n-skip
db.Query(`
    SELECT u.id, u.name, u.email
    FROM users u
    WHERE u.active = true
`)

// Regular expressions
// i18n-skip
emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`

// Configuration keys
config.Get("database.pool.maxSize") // i18n-skip

// Technical identifiers
const APIVersion = "v2.1.0" // i18n-skip

// Template syntax
// i18n-skip
tmpl := "{{.Name}} - {{.Date}}"
```

#### Multi-line Support

```go
// String concatenation
// i18n-skip
query := "SELECT u.id, u.name, u.email " +
    "FROM users u " +
    "JOIN orders o ON u.id = o.user_id " +
    "WHERE u.active = true"

// Format functions
// i18n-skip
log.Printf("Debug: User %s performed action %s at %s",
    username, action, timestamp)
```

**Note**: The directive is case-insensitive and works with block comments (`/* i18n-skip */`). For patterns like slog field names, consider using `--skip-match "^[^\s]+$"` instead of marking each one individually.

### 4. Backup Your Code

The tool creates backups by default in `.goopt-i18n-backup/`, but also:

```bash
# Commit before major transformations
git add -A && git commit -m "Before i18n transformation"

# Use a branch
git checkout -b feature/i18n-migration
```

## Complete Example

Here's a full migration for a hypothetical web service:

```bash
# 1. Initial extraction and review
goopt-i18n-gen -v -i "locales/*.json" extract -m ".*\\s+.*" -l 3 -n > strings-to-review.txt

# 2. Add TODO comments for review
goopt-i18n-gen -i "locales/*.json" extract -m ".*\\s+.*" -l 3 -u
git add -A && git commit -m "Add i18n TODO comments"

# 3. Manual review and updates
# - Change some TODOs to i18n-done after manual translation
# - Mark technical strings as i18n-skip
# - Adjust keys in locale files if needed

# 4. Transform API package first (highest priority)
goopt-i18n-gen -i "locales/*.json" extract -s "internal/api/**/*.go" -u --tr-pattern "tr.T"
go test ./internal/api/...  # Ensure nothing broke

# 5. Transform remaining packages
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T"

# 6. Generate constants
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go

# 7. Clean up
goopt-i18n-gen -i "locales/*.json" extract --clean-comments
go fmt ./...
go test ./...

# 8. Commit
git add -A && git commit -m "Complete i18n migration"
```

## Troubleshooting

### Missing Imports

If the transformation adds `tr.T()` calls but doesn't import the translator, you'll need to:

1. Add the import manually to affected files
2. Initialize the translator variable (usually in main or init)

### Key Naming

Generated keys use snake_case and are limited to 50 characters. You can:
- Use custom prefixes to organize keys better
- Manually edit keys in JSON files after extraction
- Use shorter, more meaningful strings in your code

### Performance

For very large codebases:
- Process packages separately to avoid memory issues
- Use specific file patterns instead of `**/*.go`
- Run extraction during off-hours if it takes long

## Summary

The extract command provides a flexible, safe approach to i18n migration:

1. **Discover** strings with dry-run and filters
2. **Plan** your migration strategy (comments vs. direct)
3. **Execute** incrementally, package by package
4. **Verify** with tests after each transformation
5. **Clean up** comments and generate final constants

The comment-based workflow is recommended for most projects as it allows human review and incremental migration without breaking existing functionality.