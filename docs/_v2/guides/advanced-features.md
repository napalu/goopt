---
layout: default
title: Advanced Features
parent: Guides
nav_order: 6
---

# Advanced Features

This guide covers the advanced features of goopt v2 including struct tags, nested structures, flag types, and validation mechanisms.

## Struct Tag Format

goopt v2 uses a more structured and consistent tag format:

| Feature | Format | Example |
|---------|--------|---------|
| Tag Name | `goopt` | `goopt:"name:value"` |
| Separator | Semicolon (;) | `goopt:"name:value;short:v"` |
| Key-Value Delimiter | Colon (:) | `goopt:"name:value"` |
| Kind | `kind:flag|command` | `goopt:"kind:command"` |
| Name | `name:value` | `goopt:"name:output"` |
| Short Name | `short:value` | `goopt:"short:o"` |
| Description | `desc:value` | `goopt:"desc:Output file"` |
| Description Key | `desckey:value` | `goopt:"desckey:flag.output"` |
| Type | `type:single|standalone|chained|file` | `goopt:"type:file"` |
| Required | `required:true|false` | `goopt:"required:true"` |
| Default Value | `default:value` | `goopt:"default:stdout"` |
| Secure Input | `secure:true|false` | `goopt:"secure:true"` |
| Prompt Text | `prompt:value` | `goopt:"prompt:Password:"` |
| Slice Capacity | `capacity:value` | `goopt:"capacity:5"` |
| Accepted Values | `accepted:{pattern:regex,desc:description}` | `goopt:"accepted:{pattern:json|yaml,desc:Format type}"` |
| Dependencies | `depends:{flag:name,values:[val1,val2]}` | `goopt:"depends:{flag:format,values:[json,yaml]}"` |
| Positional | `pos:{idx:value}` | `goopt:"pos:{idx:0}"` |

### Complex Tag Formats

#### Accepted Values

You can specify multiple accepted patterns using brace-comma notation:

```go
type Config struct {
    // Single pattern
    Format string `goopt:"name:format;accepted:{pattern:json|yaml,desc:Output format}"`

    // Multiple patterns
    Mode string `goopt:"name:mode;accepted:{pattern:read|write,desc:Access mode},{pattern:sync|async,desc:Operation mode}"`
}
```

#### Dependencies

Dependencies use the same brace-comma notation:

```go
type Config struct {
    // Single dependency
    Format string `goopt:"name:format;depends:{flag:output,values:[file,dir]}"`

    // Multiple dependencies
    Compress bool `goopt:"name:compress;depends:{flag:format,values:[json]},{flag:output,values:[file,dir]}"`
}
```

## Nested Struct Access

goopt supports deep flag hierarchies using dot notation:

```go
type Config struct {
    Database struct {
        Connection struct {
            Host string `goopt:"name:host;desc:Database hostname"`
            Port int    `goopt:"name:port;desc:Database port;default:5432"`
        }
        Timeout int `goopt:"name:timeout;desc:Connection timeout;default:30"`
    }
}

// Command line usage:
// --database.connection.host localhost
// --database.connection.port 5432
// --database.timeout 30
```

Nested structs are automatically treated as flag containers unless marked as commands:
- Fields are accessible via dot notation
- Validation ensures all struct fields exist
- Path components are validated at runtime

## Slice Handling

goopt supports two types of slices:

### 1. Terminal Flag Slices

Slices bound to flags with type `chained` (or inferred as such) are split using the parser's configured delimiter function (which defaults to space, comma, pipe):

```go
type Config struct {
    Tags []string `goopt:"name:tags;desc:List of tags"`
}

// Usage:
// --tags="tag1,tag2,tag3"
```

### 2. Nested Structure Slices

For slices of structs, you can specify their capacity:

```go
type Config struct {
    Users []struct {
        Name  string `goopt:"name:name;desc:User name"`
        Roles []string `goopt:"name:roles;desc:User roles"`
    } `goopt:"capacity:3"`
}

// Usage:
// --users.0.name="admin" --users.0.roles="admin,user"
// --users.1.name="guest" --users.1.roles="guest"
```

