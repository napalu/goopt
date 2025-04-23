---
layout: default
title: Flag structure patterns
parent: Guides
nav_order: 3
---

# Flag Structure Patterns

goopt supports a variety of flag organization styles.

## 1. Flat Flag Structure
```go
type Options struct {
    Host        string `goopt:"name:host"`
    Port        int    `goopt:"name:port"`
    ConfigPath  string `goopt:"name:config-path"`
    LogLevel    string `goopt:"name:log-level"`
}
```

### Advantages:
- Simple and straightforward
- Easy to see all available flags
- Good for small applications

### Trade-offs:
- Can become unwieldy with many flags
- No logical grouping
- Potential naming conflicts

## 2. Namespaced Flags (Nested Structs)
You can organize flags by nesting structs. Field names of regular (non-command) nested structs contribute to a dot-notation prefix for the long flag names invoked on the command line.
The prefix is derived using the configured FlagNameConverter.

```go
type Options struct {
    Server struct {
        Host string `goopt:"name:host"`
        Port int    `goopt:"name:port"`
    }
    Logging struct {
        Level   string `goopt:"name:level"`
        Path    string `goopt:"name:path"`
        Format  string `goopt:"name:format"`
    }
}
```

```go
type Config struct {
    // Non-command struct acts as a namespace container
    Database struct {
        Host string `goopt:"name:host"` // Invoked as --database.host
        Port int    `goopt:"name:port"` // Invoked as --database.port
    }
}
```

**Important**: Structs marked with `kind:command` define commands and scopes for flag resolution, but their field names do not add to the dot-notation prefix for invoking flags defined within them.
The prefix is determined only by the nesting of non-command structs.

```go
type Config struct {
    App struct { // 'App' is NOT a command, provides 'app.' prefix
        Service struct { // 'Service' IS a command, defines path 'service', NO 'service.' prefix
            Port int `goopt:"name:port;short:p"` // Invoked as --app.port or -p

            Stop struct { // 'Stop' IS a command, defines path 'service stop', NO 'stop.' prefix
                Force bool `goopt:"name:force;short:f"` // Invoked as --app.force or -f
            } `goopt:"kind:command"`
        } `goopt:"kind:command"`
    }
}
```

### Advantages:
- Logical grouping of related flags
- Automatic flag namespacing (e.g., `server.host`)
- Better organization for many flags
- Clear ownership of configuration

### Trade-offs:
- Longer flag names
- More nested structures
- Need to consider namespace collisions

## 3. Reusable Flag Groups
```go
type DatabaseConfig struct {
    Host     string `goopt:"name:host"`
    Port     int    `goopt:"name:port"`
    User     string `goopt:"name:user"`
    Password string `goopt:"name:password"`
}

type Options struct {
    Primary   DatabaseConfig 
    Replica   DatabaseConfig 
}
```

### Advantages:
- Reuse common flag groups
- Consistent configuration across similar components
- DRY principle applied to flags
- Good for repeated configurations

### Trade-offs:
- Need to manage prefixes carefully
- Can lead to very long flag names
- May expose unnecessary options

## 4. Mixed Approach with Pointers
```go
type LogConfig struct {
    Level  string `goopt:"name:level"`
    Format string `goopt:"name:format"`
}

type Options struct {
    Server struct {
        Host string `goopt:"name:host"`
        Port int    `goopt:"name:port"`
        Logs *LogConfig
    }
    Client struct {
        Endpoint string `goopt:"name:endpoint"`
        Logs    *LogConfig
    }
}
```

### Advantages:
- Flexible composition of flag groups
- Optional configuration sections
- Good for complex applications
- Reuse without namespace conflicts

### Trade-offs:
- Need to handle nil pointers
- Less obvious flag structure
- More complex initialization

## Best Practices

1. Choose organization based on scale:
   - Flat structure for simple applications
   - Namespaced for medium complexity
   - Reusable groups for large applications

2. Namespace Guidelines:
   - Use consistent naming patterns
   - Keep namespace depth reasonable (2-3 levels max)
   - Consider command-line usability

3. Flag Naming:
   - Prefix for component disambiguation
   - Keep names concise but clear

4. Documentation:
   - Group related flags in help output
   - Document default values
   - Explain namespace structure

5. Reusability:
   - Create common flag groups for repeated patterns
   - Use pointers for optional configurations
   - Consider versioning for shared configs

6. Validation:
   - Validate at appropriate namespace levels
   - Handle dependencies between namespaces
   - Consider required vs optional groups