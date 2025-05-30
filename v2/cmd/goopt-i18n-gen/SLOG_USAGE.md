# Using Extract with slog and Structured Logging

When using the `extract` command with codebases that use structured logging (like `log/slog`), you'll want to exclude the logging field names from extraction since these are not user-facing strings that need translation.

## The Problem

Structured logging libraries like slog use key-value pairs:

```go
slog.Info("Starting server", "port", 8080, "host", "localhost")
slog.Error("Database error", "query", sql, "error", err)
```

Without filtering, the extract command would extract:
- ✅ "Starting server" (message - should be extracted)
- ❌ "port" (field name - should NOT be extracted)
- ❌ "host" (field name - should NOT be extracted) 
- ❌ "localhost" (value - should NOT be extracted)

## Solution: Use --skip-match

The `--skip-match` (or `-S`) flag accepts a regex pattern to exclude strings from extraction.

### Recommended Patterns

#### 1. Skip strings without spaces (Most Effective)
```bash
goopt-i18n-gen -i "locales/*.json" extract -S "^[^\s]+$"
```
This excludes any string that doesn't contain spaces. Since log messages typically contain spaces and field names don't, this works very well.

#### 2. Skip snake_case and single-word identifiers
```bash
goopt-i18n-gen -i "locales/*.json" extract -S "^[a-z]+(_[a-z]+)*$"
```
This excludes strings like `port`, `user_id`, `request_count`, etc.

#### 3. Skip common identifier patterns
```bash
goopt-i18n-gen -i "locales/*.json" extract -S "^[a-z][a-zA-Z0-9_]{0,20}$"
```
This excludes strings that look like variable names: starting with lowercase, containing only letters, numbers, and underscores.

## Examples

### Basic slog usage
```go
// Your code
slog.Info("User logged in", "user_id", userID, "ip", request.RemoteAddr)
slog.Error("Failed to save record", "error", err, "retry_count", retries)

// Extract with filtering
goopt-i18n-gen -i "locales/*.json" extract -S "^[^\s]+$"

// Results in extracting only:
// - "User logged in"
// - "Failed to save record"
// (Excludes: "user_id", "ip", "error", "retry_count")
```

### Custom loggers
```go
// Works with any logger following similar patterns
logger.Info("Processing payment", "amount", 99.99, "currency", "USD")
log.Error("API call failed", "endpoint", url, "status_code", resp.StatusCode)
```

### Combining with other filters
```bash
# Extract only messages with spaces, min length 5, exclude identifiers
goopt-i18n-gen -i "locales/*.json" extract \
  -m ".*\s+.*" \
  -l 5 \
  -S "^[a-z][a-zA-Z0-9_]*$"
```

## Advanced Patterns

For more complex filtering needs, you can create custom regex patterns:

```bash
# Exclude common log field names explicitly
goopt-i18n-gen -i "locales/*.json" extract \
  -S "^(error|err|host|port|user|user_id|id|status|code|duration|latency|method|path|query|request_id|timestamp|level|msg|message)$"

# Exclude camelCase identifiers
goopt-i18n-gen -i "locales/*.json" extract \
  -S "^[a-z][a-zA-Z0-9]*$"

# Exclude short strings (likely to be keys)
goopt-i18n-gen -i "locales/*.json" extract \
  -S "^.{1,10}$"
```

## Testing Your Pattern

Before running on your entire codebase, test your pattern:

```bash
# Dry run with verbose output
goopt-i18n-gen -v -i "locales/*.json" extract \
  -s "pkg/api/**/*.go" \
  -S "^[^\s]+$" \
  -n

# Review what would be extracted
# Adjust pattern if needed
```

## Best Practices

1. **Use the "no spaces" pattern as default**: `-S "^[^\s]+$"` works well for most codebases
2. **Combine with match-only**: Use `-m ".*\s+.*"` to only extract strings with spaces
3. **Test with dry-run first**: Always use `-n` to preview what will be extracted
4. **Document your pattern**: Add a comment in your project docs about which pattern you use

## Complete Example

```bash
# Extract user-facing strings from a slog-based application
goopt-i18n-gen -i "locales/*.json" extract \
  --files "**/*.go" \
  --match-only ".*\s+.*" \
  --skip-match "^[^\s]+$" \
  --min-length 3 \
  --key-prefix "app.messages" \
  --dry-run

# If satisfied, run without --dry-run
# Then optionally auto-update with comments
goopt-i18n-gen -i "locales/*.json" extract \
  --files "**/*.go" \
  --skip-match "^[^\s]+$" \
  --auto-update
```

This approach ensures that only actual user-facing messages are extracted for translation, while structured logging field names are preserved as-is.