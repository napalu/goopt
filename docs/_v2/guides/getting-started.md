---
layout: default
title: Getting Started
parent: Guides
nav_order: 1
---

# Getting Started with goopt v2

## 1. Installation
```bash
go get github.com/napalu/goopt/v2
```

## 2. Defining Your CLI: Choose Your Style

### Style 1: Struct-First Approach (Recommended)

Using a struct-first approach both flags and commands and subcommands can be described using struct-tags.

Names are optional, but if you want to use them, you can use the `name` tag to override the default name. If the name is not provided, goopt will use the default `FlagNameConverter` to convert the field name to a valid flag name in lowerCamelCase.

```go
package main

import (
    "fmt"
    "os"
    "github.com/napalu/goopt/v2"
)

// Define your CLI structure using struct tags
type Options struct {
    // Global flags
    Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
    Output  string `goopt:"short:o;desc:Output file;required:true"`
    
    // Commands and their flags
    Create struct {
        // These flags are shared by all subcommands of Create
        Force bool `goopt:"short:f;desc:Force creation without confirmation"`
        
        User struct {
            Name     string `goopt:"short:n;desc:Username to create;required:true"`
            Password string `goopt:"short:p;desc:User password;secure:true"`
        } `goopt:"kind:command;desc:Create a new user"`
        
        Group struct {
            Name string `goopt:"short:n;desc:Group name;required:true"`
        } `goopt:"kind:command;desc:Create a new group"`
    } `goopt:"kind:command;desc:Create resources"`
}

func main() {
    opts := &Options{}
    parser, _ := goopt.NewParserFromStruct(opts)
    
    if ok := parser.Parse(os.Args); !ok {
        fmt.Fprintln(os.Stderr, "Invalid command-line arguments:")
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, " - %s\n", err)
        }
        parser.PrintUsageWithGroups(os.Stdout)
        os.Exit(1)
    }
    
    // Access values directly through the struct
    if opts.Verbose {
        fmt.Println("Verbose mode enabled")
    }
    
    // Create user command: ./myapp create user -f -n admin -p secret
    if parser.HasCommand("create user") {
        fmt.Printf("Creating user: %s\n", opts.Create.User.Name)
        if opts.Create.Force {
            fmt.Println("Force flag is set (inherited from parent command)")
        }
    }
    
    // Create group command: ./myapp create group -f -n admins
    if parser.HasCommand("create group") {
        fmt.Printf("Creating group: %s\n", opts.Create.Group.Name)
        if opts.Create.Force {
            fmt.Println("Force flag is set (inherited from parent command)")
        }
    }
}
```

### Style 2: Programmatic Approach (Functional Options)

With the programmatic approach, flags are declared explicily, and you can create hierarchical command structures using `NewCommand()` and `WithSubcommands()`. Note that the struct-first approach can be mixed with the programmatic approach for fine-grained control when
desired.

