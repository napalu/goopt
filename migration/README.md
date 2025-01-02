# goopt Migration Tools

This package provides tools to help migrate from the legacy goopt tag format to the new format.
It will be removed in a future version once the migration period is over.

## Usage

### CLI Tool

Install the migration tool:
```bash
go install github.com/your/repo/goopt/migration/cmd/goopt-migrate@latest
```

Convert a single file:
```bash
goopt-migrate -f path/to/file.go
```

Convert a directory:
```bash
goopt-migrate -d path/to/dir
```

Options:
- `-f, --file`: Process a single file
- `-d, --dir`: Process a directory (recursively)
- `--dry-run`: Show what would be changed without making changes
- `-v, --verbose`: Show detailed progress

### API Usage

```go
package main
import "github.com/napalu/goopt/migration"

// Convert a single file
err := migration.ConvertFile("path/to/file.go")

// Convert a directory
err := migration.ConvertDir("path/to/dir")
```

## Migration Period

This package will be removed in v2.0.0. Please ensure all your code is migrated before upgrading.

## Manual Migration

If you prefer to migrate manually, replace legacy tags like:
```go
`long:"output" short:"o" description:"Output file"`
```

With the new format:
```go
`goopt:"name:output;short:o;desc:Output file"`
```