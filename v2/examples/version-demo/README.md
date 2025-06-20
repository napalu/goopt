# Version Support Demo

This example demonstrates goopt v2's comprehensive version support features.

## Features Demonstrated

1. **Simple Static Version** - Basic version string
2. **Dynamic Version** - Version generated at runtime
3. **Custom Formatter** - Formatted version output
4. **Version in Help** - Show version in help header
5. **Custom Flags** - Use different flags for version
6. **Build-time Injection** - Using ldflags for version info

## Running the Examples

### Interactive Demo
```bash
go run main.go
```

### Real CLI Usage
```bash
# Show version
go run main.go --version

# Show help (with version in header)
go run main.go --help

# Run commands
go run main.go server start -p 8080
```

## Building with Version Info

The recommended approach is to inject version information at build time:

```bash
# Using the build script
./build.sh

# Or manually with ldflags
go build -ldflags "\
    -X main.Version=1.0.0 \
    -X main.GitCommit=$(git rev-parse HEAD) \
    -X main.BuildTime=$(date -u +%FT%TZ)" \
    -o myapp
```

## Version Configuration Options

```go
// Simple static version
WithVersion("1.2.3")

// Dynamic version function
WithVersionFunc(func() string {
    return fmt.Sprintf("%s-%s", version, commit)
})

// Custom output format
WithVersionFormatter(func(v string) string {
    return fmt.Sprintf("MyApp %s\nLicense: MIT", v)
})

// Custom flags (instead of --version/-v)
WithVersionFlags("ver", "V")

// Show in help header
WithShowVersionInHelp(true)

// Disable auto-version
WithAutoVersion(false)
```

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
- name: Build with version
  run: |
    VERSION=$(git describe --tags --always)
    COMMIT=$(git rev-parse --short HEAD)
    go build -ldflags "\
      -X main.Version=$VERSION \
      -X main.GitCommit=$COMMIT \
      -X main.BuildTime=$(date -u +%FT%TZ)"
```

## Best Practices

1. **Use ldflags** for production builds to inject real version info
2. **Keep -v for verbose** if it's a common flag in your domain
3. **Use version functions** for complex version strings
4. **Show version in help** for better user experience
5. **Include build metadata** like commit hash and build time