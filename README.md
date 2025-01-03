# goopt: a flexible and powerful command-line parser

[![GoDoc](https://godoc.org/github.com/napalu/go-opt?status.svg)](https://godoc.org/github.com/napalu/goopt)
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/goopt)](https://goreportcard.com/report/github.com/napalu/goopt)
![Coverage](https://img.shields.io/badge/Coverage-76.5%25-brightgreen)

`goopt` is a flexible and powerful command-line option parser for Go applications. It provides a way to define commands, subcommands, flags, and their relationships declaratively or programmatically, offering both ease of use and extensibility.

[📚 View Documentation (wip)](https://napalu.github.io/goopt)

## Key Features

- **Declarative and Programmatic Definition**: Supports both declarative struct tag parsing and programmatic definition of commands and flags.
- **Command and Flag Grouping**: Organize commands and flags hierarchically, supporting global, command-specific, and shared flags.
- **Flag Dependencies**: Enforce flag dependencies based on the presence or specific values of other flags.
- **POSIX Compatibility**: Offers POSIX-compliant flag parsing, including support for short flags.
- **Secure Flags**: Enable secure, hidden input for sensitive information like passwords.
- **Automatic Usage Generation**: Automatically generates usage documentation based on defined flags and commands.
- **Positional Arguments**: Support for positional arguments with flexible position and index constraints.
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

## When to use goopt

`goopt` is particularly well-suited for:

- **Flexible command definition** supporting struct-first, builder pattern, or imperative style
- **Multiple command organization approaches**:
  - Flag-centric (using struct base path tags)
  - Command-centric (grouping via command structs)
  - Mixed approach combining both styles
- **Type-safe configurations** with compile-time validation
- **Ordered command execution** where commands need to be processed in sequence

Feature overview:
- Multiple command definition styles:
  - Struct-based using tags
  - Builder pattern
  - Imperative
  - Mixed approaches
- Flexible command organization:
  - Flag-centric with base paths
  - Command-centric with struct grouping
  - Hybrid approaches
- Nested commands with command-specific flags
- Command callbacks (explicit or automatic)
- Environment variable support
- Configurable defaults through ParseWithDefaults:
  - Load defaults from any source (JSON, YAML, etc.)
  - Implement only the configuration features you need
  - Clear precedence: ENV vars -> defaults -> CLI flags
- Ordered command execution
- Type-safe flag parsing
- Flag dependencies and validation
- Pattern matching for flag values
- Shell completion support:
  - Bash completion (flags and commands)
  - Zsh completion (rich command/flag descriptions, file type hints)
  - Fish completion (command/flag descriptions, custom suggestions)
  - PowerShell completion (parameter sets, dynamic completion)
  - Custom completion functions for dynamic values
  - Built-in completion installation commands

While [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper) provide a comprehensive configuration management solution with persistent and global flags, `goopt` offers unique flexibility in how commands and flags can be organized, along with guaranteed execution order.

Choose `goopt` when you:
- Want freedom to choose between struct tags, builder pattern, or imperative style
- Need flexibility in organizing commands (flag-centric, command-centric, or mixed)
- Need guaranteed command execution order
- Need strong compile-time guarantees about your command structure
- Need completion support across multiple shell types
- Prefer implementing specific configuration features over a full-featured solution

## Usage

### Basic Example

### Declarative Definition Using Struct Tags

The following example creates two flags, `Verbose` and `Output`, and a command `create` with a subcommand `file`. The path `create file` is used to specify the name of the command for the `Output` flag. A path consist of the command name and any number of subcommands. Variables in the command path are evaluated in the command context they are defined in. A variable so defined can be shared with other commands and subcommands:
`path:create file,create directory` would create two commands ('create file' and 'create directory') and the `Output` flag would be set to the value of the `Output` flag for both commands.

The struct tag format shown below is the old format which is still supported but is deprecated. The reason for the deprecation, is that the new format allows the use of additional struct tags (such as json), as each struct tag is self-contained, consisting of a set of key:value pair containing the attribute name and the attribute value. The new format is shown in the next section. The basic difference is that the old format uses a tag per attribute which might collide with other struct tags whereas the new format uses a single tag to define all attributes.

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

The tag format uses semicolon-separated key:value pairs:
- `long`: Long name for the `flag` - defaults to the field name if not specified
- `short`: Short (single-character) name of the `flag` when POSIX compatibility is enabled - otherwise can be a multi-character string used as a mnemonic for the `flag` name (default)
- `desc`: Description of the `flag`
- `type`: `Flag` type (single|standalone|chained) - defaults to single
- `required`: Whether the `flag` is required (true|false) - defaults to true
- `default`: Default value for the `flag`
- `secure`: For `flag` containing password input (true|false) - defaults to false
- `prompt`: Prompt text for secure input `flag`
- `accepted`: `Flag` which matches on values using one or more patterns - a pattern can be a literal value or a regex pattern (e.g. `pattern:json|yaml,desc:Format type`)
- `depends`: `Flag` dependencies - a dependency can be a flag or set of flags or a set of flags and values (e.g. `flag:output,values:[json,yaml]`)

### New Struct Tag Format

The new `goopt` tag format provides a more flexible way to define flags and commands. Commands can still be defined using the  `path:` attribute but flags can also be grouped by `command` structure. See the next example for details.

```go
package main

import (
   "os"
   "fmt"
   "github.com/napalu/goopt"
)

type Options struct {
    // Basic flag (global)
    Verbose bool `goopt:"name:verbose;short:v;desc:Enable verbose output"`
    // Required flag with default value (global)
    Output string `goopt:"name:output;desc:Output file;required:true;default:/tmp/out.txt"`
    // Command with subcommands
    Create struct { // Command
        // Flag for 'create' command
        Force bool `goopt:"name:force;desc:Force creation"`
        User struct {       // Subcommand definition
            // Flags for 'create user' command
            Username string `goopt:"name:username;desc:Username to create;required:true"`
            Password string `goopt:"name:password;desc:User password;secure:true;prompt:Password:"`
        } `goopt:"kind:command;name:user;desc:Create user"`
    }  `goopt:"kind:command;name:create;desc:Create resources"`
    // Flag with accepted values and dependencies (global)
    Format string `goopt:"name:format;desc:Output format;accepted:{pattern:json|yaml,desc:Format type};depends:{flag:output,values:[json,yaml]}"`
    // Standalone flag (global)
    DryRun bool `goopt:"name:dry-run;desc:Dry run mode;type:standalone"`
    // List type with custom separator (global)
    Tags []string `goopt:"name:tags;desc:List of tags;type:chained"`
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

The tag format uses semicolon-separated key:value pairs:
- `goopt`: The tag name
- `kind`: Specifies if it's a `flag` or `command` (default: flag)
- `name`: Long name for the `flag`/`command` - defaults to the field name if not specified
- `short`: Short (single-character) name of the `flag` when POSIX compatibility is enabled - otherwise can be a multi-character string used as a mnemonic for the flag name (default)
- `desc`: Description of the `flag`/`command`
- `type`: `Flag` type (single|standalone|chained) - defaults to single
- `required`: Whether the `flag` is required (true|false) - defaults to true
- `default`: Default value for the `flag`
- `secure`: For `flag` containing password input (true|false) - defaults to false
- `prompt`: Prompt text for secure input `flag`
- `accepted`: `Flag` which matches on values using one or more patterns - a pattern can be a literal value or a regex pattern (e.g. `pattern:json|yaml,desc:Format type`)
- `depends`: `Flag` dependencies - a dependency can be a flag or set of flags or a set of flags and values (e.g. `flag:output,values:[json,yaml]`)

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

### Command Organization Examples

goopt offers multiple approaches to organizing commands and flags:

1. Flag-centric using base paths:
```go
type Options struct {
    // Commands defined by path in flag structs
    CreateUser string `goopt:"kind:flag;path:user create;name:name;desc:Create a new user"`
    CreateRole string `goopt:"kind:flag;path:role create;name:name;desc:Create a new role"`
    DeleteUser string `goopt:"kind:flag;path:user delete;name:name;desc:Delete a user"`
    
    // Shared flag across specific commands
    Force bool `goopt:"kind:flag;path:user create,user delete,role create;name:force;desc:Force operation"`
}
```

2. Command-centric using struct hierarchy:
```go
type Options struct {
    User struct {
        `goopt:"kind:command;name:user;desc:User management"`
        
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
        Delete struct {
            `goopt:"kind:command;name:delete;desc:Delete a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
    }
    Role struct {
        `goopt:"kind:command;name:role;desc:Role management"`
        
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a role"`
            Name string `goopt:"kind:flag;name:name;desc:Role name"`
        }
    }
    
    // Shared flag across specific commands
    Force bool `goopt:"kind:flag;path:user create,user delete,role create;name:force;desc:Force operation"`
}
```

3. Mixed approach:
```go
type Options struct {
    // Command-centric for user management
    User struct {
        `goopt:"kind:command;name:user;desc:User management"`
        
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
    }
    
    // Flag-centric for role management
    CreateRole string `goopt:"kind:flag;path:role create;name:name;desc:Create a new role"`
    DeleteRole string `goopt:"kind:flag;path:role delete;name:name;desc:Delete a role"`
    
    // Shared flag across specific commands
    Force bool `goopt:"kind:flag;path:user create,role create;name:force;desc:Force operation"`
}
```

Each approach has its benefits:
- Flag-centric is flatter and good for simpler CLIs
- Command-centric provides clear structure for complex command hierarchies
- Mixed approach allows flexibility where needed

### Positional Arguments

Goopt supports explicit position requirements for command-line arguments:


```go
parser := goopt.NewParser()

// Require source file at start
parser.AddFlag("source", goopt.NewArg(
    goopt.WithPosition(goopt.AtStart),
    goopt.WithRelativeIndex(0),
))

// Require destination file at end
parser.AddFlag("dest", goopt.NewArg(
    goopt.WithPosition(goopt.AtEnd),
    goopt.WithRelativeIndex(0),
))

// Parse: myapp source.txt --verbose dest.txt
args := parser.Parse(os.Args[1:])
```

### Position Types

- `AtStart`: Argument must appear before any flags or commands
- `AtEnd`: Argument must appear after all flags and commands

### Features

1. **Flexible Override**: Use flag syntax to override position requirements
   ```bash
   myapp --source override.txt --verbose dest.txt
   ```

2. **Multiple Ordered Positions**: Use PositionalIndex to specify order
   ```go
   parser.AddFlag("config", goopt.NewArg(
       goopt.WithPosition(goopt.AtStart),
       goopt.WithRelativeIndex(0),
   ))
   parser.AddFlag("profile", goopt.NewArg(
       goopt.WithPosition(goopt.AtStart),
       goopt.WithRelativeIndex(1),
   ))
   ```

3. **Mixed Usage**: Combine positioned and regular arguments
   ```bash
   myapp config.yaml profile.json --verbose extra.txt
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

Contributions are welcome! Contributions should be based on open issues (feel free to open one).

