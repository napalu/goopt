---
layout: default
title: Flag structure patterns
parent: Guides
nav_order: 3
---

# Flag Structure Patterns

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
    Primary   DatabaseConfig `goopt:"prefix:primary-db"`
    Replica   DatabaseConfig `goopt:"prefix:replica-db"`
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
   - Use kebab-case for flag names
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