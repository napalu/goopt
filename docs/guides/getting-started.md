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
```go
// Define your CLI structure using struct tags
type Options struct {
    // Global flags
    Verbose bool   `goopt:"name:verbose;short:v;desc:Enable verbose output"`
    Output  string `goopt:"name:output;desc:Output file;required:true"`
    
    // Commands and their flags
    Create struct {
        User struct {
            Name     string `goopt:"name:name;desc:Username to create;required:true"`
            Password string `goopt:"name:password;desc:User password;secure:true"`
        } `goopt:"kind:command;name:user;desc:Create a new user"`
    } `goopt:"kind:command;name:create;desc:Create resources"`
}

func main() {
    opts := &Options{}
    parser, _ := goopt.NewParserFromStruct(opts)
    
    if !parser.Parse(os.Args) {
        for _, err := range parser.Errors() {
            fmt.Println(err)
        }   
        parser.PrintUsageWithGroups(os.Stdout)
        return
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
    create := parser.AddCommand("create", "Create resources")
    user := create.AddCommand("user", "Create a new user")
    user.AddFlag("name", goopt.NewArg(
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
}
```

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

## Next Steps

- [Command Organization Guide](command-organization.md) - Have a look at different ways to structure your CLI
- [Advanced Features](advanced-features.md) - Explore dependencies, validation, and more
- [Configuration Guide](configuration/index.md) - Environment variables and external config
- [Shell Completion](shell/completion.md) - Set up shell completions
