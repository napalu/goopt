---
layout: default
title: Flag Patterns
parent: Defining Your CLI
nav_order: 3
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