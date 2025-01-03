---
layout: default
title: Advanced Features
parent: Guides
nav_order: 4
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
| SliceCapacity | capacity:value | N/A |
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

#### Dependencies
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

When using slices in your configuration struct, there are two distinct cases:

### 1. Terminal Flag Slices
Terminal flag slices (slices at the end of a path) automatically accept comma-separated values:

```go
type Config struct {
    Command struct {
        Items []struct {
            Flags []string `goopt:"name:flag"` // Terminal slice
        }
    }
}

// Usage:
--command.items.0.flag="one,two,three"  // Automatically splits into slice
```

### 2. Nested Structure Slices
For slices of structs (nested slices), you can specify their capacity using the `capacity` tag:

```go
type Config struct {
    Command struct {
        Items []struct `goopt:"capacity:3"` {  // Nested slice needs capacity
            Flag []string `goopt:"name:flag"`  // Terminal slice
        }
    } `goopt:"kind:command;name:command"`
}

// Usage:
--command.items.0.flag="one,two,three"
--command.items.1.flag="four,five,six"
--command.items.2.flag="seven,eight"
```

Important notes:
1. The `capacity` tag is optional and only needed for nested struct slices
2. Terminal flag slices are automatically sized based on input
3. Memory safety is ensured by flag registration - only valid paths are accepted
4. Attempting to use an index outside the registered range results in "unknown flag" error
5. Slice bounds are tracked for user feedback when using `NewParserFromStruct` but are not required for memory safety