Important notes:
1. The `capacity` tag is only needed for nested struct slices
2. Terminal flag slices are automatically sized based on input
3. Attempting to use an index outside the registered range will fail
4. Using `NewParserFromStruct` ensures proper slice initialization

## Flag Types

goopt supports several flag types (defined in `github.com/napalu/goopt/v2/types`):

| Type | Description | Example |
|------|-------------|---------|
| `Single` | Expects a single value (default) | `--output file.txt` |
| `Standalone` | Boolean flag (true by default) | `--verbose` |
| `Chained` | List of values | `--tags=one,two,three` |
| `File` | File path, reads content into flag | `--config=/path/to/file.json` |

You can specify the flag type in multiple ways:

```go
// In struct tags
type Config struct {
    Verbose bool     `goopt:"name:verbose;type:standalone"`
    Tags    []string `goopt:"name:tags;type:chained"`
}

// With functional options
parser.AddFlag("verbose", goopt.NewArg(
    goopt.WithType(types.Standalone),
    goopt.WithShortFlag("v"),
    goopt.WithDescription("Enable verbose output"),
))

parser.AddFlag("tags", goopt.NewArg(
    goopt.WithType(types.Chained),
    goopt.WithDescription("Comma-separated list of tags"),
))
```

## Secure Flags

For sensitive information like passwords, use secure flags:

```go
package main

import (
    "fmt"
    "os"
    "github.com/napalu/goopt/v2"
)

func main() {
    parser := goopt.NewParser()

    // Add a secure flag
    parser.AddFlag("password", goopt.NewArg(
        goopt.WithShortFlag("p"),
        goopt.WithDescription("Account password"),
        goopt.WithSecurePrompt("Password: "), // Secure with prompt
        goopt.WithRequired(true),
    ))

    if parser.Parse(os.Args) {
        password, _ := parser.Get("password")
        fmt.Println("Password received (securely):", password)
    } else {
        fmt.Println("Error parsing arguments:")
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, "  %s\n", err)
        }
        parser.PrintUsage(os.Stdout)
    }
}
```

Secure flags:
- Hide user input (no echoing to terminal)
- Can display an optional prompt
- Are cleared from memory after use

## Dependency Validation

You can define dependencies between flags to enforce consistency:

```go
package main

import (
    "fmt"
    "os"
    "github.com/napalu/goopt/v2"
    "github.com/napalu/goopt/v2/types"
)

func main() {
    parser := goopt.NewParser()
    
    // Define flags
    parser.AddFlag("notify", goopt.NewArg(
        goopt.WithDescription("Enable email notifications"),
        goopt.WithType(types.Standalone),
    ))
    
    parser.AddFlag("email", goopt.NewArg(
        goopt.WithDescription("Email address for notifications"),
        goopt.WithDependencyMap(map[string][]string{
            "notify": {"true"}, // Email is only valid if notify=true
        }),
    ))
    
    if !parser.Parse(os.Args) {
        fmt.Println("Error parsing arguments:")
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, "  %s\n", err)
        }
        parser.PrintUsage(os.Stdout)
        return
    }
    
    // Check for dependency warnings
    warnings := parser.GetWarnings()
    if len(warnings) > 0 {
        fmt.Println("Warnings:")
        for _, warning := range warnings {
            fmt.Println("  " + warning)
        }
    }
    
    if parser.HasFlag("email") {
        email, _ := parser.Get("email")
        fmt.Println("Email notifications will be sent to:", email)
    }
}
```

### Methods for Setting Dependencies

```go
package main

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

func main() {
	parser := goopt.NewParser()
	
	// Method 1: During flag definition
	parser.AddFlag("email", goopt.NewArg(
		goopt.WithDescription("Email address"),
		goopt.WithDependencyMap(map[string][]string{
			"notify": {"true"},
		}),
	))

	// Method 2: After flag definition
	parser.AddDependency("email", "notify")                        // email requires notify (any value)
	parser.AddDependencyValue("email", "notify", []string{"true"}) // email requires notify=true
}
```

