# Auto-Help Guide

The goopt v2 auto-help feature provides automatic help flag registration and display, eliminating the need for repetitive help handling code in every CLI application.

> **See also:** [Help Configuration Guide](https://napalu.github.io/goopt/v2/guides/help-configuration/) for information about customizing help output styles, formats, and advanced features.

## Overview

With auto-help enabled (the default), goopt automatically:
- Registers `--help` and `-h` flags if not already defined
- Displays help when these flags are used
- Respects your configured help style
- Allows user-defined help flags to take precedence

## Basic Usage

### Default Behavior

By default, auto-help is enabled for all parsers:

```go
parser := goopt.NewParser()
// --help and -h are automatically available

if !parser.Parse(os.Args) {
    // Handle errors
    os.Exit(1)
}

// Check if help was shown
if parser.WasHelpShown() {
    os.Exit(0) // Exit cleanly after help
}
```

### With Struct Tags

Auto-help works seamlessly with struct-based configuration:

```go
type Config struct {
    Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
    Config  string `goopt:"short:c;desc:Configuration file"`
}

cfg := &Config{}
parser, _ := goopt.NewParserFromStruct(cfg)

// --help is automatically available
```

## Configuration

### Disabling Auto-Help

If you want complete control over help handling:

```go
parser, _ := goopt.NewParserWith(
    goopt.WithAutoHelp(false),
)
// --help and -h are NOT registered
```

### Custom Help Flags

Change the help flag names to suit your needs:

```go
// Use different flag names
parser, _ := goopt.NewParserWith(
    goopt.WithHelpFlags("ayuda", "a"), // Spanish: --ayuda, -a
)

// Use only long form
parser, _ := goopt.NewParserWith(
    goopt.WithHelpFlags("help"), // Only --help, no short form
)

// Use multiple variations
parser, _ := goopt.NewParserWith(
    goopt.WithHelpFlags("help", "h", "?"), // --help, -h, -?
)
```

## Help Styles Integration

Auto-help respects all configured help styles:

```go
// Compact style for large CLIs
parser, _ := goopt.NewParserWith(
    goopt.WithHelpStyle(goopt.HelpStyleCompact),
)

// Hierarchical style for complex CLIs
parser, _ := goopt.NewParserWith(
    goopt.WithHelpStyle(goopt.HelpStyleHierarchical),
)
```

When `--help` is triggered, it uses the configured style automatically.

## User-Defined Help Flags

### Taking Control

If you define your own help flag, auto-help respects it:

```go
type Config struct {
    // User-defined help flag
    Help bool `goopt:"name:help;short:?;desc:Show usage"`
    // User wants -h for host
    Host string `goopt:"short:h;desc:Database host"`
}

cfg := &Config{}
parser, _ := goopt.NewParserFromStruct(cfg)

if !parser.Parse(os.Args) {
    os.Exit(1)
}

// Auto-help won't trigger - check user's flag
if cfg.Help {
    // Implement custom help display
    fmt.Println("My custom help...")
    os.Exit(0)
}
```

### Partial Override

You can define a custom help flag while still using auto-help for display:

```go
// Define help with different short flag
parser.AddFlag("help", &goopt.Argument{
    Short:       "?",
    Description: "Display help",
    TypeOf:      goopt.Standalone,
})

// Auto-help will use your flag definition
// but still handle the display automatically
```

## Best Practices

### 1. Use Default Behavior

For most CLIs, the default auto-help is perfect:

```go
parser := goopt.NewParser()
// That's it! Help is ready
```

### 2. Check WasHelpShown()

Always check if help was displayed:

```go
if !parser.Parse(os.Args) {
    // Handle errors
    os.Exit(1)
}

if parser.WasHelpShown() {
    os.Exit(0) // Clean exit
}

// Continue with normal execution
```

### 3. Customize for International Users

Provide help in the user's language:

```go
parser, _ := goopt.NewParserWith(
    goopt.WithHelpFlags("aide", "a"),     // French
    goopt.WithHelpFlags("hilfe", "h"),   // German
    goopt.WithHelpFlags("ayuda", "a"),   // Spanish
)
```

### 4. Respect User Expectations

If users expect `-h` for host or `-?` for help (Windows style), configure accordingly:

```go
// Windows-style help
parser, _ := goopt.NewParserWith(
    goopt.WithHelpFlags("help", "?"),
)

// Or disable auto-help if -h is commonly used
parser, _ := goopt.NewParserWith(
    goopt.WithAutoHelp(false),
)
// Then implement help manually
```

## Examples

### Simple CLI

```go
func main() {
    parser := goopt.NewParser()
    parser.AddFlag("verbose", &goopt.Argument{
        Short:       "v",
        Description: "Enable verbose output",
    })
    
    if !parser.Parse(os.Args) {
        os.Exit(1)
    }
    
    if parser.WasHelpShown() {
        os.Exit(0)
    }
    
    // Your app logic here
}
```

### Complex CLI with Commands

```go
type App struct {
    Global string `goopt:"short:g;desc:Global option"`
    
    Server struct {
        Port int `goopt:"short:p;default:8080;desc:Server port"`
        
        Start struct{} `goopt:"kind:command;desc:Start server"`
        Stop  struct{} `goopt:"kind:command;desc:Stop server"`
    } `goopt:"kind:command;desc:Server management"`
}

func main() {
    app := &App{}
    parser, _ := goopt.NewParserFromStruct(app,
        goopt.WithHelpStyle(goopt.HelpStyleHierarchical),
    )
    
    if !parser.Parse(os.Args) {
        os.Exit(1)
    }
    
    if parser.WasHelpShown() {
        os.Exit(0)
    }
    
    // Command handling
}
```

### Custom Help Implementation

```go
type Config struct {
    Help    bool `goopt:"short:h;desc:Show help"`
    Version bool `goopt:"short:v;desc:Show version"`
}

func main() {
    cfg := &Config{}
    parser, _ := goopt.NewParserFromStruct(cfg,
        goopt.WithAutoHelp(false), // Disable auto-help
    )
    
    if !parser.Parse(os.Args) {
        os.Exit(1)
    }
    
    if cfg.Help {
        showCustomHelp()
        os.Exit(0)
    }
    
    if cfg.Version {
        fmt.Println("v1.0.0")
        os.Exit(0)
    }
}
```

## API Reference

### Parser Methods

```go
// Enable/disable auto-help
parser.SetAutoHelp(enabled bool)
parser.GetAutoHelp() bool

// Configure help flags
parser.SetHelpFlags(flags []string)
parser.GetHelpFlags() []string

// Check help status
parser.IsHelpRequested() bool
parser.WasHelpShown() bool
```

### Configuration Functions

```go
// Enable/disable auto-help
goopt.WithAutoHelp(enabled bool)

// Set custom help flags
goopt.WithHelpFlags(flags ...string)
```

## Migration Guide

If you're upgrading from manually handling help:

### Before (Manual)
```go
type Config struct {
    Help bool `goopt:"short:h;desc:Show help"`
}

cfg := &Config{}
parser, _ := goopt.NewParserFromStruct(cfg)

if !parser.Parse(os.Args) {
    os.Exit(1)
}

if cfg.Help {
    parser.PrintHelp(os.Stdout)
    os.Exit(0)
}
```

### After (Auto-Help)
```go
type Config struct {
    // No need for help flag!
}

cfg := &Config{}
parser, _ := goopt.NewParserFromStruct(cfg)

if !parser.Parse(os.Args) {
    os.Exit(1)
}

if parser.WasHelpShown() {
    os.Exit(0)
}
```

The auto-help feature makes your CLI cleaner and ensures consistent help behavior across all your applications!