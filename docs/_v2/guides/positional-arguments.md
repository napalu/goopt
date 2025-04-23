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

## Mixing Positional Arguments and Named Flags

Goopt seamlessly handles command lines that contain both positional arguments (defined with `pos:N`) and named flags (e.g., `--verbose`, `--output file`).

**Parsing Order and Precedence:**

Understanding how these interact is key:

1.  **Flags First:** The parser first scans the arguments to identify all named flags (like `--output`) and their corresponding values (`file.txt`). These tokens are consumed by the flag parsing logic.
2.  **Positional Candidates:** Any arguments *not* consumed as flags or their values become candidates for positional binding.
3.  **Relative Matching:** These remaining arguments are matched against the defined positional arguments (`pos:0`, `pos:1`, `pos:2`, etc.) based on their *relative order*. The first remaining argument tries to match `pos:0`, the second remaining argument tries to match `pos:1`, and so on.
4.  **Explicit Flag Precedence:** If a single configuration item (e.g., a specific struct field) is defined with *both* a `pos:N` tag *and* a name (`name:myarg` or inferred name), providing the value via the named flag (`--myarg value`) **always takes precedence**.
    *   The value from the explicit flag (`value`) will be bound to the struct field.
    *   The parser will *not* attempt to bind an argument from the command line's Nth positional slot *to that specific field*. That positional argument might become an unbound positional or match a *different* field tagged with `pos:N+M`.

**Example:**

Consider this configuration:

```go
package main

import (
	"fmt"
	"os"
	"github.com/napalu/goopt/v2"
)

type Config struct {
	InputFile  string `goopt:"pos:0;required:true;desc:Input file"`
	// OutputFile can be set positionally (pos:1) OR via --output / -o
	OutputFile string `goopt:"pos:1;name:output;short:o;default:-;desc:Output file ('-' for stdout)"`
	Verbose    bool   `goopt:"short:v;desc:Verbose output"`
}

func main() {
	cfg := &Config{}
	parser, _ := goopt.NewParserFromStruct(cfg)
	// ... (Error handling omitted for brevity)
	parser.Parse(os.Args)

	fmt.Printf("Input:  %s\n", cfg.InputFile)
	fmt.Printf("Output: %s\n", cfg.OutputFile) // Will hold the final value
	fmt.Printf("Verbose: %t\n", cfg.Verbose)

	// Show remaining unbound positionals, if any
	fmt.Println("Unbound Positionals:")
	for _, arg := range parser.GetPositionalArgs() {
		if arg.Argument == nil { // Argument is nil if it wasn't bound to a pos:N field
			fmt.Printf("  Index %d: %s\n", arg.Position, arg.Value)
		}
	}
}
```

***Command Line Scenarios***:

```bash
    ./myapp in.txt out.txt

        cfg.InputFile = "in.txt" (from pos:0)

        cfg.OutputFile = "out.txt" (from pos:1)

    ./myapp in.txt --output flag.txt

        cfg.InputFile = "in.txt" (from pos:0)

        cfg.OutputFile = "flag.txt" (from --output, takes precedence over default for pos:1)

    ./myapp in.txt --output flag.txt pos.txt

        cfg.InputFile = "in.txt" (from pos:0)

        cfg.OutputFile = "flag.txt" (from --output, takes precedence)

        pos.txt remains as an unbound positional argument (original index 3), available via GetPositionalArgs() but not bound to cfg.OutputFile.

    ./myapp -v in.txt pos.txt --output flag.txt

        cfg.Verbose = true

        cfg.InputFile = "in.txt" (from pos:0)

        cfg.OutputFile = "flag.txt" (from --output, takes precedence)

        pos.txt remains as an unbound positional argument (original index 2).
```

***Accessing all positional arguments***

The parser.GetPositionalArgs() method returns a slice of PositionalArgument structs representing all arguments identified as positional candidates after flag processing.

```go
type PositionalArgument struct {
    Position int       // Original index in the os.Args slice (after program name).
    ArgPos   int       // The N from the `pos:N` tag this argument was bound to (or relative index if unbound).
    Value    string    // The argument value.
    Argument *Argument // Pointer to the Argument definition if bound, otherwise nil.
}
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
