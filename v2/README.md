# goopt: a flexible and powerful command-line parser

![Coverage](https://img.shields.io/badge/Coverage-83.6%25-brightgreen)
[![Go Reference v2](https://pkg.go.dev/badge/github.com/napalu/goopt/v2.svg)](https://pkg.go.dev/github.com/napalu/goopt/v2)
[![Go Report Card v2](https://goreportcard.com/badge/github.com/napalu/goopt)](https://goreportcard.com/report/github.com/napalu/goopt/v2)


`goopt` is a flexible and powerful command-line option parser for Go applications. It provides a way to define commands, subcommands, flags, and their relationships declaratively or programmatically, offering both ease of use and extensibility.

[📚 View Documentation](https://napalu.github.io/goopt)

## Why Choose goopt? Beyond the Basics

While simple CLIs are easy with goopt, its real power shines in complex applications:
- **First-Class Internationalization**: Unlike other CLI libraries where i18n is an afterthought, goopt v2 features deeply integrated internationalization support. Use the `descKey` tag to reference translations, generate type-safe message keys, and ship your CLI in multiple languages with zero boilerplate. The included `goopt-i18n-gen` tool provides a complete workflow from string extraction to validation.
- **Deep Integration with Go Structs**: Define your entire CLI, including commands, subcommands, namespaced flags, and inheritance, directly within your Go configuration structs using goopt tags. This provides compile-time checks and a highly declarative approach not found elsewhere.
- **True Hierarchical Flags**: Define flags at any level of your command structure (app, app service, app service start). Flags are automatically inherited by subcommands but can be precisely overridden where needed.
- **Flexible Organization**: Mix and match struct-tag definitions with programmatic configuration. Use nested structs for flag namespacing or embed shared configurations directly into commands.
- **Contextual Flags**: Easily define flags that are only valid after a command, like `create --force file.txt`, without needing complex manual checks.

## Key Features

- **Declarative and Programmatic Definition**: Supports both declarative struct tag parsing and programmatic definition of commands and flags.
- **Command and Flag Grouping**: Organize commands and flags hierarchically, supporting global, command-specific, and shared flags.
- **Flag Dependencies**: Enforce flag dependencies based on the presence or specific values of other flags.
- **POSIX Compatibility**: Offers POSIX-compliant flag parsing, including support for short flags.
- **Secure Flags**: Enable secure, hidden input for sensitive information like passwords.
- **Automatic Usage Generation**: Automatically generates usage documentation based on defined flags and commands.
- **Positional Arguments**: Support for positional arguments with flexible position and index constraints.
- **Command Callbacks**: Define executable functions tied to specific commands with access to the command context.
- **Struct Context Access**: Access the original configuration struct from command callbacks, enabling better separation of concerns.
- **Internationalization (i18n)**: Built-in support for translating command descriptions, flag descriptions, error messages, and help text into multiple languages. Includes the powerful `goopt-i18n-gen` tool for complete i18n workflow automation.

## Installation

Install `goopt` via `go get`:

```bash
go get github.com/napalu/goopt/v2
```

## Quick Start

### Struct-based Definition

```go
package main

import (
    "os"
    "fmt"
    "github.com/napalu/goopt/v2"
)

type Config struct {
    // Global flags
    Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
    Output  string `goopt:"short:o;desc:Output file;required:true"`
    // Command with subcommands
    Create struct {
        Force bool `goopt:"short:f;desc:Force creation"`
        User struct {
            Username string `goopt:"short:u;desc:Username;required:true"`
            Password string `goopt:"short:p;desc:Password;secure:true"`
        } `goopt:"kind:command;name:user;desc:Create user"`
    } `goopt:"kind:command;name:create;desc:Create resources"`
}

func main() {
    cfg := &Config{}
    parser, _:= goopt.NewParserFromStruct(cfg)
    if !parser.Parse(os.Args) {
        parser.PrintUsage(os.Stdout)
        return
    }
}
```

### Programmatic Definition

```go
package main

import (
    "os"
    "fmt"
    "github.com/napalu/goopt/v2"
)   

func main() {
    parser := goopt.NewParser()
    parser.AddCommand(goopt.NewCommand(
        goopt.WithName("create"),
        goopt.WithCommandDescription("Create resources"),
    ))
    parser.AddFlag("verbose", goopt.NewArg(
        goopt.WithShortFlag("v"),
        goopt.WithDescription("Enable verbose output"),
    ))

    if !parser.Parse(os.Args) {
        parser.PrintUsage(os.Stdout)
        return
    }
}
```

## Examples

The `examples/` directory contains complete, runnable examples demonstrating various goopt features:

- **i18n-demo**: Comprehensive internationalization example showing how to create a multi-language CLI application with translated commands, flags, and messages
- **file-stats**: Simple example showing positional arguments and basic flag usage
- **simple-service**: Demonstrates hierarchical commands and flag inheritance

## Internationalization (i18n)

goopt provides built-in support for creating CLIs in multiple languages:

```go
type Config struct {
    User struct {
        Create struct {
            Username string `goopt:"short:u;descKey:user.create.username_desc;required:true"`
            Exec     goopt.CommandFunc
        } `goopt:"kind:command;name:create;descKey:user.create_desc"`
    } `goopt:"kind:command;name:user;descKey:user_desc"`
}

// Load translations
bundle, _ := i18n.NewBundleWithFS(locales, "locales")
parser, _ := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
```

Key i18n features:
- Use `descKey` in struct tags to reference translation keys
- Automatic translation of error messages and help text
- Support for multiple languages with easy runtime switching
- Separation of system and user translation bundles
- **Complete i18n workflow automation** with `goopt-i18n-gen`:
  - Automatically scan your code for missing `descKey` tags
  - Generate type-safe message constants from JSON locale files
  - Extract hardcoded strings for translation
  - Validate all translations are complete across locales
  - Transform existing CLIs to be i18n-ready

### Quick i18n Setup

```bash
# Install the i18n tool
go install github.com/napalu/goopt/v2/cmd/goopt-i18n-gen@latest

# Initialize locale files
goopt-i18n-gen -i locales/en.json init

# Scan code and generate translations
goopt-i18n-gen -i locales/en.json audit -d -g -u

# Generate type-safe constants
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go
```

See the [i18n-demo example](examples/i18n-demo) and [goopt-i18n-gen documentation](cmd/goopt-i18n-gen/README.md) for complete implementation details.

For more examples and detailed documentation, visit the [documentation site](https://napalu.github.io/goopt).

---

## License

`goopt` is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Contributions should be based on open issues (feel free to open one).

