---
layout: default
title: Getting Started
parent: Guides
nav_order: 1
---

# Getting Started with goopt

## Quick Start

### 1. Installation
```bash
go get github.com/napalu/goopt
```

### 2. Choose Your Style

#### Struct-First Approach

Names are optional, but if you want to use them, you can use the `name` tag to override the default name. If the name is not provided, goopt will use the default `FlagNameConverter` to convert the field name to a valid flag name in lowerCamelCase.

```go
// Define your CLI structure using struct tags
type Options struct {
    // Global flags
    Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
    Output  string `goopt:"short:o;desc:Output file;required:true"`
    
    // Commands and their flags
    Create struct {
        User struct {
            Name     string `goopt:"short:n;desc:Username to create;required:true"`
            Password string `goopt:"short:p;desc:User password;secure:true"`
        } `goopt:"kind:command;desc:Create a new user"`
    } `goopt:"kind:command;desc:Create resources"`
}

func main() {
    opts := &Options{}
    parser, _ := goopt.NewParserFromStruct(opts)
    
    if ok := parser.Parse(os.Args); !ok {
        fmt.Fprintln(os.Stderr, "Invalid command-line arguments:")
        for _, err := range parser.Errors() {
            fmt.Fprintf(os.Stderr, " - %s\n", err)
        }
        parser.PrintUsageWithGroups(os.Stdout)
        os.Exit(1)
    }
    
    // Access values directly through the struct
    if opts.Verbose {
        fmt.Println("Verbose mode enabled")
    }
}
```

#### Builder Approach

```go
func main() {
    parser := goopt.NewParser()
    
    // Add global flags
    parser.AddFlag("verbose", goopt.NewArg(
        goopt.WithDescription("Enable verbose output"),
        goopt.WithShort("v"),
    ))
    
    // Add commands and their flags
    create := parser.AddCommand(goopt.NewCommand(
        goopt.WithName("create"),
        goopt.WithDescription("Create resources"),
    ))
    user := create.AddCommand(goopt.NewCommand(
        goopt.WithName("user"),
        goopt.WithDescription("Create a new user"),
    ))
    user.AddFlag(goopt.NewArg(
        goopt.WithShortFlag("n"),
        goopt.WithDescription("Username to create"),
        goopt.WithRequired(true),
    ))
    
    if !parser.Parse(os.Args) {
        parser.PrintUsageWithGroups(os.Stdout)
        return
    }
    
    // Access values through the parser
    if verbose, _ := parser.Get("verbose"); verbose == "true" {
        fmt.Println("Verbose mode enabled")
    }

    // Or access it as a boolean
    if v, _ := parser.GetBool("verbose"); v {
        fmt.Println("Verbose mode enabled")
    }

    // Or check if the flag is present and provide a default value
    if vd := parser.GetOrDefault("verbose", "false"); vd == "false" {
        fmt.Println("Verbose mode is disabled")
    }
}
```

#### Programmatic Definition with Commands

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

	// Define flags
	parser.AddFlag("output", goopt.NewArg(
        goopt.WithDescription("Output file"),
        goopt.WithType(types.Single),
        goopt.WithRequired(true),
    ))

	// Define commands and subcommands
	createCmd := &goopt.Command{
		Name: "create",
		Subcommands: []goopt.Command{
			{Name: "user"},
			{Name: "group"},
		},
	}

	parser.AddCommand(createCmd)

	// Parse the command-line arguments
	if !parser.Parse(os.Args) {
		parser.PrintUsage(os.Stdout)
		return
	}

	// Access parsed flags
	output, _ := parser.Get("output")
	fmt.Println("Output:", output)

	// Access parsed commands
	cmdValue, _ := parser.GetCommandValue("create user")
	fmt.Println("Command value:", cmdValue)
}
```

#### Initialization using option functions

The library provides an interface for defining flags and commands.

```go
package main

import (
	"os"
	"github.com/napalu/goopt"
    "github.com/napalu/goopt/types"
)

func main() {
	parser, _ := goopt.NewParser(
		goopt.WithFlag("testFlag", goopt.NewArg(goopt.WithType(types.Single))),
		goopt.WithCommand(
			goopt.NewCommand(goopt.WithName("testCommand")),
		),
	)

	parser.Parse(os.Args)
}
```
This interface allows for dynamic and flexible construction of command-line parsers.


### 3. Add Shell Completion (Optional)
```go
import (
    "os"
    "log"
    "github.com/napalu/goopt"
    c "github.com/napalu/goopt/completion"
)

func main() {
    // ... parser setup as above ...
    
    // Add completion support
    exec, err := os.Executable()
    if err != nil {
        log.Fatal(err)
    }

    manager, err := c.NewManager("fish", exec)
    if err != nil {
        log.Fatal(err)
    }
    manager.Accept(parser.GetCompletionData())
    err = manager.Save()
    if err != nil {
        log.Fatal(err)
    }
    // depending on the shell, you may need to source the completion file

    // ... rest of your code ...
}
```

### 4. Environment Variables (Optional)
```go
func main() {
    // ... parser setup as above ...
    
    // Enable environment variable support
    parser.SetEnvNameConverter(func(s string) string {
        // default flag name converter is lowerCamelCase
        return parser.DefaultFlagNameConverter(s)
    })
    
    // ... rest of your code ...
}
```

## Version Compatibility

![Go Version](https://img.shields.io/badge/go-1.18%2B-blue)
![goopt Version](https://img.shields.io/github/v/tag/napalu/goopt)

goopt supports all Go versions from 1.18 onward. See our [compatibility policy]({% link _v1/compatibility.md %}) for details.

## Next Steps

- [Command structure patterns]({% link _v1/guides/command-organization.md %}) - Have a look at different ways to structure your CLI
- [Flag structure patterns]({% link _v1/guides/flag-organization.md %}) - Have a look at different ways to structure your flags
- [Positional Arguments]({% link _v1/guides/positional-arguments.md %}) - Explore positional arguments
- [Struct Tags]({% link _v1/guides/struct-tags.md %}) - Explore struct tags
- [Configuration Guide]({% link _v1/configuration/index.md %}) - Environment variables and external config
- [Shell Completion]({% link _v1/shell/completion.md %}) - Set up shell completions
- [Advanced Features]({% link _v1/guides/advanced-features.md %}) - Explore dependencies, validation, and more
