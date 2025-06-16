---
layout: default
title: Command structure patterns
parent: Guides
nav_order: 2
---

# Command structure patterns

goopt offers several approaches to organizing commands and flags:

## 1. Flag-Centric Approach (Path-Based)

For scenarios where multiple commands share common flags, or when you prefer a more declarative, flat structure, 
the path tag is an exceptionally powerful tool. It allows you to define your command structure directly from your flag definitions, rather than through nested structs.

### Core Concepts
1. **Dynamic Command Creation**: When you specify a path like create user, goopt automatically creates the create command and its subcommand user if they don't already exist. This lets you define your entire command hierarchy without a single kind:command tag.
2. **Flag Association and Sharing**: By listing comma-separated command paths, you make a single flag field available to multiple commands. The flag is evaluated within the context of the command that is invoked, and its value is populated into the shared struct field.

### Example

In this example, SharedFlag is made available to both the create user and create group commands. Its value is populated into the CLI.SharedFlag field, which can be checked after determining which command was run.

```go

package main

import (
   "fmt"
   "github.com/napalu/goopt/v2"
   "log"
   "os"
)

// The CLI struct defines flags. Commands are created dynamically via `path`.
type CLI struct {
      // This flag is associated with two different command paths.
      SharedFlag string `goopt:"short:s;desc:A shared flag for user and group creation;path:create user,create group"`
      
      // This flag is specific to the 'create user' command.
      UserEmail string `goopt:"short:e;desc:Email for the new user;path:create user"`
}

func main() {
      opts := &CLI{}
      
      // The `path` directives in the struct will dynamically create the command
      // hierarchy: 'create user' and 'create group'.
      parser, err := goopt.NewParserFromStruct(opts)
      if err != nil {
		  log.Fatalf("failed to create parser: %w", err)
      }
      
      ok := parser.Parse(os.Args)
      if !ok {
         // If parsing fails, PrintUsage will show the dynamically created commands.
         parser.PrintUsage(os.Stderr)
         os.Exit(1)
      }
      
      // -- Command Line Examples --
      //
      // 1. Run with the 'create user' command:
      //    go run . create user -s "common value" -e "user@example.com"
      //
      // 2. Run with the 'create group' command:
      //    go run . create group -s "common value"
      
      // Check which command was executed and access the flag values.
      if parser.HasCommand("create user") {
         fmt.Println("Executing 'create user' command...")
         fmt.Printf(" -> Shared Flag: %s\n", opts.SharedFlag)
         fmt.Printf(" -> User Email: %s\n", opts.UserEmail)
      } else if parser.HasCommand("create group") {
         fmt.Println("Executing 'create group' command...")
         // Note: opts.UserEmail would be empty here, as it's not in the path.
         //fmt.Printf(" -> Shared Flag: %s\n", opts.SharedFlag)
      } else {
         fmt.Println("No specific 'create' command was run.")
      }
}
  
```

### Advantages:
- Flags can be shared across commands
- Clear visibility of flag reuse
- Flexible command path definition
- Good for commands sharing many flags

### Trade-offs:
- Command structure less visible in code
- Can become hard to maintain for complex hierarchies
- Need to carefully manage path strings

## 2. Explicit Command Declaration with flags
```go
type Options struct {
    Server Command {
        Name: "server",
        Subcommands: []Command{
            {Name: "start", Description: "Start server"},
            {Name: "stop", Description: "Stop server"},
        }
    },
    Host string `goopt:"name:host;path:server start,server stop"` // This is a flag - the path is a comma-separated list of command paths - commands are created on the fly if not found
}
```

### Advantages:
- Clear command hierarchy
- Full control over command properties
- Good for static command structures
- Easy to add command-specific behavior

### Trade-offs:
- More verbose
- Less flexible for dynamic command structures
- Flags can be shared across commands via the path but must be defined outside of the command struct

## 3. Struct Tag Command Definition
```go
type Options struct {
    Server struct {
        Start struct {
            Host string `goopt:"name:host"` // This is a flag
            Port int    `goopt:"name:port"` // This is a flag
        } `goopt:"kind:command"`
        Stop struct{} `goopt:"kind:command"`
    } `goopt:"kind:command"`
}
```

### Advantages:
- Clear command hierarchy in code structure
- Natural nesting of commands
- Command-specific flags clearly grouped
- Good for complex command hierarchies

### Trade-offs:
- Flags can only be shared between commands sharing the same parents
- More nested structures
- Can lead to deeper type hierarchies

## 4. Shared Resources via Pointers
```go
type ServerConfig struct {
    Host string `goopt:"name:host"`
    Port int    `goopt:"name:port"`
}

type Options struct {
    Start struct {
        *ServerConfig `goopt:"name:server"` // This is a pointer to a ServerConfig struct - the name can be overridden with the name tag
    } `goopt:"kind:command"` // command start has a pointer to a ServerConfig struct
    Stop struct {
        *ServerConfig `goopt:"name:server"` // This is a pointer to a ServerConfig struct - the name can be overridden with the name tag
    } `goopt:"kind:command"` // command stop has a pointer to a ServerConfig struct
}
```

### Advantages:
- Reuse configuration structures
- DRY principle for shared settings
- Good for modular command configurations
- Flexible composition of command options

### Trade-offs:
- Need to manage nil pointers
- Less explicit flag visibility
- Can hide dependencies

## Best Practices

1. Choose based on your primary use case:
   - Flag-centric for shared flags across commands
   - Explicit commands for static hierarchies
   - Struct tags for complex, unique command trees
   - Pointer sharing for modular configurations

2. Consider maintenance:
   - Prefer struct tags for deep hierarchies
   - Use explicit commands for simple hierarchies
   - Use flag paths for flat command structures

3. Consider visibility:
   - Make command structure clear in code
   - Document shared flags
   - Use consistent naming patterns

4. Handle complexity:
   - Break down deep hierarchies
   - Use shared structs for common patterns
   - Consider generating complex structures

5. Error handling:
   - Validate command paths
   - Check for flag name collisions
   - Handle nil pointers appropriately

