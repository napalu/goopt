---
layout: default
title: Version Support
parent: Built-in Features
nav_order: 2
version: v2
---

# Version Support

The goopt v2 version support feature provides automatic version flag registration and display, eliminating boilerplate version handling code in CLI applications.

## Overview

With version support enabled (the default), goopt automatically:
- Registers `--version` and `-v` flags if not already defined
- Displays version information when these flags are used
- Respects user-defined flags that conflict with version flags
- Optionally shows version in help output

## Basic Usage

### Simple Static Version

```go
parser, err := goopt.NewParserWith(
    goopt.WithVersion("1.2.3"),
)

// User runs: myapp --version
// Output: 1.2.3
```

### Dynamic Version with Build Information

```go
// Build-time variables (set via ldflags)
var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildTime = "unknown"
)

parser, err := goopt.NewParserWith(
    goopt.WithVersionFunc(func() string {
        return fmt.Sprintf("%s (commit: %s, built: %s)",
            Version, GitCommit, BuildTime)
    }),
)

// User runs: myapp --version
// Output: 1.2.3 (commit: abc123, built: 2024-01-20T15:30:00Z)
```

### Custom Version Formatter

```go
parser, err := goopt.NewParserWith(
    goopt.WithVersion("1.0.0"),
    goopt.WithVersionFormatter(func(version string) string {
        return fmt.Sprintf(`MyApp v%s
Copyright (c) 2024 MyCompany
License: MIT
Homepage: https://github.com/mycompany/myapp`, version)
    }),
)

// User runs: myapp --version
// Output:
// MyApp v1.0.0
// Copyright (c) 2024 MyCompany
// License: MIT
// Homepage: https://github.com/mycompany/myapp
```

## Configuration Options

### WithVersion
Sets a static version string and enables auto-version:

```go
WithVersion("1.2.3")
```

### WithVersionFunc
Sets a function to dynamically generate version info:

```go
WithVersionFunc(func() string {
    return getVersionFromBuildInfo()
})
```

### WithVersionFormatter
Customizes the version output format:

```go
WithVersionFormatter(func(version string) string {
    return fmt.Sprintf("=== %s ===", version)
})
```

### WithVersionFlags
Sets custom version flag names (default: "version", "v"):

```go
// Use --ver and -V instead of defaults
WithVersionFlags("ver", "V")
```

### WithShowVersionInHelp
Shows version information in help output header:

```go
WithShowVersionInHelp(true)

// Help output will start with:
// myapp 1.2.3
//
// Usage: myapp [options] command
```

### WithAutoVersion
Disables automatic version flag registration:

```go
WithAutoVersion(false)
```

## Advanced Features

### Build-Time Version Injection

The recommended approach for production builds:

```bash
# Build script
VERSION=$(git describe --tags --always)
COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +%FT%TZ)

go build -ldflags "\
    -X main.Version=$VERSION \
    -X main.GitCommit=$COMMIT \
    -X main.BuildTime=$BUILD_TIME" \
    -o myapp
```

### Handling Flag Conflicts

If the user defines `-v` for verbose, goopt automatically handles it:

```go
parser, err := goopt.NewParserWith(
    goopt.WithVersion("1.0.0"),
    goopt.WithFlag("verbose", goopt.NewArg(
        goopt.WithType(types.Standalone),
        goopt.WithShortFlag("v"),
    )),
)

// -v maps to verbose
// --version still shows version
```

### User-Defined Version Flags

Users can define their own version handling:

```go
type Config struct {
    Version bool `goopt:"name:version;short:V;desc:Print version"`
}

cfg := &Config{}
parser, _ := goopt.NewParserFromStruct(cfg,
    goopt.WithVersion("1.0.0"), // Still set version for other uses
)

// Auto-version won't trigger since user defined the flag
if cfg.Version {
    // User handles version display
    fmt.Printf("MyApp %s\n", parser.GetVersion())
}
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
- name: Build with version
  run: |
    VERSION="${{ github.ref_name }}"
    if [[ "$VERSION" != v* ]]; then
      VERSION="dev-$(git rev-parse --short HEAD)"
    fi
    go build -ldflags "\
      -X main.Version=$VERSION \
      -X main.GitCommit=${{ github.sha }} \
      -X main.BuildTime=$(date -u +%FT%TZ)"
```

### Makefile Example

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u +%FT%TZ)

LDFLAGS := -ldflags "\
    -X main.Version=$(VERSION) \
    -X main.GitCommit=$(COMMIT) \
    -X main.BuildTime=$(BUILD_TIME)"

build:
    go build $(LDFLAGS) -o myapp
```

## Best Practices

1. **Use ldflags for production** - Inject real version info at build time
2. **Keep -v for verbose** - If verbose is common in your domain, use only --version
3. **Include build metadata** - Commit hash and build time help with debugging
4. **Use semantic versioning** - Follow v1.2.3 format for consistency
5. **Show version in help** - Helps users verify they're using the right version

## Complete Example

```go
package main

import (
    "fmt"
    "os"
    "runtime"
    
    "github.com/napalu/goopt/v2"
)

// Build-time variables
var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildTime = "unknown"
)

func main() {
    type Config struct {
        Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
        Config  string `goopt:"short:c;desc:Configuration file"`
        
        Server struct {
            Port int `goopt:"short:p;default:8080;desc:Server port"`
        } `goopt:"kind:command;desc:Server management"`
    }
    
    cfg := &Config{}
    parser, err := goopt.NewParserFromStruct(cfg,
        goopt.WithVersionFunc(func() string {
            return fmt.Sprintf("%s (%s/%s, commit: %s, built: %s)",
                Version,
                runtime.GOOS,
                runtime.GOARCH,
                GitCommit,
                BuildTime)
        }),
        goopt.WithShowVersionInHelp(true),
    )
    
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    if !parser.Parse(os.Args) {
        parser.PrintHelp(os.Stderr)
        os.Exit(1)
    }
    
    // Version is handled automatically by goopt
}
```

## Comparison with Other CLI Libraries

Unlike many CLI libraries that require manual version flag setup:

```go
// Other libraries often require:
rootCmd.Version = "1.2.3"
rootCmd.SetVersionTemplate("...")
rootCmd.Flags().BoolP("version", "v", false, "version")

// goopt v2:
WithVersion("1.2.3") // That's it!
```

The version support in goopt v2 is designed to be zero-configuration for common cases while providing full flexibility when needed.