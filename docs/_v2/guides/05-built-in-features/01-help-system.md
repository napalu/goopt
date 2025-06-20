---
layout: default
title: The Help System
parent: Built-in Features
nav_order: 1
version: v2
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

---

## Configuring Help Output

You can customize every aspect of the help system's appearance and behavior.

### Help Styles

`goopt` offers several help styles to best match your CLI's complexity. You can set the style with `parser.SetHelpStyle()` or `goopt.WithHelpStyle()`.

#### `HelpStyleSmart` (Default)
`goopt` analyzes your CLI's complexity (number of flags and commands) and automatically selects the most appropriate style. This is the recommended default for most applications.

*Detection Logic:*
- **Flat:** For CLIs with fewer than 20 flags and 3 commands.
- **Grouped:** For CLIs with a few commands that have multiple flags each.
- **Compact:** For CLIs with over 20 flags.
- **Hierarchical:** For very large CLIs with many flags and commands.

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
  user            Manage users                    [12 flags]```

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

---

## Advanced Configuration & Features

### The `HelpConfig` Struct
For fine-grained control over what is displayed, you can modify the `HelpConfig` struct.

```go
// The HelpConfig struct definition
type HelpConfig struct {
    Style            HelpStyle
    ShowDefaults     bool // default: true
    ShowShortFlags   bool // default: true
    ShowRequired     bool // default: true
    ShowDescription  bool // default: true
    MaxGlobals       int  // default: 15
    MaxWidth         int  // default: 80
    GroupSharedFlags bool // default: true
    CompactThreshold int  // default: 20
}

// Example: Customize for expert users who need less detail
parser.SetHelpConfig(goopt.HelpConfig{
    Style:           goopt.HelpStyleCompact,
    ShowDefaults:    false, // Don't show "(defaults to: ...)"
    ShowDescription: false, // Hide the description text
})
```

### Context-Aware Help Output
By default, help requested via `--help` goes to `stdout`, while help shown due to a parsing error goes to `stderr`. You can control this with `SetHelpBehavior`.

```go
// Help always goes to stderr
parser.SetHelpBehavior(goopt.HelpBehaviorStderr)
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

# Get help on the help system itself
myapp --help --help
```

### Version Integration
If you use the [Version Support]({{ site.baseurl }}/v2/05-built-in-features/02-version-support/) feature, you can configure it to display the version in the help header.
```go
parser.SetShowVersionInHelp(true)
```

---

## Advanced Customization with a Custom Renderer

While `goopt`'s built-in help styles and configuration options cover most use cases, you can further customize the help output by implementing the `Renderer` interface.

Unlike template-based systems where you might need to rewrite the entire help logic, `goopt`'s `Renderer` interface allows you to surgically override specific parts of the help output, such as how a single flag or command is formatted, while keeping the rest of the system's logic intact.

#### Example: Overriding Flag Formatting

```go
import "github.com/napalu/goopt/v2"

type CustomRenderer struct {
    *goopt.DefaultRenderer // Embed the default renderer to reuse its logic
}

// Override only the FlagUsage method.
func (r *CustomRenderer) FlagUsage(arg *goopt.Argument) string {
    // Custom flag formatting, e.g., a table-like layout
    name := r.FlagName(arg)
    if arg.Short != "" {
        name = fmt.Sprintf("-%s, --%s", arg.Short, name)
    } else {
        name = fmt.Sprintf("    --%s", name)
    }
    return fmt.Sprintf("  %-25s %s", name, r.FlagDescription(arg))
}

// Use the custom renderer in your parser
parser.SetRenderer(&CustomRenderer{
    DefaultRenderer: goopt.NewDefaultRenderer(parser),
})

parser.PrintHelp(os.StdErr)
```
This approach provides a structured way to customize the output without losing the benefits of the adaptive styling and interactive help parser.
      
---
## Testing and Advanced Control

### Overriding the Exit-on-Help Behavior

By default, when `goopt`'s auto-help system is triggered (e.g., by a user passing the `--help` flag), it will display the help text and then immediately exit the program by calling `os.Exit(0)`. This is the expected behavior for most command-line applications.

However, in some cases, particularly during **unit testing** or when embedding a `goopt`-based tool within a larger application, you may want to prevent this automatic exit.

You can override this behavior using the `SetEndHelpFunc` method on your parser.

#### How It Works
The `SetEndHelpFunc` method allows you to replace the default `os.Exit(0)` call with a custom function.

```go
// In your main function:
parser, _ := goopt.NewParserFromStruct(&Config{})

// Set a custom function that does *not* exit.
// This is perfect for testing.
parser.SetEndHelpFunc(func() error {
// We can log that help was shown and then return nil
// to allow the application to continue running.
fmt.Println("[test harness] Help was displayed, not exiting.")
return nil
})

// Now, when you parse with a help flag...
parser.Parse([]string{"--help"})

// ...the application will NOT exit. You can then make assertions.
if !parser.WasHelpShown() {
t.Errorf("Expected help to have been shown")
}
```

#### Primary Use Cases:

1.  **Unit & Integration Testing:** This is the most common reason to use `SetEndHelpFunc`. Your tests can verify that help is displayed correctly without causing the test runner to terminate prematurely.
2.  **Embedded CLIs:** If you are using `goopt` to parse arguments within a larger, long-running application (like a GUI or a server that exposes a command-line interface), you can use this function to handle the help request gracefully and return to your main application loop.

In your application's `main` function, you typically don't need to override this. But when writing tests, it becomes an indispensable tool.