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
