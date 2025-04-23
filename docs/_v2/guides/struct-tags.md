---
layout: default
title: Struct Tags
parent: Guides
nav_order: 5
---

# Struct Tag Reference

## Tag Format

The `goopt` tag format uses semicolon-separated key:value pairs. All options are defined under the `goopt` namespace:

```go
type Config struct {
    Output string `goopt:"name:output;desc:Output file;required:true"`
}
```

## Available Tags

| Tag | Description | Example |
|-----|-------------|---------|
| `kind` | Specifies if it's a flag or command (default: flag) | `kind:flag|command` |
| `name` | Long name for the flag/command | `name:output` |
| `short` | Short name (single-char for POSIX mode) | `short:o` |
| `desc` | Description shown in help | `desc:Output file` |
| `type` | Flag type | `type:single\|standalone\|chained\|file` |
| `required` | Whether flag is required | `required:true\|false` |
| `default` | Default value | `default:stdout` |
| `secure` | For password input | `secure:true\|false` |
| `prompt` | Prompt text for secure input | `prompt:Password:` |
| `capacity` | Slice capacity for nested structs | `capacity:3` |
| `pos` | Position requirements | `pos:0` |
| `accepted` | Accepted values/patterns | `accepted:{pattern:json\|yaml,desc:Format}` |
| `depends` | Flag dependencies | `depends:{flag:output,values:[json]}` |

## Position Tag

The `pos` tag allows specifying position requirements for arguments:

```go
type Config struct {
    // Must be first argument
    Source string `goopt:"name:source;pos:0"`
        // Second argument from start
    Profile string `goopt:"name:profile;pos:1"`
    // Must be last argument
    Dest string `goopt:"name:dest;pos:2"`
    
}
```

## Complex Tag Examples

### Multiple Accepted Values
```go
type Config struct {
    Format string `goopt:"name:format;accepted:{pattern:json|yaml,desc:Format},{pattern:text|binary,desc:Type}"`
}
```

### Multiple Dependencies
```go
type Config struct {
    Compress bool `goopt:"name:compress;depends:{flag:format,values:[json]},{flag:output,values:[file]}"`
}
```

## Flag Namespacing with Nested Structs:

You can organize flags by nesting structs. Field names of regular (non-command) nested structs contribute to a dot-notation prefix for the long flag names invoked on the command line.
The prefix is derived using the configured FlagNameConverter.

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