```go
package main

import (
    "fmt"
    "os"
    "github.com/napalu/goopt/v2"
)

func main() {
    parser := goopt.NewParser()
    
    // Add global flags
    parser.AddFlag("verbose", goopt.NewArg(
        goopt.WithDescription("Enable verbose output"),
        goopt.WithShortFlag("v"),
    ))
    
    // Create command hierarchy
    parser.AddCommand(
        goopt.NewCommand(
            goopt.WithName("create"),
            goopt.WithCommandDescription("Create resources"),
            // Add a force flag to the create command
            goopt.WithSubcommands(
                goopt.NewCommand(
                    goopt.WithName("user"),
                    goopt.WithCommandDescription("Create a new user"),
                    goopt.WithCallback(func(p *goopt.Parser, cmd *goopt.Command) error {
                        fmt.Println("Creating user...")
                        return nil
                    }),
                ),
                goopt.NewCommand(
                    goopt.WithName("group"),
                    goopt.WithCommandDescription("Create a new group"),
                    goopt.WithCallback(func(p *goopt.Parser, cmd *goopt.Command) error {
                        fmt.Println("Creating group...")
                        return nil
                    }),
                ),
            ),
        ),
    )
    
    // Add flags for specific commands
    parser.AddFlag("force", goopt.NewArg(
        goopt.WithShortFlag("f"),
        goopt.WithDescription("Force creation without confirmation"),
    ), "create")
    
    parser.AddFlag("name", goopt.NewArg(
        goopt.WithShortFlag("n"),
        goopt.WithDescription("User name"),
        goopt.WithRequired(true),
    ), "create", "user")
    
    parser.AddFlag("password", goopt.NewArg(
        goopt.WithShortFlag("p"),
        goopt.WithDescription("User password"),
        goopt.WithSecurePrompt("Enter password: "),
    ), "create", "user")
    
    parser.AddFlag("name", goopt.NewArg(
        goopt.WithShortFlag("n"),
        goopt.WithDescription("Group name"),
        goopt.WithRequired(true),
    ), "create", "group")
    
    // Parse arguments
    if !parser.Parse(os.Args) {
        fmt.Fprintln(os.Stderr, "Error parsing arguments:")
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, "  - %s\n", err)
        }
        parser.PrintUsageWithGroups(os.Stdout)
        return
    }
    
    // Execute the callbacks
    parser.ExecuteCommands()
    
    // Access flags via the parser
    if verbose, _ := parser.GetBool("verbose"); verbose {
        fmt.Println("Verbose mode enabled")
    }
    
    // Check which command was executed and access command-specific flags
    if parser.HasCommand("create user") {
        name, _ := parser.Get("name", "create", "user")
        fmt.Printf("User name: %s\n", name)
        
        // Access inherited flag from parent command
        if force, _ := parser.GetBool("force", "create"); force {
            fmt.Println("Force mode enabled")
        }
    } else if parser.HasCommand("create group") {
        name, _ := parser.Get("name", "create", "group")
        fmt.Printf("Group name: %s\n", name)
        
        // Access inherited flag from parent command
        if force, _ := parser.GetBool("force", "create"); force {
            fmt.Println("Force mode enabled")
        }
    }
}
```

## 3. Understanding Flag Types
   - goopt supports different types of flags which exhibit different behaviours: (take values, act as switches, handle lists).
   - **`Single`:** Default for most types (`string`, `int`, `float`, `time.Time`, etc.). Expects one value (`--output file.txt`). Type inference usually handles this. Tag: `type:single`.
   - **`Standalone`:** Default for `bool`. A switch, present means `true`. Can optionally take `true`/`false` (`--verbose`, `--verbose false`). Tag: `type:standalone`.
   - **`Chained`:** For list-like input (`[]string`, `[]int`, etc.). Expects a single string value split by delimiters (space, comma, pipe by default) (`--tags "go,lib,cli"`). Tag: `type:chained`.
   - **`File`:** Expects a file path. The *content* of the file becomes the flag's value (`--config settings.json`). Tag: `type:file`.
   
   When using the struct-first approach, flag types are often inferred from the field type. For instance, a `bool` will be mapped to a `Standalone` type,
   an array/slice of a supported type (like []string, []int) typically becomes `Chained,` a bool becomes `Standalone`, and most other basic Go types default to `Single`.  
   You can use the option tag to override type inference when using struct-tags or specify the type `WithType()` when using the programmatic approach. See  [Advanced Features]({{ site.baseurl }}/v2/guides/advanced-features/) for details.
 
## 4. Core Concepts
1. **Command Hierarchy**: 
   - Parent commands act as containers for subcommands
   - Only terminal commands (commands without subcommands) are executable
   - Commands form paths like "create user" or "create group"

2. **Flag Inheritance**:
   - Flags added to a parent command are inherited by all subcommands
   - Command-specific flags take precedence over inherited flags with the same name
   - You can access flags using their full path (`parser.Get("flag", "command", "subcommand")`)
   - Flags can be nested in structs to provide namespacing:
   
   ```go

   type Config struct {
      bool Verbose `goopt:"short:v"`
      type Update struct {
        CheckInterval int `goopt:"name:checkInterval;short:c"` // is available from command-line as --update.checkInterval 10
      }
   }
   ```

