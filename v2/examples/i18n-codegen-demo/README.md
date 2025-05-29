# goopt-i18n-gen Demo

This example demonstrates the "360° i18n workflow" using `goopt-i18n-gen` to automatically generate internationalization code for goopt applications.

## The 360° Workflow

1. **Start with a clean struct** - Define your configuration struct with goopt tags but WITHOUT descKey tags
2. **Generate descKey suggestions** - Run goopt-i18n-gen with `-d` flag to generate descKey tags and translation stubs
3. **Add descKey tags** - Update your struct with the suggested descKey tags
4. **Generate constants** - Run goopt-i18n-gen to generate type-safe translation constants

## Quick Start

### Using Make (Recommended)

```bash
# Complete 360° workflow demo
make demo-360

# Or step by step:
make init        # Create empty translation file
make analyze     # Find fields without descKeys
# ... add descKey tags to your struct ...
make generate    # Generate constants file
make validate    # Validate everything is set up correctly

# Or use the fully automated workflow:
make analyze-update  # Automatically add descKey tags to source files

# Working with multiple locales:
# Option 1: Wildcards (recommended)
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go
goopt-i18n-gen -i "locales/*.json" validate -s "*.go"

# Option 2: Comma-separated list
goopt-i18n-gen -i "locales/en.json,locales/de.json,locales/fr.json" generate -o messages/keys.go
```

### Using go:generate

The example includes several go:generate directives for different workflow stages:

```go
// Step 1: Initial analysis and stub generation
//go:generate goopt-i18n-gen -i locales/en.json audit -d -g --key-prefix app

// Step 2: Validation during development  
//go:generate goopt-i18n-gen -i locales/en.json validate -s main.go

// Step 3: Final generation
//go:generate goopt-i18n-gen -i locales/en.json generate -o messages/messages.go -p messages

// Step 4: CI/CD strict validation
//go:generate goopt-i18n-gen -i locales/en.json validate -s main.go --strict
```

Comment/uncomment the appropriate directive for your current workflow stage.

### Manual Steps

#### Step 1: Initial Setup

Use the init command to create the initial setup:

```bash
../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json init
```

Or manually:

```bash
mkdir -p locales
echo '{}' > locales/en.json
```

**Note:** Fields only need `goopt` tags - the `desc` attribute is optional. If a field doesn't have a `desc`, the tool will generate a sensible default translation based on the field name.

### Step 2: Generate descKey Suggestions

Run goopt-i18n-gen to analyze your struct and generate descKey suggestions:

```bash
# From the i18n-codegen-demo directory
../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json audit \
  -d -g \
  --key-prefix app

# This will:
# - Scan *.go files (default) for goopt fields without descKey tags
# - Generate suggested descKey values
# - Create translation stubs in locales/en.json
# - Show you exactly which tags to add to your struct

# To audit specific files:
../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json audit \
  -d -g \
  --key-prefix app \
  --files "main.go,config.go"
```

### Step 3: Add descKey Tags

You can either manually add the suggested descKey tags or use the `-u` flag for automatic updates:

#### Option A: Manual Update

Follow the suggestions from the tool output. For example, if it suggests:

```
In main.go:18, update the goopt tag to include:
  descKey:app.app_config.verbose_desc
```

Update your struct field:

```go
// Before (with desc):
Verbose bool `goopt:"short:v;desc:Enable verbose output"`

// After:
Verbose bool `goopt:"short:v;desc:Enable verbose output;descKey:app.app_config.verbose_desc"`

// Or without desc (tool generates default):
Verbose bool `goopt:"short:v;descKey:app.app_config.verbose_desc"`
```

#### Option B: Automatic Update

Use the `-u` flag to automatically update your source files:

```bash
../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json audit \
  -d -g -u \
  --key-prefix app
```

This will:
- Create timestamped backups of your source files
- Automatically insert the descKey tags into your struct fields
- Update both field tags and command struct tags
- Preserve all existing tag attributes

### Step 4: Generate Translation Constants

Once all descKey tags are added, generate the constants file:

```bash
go generate
# Or run directly:
../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json generate -o messages/messages.go -p messages
```

### Step 5: Use in Your Code

```go
import "github.com/napalu/goopt/v2/examples/i18n-codegen-demo/messages"

// In your command functions:
fmt.Println(cfg.TR.T(messages.Keys.AppAppConfig.VerboseDesc))
```

## Building and Running

```bash
# Install dependencies
go mod tidy

# Build
go build -o i18n-codegen-demo .

# Run commands
./i18n-codegen-demo process -i input.txt -f json -c
./i18n-codegen-demo process -i input.txt validate -s
./i18n-codegen-demo convert -f json -t xml
```

## Benefits

1. **Type Safety** - Generated constants prevent typos in translation keys
2. **Compile-Time Checking** - Missing or renamed keys are caught at compile time
3. **IDE Support** - Full autocomplete for translation keys
4. **Workflow Automation** - Easily find missing translations and generate stubs
5. **CI/CD Integration** - Use `--strict` flag to enforce all keys have translations
6. **Multi-Locale Support** - Process all locale files at once with wildcards (e.g., `locales/*.json`)

## Adding New Languages

1. Copy your base translation file:
   ```bash
   cp locales/en.json locales/es.json
   ```

2. Translate the values in the new file

3. **Important**: Regenerate constants including ALL locale files:
   ```bash
   # This ensures the generated code includes keys from all locales
   goopt-i18n-gen -i "locales/*.json" generate -o messages/messages.go -p messages
   ```

4. The generated code automatically works with all available translations

### Why Include All Locales in Generation?

When you run the generate command with multiple locale files:
- The tool merges ALL unique keys from ALL files
- This ensures your constants include every possible translation key
- Prevents runtime errors if one locale has additional keys
- Supports locale-specific features (e.g., a key that only exists in certain languages)

## Tips

- Use meaningful key prefixes (e.g., `app`, `cli`, your app name)
- Group related functionality with nested structs for better key organization
- Run with `-v` flag to validate all descKey references have translations
- Use `--strict` in CI/CD to ensure no missing translations