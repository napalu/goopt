---
layout: default
title: The Help System
parent: Built-in Features
nav_order: 1
---

# The Help System

`goopt` features a powerful, adaptive help system that is enabled by default. It's designed to provide sensible, attractive help text with zero configuration, while also offering deep customization options for complex applications.

This guide covers everything from the automatic `--help` flag to creating advanced, hierarchical help displays.

## Automatic Help (`auto-help`)

Out of the box, `goopt` handles help generation for you.

*   **Automatic Flags:** The `--help` and `-h` flags are automatically registered if you haven't defined them yourself.
*   **Automatic Display:** When a user passes `--help`, `goopt` displays the help text and your program can exit cleanly.

### Default Behavior

You don't need to do anything to enable basic help functionality.

```go
// In your main function:
parser, _ := goopt.NewParserFromStruct(&Config{})

// Parse arguments
if !parser.Parse(os.Args) {
    // Handle parsing errors...
    os.Exit(1)
}

// Check if help was shown and exit cleanly.
if parser.WasHelpShown() {
    os.Exit(0)
}

// ... your application logic continues here ...
```

### Disabling Auto-Help

If you need complete control over the `--help` flag, you can disable the automatic behavior.

```go
parser, _ := goopt.NewParserWith(
    goopt.WithAutoHelp(false),
)
// Now, --help and -h are not registered automatically.
```

## Configuring Help Output

You can customize every aspect of the help system's appearance and behavior.

### Help Styles

`goopt` offers several help styles to best match your CLI's complexity. You can set the style with `parser.SetHelpStyle()` or `goopt.WithHelpStyle()`.

#### `HelpStyleSmart` (Default)
`goopt` analyzes your CLI's complexity (number of flags and commands) and automatically selects the most appropriate style. This is the recommended default for most applications.

#### `HelpStyleFlat`
The traditional, simple list of all flags and commands. Best for small tools.
```
Usage: myapp
 --verbose, -v     Enable verbose output (optional)
 --config, -c      Configuration file (required)
```

#### `HelpStyleGrouped`
Groups flags by their associated commands. Ideal for CLIs where different commands have different sets of flags.
```
Usage: myapp

Global Flags:
 --verbose, -v     Enable verbose output

Commands:
 + service
 │─ start         Start the service
 |   --port, -p    Service port (defaults to: 8080)
 └─ stop          Stop the service
```

#### `HelpStyleCompact`
A minimal, deduplicated output for large CLIs with many shared flags.
```
Global Flags:
  --verbose, -v (optional)
  --config, -c (required)

Shared Flags:
  core.ldap.* (used by: auth, user)
    --core.ldap.host, --core.ldap.port, ... and 5 more

Commands:
  auth            Authenticate users              [15 flags]
  user            Manage users                    [12 flags]
```

#### `HelpStyleHierarchical`
A command-focused view for deeply nested CLIs (like `git` or `kubectl`). It shows the command structure and encourages users to explore subcommands.
```
Usage: myapp [global-flags] <command> [command-flags]

Command Structure:
service
  ├─ start       Start the service
  └─ stop        Stop the service
database
  ├─ backup      Backup database
  └─ restore     Restore database

Examples:
  myapp --help                    # Show this help
  myapp service --help            # Show service command help
```

### Advanced Help Configuration

For fine-grained control, you can use the `HelpConfig` struct.

```go
// For expert users who might not need detailed descriptions
parser.SetHelpConfig(goopt.HelpConfig{
    Style:           goopt.HelpStyleCompact,
    ShowDefaults:    false, // Don't show "(defaults to: ...)"
    ShowDescription: false, // Hide the description text
})
```

### Interactive Help Parser

`goopt` includes an advanced help parser that allows users to query the help system itself. This is enabled automatically.

```bash
# Show only the global flags
myapp --help globals

# Show only the command hierarchy
myapp --help commands

# Search for any flag or command containing "user"
myapp --help --search "user"

# Filter flags to show only those matching a pattern
myapp --help --filter "*.port"

# Override the configured style at runtime
myapp --help --style compact
```

## Best Practices

1.  **Stick with `HelpStyleSmart`:** Let `goopt` choose the best style for you unless you have a specific reason to override it.
2.  **Check `WasHelpShown()`:** Always check this after parsing to ensure your application exits cleanly after displaying help.
3.  **Provide Good Descriptions:** Your `desc` tags are the most important part of creating useful help text. Be concise but clear.
4.  **Leverage Namespacing:** Use nested structs to group flags logically (e.g., `database.host`, `database.port`). The `Compact` and `Hierarchical` help styles will use these namespaces to create structured output.