## Command-Specific Flags

You can associate flags with specific commands:

```go
package main

import (
    "fmt"
    "os"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

func main() {
    parser := goopt.NewParser()

    // Define commands using the builder pattern
    parser.AddCommand(
        goopt.NewCommand(
            goopt.WithName("create"),
            goopt.WithCommandDescription("Create resources"),
            goopt.WithSubcommands(
                goopt.NewCommand(
                    goopt.WithName("user"),
                    goopt.WithCommandDescription("Create a new user"),
                ),
                goopt.NewCommand(
                    goopt.WithName("group"),
                    goopt.WithCommandDescription("Create a new group"),
                ),
            ),
        ),
    )

    // Add flags for specific commands
    parser.AddFlag("name", goopt.NewArg(
        goopt.WithShortFlag("n"),
        goopt.WithDescription("Name for the new resource"),
        goopt.WithRequired(true),
    ), "create", "user")  // Specific to "create user" command
    
    parser.AddFlag("email", goopt.NewArg(
        goopt.WithDescription("Email address for the user"),
    ), "create", "user")
    
    parser.AddFlag("name", goopt.NewArg(
        goopt.WithShortFlag("n"),
        goopt.WithDescription("Name for the new group"),
        goopt.WithRequired(true),
    ), "create", "group")  // Specific to "create group" command

    // Add flag to parent command (shared by all subcommands)
    parser.AddFlag("force", goopt.NewArg(
        goopt.WithShortFlag("f"),
        goopt.WithDescription("Force creation without confirmation"),
        goopt.WithType(types.Standalone),
    ), "create")  // Available to all "create" subcommands

    if !parser.Parse(os.Args) {
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, "Error: %s\n", err)
        }
        parser.PrintUsageWithGroups(os.Stdout)
        return
    }

    // Check which command was executed
    if parser.HasCommand("create user") {
        name, _ := parser.Get("name", "create", "user")
        email, _ := parser.Get("email", "create", "user")
        fmt.Printf("Creating user: %s <%s>\n", name, email)
        
        // Access shared flag
        if force, _ := parser.GetBool("force", "create"); force {
            fmt.Println("Force mode enabled")
        }
    } else if parser.HasCommand("create group") {
        name, _ := parser.Get("name", "create", "group")
        fmt.Printf("Creating group: %s\n", name)
        
        // Access shared flag
        if force, _ := parser.GetBool("force", "create"); force {
            fmt.Println("Force mode enabled")
        }
    }
}
```

## Flag Inheritance

In the command hierarchy:
- Flags defined on parent commands are available to all child commands
- Child command flags override parent flags with the same name
- When accessing command-specific flags, use the full command path

