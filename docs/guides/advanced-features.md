---
layout: default
title: Advanced Features
parent: Guides
nav_order: 3
---

# Advanced Features

## Struct Tag formats

| Feature | New Format | Old Format (Deprecated) |
|---------|------------|------------------------|
| Separator | Semicolon (;) | Space |
| Key-Value Delimiter | Colon (:) | Colon (:) |
| Tag Name | goopt | N/A |
| Kind | kind:flag\|command | N/A |
| Long Name | name:value | long:value |
| Short Name | short:value | short:value |
| Description | desc:value | description:value |
| Type | type:single\|standalone\|chained\|file | type:single\|standalone\|chained\|file |
| Required | required:true\|false | required:true\|false |
| Default Value | default:value | default:value |
| Secure Input | secure:true\|false | secure:true\|false |
| Prompt Text | prompt:value | prompt:value |
| Accepted Values | accepted:{pattern:json\|yaml,desc:Format type},{pattern:text\|binary,desc:Type} | accepted:pattern:json\|yaml,desc:Format type |
| Dependencies | depends:{flag:output,values:[json,yaml]},{flag:mode,values:[text]} | depends:flag:output,values:[json,yaml] |

The new format offers several advantages:
- Namespace Isolation: Using goopt: prefix prevents conflicts with other tag parsers
- Better Compatibility: Semicolon-separated format is more common in Go struct tags
- Clearer Structure: All options are under the goopt namespace
- Future Extensibility: New features can be added without breaking existing parsers

To migrate from the old format to the new one, you can use the migration tool:

[Migration Tool Documentation](https://github.com/napalu/goopt/blob/main/migration/README.md)

The tool will automatically update your struct tags while preserving functionality.

### Complex Tag Formats

#### Accepted Values

Multiple accepted patterns can be specified using brace-comma notation:

```go
type Config struct {
    // Single pattern
    Format string `goopt:"name:format;accepted:{pattern:json|yaml,desc:Output format}"`

    // Multiple patterns
    Mode string `goopt:"name:mode;accepted:{pattern:read|write,desc:Access mode},{pattern:sync|async,desc:Operation mode}"`
}
```

### Dependencies
Dependencies use the same brace-comma notation:

```go
type Config struct {
    // Single dependency
    Format string `goopt:"name:format;depends:{flag:output,values:[file,dir]}"`

    // Multiple dependencies
    Compress bool `goopt:"name:compress;depends:{flag:format,values:[json]},{flag:output,values:[file,dir]}"`
}
```

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
