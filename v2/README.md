# goopt: A Powerful & Flexible CLI Parser for Go


[![v2 Coverage](https://codecov.io/gh/napalu/goopt/branch/main/graph/badge.svg?flag=v2)](https://app.codecov.io/gh/napalu/goopt?flags%5B0%5D=v2)
[![Go Reference v2](https://pkg.go.dev/badge/github.com/napalu/goopt/v2.svg)](https://pkg.go.dev/github.com/napalu/goopt/v2)
[![Go Report Card v2](https://goreportcard.com/badge/github.com/napalu/goopt/v2)](https://goreportcard.com/report/github.com/napalu/goopt/v2)
![Go Version](https://img.shields.io/badge/go-1.18%2B-blue)
[![Awesome](https://awesome.re/badge.svg)](https://awesome-go.com)

`goopt` is a flexible and powerful command-line option parser for Go that supports multiple declarative and programmatic approaches to building your CLI. It's designed to be simple for small tools but scales elegantly for complex applications with features like first-class internationalization, advanced validation, and command lifecycle hooks.

---

### See It in Action: Multi-Language Help on the Fly

`goopt`'s built-in internationalization system allows you to ship a single binary that provides a localized experience for users around the world. All system messages, error descriptions, and help text can be translated.

![goopt Internationalization Demo](https://github.com/napalu/goopt/blob/main/v2/docs/assets/i18n-demo.gif?raw=true)
*(This demo shows the same `--help` flag producing fully translated output for English, German, French, Spanish, and Japanese, switched dynamically at runtime.)*

---

## Why Choose `goopt`?

While `goopt` handles the basics with ease, its real strength lies in providing sophisticated features that solve common, complex CLI development challenges out of the box.

*   **Adaptive Help System:** Move beyond static help text. `goopt` features an **adaptive help system** that automatically chooses the best display style (`flat`, `grouped`, `compact`, `hierarchical`) based on your CLI's complexity. Its **interactive help parser** allows users to query for help on specific topics(e.g., `myapp --help globals`, `myapp --help --search "database"`), providing a superior user experience. Plus, intelligent "did you mean?" suggestions help users recover from typos.
*   **Powerful, Composable Validation:** No tedious input validation boilerplate. `goopt`'s validation engine allows you to define validation rules directly in your struct tags. Compose built-in validators (`email`, `port`, `range`) with custom logic and logical operators (`oneof`, `all`, `not`) to create robust and readable validation for your flags.
*   **Lifecycle Management with Execution Hooks:** Implement cross-cutting concerns cleanly with a powerful pre- and post-execution hook system. Perfect for handling authentication, setting up database connections, logging metrics, or ensuring resource cleanup, hooks can be applied globally or to specific commands, giving you full control over the command lifecycle.
*   **First-Class Internationalization (i18n):** i18n is a core feature of `goopt`. Use `descKey` and `nameKey` tags to translate descriptions and names, enjoy automatic RTL language support, locale-aware number/date formatting, and regional variants (e.g., Swiss German). The included `goopt-i18n-gen` tool automates the entire workflow—from auditing your code for missing keys to generating type-safe message constants that eliminate runtime errors.

## Key Features

#### Flexible by Design
- **Multiple Definition Styles:** Use a declarative struct-first approach, a programmatic builder pattern, or a hybrid of both.
- **Hierarchical Commands:** Create deeply nested commands and subcommands with natural flag inheritance and overriding.
- **Flag Organization:** Namespace your flags with nested structs (`--db.host`) or create reusable flag groups by embedding structs.

#### Powerful Features
- **Advanced Validation:** A rich, composable validation system with built-in and custom rules.
- **Execution Hooks:** Pre- and post-execution hooks for command lifecycle management.
- **Positional Arguments:** Robust support for required, optional, and default-valued positional arguments.
- **Flag Dependencies:** Enforce rules where one flag depends on the presence or value of another.
- **Repeated Flags:** Natively supports both `--tag one --tag two` and `--tag "one,two"` for slice flags.

#### Developer Experience
- **Auto-Help & Auto-Version:** Zero-config `--help` and `--version` flags that just work. 
- **Intelligent Suggestions:** Automatic "did you mean?" suggestions for mistyped commands and flags.
- **Full i18n Tooling:** The `goopt-i18n-gen` tool facilitates translation management.
- **Shell Completion:** Generate completion scripts for Bash, Zsh, Fish, and PowerShell.
- **Secure Input:** Built-in support for securely prompting for passwords and other secrets.

## Quick Start

Define your entire CLI with a single Go struct.

```go
package main

import (
	"fmt"
	"os"
	"github.com/napalu/goopt/v2"
)

// Define your CLI structure using struct tags.
type Config struct {
	Verbose bool `goopt:"short:v;desc:Enable verbose output"`
	
	// 'greet' is a command with its own flag.
	Greet struct {
		Name string `goopt:"short:n;desc:Name to greet;default:World"`
	} `goopt:"kind:command;desc:Prints a greeting"`
}

func main() {
	cfg := &Config{}
	// It's good practice to handle the potential error from the constructor.
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse returns false on failure or if help was requested.
	// goopt automatically handles printing errors or help text.
	if !parser.Parse(os.Args) {
		os.Exit(1)
	}

	if parser.HasCommand("greet") {
		if cfg.Verbose {
			fmt.Println("Greeting verbosely...")
		}
		fmt.Printf("Hello, %s!\n", cfg.Greet.Name)
	} else {
		// If no command is given, it's good practice to show the help.
		parser.PrintHelp(os.Stdout)
	}
}
```

**Run it:**
```bash
# Get help automatically
$ go run . --help

# Run the command
$ go run . greet --name "Goopt User"
Hello, Goopt User!

# Use a global flag with the command
$ go run . --verbose greet
Greeting verbosely...
Hello, World!
```

## Documentation

For detailed guides, tutorials, and a full API reference, visit the official documentation site.

**[📚 Explore the documentation](https://napalu.github.io/goopt/)**

Start with these key topics:
- **[Core Concepts](https://napalu.github.io/goopt/v2/guides/02-core-concepts/)**
- **[Defining Your CLI](https://napalu.github.io/goopt/v2/guides/03-defining-your-cli/index/)**
- **[Validation Guide](https://napalu.github.io/goopt/v2/guides/04-advanced-features/01-validation/)**
- **[Execution Hooks](https://napalu.github.io/goopt/v2/guides/04-advanced-features/02-execution-hooks/)**
- **[Internationalization](https://napalu.github.io/goopt/v2/guides/06-internationalization/index/)**

---

## License

`goopt` is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to open an issue to discuss a bug or new feature.
