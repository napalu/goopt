---
layout: default
title: Flag Patterns
parent: Defining Your CLI
nav_order: 3
version: v2
---

# Flag Patterns

`goopt` provides several patterns for organizing and structuring your flags, from simple flat layouts to reusable, namespaced groups.

## 1. Flat Flag Structure

The simplest pattern is a single struct containing all your application's flags. This is ideal for small tools with a limited number of options.

```go
type Options struct {
    Host        string `goopt:"name:host"`
    Port        int    `goopt:"name:port"`
    ConfigPath  string `goopt:"name:config-path"`
    LogLevel    string `goopt:"name:log-level"`
}
```
**Usage:** `--host localhost --port 8080`

## 2. Namespaced Flags (Nested Structs)

To avoid naming conflicts and group related flags, you can nest structs. `goopt` automatically creates dot-delimited flag names based on the struct hierarchy.

**Important:** Only regular structs (which are not `kind:command`) contribute to the namespace prefix.

```go
type Config struct {
    // 'Database' is not a command, so it creates a 'database.' prefix.
    Database struct {
        Host string `goopt:"name:host"` // Invoked as --database.host
        Port int    `goopt:"name:port"` // Invoked as --database.port
    }

    // 'Cache' is also a namespace.
    Cache struct {
        Host string `goopt:"name:host"` // Invoked as --cache.host
        TTL  int    `goopt:"name:ttl"`  // Invoked as --cache.ttl
    }
}
```
**Usage:** `--database.host db.server.com --cache.host cache.server.com`

**Advantages:**
- **Logical Grouping:** Organizes flags into clear, related sections.
- **Prevents Collisions:** You can have multiple `--host` flags under different namespaces.

## 3. Reusable Flag Groups (Embedded Structs)

For configurations that are repeated, you can define them once in a struct and embed it multiple times.

```go
// Define a reusable group of database flags.
type DatabaseConfig struct {
    Host     string `goopt:"name:host"`
    Port     int    `goopt:"name:port"`
    User     string `goopt:"name:user"`
}

// Reuse the config for primary and replica databases.
type Options struct {
    // goopt will create flags like --primary.host, --primary.port, etc.
    Primary   DatabaseConfig 
    
    // And also --replica.host, --replica.port, etc.
    Replica   DatabaseConfig 
}
```

**Advantages:**
- **DRY:** Keeps your configuration consistent and avoids repetition.
- **Modularity:** Encourages building your configuration from smaller, reusable components.

## 4. Repeated Flags (Slices)

For flags that can be specified multiple times, use a slice. `goopt` will automatically collect all values. This is often used for including directories, setting tags, or passing multiple values.

```go
type Config struct {
    IncludeDirs []string `goopt:"short:I;desc:Paths to include"`
    Tags        []string `goopt:"short:t;desc:Tags to apply"`
}
```

**Usage:** `goopt` supports two common styles for providing multiple values:

1.  **Repeated Flag:** The standard way in many CLI tools.
    ```bash
    ./myapp -I /path/one -I /path/two -t feature-a -t feature-b
    ```

2.  **Delimited String:** A more compact way to provide values.
    ```bash
    ./myapp --include-dirs "/path/one,/path/two" --tags "feature-a,feature-b"
    ```

`goopt` handles both styles seamlessly for any slice-based flag.

## 5. Naming Conventions and Converters

`goopt` provides name converters to enforce consistent naming conventions across your CLI. These converters automatically transform struct field names to match your preferred style.

### Setting Name Converters

```go
parser, _ := goopt.NewParserWith(
    // Convert struct fields to flag names
    goopt.WithFlagNameConverter(goopt.ToKebabCase),      // MaxConnections → max-connections
    goopt.WithCommandNameConverter(goopt.ToLowerCamel),   // ServerStart → serverStart
    goopt.WithEnvNameConverter(goopt.ToUpperSnake),      // MAX_CONNECTIONS → max_connections
)
```

### Available Converters

- `goopt.ToLowerCamel`: `MaxConnections` → `maxConnections`
- `goopt.ToKebabCase`: `MaxConnections` → `max-connections`
- `goopt.ToSnakeCase`: `MaxConnections` → `max_connections`
- `goopt.ToUpperSnake`: `MaxConnections` → `MAX_CONNECTIONS`
- Custom function: `func(string) string`

### How Converters Work

Converters apply to struct fields that don't have an explicit `name` tag:

```go
type Config struct {
    // Uses converter: MaxConnections → max-connections (with ToKebabCase)
    MaxConnections int `goopt:"short:m"`
    
    // Explicit name always wins, converter not applied
    ServerPort int `goopt:"name:port;short:p"`
    
    // Nested struct field also uses converter
    Database struct {
        ConnectionLimit int  // → database.connection-limit
    }
}
```

### Naming Consistency Warnings

`goopt` can warn you about naming inconsistencies in your CLI. This helps maintain a consistent style across your entire application:

```go
// After parsing, check for warnings
warnings := parser.GetWarnings()
for _, warning := range warnings {
    fmt.Println("Warning:", warning)
}

// Example warnings:
// "Flag '--my_flag' doesn't follow naming convention (converter would produce '--my-flag')"
// "Translation '--max-verbindungen' for flag '--maxConnections' doesn't follow naming convention"
```

This is particularly useful when:
- Migrating an existing CLI to use consistent naming
- Working with translations that should follow the same convention
- Ensuring environment variable mappings work correctly

### Best Practices

1. **Set converters early**: Define your naming convention when creating the parser
2. **Be consistent**: Use the same convention for flags, commands, and environment variables
3. **Document your choice**: Make it clear to users what naming style your CLI uses
4. **Check warnings**: Especially when adding new flags or translations