---
layout: default
title: Positional Arguments
parent: Guides
nav_order: 4
---

# Positional Arguments

Goopt provides robust support for positional arguments, allowing you to specify exact positions for command-line arguments.

## Overview

Positional arguments are command-line arguments that must appear in specific positions. This is useful for:
- Enforcing input/output file ordering
- Creating intuitive command-line interfaces
- Maintaining compatibility with existing scripts

## Basic Usage

### Using Struct Tags (Recommended)

The simplest way to define positional arguments is using struct tags:

```go
type Config struct {
    Source      string `goopt:"pos:0;required:true"`      // First argument
    Destination string `goopt:"pos:1"`                    // Second argument
    Optional    string `goopt:"pos:2;default:backup.txt"` // Third argument with default
}

var cfg Config
parser, err := goopt.NewParserFromStruct(&cfg)
```

### Programmatic API

You can also define positions programmatically:

```go
parser := goopt.NewParser()

// Add positional arguments
parser.AddFlag("source", goopt.NewArg(
    goopt.WithPosition(0),
    goopt.WithRequired(true),
))
parser.AddFlag("dest", goopt.NewArg(
    goopt.WithPosition(1),
))
```

## Advanced Features

### Gaps in Positions

You can leave gaps between positions:

```go
type Config struct {
    First     string `goopt:"pos:0"`           // First argument
    Last      string `goopt:"pos:10"`          // Much later argument
    VeryLast  string `goopt:"pos:100"`         // Even later
}
```

### Optional Arguments with Defaults

Positional arguments can have default values:

```go
type Config struct {
    Required string `goopt:"pos:0;required:true"`      // Must be provided
    Optional string `goopt:"pos:1;default:fallback"`   // Uses default if missing
}
```

### Mixed Positional and Regular Arguments

Unbound arguments preserve their relative positions:

```go
// Command: myapp source.txt extra1 extra2 dest.txt
type Config struct {
    Source string `goopt:"pos:0"`  // Gets "source.txt"
    Dest   string `goopt:"pos:3"`  // Gets "dest.txt"
}
// "extra1" and "extra2" are available as unbound positional arguments
```

## Error Handling

Goopt provides clear error messages for position violations:

```go
if !parser.Parse(os.Args[1:]) {
    for _, err := range parser.GetErrors() {
        fmt.Println("Error:", err)
        // Example: "Error: missing required positional argument 'source' at position 0"
    }
}
```

## Best Practices

1. **Use Sequential Positions**: When possible, use consecutive positions (0, 1, 2...)
2. **Required First**: Place required positional arguments before optional ones
3. **Default Values**: Provide defaults for optional positions when it makes sense
4. **Documentation**: Clearly document position requirements in help text
5. **Reasonable Gaps**: While gaps are allowed, keep them small unless there's a good reason

## Accessing Positional Arguments

You can access all positional arguments, including unbound ones:

```go
// After parsing
args := parser.GetPositionalArgs()
for _, arg := range args {
    fmt.Printf("Position %d: %s\n", arg.Position, arg.Value)
}
```
