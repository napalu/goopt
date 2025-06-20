---
layout: default
title: Getting Started
parent: Guides
nav_order: 1
version: v2
---

# Getting Started with goopt

Welcome to `goopt`! This guide will walk you through building your first command-line application in just a few minutes. We'll create a simple "greeter" tool that demonstrates some of `goopt`'s core features.

## 1. Installation

First, add `goopt` to your project:

```bash
go get github.com/napalu/goopt/v2
```

## 2. Define Your CLI

The easiest way to get started is with the "struct-first" approach. Create a file named `main.go` and add the following code. We'll define a simple CLI with a global `--verbose` flag and a `greet` command that takes a `--name` flag.

```go
package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
)

// Config defines the structure of our CLI using Go struct tags.
type Config struct {
	// A global flag available to all commands.
	Verbose bool `goopt:"short:v;desc:Enable verbose output"`

	// The 'greet' command.
	Greet struct {
		// A flag specific to the 'greet' command.
		Name string `goopt:"short:n;desc:Name to greet;default:World"`
	} `goopt:"kind:command;desc:Prints a greeting"`
}

func main() {
	cfg := &Config{}

	// Create a new parser from our struct definition.
	// goopt automatically handles flags, commands, and help text.
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse the command-line arguments.
	if !parser.Parse(os.Args) {
		// If parsing fails (e.g., missing required flag), goopt
		// populates an error list. We can print them and show the help.
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "Error: %v\n", e)
		}
		fmt.Fprintln(os.Stderr)
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Check if a command was run and access its flags.
	if parser.HasCommand("greet") {
		if cfg.Verbose {
			fmt.Println("Verbose mode is enabled. Preparing to greet...")
		}
		fmt.Printf("Hello, %s!\n", cfg.Greet.Name)
	} else {
		// If no command was given, show the help.
		parser.PrintHelp(os.Stdout)
	}
}
```

## 3. Run Your Application

Now, run your new CLI from the terminal to see it in action.

**Run the `greet` command:**
```bash
go run . greet --name "Goopt User"
```
Output:
```
Hello, Goopt User!
```

**Use the default name:**
```bash
go run . greet
```
Output:
```
Hello, World!
```

**Use the global `--verbose` flag:**
```bash
go run . --verbose greet --name Alice
```
Output:
```
Verbose mode is enabled. Preparing to greet...
Hello, Alice!
```

**See the auto-generated help text:**
Because we enabled `auto-help` (the default), `goopt` automatically handles the `--help` flag for you.
```bash
go run . --help
```
Output:
```
Usage: main [global-flags] <command> [command-flags] [args]

Global Flags:
  --help, -h      Show help information
  --verbose, -v "Enable verbose output" (optional)
  ...

Commands:
  greet           Prints a greeting

Examples:
  main --help                    # Show this help
  main greet --help            # Show greet command help
```

## What's Happening?

*   **`type Config struct {...}`**: You defined the entire CLI structure—flags, commands, and descriptions—using a single Go struct.
*   **`goopt:"..."`**: These struct tags tell `goopt` how to create flags and commands. `short:v` creates a `-v` alias, and `desc:"..."` sets the help text.
*   **`goopt.NewParserFromStruct(cfg)`**: This powerful function inspects your struct and builds the entire command-line parser for you.
*   **`parser.Parse(os.Args)`**: This is where `goopt` processes the command-line arguments and populates your `cfg` struct with the values.
*   **`parser.HasCommand("greet")`**: This lets you check which command the user ran so you can execute the correct logic.
*   **`cfg.Greet.Name`**: You access the parsed flag values directly from your typed struct, which is clean and type-safe.

## Next Steps

You've built your first application with `goopt`. Now that you have the basics, you're ready to explore more powerful features:

*   **Learn the Fundamentals**: Dive into the [Core Concepts]({{ site.baseurl }}/v2/guides/02-core-concepts/) to understand how `goopt` works under the hood.
*   **Structure Your CLI**: See different ways to organize your application in [Defining Your CLI]({{ site.baseurl }}/v2/guides/03-defining-your-cli/).
*   **Add Powerful Features**: Explore [Advanced Features]({{ site.baseurl }}/v2/guides/04-advanced-features/) like input validation, execution hooks, and error handling.
*   **See More Examples**: Check out the [Examples](https://github.com/napalu/goopt/tree/main/v2/examples/) for complete application examples.
