# goopt: a flexible and powerful command-line parser
![Coverage](https://img.shields.io/badge/Coverage-78.7%25-brightgreen)
[![Go Reference v1](https://pkg.go.dev/badge/github.com/napalu/goopt.svg)](https://pkg.go.dev/github.com/napalu/goopt)
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/goopt)](https://goreportcard.com/report/github.com/napalu/goopt)

`goopt` is a flexible and powerful command-line option parser for Go applications. It provides a way to define commands, subcommands, flags, and their relationships declaratively or programmatically, offering both ease of use and extensibility.

---

**Version 2 Available**

**This README describes goopt v1.x.** Version 2 (`v2`) is now the recommended version for new projects and includes significant improvements like **hierarchical flag inheritance**, API cleanup, and bug fixes.

*   **New Users:** Please start with [goopt v2](https://github.com/napalu/goopt/tree/main/v2).
*   **v1 Users:** Consider migrating to v2. See the [v2 Migration Guide](https://napalu.github.io/goopt/v2/migration/).

[📚 View Full Documentation (v1 & v2)](https://napalu.github.io/goopt)

---

## Key Features (v1.x)

*Note: v2 includes these features plus enhancements like flag inheritance.*

- **Declarative and Programmatic Definition**: Supports both declarative struct tag parsing (`goopt:"..."` format) and programmatic definition.
- **Command and Flag Grouping**: Organize commands and flags hierarchically.
- **Flag Dependencies**: Enforce flag dependencies based on presence or specific values.
- **POSIX Compatibility**: Offers optional POSIX-compliant flag parsing.
- **Secure Flags**: Enable secure, hidden input for sensitive information.
- **Automatic Usage Generation**: Generates usage documentation.
- **Positional Arguments**: Supports arguments defined by position.
- **i18n**: Built-in internationalization support (backported from v2).
- **Struct Tag Parsing**: Supports the modern `goopt:"key:value;"` format (backported from v2).

## Installation (v1.x)

```bash
# For the legacy v1 version:
go get github.com/napalu/goopt # Or specify latest v1.x tag e.g. @v1.9.9
```

## Quick Start

### Struct-based Definition

```go
package main

import (
    "os"
    "fmt"
    "github.com/napalu/goopt"
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
    "github.com/napalu/goopt"
)   

func main() {
    parser := goopt.NewParser()
    parser.AddCommand(goopt.NewCommand(
        goopt.WithName("create"),
        goopt.WithDescription("Create resources"),
    ))
    parser.AddFlag("verbose", goopt.NewArg(
        goopt.WithShort("v"),
        goopt.WithDescription("Enable verbose output"),
    ))

    if !parser.Parse(os.Args) {
        parser.PrintUsage(os.Stdout)
        return
    }
}
```

For more examples and detailed documentation, visit the [documentation site](https://napalu.github.io/goopt).

---

## License

`goopt` is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Contributions should be based on open issues (feel free to open one).
<!-- Updated: Wed Apr 23 15:38:43 UTC 2025 -->
<!-- Updated: Thu Apr 24 05:51:17 UTC 2025 -->
