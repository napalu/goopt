# goopt: a flexible and powerful command-line parser

[![v2 Coverage](https://codecov.io/gh/napalu/goopt/branch/main/graph/badge.svg?flag=v2)](https://codecov.io/gh/napalu/goopt/v2?flag=v2)
[![Go Reference v2](https://pkg.go.dev/badge/github.com/napalu/goopt/v2.svg)](https://pkg.go.dev/github.com/napalu/goopt/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/goopt)](https://goreportcard.com/report/github.com/napalu/goopt)

`goopt` is a flexible and powerful command-line option parser for Go. It provides a declarative, struct-first approach to building CLIs that are robust, maintainable, and user-friendly.

The library is designed to be intuitive for simple tools and scalable for complex applications, with "batteries-included" features like an **advanced help system**, a **composable validation engine**, **command lifecycle hooks**, and comprehensive **internationalization (i18n)** support.

**Looking for the latest version?** `goopt v2` is here with major improvements!
> 👉 [Check out v2 on GitHub](https://github.com/napalu/goopt/tree/main/v2) or [📚 read the full docs](https://napalu.github.io/goopt)

---

## Installation (v2)

Version 2 is the recommended version for all new projects.

```bash
go get github.com/napalu/goopt/v2
```

## Quick Start (v2)

Define your entire CLI structure—commands, flags, and descriptions—using a single Go struct.

```go
package main

import (
    "fmt"
    "os"
    "github.com/napalu/goopt/v2"
)

type Config struct {
    // Global flags
    Verbose bool `goopt:"short:v;desc:Enable verbose output"`

    // 'create' command with a subcommand
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
    // Note: In v2, the parser type is simply `Parser`
    parser, err := goopt.NewParserFromStruct(cfg)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
        os.Exit(1)
    }

    // Parse returns false on failure or if --help was requested
    if !parser.Parse(os.Args) {
        // goopt handles printing errors and help text by default
        os.Exit(1)
    }

    // Your application logic here...
    if parser.HasCommand("create", "user") {
        fmt.Printf("Creating user: %s\n", cfg.Create.User.Username)
    }
}
```

For more examples and advanced guides, please visit the [**v2 Documentation Site**](https://napalu.github.io/goopt/).

---

## Legacy Version (v1.x)

<details>
<summary>Click to expand information for goopt v1.x</summary>

This version is in maintenance mode. For new projects, please use **[goopt v2](https://github.com/napalu/goopt/tree/main/v2)**.

- **Installation (v1):** `go get github.com/napalu/goopt@v1`
- **[Documentation (v1)](https://napalu.github.io/goopt/)**
- **[Migration Guide to v2](https://napalu.github.io/goopt/v2/migration/)**

### Quick Start (v1)

```go
package main

import (
    "os"
    "fmt"
    "github.comcom/napalu/goopt"
)

// ...Config struct is identical to v2 example...

func main() {
    cfg := &Config{}
    parser, _:= goopt.NewCmdLineFromStruct(cfg)
    if !parser.Parse(os.Args) {
        parser.PrintUsage(os.Stdout)
        return
    }
}
```

</details>

---

## License

`goopt` is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Contributions should be based on open issues (feel free to open one).