# Go-opt an opinionated command-line parser

[![GoDoc](https://godoc.org/github.com/napalu/go-opt?status.svg)](https://godoc.org/github.com/napalu/goopt)
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/goopt)](https://goreportcard.com/report/github.com/napalu/goopt)
![Coverage](https://img.shields.io/badge/Coverage-71.0%25-brightgreen)

`goopt` is a flexible and powerful command-line option parser for Go applications. It provides a way to define commands, subcommands, flags, and their relationships declaratively or programmatically, offering both ease of use and extensibility.

## Key Features

- **Declarative and Programmatic Definition**: Supports both declarative struct tag parsing and programmatic definition of commands and flags.
- **Command and Flag Grouping**: Organize commands and flags hierarchically, supporting global, command-specific, and shared flags.
- **Flag Dependencies**: Enforce flag dependencies based on the presence or specific values of other flags.
- **POSIX Compatibility**: Offers POSIX-compliant flag parsing, including support for short flags.
- **Secure Flags**: Enable secure, hidden input for sensitive information like passwords.
- **Automatic Usage Generation**: Automatically generates usage documentation based on defined flags and commands.

## Installation

Install `goopt` via `go get`:

```bash
go get github.com/napalu/goopt
```

## Basic Design

`goopt` follows a design that allows flexibility in how flags and commands are defined and parsed.

- **Declarative Flags via Struct Tags**: Flags can be defined using struct tags. The parser introspects the struct and automatically binds the struct fields to flags.
- **Programmatic Definition**: Commands and flags can also be defined programmatically or declaratively. This allows dynamic construction of commands based on runtime conditions.
- **Flag Grouping**: Flags can be associated with specific commands or shared across multiple commands. Global flags are available across all commands.
- **Dependency Validation**: Flags can be defined to depend on the presence or value of other flags. This validation is performed automatically after parsing.

## Usage

### Basic Example

### Declarative Definition Using Struct Tags

```go
package main

import (
   "os"
   "fmt"
   "github.com/napalu/goopt"
)

type Options struct {
    Verbose bool   `long:"verbose" short:"v" description:"Enable verbose output"`
    Output  string `long:"output" description:"Output file" required:"true" path:"create file"`
}

func main() {
    opts := &Options{}
    parser, err := goopt.NewParserFromStruct(opts)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }

   if !parser.Parse(os.Args) {
        parser.PrintUsageWithGroups(os.Stdout)
        return
    }

    fmt.Println("Verbose:", opts.Verbose)
    fmt.Println("Output:", opts.Output)
}
```
### Programmatic Definition with Commands

```go
package main

import (
	"os"
	"fmt"
	"github.com/napalu/goopt"
)

func main() {
	parser := goopt.NewParser()

	// Define flags
	parser.AddFlag("output", goopt.NewArgument("Output file", goopt.Single, true))

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

---

### Advanced Features

#### Secure Flags
Some flags contain sensitive information (like passwords) and should be kept secure during input. `goopt` supports secure flags, which prompt the user without echoing input back to the terminal.

```go
package main

import (
	"os"
	"fmt"
	"github.com/napalu/goopt"
)

func main() {
	parser := goopt.NewParser()

	// Define a secure flag
	parser.AddFlag("password", goopt.NewArgument("p", "password for app", goopt.Single, true, goopt.Secure{IsSecure: true, Prompt: "Password: "}, ""))

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

### Flag Dependency Validation

`goopt` allows you to define dependencies between flags, ensuring that certain flags are present or have specific values when others are set. This is useful for enforcing consistency in user input.

```go
package main

import (
	"os"
	g "github.com/napalu/goopt"
)

func main() {
    parser := g.NewParser()
    
    // Define flags
    parser.AddFlag("notify", g.NewArg( 
        g.WithDescription("Enable email notifications"), 
        g.WithType(g.Standalone))
    parser.AddFlag("email", g.NewArg(
        g.WithDescription("Email address for notifications"), 
        g.WithType(g.Single))
    
    // Set flag dependencies - new style
    parser.AddDependencyValue("email", "notify", []string{"true"})
    
    // Or using WithDependencyMap in flag definition
    parser.AddFlag("email", g.NewArg(
        g.WithDescription("Email address for notifications"),
        g.WithType(g.Single),
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
	parser.AddFlag("username", goopt.NewArgument("Username for user creation", goopt.Single), "create user")
	parser.AddFlag("email", goopt.NewArgument("Email address for user creation", goopt.Single), "create user")

	// Parse the command-line arguments
	if parser.Parse(os.Args) {
		username, _ := parser.Get("username")
		email, _ := parser.Get("email")
		fmt.Println("Creating user with username:", username)
		fmt.Println("Email address:", email)
	}
}
```

### Initialization using option functions

The library provides an interface for defining flags and commands.

```go
package main

import (
	"os"
	"github.com/napalu/goopt"
)

func main() {
	parser, _ := goopt.NewParser(
		goopt.WithFlag("testFlag", goopt.NewArg(goopt.WithType(goopt.Single))),
		goopt.WithCommand(
			goopt.NewCommand(goopt.WithName("testCommand")),
		),
	)

	parser.Parse(os.Args)
}
```
This interface allows for dynamic and flexible construction of command-line parsers.

---

### Automatic Usage Generation

`goopt` automatically generates usage documentation based on defined commands and flags. To print the usage:

```go
parser.PrintUsage(os.Stdout)
```

To print the usage grouped by command:

```go
parser.PrintUsageWithGroups(os.Stdout)
```

---

### Contributing

Contributions are welcome! Please check the issues for open tasks and feel free to submit a pull request.