```go
package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
)

// Config demonstrates hierarchical flag definition and inheritance.
type Config struct {
	// --- Root Level Flag ---
	// Defined at the top level, available to all commands unless overridden.
	LogLevel string `goopt:"name:log-level;short:l;default:INFO;desc:Global log level"`

	// --- Command Structure ---
	// 'App' serves as a container, its fields are not flags/commands themselves.
	App struct {
		// --- 'Service' Command ---
		Service struct {
			// --- 'Service' Level Flag ---
			// Specific to the 'service' command and its subcommands.
			Port int `goopt:"name:port;short:p;default:8080;desc:Service port"`

			// --- 'Start' Subcommand ---
			Start struct {
				// --- 'Start' Level Flag (Overrides Parent) ---
				// Overrides 'log-level' specifically for 'start'.
				LogLevel string `goopt:"name:log-level;default:DEBUG;desc:Log level for start command"`
				// Specific flag for 'start'
				Workers int `goopt:"name:workers;default:4;desc:Number of workers"`
			} `goopt:"kind:command;name:start;desc:Start the service"`

			// --- 'Stop' Subcommand ---
			Stop struct {
				// Inherits 'log-level' from the root level (INFO).
				// Inherits 'port' from the 'service' level (8080).
				Force bool `goopt:"name:force;short:f;desc:Force stop"`
			} `goopt:"kind:command;name:stop;desc:Stop the service"`

		} `goopt:"kind:command;name:service;desc:Manage the service"`
	}
}

func main() {
	cfg := &Config{}
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing parser: %v\n", err)
		os.Exit(1)
	}

	// --- Example Command Lines ---
	// 1. ./myapp service start                  (LogLevel=DEBUG, Port=8080, Workers=4)
	// 2. ./myapp service stop --force           (LogLevel=INFO, Port=8080, Force=true)
	// 3. ./myapp -l WARN service start -p 9090 (LogLevel=DEBUG, Port=9090, Workers=4) <- Global -l WARN ignored for start
	// 4. ./myapp -l WARN service stop           (LogLevel=WARN, Port=8080, Force=false) <- Global -l WARN used for stop

	if !parser.Parse(os.Args) {
		fmt.Fprintln(os.Stderr, "Error: Invalid command-line arguments.")
		for _, parseErr := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, " - %s\n", parseErr)
		}
		fmt.Fprintln(os.Stderr, "")
		parser.PrintUsageWithGroups(os.Stdout)
		os.Exit(1)
	}

	// --- Accessing Values (Rely on bound struct fields) ---

	if parser.HasCommand("service start") {
		fmt.Println("Executing 'service start'...")
		// Access fields directly. goopt ensures the correct values based on hierarchy are bound.
		fmt.Printf("  Log Level (Specific to Start): %s\n", cfg.App.Service.Start.LogLevel) // Should be DEBUG or overridden
		fmt.Printf("  Port (Inherited from Service): %d\n", cfg.App.Service.Port)           // Should be 8080 or overridden
		fmt.Printf("  Workers (Specific to Start):   %d\n", cfg.App.Service.Start.Workers)  // Should be 4 or overridden
		fmt.Printf("  Global LogLevel (Not used directly here): %s\n", cfg.LogLevel)        // Shows the global value if set

	} else if parser.HasCommand("service stop") {
		fmt.Println("Executing 'service stop'...")
		// Access inherited LogLevel from the root 'Config' struct
		fmt.Printf("  Log Level (Inherited from Root): %s\n", cfg.LogLevel) // Should be INFO or overridden globally
		// Access inherited Port from the parent 'Service' struct
		fmt.Printf("  Port (Inherited from Service):   %d\n", cfg.App.Service.Port) // Should be 8080 or overridden
		// Access flag specific to Stop
		fmt.Printf("  Force Stop:                      %t\n", cfg.App.Service.Stop.Force)

	} else {
		fmt.Println("No specific service command executed.")
		// Example: Check global log level if set directly
		if parser.HasRawFlag("log-level") { // Check if --log-level was explicitly passed globally
			fmt.Printf("Global Log Level explicitly set to: %s\n", cfg.LogLevel)
		}
	}
}
```

## Pattern Matching and Validation

goopt supports pattern matching for flag values:

```go
package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

func main() {
	parser := goopt.NewParser()
	parser.AddFlag("format", goopt.NewArg(
		goopt.WithDescription("Output format"),
		goopt.WithAcceptedValues([]types.PatternValue{
			{Pattern: "json|yaml|csv", Description: "Supported formats: json, yaml, csv"}}),
	))

	// Or add multiple patterns after definition
	parser.AcceptPatterns("format", []types.PatternValue{
		{Pattern: "json|yaml", Description: "Structured formats"},
		{Pattern: "csv|tsv", Description: "Tabular formats"},
	})
}
```

In struct tags:
```go
type Config struct {
    Format string `goopt:"name:format;accepted:{pattern:json|yaml|csv,desc:Supported formats}"` 
}
```

## Error Handling

goopt provides comprehensive error handling:

