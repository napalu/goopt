---
layout: default
title: Advanced Features
parent: Guides
nav_order: 3
---

# Advanced Features

## Struct Tag formats

goopt supports two different formats for struct tags:
New format:
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

Old format (deprecated will be removed in the next major release):
The tag format uses space-separated quoted key:value pairs:
- `long`: Long name for the `flag` - defaults to the field name if not specified
- `short`: Short (single-character) name of the `flag` when POSIX compatibility is enabled - otherwise can be a multi-character string used as a mnemonic for the `flag` name (default)
- `description`: Description of the `flag`
- `type`: `Flag` type (single|standalone|chained|file) - defaults to single
- `required`: Whether the `flag` is required (true|false) - defaults to true
- `default`: Default value for the `flag`
- `secure`: For `flag` containing password input (true|false) - defaults to false
- `prompt`: Prompt text for secure input `flag`
- `accepted`: `Flag` which matches on values using one or more patterns - a pattern can be a literal value or a regex pattern (e.g. `pattern:json|yaml,desc:Format type`)
- `depends`: `Flag` dependencies - a dependency can be a flag or set of flags or a set of flags and values (e.g. `flag:output,values:[json,yaml]`)


## Nested Struct Access

Nested structs can be accessed using dot notation, allowing for deep flag hierarchies:

```go
type Config struct {
    Database struct {
        Connection struct {
            Host string `goopt:"name:host"`
            Port int    `goopt:"name:port"`
        }
        Timeout int `goopt:"name:timeout"`
    }
}

// Access using dot notation:
--database.connection.host localhost
--database.connection.port 5432
--database.timeout 30
```

Nested structs are automatically treated as flag containers unless explicitly marked as commands:
- No special initialization required
- Fields are accessible via dot notation
- Validation ensures struct fields exist


## Slice Handling

When using slices in your configuration struct, you must initialize them to the desired size before parsing flags. This is a deliberate design decision for security and resource management:

```go
type Config struct {
    Command struct {
        Items []struct {
            Flag string `goopt:"name:flag"`
        }
    } `goopt:"kind:command;name:command"`
}

// Initialize slice before use
config.Command.Items = make([]struct{ Flag string }, expectedSize)
```

### Nested Slice Access

When accessing nested slices, use dot notation with indices:
```go
// Valid patterns:
--items.0.flag value        // Simple slice access
--items.0.nested.1.flag value  // Nested slice access

// Invalid patterns:
--items.flag value          // Missing index
--items.0.1.flag value      // Missing field name between indices
```

All slice access is validated during parsing:
- Index bounds are checked (e.g., "index out of bounds at 'items.5': valid range is 0-2")
- Struct field access is validated (e.g., "cannot access field 'flag' on non-struct at 'items.0'")
- Path format is verified before processing

### Why Pre-initialization?

1. **Security**: Prevents resource exhaustion attacks through flag manipulation
2. **Memory Control**: Explicit control over slice allocation
3. **Predictable Behavior**: Clear boundaries for slice-based flags
4. **Code Clarity**: Explicit initialization makes the expected slice size immediately clear to readers, simplifying both implementation and usage

Without this requirement, malicious input could cause uncontrolled memory growth:
```go
// This is prevented by requiring explicit initialization
app --items.100000000000000000000.flag=value // Could exhaust memory
``` 
