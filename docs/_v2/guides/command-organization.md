---
layout: default
title: Command structure patterns
parent: Guides
nav_order: 2
---

# Command structure patterns

goopt offers several approaches to organizing commands and flags:

## 1. Flag-Centric Approach (Path-Based)
```go

 // path is a comma-separated list of command paths - commands are created on the fly and flags are shared across commands
type Options struct {
    Host string `goopt:"name:host;path:server start,server stop"`
    Port int    `goopt:"name:port;path:server start"`
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

