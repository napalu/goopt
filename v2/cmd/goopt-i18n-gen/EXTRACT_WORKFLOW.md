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

# Minimum length to avoid single characters
goopt-i18n-gen -i "locales/*.json" extract -l 5

# Combine filters
goopt-i18n-gen -i "locales/*.json" extract -m ".*\\s+.*" -S "^TEST_" -l 3
```

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

## Auto-Update Workflows

### Comment-Based Workflow (Recommended for Large Codebases)

This approach adds comments next to strings, allowing manual review before transformation.

```bash
# Step 1: Add TODO comments
goopt-i18n-gen -i "locales/*.json" extract -u

# Your code now looks like:
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

### Direct Transformation Workflow (For Smaller Codebases)

Replace strings directly with translation calls:

```bash
# Preview what will be changed
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T" -n

# Apply transformations
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T"

# Keep comments for documentation
goopt-i18n-gen -i "locales/*.json" extract -u --tr-pattern "tr.T" --keep-comments
```

## Format Function Handling

The extract command intelligently transforms format functions:

### Transformations Applied

```go
// Printf-style → Print with tr.T
fmt.Printf("User %s logged in", name)
// becomes:
fmt.Print(tr.T(messages.Keys.AppExtracted.UserSLoggedIn, name))

// Sprintf → Direct tr.T call
msg := fmt.Sprintf("Welcome %s!", name)
// becomes:
msg := tr.T(messages.Keys.AppExtracted.WelcomeS, name)

// Errorf → errors.New with tr.T
return fmt.Errorf("failed to connect: %v", err)
// becomes:
return errors.New(tr.T(messages.Keys.AppExtracted.FailedToConnectV, err))

// Errorf with %w → fmt.Errorf with tr.T (preserves error wrapping)
return fmt.Errorf("connection failed: %w", err)
// becomes:
return fmt.Errorf(tr.T(messages.Keys.AppExtracted.ConnectionFailedW), err)
```

### String Concatenation

The extractor detects concatenated strings in format functions:

```go
// Concatenated format strings are extracted as one
fmt.Printf("User: %s" + " Status: %s", user, status)
// Extracted as: "User: %s Status: %s"
// Transformed to:
fmt.Print(tr.T(messages.Keys.AppExtracted.UserSStatusS, user, status))
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

### 3. Handle Special Cases

Some strings shouldn't be translated:
- Technical identifiers
- Protocol strings
- Debug output

Mark these with `// i18n-skip` during the comment phase.

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