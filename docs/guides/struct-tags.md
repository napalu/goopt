---
layout: default
title: Struct Tags
parent: Guides
nav_order: 4
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
| `kind` | Specifies if it's a flag or command (default: flag) | `kind:flag\|command` |
| `name` | Long name for the flag/command | `name:output` |
| `short` | Short name (single-char for POSIX mode) | `short:o` |
| `desc` | Description shown in help | `desc:Output file` |
| `type` | Flag type | `type:single\|standalone\|chained\|file` |
| `required` | Whether flag is required | `required:true\|false` |
| `default` | Default value | `default:stdout` |
| `secure` | For password input | `secure:true\|false` |
| `prompt` | Prompt text for secure input | `prompt:Password:` |
| `capacity` | Slice capacity for nested structs | `capacity:3` |
| `pos` | Position requirements | `pos:{at:start,idx:0}` |
| `accepted` | Accepted values/patterns | `accepted:{pattern:json\|yaml,desc:Format}` |
| `depends` | Flag dependencies | `depends:{flag:output,values:[json]}` |

## Position Tag

The `pos` tag allows specifying position requirements for arguments:

```go
type Config struct {
    // Must be first argument
    Source string `goopt:"name:source;pos:{at:start,idx:0}"`
    
    // Must be last argument
    Dest string `goopt:"name:dest;pos:{at:end,idx:0}"`
    
    // Second argument from start
    Profile string `goopt:"name:profile;pos:{at:start,idx:1}"`
}
```

### Position Types
- `start`: Argument must appear before flags/commands
- `end`: Argument must appear after flags/commands

### Index
The optional `idx` field specifies relative position when multiple arguments share the same position type.

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
