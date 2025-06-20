---
layout: default
title: Command Callbacks
parent: Defining Your CLI
nav_order: 5
version: v2
---

# Command Callbacks: Adding Behavior

Defining your commands and flags is only the first step. To make your application do something useful, you need to attach logic to your commands. In `goopt`, this is done using **command callbacks**.

A callback is simply a Go function that gets executed when its associated command is run.

## The `goopt.CommandFunc` Type

All command callbacks in `goopt` have the following signature:

```go
type CommandFunc func(p *goopt.Parser, cmd *goopt.Command) error
```
This function receives two arguments:
*   `p *goopt.Parser`: The main parser instance, which gives you access to all parsed flags and application state.
*   `cmd *goopt.Command`: The specific `Command` object that triggered this callback.

## Defining Callbacks

You can define callbacks using either the programmatic or struct-based approach.

### 1. Programmatic Approach

When creating a command programmatically, you can attach a callback using `WithCallback()`.

```go
import "github.com/napalu/goopt/v2"

func handleCreateUser(p *goopt.Parser, cmd *goopt.Command) error {
    fmt.Println("Executing the create user command...")
    // Your logic here...
    return nil
}

func main() {
    parser := goopt.NewParser()
    parser.AddCommand(
        goopt.NewCommand(
            goopt.WithName("create-user"),
            goopt.WithCallback(handleCreateUser),
        ),
    )
    // ...
}
```

### 2. Struct-Based Approach (Recommended)

When using structs, you add a field of type `goopt.CommandFunc` to your command's struct definition. A common convention is to name this field `Exec`.

```go
type Config struct {
    Create struct {
        // This field will hold the function to execute.
        Exec goopt.CommandFunc
    } `goopt:"kind:command"`
}

func main() {
    cfg := &Config{}
    // Assign your handler function to the Exec field.
    cfg.Create.Exec = handleCreate // 'handleCreate' is a function you've written
    
    parser, _ := goopt.NewParserFromStruct(cfg)
    // ...
}
```

## Accessing Flag Data in Callbacks

This is the most critical part of using callbacks. Since your callback function might be in a different package, you need a reliable way to access the parsed configuration from your main `Config` struct.

`goopt` provides the helper function `goopt.GetStructCtxAs[T](parser)` for this exact purpose. It safely retrieves and type-casts the original struct you used to create the parser.

### Complete Example: Separating Logic

This pattern is ideal for keeping your command logic separate from your CLI definition.

**`main.go`:**
```go
package main

import (
    "fmt"
    "os"
    "myapp/handlers"
    "myapp/types"
    "github.com/napalu/goopt/v2"
)

func main() {
    cfg := &types.Config{}
    
    // Assign the callback from the handlers package.
    cfg.Create.File.Exec = handlers.CreateFileHandler
    
    parser, _ := goopt.NewParserFromStruct(cfg)
    
    // Parse args and then manually execute the commands.
    if !parser.Parse(os.Args) {
        // ... error handling ...
        os.Exit(1)
    }
    
    if errs := parser.ExecuteCommands(); errs > 0 {
        fmt.Fprintln(os.Stderr, "One or more commands failed.")
        os.Exit(1)
    }
}
```

**`types/config.go`:**
```go
package types

import "github.com/napalu/goopt/v2"

type Config struct {
    Verbose bool `goopt:"short:v"`
    Create struct {
        File struct {
            Output string `goopt:"short:o;required:true"`
            Exec   goopt.CommandFunc // The callback field
        } `goopt:"kind:command"`
    } `goopt:"kind:command"`
}
```

**`handlers/file.go`:**
```go
package handlers

import (
    "fmt"
    "myapp/types"
    "github.com/napalu/goopt/v2"
)

// CreateFileHandler is the callback function.
func CreateFileHandler(p *goopt.Parser, cmd *goopt.Command) error {
    // Use GetStructCtxAs to safely get the fully-populated config struct.
    cfg, ok := goopt.GetStructCtxAs[*types.Config](p)
    if !ok {
        return fmt.Errorf("internal error: could not get struct context")
    }
    
    // Now you have type-safe access to all flags.
    if cfg.Verbose {
        fmt.Println("Verbose mode enabled.")
    }
    
    fmt.Printf("Creating file: %s\n", cfg.Create.File.Output)
    // ... file creation logic ...
    
    return nil
}
```

## Controlling Callback Execution

You have full control over *when* your callbacks run.

#### 1. Manual Execution (Default & Recommended)
This is the simplest and most flexible approach. After `parser.Parse()` returns successfully, you call `parser.ExecuteCommands()`.

```go
if !parser.Parse(os.Args) {
    // ... handle errors ...
}

// Execute all recognized commands in the order they appeared.
if errCount := parser.ExecuteCommands(); errCount > 0 {
    // Handle execution errors...
}
```
**Use this when:** You want to validate all input first before running any logic.

#### 2. Automatic Execution During Parsing
Callbacks execute immediately as `goopt` recognizes their command during the parsing process.

```go
// At parser creation:
parser, _ := goopt.NewParserFromStruct(cfg, goopt.WithExecOnParse(true))

// Or set it on the instance:
parser.SetExecOnParse(true)
```
**Use this when:** Early commands need to set up state for later commands or flags on the same command line.

#### 3. Automatic Execution After Parsing
Callbacks are queued during parsing and all are executed automatically at the very end of a successful `parser.Parse()` call.

```go
parser, _ := goopt.NewParserFromStruct(cfg, goopt.WithExecOnParseComplete(true))
```
**Note:** This has no effect if `ExecOnParse` is also true.

## Execution Order
When a command and subcommand are invoked (e.g., `myapp create user`), their callbacks are executed in hierarchical order:
1.  Callback for the parent command (`create`) runs first.
2.  Callback for the child command (`user`) runs second.

This allows parent commands to perform setup tasks (like initializing a client) that subcommands can then use.