3. **Explicit by design**: 
   - Unlike some other CLI libraries, goopt does not automatically generate --help or -h flags. 
     This design choice prioritizes explicit definition and avoids potential conflicts with user-defined 
     flags (e.g., using -h for 'host'). You have complete control over how help is implemented.

## 5. Parsing & Error Handling
   Here's a brief example, showing how arguments are passed, flags are evaluated, and errors are handled.

   ```go

package main

import (
    "fmt"
    "os"
    "github.com/napalu/goopt/v2"
)

type Config struct {
    // Your other options...
    // explicit help 
    // accepts <program> -h for help 
    Help bool `goopt:"short:h;desc:Show this help message"`
}

func main() {

    cfg := &Config{}
    parser, err := goopt.NewParserFromStruct(cfg)
    if err != nil {
        // a parsing error occurred - the struct tag definition may be incorrect - check the error message for details.
        fmt.Fprintf(os.Stderr, err.Error())
        os.Exit(1)
    }
    if !parser.Parse(os.Args) {
        // Handle parsing errors by iterating over errors
        for _, err := range parser.GetErrors() {
            fmt.Fprintf(os.Stderr, " - %s\n", err)
        }

        parser.PrintUsageWithGroups(os.Stdout)
        os.Exit(1)
    }

    if cfg.Help { // cfg.Help will be true if use passed --help or -h from the command line
        parser.PrintUsageWithGroups(os.Stdout)
        os.Exit(0)
    }
}
```

## 6. Command Callbacks

goopt allows you to define callback functions that execute when specific commands are run. These callbacks provide a clean way to organize your command implementation logic.

### Defining Command Callbacks

You can define command callbacks in two ways:

#### 1. Programmatically with functional options:

```go
package main

import (
	"fmt"
   "github.com/napalu/goopt/v2"
)

func main() {
	parser := goopt.NewParser()
    // create a new command with callback
	parser.AddCommand(
        goopt.NewCommand(
			goopt.WithName("create"),
            goopt.WithCommandDescription("Create a resource"),
            goopt.WithCallback(func(p *goopt.Parser, cmd *goopt.Command) error {
                fmt.Println("Creating resource...")
                // Access flags via parser.Get(), parser.GetBool(), etc.
                return nil
            }),
      ),
   )
}


```
#### 2. Using a struct field of type `goopt.CommandFunc`:

```go
package main

import (
	"fmt"
	"os"
	"github.com/napalu/goopt/v2"
)

type Options struct {
   Create struct {
      Output string `goopt:"short:o;desc:Output file;required:true"`
      Exec   goopt.CommandFunc // Store the callback function
   } `goopt:"kind:command;desc:Create a resource"`
}

func main() {
      opts := &Options{}
      
      // Assign the callback function
      opts.Create.Exec = func(p *goopt.Parser, cmd *goopt.Command) error {
		  fmt.Println("Creating resource...")
          // Access flags via parser.Get() or the struct itself
          return nil
      }
      
      parser, _ := goopt.NewParserFromStruct(opts)
      
      // Configure parser to execute callbacks on successful parse
      parser.SetExecOnParse(true)
      
      // Parse arguments and execute callbacks automatically
      if !parser.Parse(os.Args) { 
		  // Handle errors...
      }
}
```

### Executing Command Callbacks

There are two ways to execute command callbacks:
1. **Automatically during parsing**: Set `parser.SetExecOnParse(true)` before parsing, or use `goopt.WithExecOnParse(true)` when creating the parser.
2. **Manually after parsing**: Call after a successful parse. `parser.ExecuteCommands()`

## 7. Command Callbacks with Struct Context
When defining callbacks for commands, you can access the original struct from any callback function:

