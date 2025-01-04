---
layout: default
title: Positional Arguments
parent: Guides
nav_order: 3
---

# Positional Arguments

Goopt provides robust support for positional arguments, allowing you to enforce specific positions for command-line arguments while maintaining flexibility.

## Overview

Positional arguments are command-line arguments that must appear in specific positions relative to flags and commands. This is useful for:
- Enforcing input/output file ordering
- Maintaining compatibility with existing scripts
- Creating intuitive command-line interfaces

## Basic Usage

### Defining Positional Arguments

```go
parser := goopt.NewParser()

// Source file must be first
parser.AddFlag("source", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(0),
))

// Output file must be last
parser.AddFlag("output", goopt.NewArg(
    goopt.WithPosition(goopt.AtEnd),
    goopt.WithRelativeIndex(0),
))
```

### Using Positional Arguments

```bash
# Correct usage
myapp source.txt --verbose --format json output.txt

# Incorrect usage (source not at start)
myapp --verbose source.txt --format json output.txt
```

## Position Types

### AtStart
- Must appear before any flags or commands
- Ordered by PositionalIndex
- Example: Source files, configuration files

### AtEnd
- Must appear after all flags and commands
- Ordered by PositionalIndex
- Example: Output files, destination paths

## Struct Tag Support

Positional arguments can be specified using struct tags:

The position tag uses a brace-enclosed format with two optional fields:
- `at`: Position type (`start` or `end`)
- `idx`: Relative index within the position (zero-based)

```go
type Config struct {
    Source string `goopt:"name:source;pos:{at:start,idx:0}"` // Must be first argument
    Profile string `goopt:"name:profile;pos:{at:start,idx:1}"` // Must be second argument
    Dest   string `goopt:"name:dest;pos:{at:end,idx:0}"`   // Must be last argument
}
```

## Advanced Features

### Multiple Ordered Arguments

You can specify multiple arguments at the same position type using indices:

```go
parser.AddFlag("config", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(0),
))
parser.AddFlag("profile", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(1),
))

// Usage: myapp config.yaml profile.json --verbose
```

### Flag Override

Position requirements can be overridden using flag syntax:

```go
parser.AddFlag("source", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(0),
))

// Both are valid:
// myapp source.txt --verbose
// myapp --source source.txt --verbose
```

### Mixed Positional and Regular Arguments

You can mix positioned and regular arguments:

```go
parser := goopt.NewParser()

// Config must be first
parser.AddFlag("config", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(0),
))

// Output must be last
parser.AddFlag("output", goopt.NewArg(
    goopt.WithPosition(goopt.AtEnd),
    goopt.WithRelativeIndex(0),
))

// Any other arguments are captured as regular positional arguments
// myapp config.yaml data1.txt data2.txt --verbose output.txt
```

### Multiple Ordered Arguments

You can specify multiple arguments at the same position type using relative indices:

```go
parser.AddFlag("config", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(0),  // First at start
))
parser.AddFlag("profile", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(1),  // Second at start
))

// Usage: myapp config.yaml profile.json --verbose
```

## Error Handling

Goopt provides clear error messages for position violations:

```go
if !parser.Parse(os.Args[1:]) {
    for _, err := range parser.GetErrors() {
        fmt.Println("Error:", err)
        // Example: "Error: argument 'source' must appear at start position"
        // Example: "Error: argument 'output' must appear at end position"
    }
}
```

## Best Practices

1. **Clear Positions**: Use positional arguments when the order is meaningful to users
2. **Flexible Override**: Allow flag syntax override for scripting and automation
3. **Documentation**: Clearly document position requirements in help text