```go
if !parser.Parse(os.Args) {
    fmt.Println("Error parsing arguments:")
    for _, err := range parser.GetErrors() {
        fmt.Fprintf(os.Stderr, "  %s\n", err)
    }
    parser.PrintUsage(os.Stdout)
    return
}

// Check for warnings (non-fatal issues)
warnings := parser.GetWarnings()
if len(warnings) > 0 {
    fmt.Println("Warnings:")
    for _, warning := range warnings {
        fmt.Println("  " + warning)
    }
}
```

## Usage Documentation

goopt can automatically generate usage documentation:

```go
// Basic usage (shows all flags and commands)
parser.PrintUsage(os.Stdout)

// Usage grouped by command (shows command hierarchy)
parser.PrintUsageWithGroups(os.Stdout)

// Customize usage presentation
config := &goopt.PrettyPrintConfig{
    NewCommandPrefix:     " +  ",
    DefaultPrefix:        " │─ ",
    TerminalPrefix:       " └─ ",
    InnerLevelBindPrefix: " ** ",
    OuterLevelBindPrefix: " |  ",
}
parser.PrintCommandsUsing(os.Stdout, config)

parser.PrintPositionalArgs(os.Stdout)
```

## Command Callbacks with Struct Context

When using struct-based configuration, goopt provides a way to access the original struct from command callbacks, which is especially useful when organizing callbacks in separate packages. This allows you to maintain clean separation of concerns.

### Accessing the Struct Context

The parser stores the original struct passed to `NewParserFromStruct` or `NewParserFromInterface`, which you can retrieve in two ways:
```go
// 1. Non-generic method 
structCtx := parser.GetStructCtx() 
cfg, ok := structCtx.(*MyConfig)
if !ok { 
	return fmt.Errorf("invalid struct context type") 
}

// 2. Generic function (Go 1.18+) 
cfg, ok := goopt.GetStructCtxAs[*MyConfig](parser)
if !ok { 
	return fmt.Errorf("invalid struct context type") 
}
```

### Organizing Command Callbacks in Separate Packages

This feature is particularly useful when organizing command handlers in separate packages:

```go
// In myapp/types.go
package myapp

import "github.com/napalu/goopt/v2"

type Config struct {
	Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
	Create  struct {
		File struct {
			Output string `goopt:"short:o;desc:Output file;required:true"`
			Exec   goopt.CommandFunc // Store the callback function here
		} `goopt:"kind:command;desc:Create a file"`
	} `goopt:"kind:command;desc:Create commands"`
}
```

```go
// In main.go
package main

import (
    "fmt"
    "os"
    
    "github.com/napalu/goopt/v2"
    "myapp/handlers"
	"myapp/types"
)


func main() {
    cfg := &types.Config{}
    
    // Assign the callback from the handlers package
    cfg.Create.File.Exec = handlers.CreateFileHandler
    
    parser, err := goopt.NewParserFromStruct(cfg, goopt.WithExecOnParse(true)) // Execute the callback on successful parse
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    if !parser.Parse(os.Args) {
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, "Error: %s\n", err)
        }
        parser.PrintUsageWithGroups(os.Stdout)
        os.Exit(1)
    }
}
```

```go 
// In handlers/file.go
package handlers

import (
    "fmt"
    
    "github.com/napalu/goopt/v2"
    "myapp/types"
)

// CreateFileHandler handles file creation
func CreateFileHandler(p *goopt.Parser, cmd *goopt.Command) error {
    // Access the original struct using the generic function (Go 1.18+)
    cfg, ok := goopt.GetStructContextAs[*types.Config](p)
    if !ok {
        return fmt.Errorf("invalid struct context type")
    }
    
    // Now you have access to all configuration values
    if cfg.Verbose {
        fmt.Println("Creating file in verbose mode")
    }
    
    fmt.Printf("Creating file: %s\n", cfg.Create.File.Output)
    
    // Perform file creation...
    return nil
}
```

This pattern allows for better code organization in large applications, separating command handling logic from CLI definition while maintaining type safety.
