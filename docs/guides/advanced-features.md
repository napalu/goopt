---
layout: default
title: Advanced Features
parent: Guides
nav_order: 6
---

# Advanced Features

## Struct Tag formats (New vs Old)

| Feature | New Format | Old Format (Deprecated) |
|---------|------------|------------------------|
| Separator | Semicolon (;) | Space |
| Key-Value Delimiter | Colon (:) | Colon (:) |
| Tag Name | goopt | N/A |
| Kind | kind:flag\|command | N/A |
| Long Name | name:value | long:value |
| Short Name | short:value | short:value |
| Description | desc:value | description:value |
| Type | type:single\|standalone\|chained\|file | type:single\|standalone\|chained\|file |
| Required | required:true\|false | required:true\|false |
| Default Value | default:value | default:value |
| Secure Input | secure:true\|false | secure:true\|false |
| Prompt Text | prompt:value | prompt:value |
| SliceCapacity | capacity:value | N/A |
| Accepted Values | accepted:{pattern:json\|yaml,desc:Format type},{pattern:text\|binary,desc:Type} | accepted:{pattern:json\|yaml,desc:Format type} |
| Dependencies | depends:{flag:output,values:[json,yaml]},{flag:mode,values:[text]} | depends:flag:{output,values:[json,yaml]} |
| Positional Flag | pos:{at:start\|end,idx:value} | N/A |

The new format offers several advantages:
- Namespace Isolation: Using goopt: prefix prevents conflicts with other tag parsers
- Better Compatibility: Semicolon-separated format is more common in Go struct tags
- Clearer Structure: All options are under the goopt namespace
- Future Extensibility: New features can be added without breaking existing parsers

To migrate from the old format to the new one, you can use the migration tool:

