---
layout: default
title: Configuring help
parent: Guides
nav_order: 4
---

# Help Configuration Guide

The goopt v2 help system provides flexible, adaptive help output that automatically adjusts to your CLI's complexity. This guide covers the various help styles, configuration options, and best practices for creating user-friendly help documentation.

> **See also:** [Auto-Help Guide](https://github.com/napalu/goopt/tree/v2/docs/auto_help.md) for information about automatic help flag registration and handling.

## Overview

The help system in goopt v2 addresses common challenges with CLI help output:
- **Small CLIs** benefit from detailed, grouped output showing all flags and commands
- **Large CLIs** need compact, deduplicated output to avoid overwhelming users
- **Complex CLIs** require hierarchical navigation to explore command structures

## Help Styles

### 1. Flat Style (`HelpStyleFlat`)
The traditional flat list of all flags and commands. Best for simple CLIs with few options.

```go
parser.SetHelpStyle(goopt.HelpStyleFlat)
```

Output example:
```
Usage: myapp
 --verbose, -v     Enable verbose output (optional)
 --config, -c      Configuration file (required)
 --output, -o      Output directory (defaults to: ./output)
```

### 2. Grouped Style (`HelpStyleGrouped`)
Groups flags by their associated commands. Ideal for CLIs with multiple commands where each command has specific flags.

```go
parser.SetHelpStyle(goopt.HelpStyleGrouped)
```

Output example:
```
Usage: myapp

Global Flags:
 --verbose, -v     Enable verbose output

Commands:
 + service
 │─ start         Start the service
 |   --port, -p    Service port (defaults to: 8080)
 |   --workers     Number of workers (defaults to: 4)
 └─ stop          Stop the service
     --force, -f   Force stop
```

### 3. Compact Style (`HelpStyleCompact`)
Deduplicated, minimal output for large CLIs. Automatically groups shared flags and shows flag counts per command.

```go
parser.SetHelpStyle(goopt.HelpStyleCompact)
```

Output example:
```
Global Flags:
  --verbose, -v (optional)
  --config, -c (required)

Shared Flags:
  core.ldap.* (used by: auth, user)
    --core.ldap.host
    --core.ldap.port
    ... and 5 more

Commands:
  auth            Authenticate users              [15 flags]
  user            Manage users                    [12 flags]
  service         Manage services                 [8 flags]

Use --help for more information about a command.
```

### 4. Hierarchical Style (`HelpStyleHierarchical`)
Command-focused view for complex CLIs with deep command structures. Shows only essential information at the top level.

```go
parser.SetHelpStyle(goopt.HelpStyleHierarchical)
```

Output example:
```
Usage: myapp [global-flags] <command> [command-flags] [args]

Global Flags:
  --help, -h      Show help
  --verbose, -v   Enable verbose output
  ... and 3 more

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
  myapp service start --help      # Show start subcommand help
```

### 5. Smart Style (`HelpStyleSmart`) - Default
Tries to select the best style based on your CLI's complexity:

```go
parser.SetHelpStyle(goopt.HelpStyleSmart) // This is the default
```

Detection logic:
- **Flat**: < 20 flags, ≤ 3 commands
- **Grouped**: ≤ 3 commands with multiple flags
- **Compact**: > 20 flags (or > CompactThreshold if configured)
- **Hierarchical**: > CompactThreshold * 1.4 flags AND > 5 commands

## Configuration Options

### HelpConfig Structure

```go
type HelpConfig struct {
    Style            HelpStyle  // Help output style
    ShowDefaults     bool       // Show default values (default: false)
    ShowShortFlags   bool       // Show short flag forms (default: true)
    ShowRequired     bool       // Show required indicators (default: true)
    ShowDescription  bool       // Show descriptions (default: false)
    MaxGlobals       int        // Max global flags to show (default: 15)
    MaxWidth         int        // Maximum line width (default: 80)
    GroupSharedFlags bool       // Group shared flags (default: true)
    CompactThreshold int        // Compact mode threshold (default: 20)
}
```

### Setting Configuration

Using Set methods:
```go
parser := goopt.NewParser()
parser.SetHelpStyle(goopt.HelpStyleCompact)
parser.SetHelpConfig(goopt.HelpConfig{
    Style:            goopt.HelpStyleCompact,
    ShowDefaults:     true,
    ShowShortFlags:   false,
    MaxWidth:         100,
    CompactThreshold: 30,
})
```

Using With functions (fluent API):
```go
parser, err := goopt.NewParserWith(
    goopt.WithHelpStyle(goopt.HelpStyleHierarchical),
    goopt.WithCompactHelp(), // Shortcut for compact style
    goopt.WithHierarchicalHelp(), // Shortcut for hierarchical style
)
```

## Flag Deduplication

The help system automatically deduplicates flags when the same flag name appears in multiple commands:

```go
// Define the same flag for multiple commands
parser.AddFlag("verbose", arg, "service", "start")
parser.AddFlag("verbose", arg, "service", "stop")
parser.AddFlag("verbose", arg, "database", "backup")

// PrintUsage will show "verbose" only once
parser.PrintUsage(os.Stdout)
```

## Shared Flag Groups

The compact and hierarchical styles automatically detect and group shared flags:

```go
// Flags with common prefixes are grouped
parser.AddFlag("ldap.host", ...)
parser.AddFlag("ldap.port", ...)
parser.AddFlag("ldap.username", ...)
parser.AddFlag("ldap.password", ...)

// Output groups them as:
// ldap.* (used by: auth, user)
//   --ldap.host
//   --ldap.port
//   ... and 2 more
```

## Best Practices

### 1. Let Smart Mode Choose
The default smart mode works well for most CLIs:
```go
parser := goopt.NewParser() // Uses HelpStyleSmart by default
```

### 2. Override for Specific Needs
Override the style when you have specific requirements:
```go
// Force compact for a plugin system with many commands
parser.SetHelpStyle(goopt.HelpStyleCompact)

// Force hierarchical for a complex tool like kubectl
parser.SetHelpStyle(goopt.HelpStyleHierarchical)
```

### 3. Customize Display Options
Adjust display options based on your users:
```go
// For expert users who know the CLI well
parser.SetHelpConfig(goopt.HelpConfig{
    Style:           goopt.HelpStyleCompact,
    ShowDefaults:    false,
    ShowShortFlags:  false,
    ShowDescription: false,
    MaxGlobals:      10,  // Limit global flags shown
})

// For new users who need more guidance
parser.SetHelpConfig(goopt.HelpConfig{
    Style:           goopt.HelpStyleGrouped,
    ShowDefaults:    true,
    ShowShortFlags:  true,
    ShowRequired:    true,
    ShowDescription: true,
    MaxGlobals:      -1,  // Show all global flags
})
```

### 4. Use Structured Flag Names
Use dots to create logical groups that the help system can detect:
```go
type Config struct {
    Database struct {
        Host string `goopt:"name:db.host"`
        Port int    `goopt:"name:db.port"`
    }
    Cache struct {
        Host string `goopt:"name:cache.host"`
        TTL  int    `goopt:"name:cache.ttl"`
    }
}
```

### 5. Provide Context-Aware Help
The hierarchical style encourages exploring commands:
```go
// Users can drill down into specific commands
myapp --help                    # Top-level help
myapp service --help            # Service command help
myapp service start --help      # Start subcommand help
```

## Examples

### Simple CLI with Smart Detection
```go
type Config struct {
    Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
    Config  string `goopt:"short:c;desc:Configuration file"`
    Output  string `goopt:"short:o;desc:Output file"`
}

parser, _ := goopt.NewParserFromStruct(&Config{})
// Automatically uses flat style due to low complexity
```

### Large CLI with Forced Compact Style
```go
parser := goopt.NewParser()
parser.SetHelpStyle(goopt.HelpStyleCompact)

// Add many flags...
for i := 0; i < 50; i++ {
    parser.AddFlag(fmt.Sprintf("flag%d", i), &goopt.Argument{
        Description: fmt.Sprintf("Flag number %d", i),
    })
}
```

### Complex CLI with Hierarchical Navigation
```go
type App struct {
    Kubernetes struct {
        Cluster struct {
            Create struct{} `goopt:"kind:command;desc:Create a cluster"`
            Delete struct{} `goopt:"kind:command;desc:Delete a cluster"`
            List   struct{} `goopt:"kind:command;desc:List clusters"`
        } `goopt:"kind:command;desc:Manage clusters"`
        
        Pod struct {
            Create struct{} `goopt:"kind:command;desc:Create a pod"`
            Delete struct{} `goopt:"kind:command;desc:Delete a pod"`
            List   struct{} `goopt:"kind:command;desc:List pods"`
        } `goopt:"kind:command;desc:Manage pods"`
    } `goopt:"kind:command;name:k8s;desc:Kubernetes operations"`
}

parser, _ := goopt.NewParserFromStruct(&App{})
parser.SetHelpStyle(goopt.HelpStyleHierarchical)
```

## Internationalization

The help system fully supports i18n for all messages:

```go
// All help messages use translation keys
parser.PrintHelp(os.Stdout) // Uses configured language

// Messages like these are translatable:
// - "Commands"
// - "Global Flags"
// - "required"
// - "optional"
// - "defaults to"
// - "Use --help for more information"
```

## Advanced Help Features

The help system includes an advanced parser with powerful interactive capabilities:

### Interactive Help Modes

The help system supports various modes for exploring your CLI:

```bash
# Show only global flags
myapp --help globals

# Show only commands
myapp --help commands

# Show only flags (no commands)
myapp --help flags

# Show examples
myapp --help examples

# Show everything
myapp --help all

# Show help about the help system itself
myapp --help --help
```

### Search and Filter

Find specific flags or commands:

```bash
# Search for flags/commands containing "user"
myapp --help --search "user"

# Use wildcards
myapp --help --search "use*"
myapp --help --search "*user"
myapp --help --search "us?r"

# Filter by attributes
myapp --help --filter required    # Show only required flags
myapp --help --filter optional    # Show only optional flags
```

### Context-Aware Output

Control where help output goes:

```go
// Configure help output behavior
parser.SetHelpBehavior(goopt.HelpBehaviorSmart)

// HelpBehavior options:
// - HelpBehaviorStdout: Always use stdout (default)
// - HelpBehaviorSmart: stdout for --help, stderr for errors
// - HelpBehaviorStderr: Always use stderr

// Get the appropriate writer for help
writer := parser.GetHelpWriter(isError)
```

### Runtime Configuration

The help parser accepts runtime options:

```bash
# Control help output dynamically
myapp --help --no-desc              # Hide descriptions
myapp --help --no-defaults          # Hide default values
myapp --help --no-short             # Hide short flags
myapp --help --style compact        # Override configured style
myapp --help --depth 2              # Limit command depth
```

### Programmatic Help Generation

```go
// Print help with context awareness
parser.PrintHelpWithContext(isError)

// Get help writer based on context
writer := parser.GetHelpWriter(isError)
parser.PrintHelp(writer)
```

## Version Integration

Show version information in help output:

```go
// Set version information
parser.SetVersion("1.2.3")
// or
parser.SetVersionFunc(func() string {
    return fmt.Sprintf("v%s (built %s)", version, buildDate)
})

// Show version in help header
parser.SetShowVersionInHelp(true)

// Custom version formatting
parser.SetVersionFormatter(func(version string) string {
    return fmt.Sprintf("MyApp %s\nCopyright (c) 2024", version)
})
```

## Custom Rendering

For advanced customization, implement the Renderer interface:

```go
type CustomRenderer struct {
    *goopt.DefaultRenderer
}

func (r *CustomRenderer) FlagUsage(arg *goopt.Argument) string {
    // Custom flag formatting
    return fmt.Sprintf("  %-20s %s", r.FlagName(arg), r.FlagDescription(arg))
}

// Use custom renderer
parser.SetRenderer(&CustomRenderer{
    DefaultRenderer: goopt.NewDefaultRenderer(parser),
})
```

## Goal

The goopt v2 help system tries to adapt to your CLI's needs, providing appropriate detail levels for different complexities. 
Whether you're building a simple tool or a complex command suite, the help system ensures your users get the information they need without being overwhelmed.