```go
package main

import (
   "fmt"
   "os"
   "github.com/napalu/goopt/v2"
)

type Options struct {
   Verbose bool `goopt:"short:v;desc:Enable verbose output"`
   Create struct {
      Output string `goopt:"short:o;desc:Output file;required:true"`
      Exec   goopt.CommandFunc // Store the callback function
   } `goopt:"kind:command;desc:Create a resource"`
}

// Define a callback that can access the struct
func createHandler(p *goopt.Parser, cmd *goopt.Command) error {
   // Access the original struct (Go 1.18+)
   opts, ok := goopt.GetStructContextAs[*Options](p)
   if !ok {
      return fmt.Errorf("invalid struct context")
   }

   // Use the struct fields
   fmt.Printf("Creating resource: %s\n", opts.Create.Output)
   if opts.Verbose {
      fmt.Println("Verbose mode enabled")
   }

   return nil
}

func main() {
   opts := &Options{}
   opts.Create.Exec = createHandler // Assign the callback

   parser, _ := goopt.NewParserFromStruct(opts, goopt.WithExecOnParse(true)) // Auto-execute callback after parsing

   if !parser.Parse(os.Args) {
      // Handle errors
      for _, err := range parser.GetErrors() {
         fmt.Fprintf(os.Stderr, "Error: %s\n", err)
      }
      parser.PrintUsage(os.Stdout)
      os.Exit(1)
   }
}
```

## 8. Optional Features

### Shell Completion

You can add and install shell-completions for bash, zsh, fish, and powershell easily from within your program

```go
import (
    "os"
    "log"
    "fmt"
    "github.com/napalu/goopt/v2"
    c "github.com/napalu/goopt/v2/completion"
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
    var path string
    path, err = manager.Save()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%s completion script saved. Depending on your shell, you may need restart your shell or `source %s` the completion script.\n", "fish", path)
    // ... rest of your code ...
}
```
This pattern is especially useful for organizing callbacks in separate packages. See [Advanced Features: Command Callbacks with Struct Context]({{ site.baseurl }}/v2/guides/advanced-features/#command-callbacks-with-struct-context) for more details.


### Environment Variables

Environment variables are mapped to flags using string converters. In order to map env vars
to your flags you set a converter which ensures that environment variable are automatically
available as flags. The following example illustrates the concept:

```go
package main

import "github.com/napalu/goopt/v2"

type Config struct {
    // Your other options...
    // explicit help with short-hand mnemonics - this approach avoids conflicts
    // accepts <program> -h for help or -hu for for hostUrl
    HostUrl string `goopt:"short:hu"`
    Help bool `goopt:"short:h;desc:Show this help message"`
}

func main() {
    cfg := &Config{}
    parser, _ := goopt.NewParserFromStruct(cfg)
   
    // Enable environment variable support
    parser.SetEnvNameConverter(func(s string) string {
        // default flag name converter is lowerCamelCase
        return parser.DefaultFlagNameConverter(s)
    })

    // now env var HOST_URL would automatically be bound to your HostUrl var in struct
    // IMPORTANT: vars from config and explicit CLI vars have precedence over Env vars
    // Precedence is: 
    // Explicit CLI arg -> External Config (via ParseWithDefaults) -> Env vars -> Default values
    // where explit CLI args have the highest precedence
}
```

## 7. Version Compatibility

![Go Version](https://img.shields.io/badge/go-1.18%2B-blue)
![goopt Version](https://img.shields.io/github/v/tag/napalu/goopt)

See [Migration Guide]({{ site.baseurl }}/v2/migration/) for updates between versions.
goopt supports all Go versions from 1.18 onward. See our [compatibility policy]({{ site.baseurl }}/v2/compatibility/) for details.

## 8. Next Steps

- [Command structure patterns]({{ site.baseurl }}/v2/guides/command-organization/) - Have a look at different ways to structure your CLI
- [Flag structure patterns]({{ site.baseurl }}/v2/guides/flag-organization/) - Have a look at different ways to structure your flags
- [Positional Arguments]({{ site.baseurl }}/v2/guides/positional-arguments/) - Explore positional arguments
- [Struct Tags]({{ site.baseurl }}/v2/guides/struct-tags/) - Explore struct tags
- [Configuration Guide]({{ site.baseurl }}/v2/configuration/index/) - Environment variables and external config
- [Shell Completion]({{ site.baseurl }}/v2/shell/completion/) - Set up shell completions
- [Advanced Features]({{ site.baseurl }}/v2/guides/advanced-features/) - Explore dependencies, validation, and more