[Migration Tool Documentation](https://github.com/napalu/goopt/blob/main/migration/README.md)

The tool will automatically update your struct tags while preserving functionality.

### Complex Tag Formats

#### Accepted Values

Multiple accepted patterns can be specified using brace-comma notation:

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

Nested structs can be accessed using dot notation, allowing for deep flag hierarchies:

```go
type Config struct {
    Database struct {
        Connection struct {
            Host string `goopt:"name:host"`
            Port int    `goopt:"name:port"`
        }
        Timeout int `goopt:"name:timeout"`
    }
}

// Access using dot notation:
--database.connection.host localhost
--database.connection.port 5432
--database.timeout 30
```

Nested structs are automatically treated as flag containers unless explicitly marked as commands:
- No special initialization required
- Fields are accessible via dot notation
- Validation ensures struct fields exist


## Slice Handling

When using slices in your configuration struct, there are two distinct cases:

### 1. Terminal Flag Slices
Terminal flag slices (slices at the end of a path) automatically accept comma-separated values:

```go
type Config struct {
    Command struct {
        Items []struct {
            Flags []string `goopt:"name:flag"` // Terminal slice
        }
    }
}

// Usage:
--command.items.0.flag="one,two,three"  // Automatically splits into slice
```

### 2. Nested Structure Slices
For slices of structs (nested slices), you can specify their capacity using the `capacity` tag:

```go
type Config struct {
    Command struct {
        Items []struct `goopt:"capacity:3"` {  // Nested slice needs capacity
            Flag []string `goopt:"name:flag"`  // Terminal slice
        }
    } `goopt:"kind:command;name:command"`
}

// Usage:
--command.items.0.flag="one,two,three"
--command.items.1.flag="four,five,six"
--command.items.2.flag="seven,eight"
```

Important notes:
1. The `capacity` tag is optional and only needed for nested struct slices
2. Terminal flag slices are automatically sized based on input
3. Memory safety is ensured by flag registration - only valid paths are accepted
4. Attempting to use an index outside the registered range results in "unknown flag" error
5. Slice bounds are tracked for user feedback when using `NewParserFromStruct` but are not required for memory safety

## Advanced Flag features


### Flag Types
Flag types are defined in the `github.com/napalu/goopt/types` package and are used to define the type of a flag.

The following types are supported:
- `Single`: A flag which expects a single value - if you do not specify a type, this is the default
- `Standalone`: A boolean flag which is true by default - can be overriden with a false value on the command line
- `Chained`: A flag which is evaluated as a list of values - by default, the delimiters are ',' || r == '|' || r == ' '. This can be overridden by providing a custom delimiter function `ListDelimiterFunc`.
- `File`: A flag which expects a file path and is set to the contents of the file

The flag type can be specified using the `WithType` option function. The flag type can also be specified in the struct tag using the `type` tag.
Flag types in structs which are parsed from a struct (see for example `NewParserFromStruct`) are automatically inferred from the field type.

### Secure Flags
Some flags contain sensitive information (like passwords) and should be kept secure during input. `goopt` supports secure flags, which prompt the user without echoing input back to the terminal.

```go
package main

import (
	"os"
	"fmt"
	"github.com/napalu/goopt"
    "github.com/napalu/goopt/types"
)

func main() {
	parser := goopt.NewParser()

	// Define a secure flag
	parser.AddFlag("password", goopt.NewArgument("p", "password for app", types.Single, true, goopt.Secure{IsSecure: true, Prompt: "Password: "}, ""))

	// Parse the arguments
	if parser.Parse(os.Args) {
		password, _ := parser.Get("password")
		fmt.Println("Password received (but not echoed):", password)
	} else {
		parser.PrintUsage(os.Stdout)
	}
}
```

Secure flags ensure that user input for sensitive fields is hidden, and can optionally display a prompt message.

### Dependency Validation

`goopt` allows you to define dependencies between flags, ensuring that certain flags are present or have specific values when others are set. This is useful for enforcing consistency in user input.

```go
package main

import (
	"os"
	g "github.com/napalu/goopt"
    "github.com/napalu/goopt/types"
)

func main() {
    parser := g.NewParser()
    
    // Define flags
    parser.AddFlag("notify", g.NewArg( 
        g.WithDescription("Enable email notifications"), 
        g.WithType(types.Standalone))
    parser.AddFlag("email", g.NewArg(
        g.WithDescription("Email address for notifications"), 
        g.WithType(types.Single))
    
    // Set flag dependencies - new style
    parser.AddDependencyValue("email", "notify", []string{"true"})
    
    // Or using WithDependencyMap in flag definition
    parser.AddFlag("email", g.NewArg(
        g.WithDescription("Email address for notifications"),
        g.WithType(types.Single),
        g.WithDependencyMap(map[string][]string{
            "notify": {"true"},
        })))
    
    // Parse the arguments
    if !parser.Parse(os.Args) {
        parser.PrintUsage(os.Stdout)
    } else {
        email, _ := parser.Get("email")
        fmt.Println("Email notifications enabled for:", email)
    }
}
```

In this example, the email flag is only valid if the notify flag is set to true. If the dependency is not satisfied, goopt will add a warning which can be displayed to the user (see `goopt.GetWarnings()`).

### Command-Specific Flags

`goopt` supports associating flags with specific commands or subcommands. This allows you to define different behaviors and options for different parts of your application.

```go
package main

import (
	"os"
	"github.com/napalu/goopt"
    "github.com/napalu/goopt/types"
)

func main() {
	parser := goopt.NewParser()

	// Define commands
	parser.AddCommand(&goopt.Command{
		Name: "create",
		Subcommands: []goopt.Command{
			{Name: "user"},
			{Name: "group"},
		},
	})

	// Define flags for specific commands
	parser.AddFlag("username", goopt.NewArgument("Username for user creation", types.Single), "create user")
	parser.AddFlag("email", goopt.NewArgument("Email address for user creation", types.Single), "create user")

	// Parse the command-line arguments
	if parser.Parse(os.Args) {
		username, _ := parser.Get("username")
		email, _ := parser.Get("email")
		fmt.Println("Creating user with username:", username)
		fmt.Println("Email address:", email)
	}
}
```

## Automatic Usage Generation

`goopt` automatically generates usage documentation based on defined commands and flags. To print the usage:

```go
parser.PrintUsage(os.Stdout)
```

To print the usage grouped by command:

```go
parser.PrintUsageWithGroups(os.Stdout)
```
