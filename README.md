# goopt: a flexible and powerful command-line parser

[![GoDoc](https://godoc.org/github.com/napalu/go-opt?status.svg)](https://godoc.org/github.com/napalu/goopt)
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/goopt)](https://goreportcard.com/report/github.com/napalu/goopt)
![Coverage](https://img.shields.io/badge/Coverage-80.8%25-brightgreen)

`goopt` is a flexible and powerful command-line option parser for Go applications. It provides a way to define commands, subcommands, flags, and their relationships declaratively or programmatically, offering both ease of use and extensibility.

[📚 View Documentation (wip)](https://napalu.github.io/goopt)

## Key Features

- **Declarative and Programmatic Definition**: Supports both declarative struct tag parsing and programmatic definition of commands and flags.
- **Command and Flag Grouping**: Organize commands and flags hierarchically, supporting global, command-specific, and shared flags.
- **Flag Dependencies**: Enforce flag dependencies based on the presence or specific values of other flags.
- **POSIX Compatibility**: Offers POSIX-compliant flag parsing, including support for short flags.
- **Secure Flags**: Enable secure, hidden input for sensitive information like passwords.
- **Automatic Usage Generation**: Automatically generates usage documentation based on defined flags and commands.
- **Positional Arguments**: Support for positional arguments with flexible position and index constraints.


## Installation

Install `goopt` via `go get`:

```bash
go get github.com/napalu/goopt
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
