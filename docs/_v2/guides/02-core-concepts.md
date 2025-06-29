---
layout: default
title: Core Concepts
parent: Guides
nav_order: 5
version: v2
---

# Core Concepts of goopt

Welcome to the core concepts guide! After completing the [Getting Started]({{ site.baseurl }}/v2/01-getting-started/) tutorial, this page will give you a deeper understanding of the fundamental building blocks and design philosophy behind `goopt`.

## The Building Blocks

At its heart, `goopt` is built around three main components that work together to define your CLI.

### 1. The `Parser`
The `Parser` is the engine of your application. It holds the entire configuration of your CLI, including all commands, flags, and their relationships. You create one central `Parser` instance and use it to:
- Define your commands and flags.
- Parse the command-line arguments from `os.Args`.
- Access the parsed values.
- Handle errors and display help text.

### 2. The `Command`
A `Command` represents an action your application can perform (e.g., `git commit`, `docker build`). Commands can be nested to create a rich hierarchy of subcommands (like `goopt i18n-gen generate`). They act as containers for their own specific flags.

### 3. The `Argument`
An `Argument` is the configuration object for a single flag. It defines everything about a flag, including its:
- Type (`Single`, `Standalone`, `Chained`)
- Description for help text
- Default value
- Validation rules
- Dependencies on other flags

## The Three Ways to Build Your CLI

`goopt` is uniquely flexible, allowing you to define your CLI in the way that best suits your project's needs.

### 1. The Struct-First Approach (Recommended)

This is the most common and recommended approach. You define your entire CLI structure—commands, subcommands, and flags—as a single Go `struct`. `goopt` then uses reflection and struct tags to build the parser automatically.

**Best for:** Most applications. It's declarative, type-safe, and keeps your configuration in one easy-to-read place.

```go
// A simple struct defining a 'server start' command.
type Config struct {
    Verbose bool `goopt:"short:v"`
    Server struct {
        Port int `goopt:"name:port;default:8080"`
    } `goopt:"kind:command;name:start"`
}
// Create the parser from the struct.
parser, _ := goopt.NewParserFromStruct(&Config{})
```

### 2. The Programmatic Approach (Builder)

For more dynamic scenarios, you can build your parser imperatively using a fluent, builder-style API. You create commands and add flags to them step-by-step.

**Best for:** Applications where commands or flags are generated dynamically at runtime, or for developers who prefer an explicit, code-based definition over struct tags.

```go
// Programmatically define the same 'server start' command.
parser := goopt.NewParser()
parser.AddFlag("verbose", goopt.NewArg(goopt.WithShortFlag("v")))
parser.AddCommand(goopt.NewCommand(
    goopt.WithName("start"),
    goopt.WithCallback(startServer),
))
parser.AddFlag("port", goopt.NewArg(goopt.WithDefaultValue("8080")), "start")
```

### 3. The Hybrid Approach

You can mix and match! Start with a struct-based definition and then programmatically add or modify commands and flags on the parser instance. This gives you the best of both worlds.

**Best for:** Adding dynamic or complex behavior to a largely static, struct-defined CLI.

## Configuration Precedence

`goopt` resolves the final value for a flag by following a strict order of precedence. Sources with a higher number override sources with a lower number.

1.  **Default Values:** The value specified in a `default:"..."` struct tag or with `WithDefaultValue()`. (Lowest priority)
2.  **Environment Variables:** Values from environment variables (if enabled with `SetEnvNameConverter`).
3.  **External Configuration:** Values provided via the `ParseWithDefaults` map (e.g., from a JSON or YAML file).
4.  **Command-Line Flags:** The value explicitly provided by the user on the command line. (Highest priority)

For example, if a port is defined with `default:8080`, but an environment variable `MYAPP_PORT=9000` exists, the port will be `9000`. If the user then runs `./myapp --port=3000`, the final value will be `3000`.

## Flag Scopes & Inheritance

Flags in `goopt` can be global or tied to a specific command. This is a core concept for organizing complex applications.

*   **Global Flags:** A flag defined without a command path is "global" and is available to all commands and subcommands.
*   **Command-Specific Flags:** A flag associated with a command (e.g., `... "server", "start"`) is only available when that command is active.
*   **Inheritance:** Flags defined on a parent command are automatically available to all of its children, unless a child defines its own flag with the same name (which would override the parent's).

For example, in a command like `myapp --verbose service start`, the `--verbose` flag (if global) is available to and can be checked by the logic for both the `service` and `start` commands.

## Batteries-Included Features

Finally, `goopt` is designed to reduce boilerplate by providing powerful features out of the box.

*   **Auto-Help:** A rich, adaptive help system is enabled by default. It automatically generates help text and handles the `--help` flag. See the [Help System Guide](./built-in-features/01-help-system.md) for more.
*   **Auto-Version:** You can easily add version information to your CLI with a single line of configuration. See the [Version Support Guide](./built-in-features/02-version-support.md).
*   **Internationalization (i18n):** All system messages are pre-translated, and the library provides powerful tools to make your entire application multilingual. See the [Internationalization Guide](./internationalization/index.md).

## Next Steps

Now that you understand the core concepts, you're ready to learn how to structure your CLI in more detail.

*   **Explore how to define flags, commands, and positional arguments in the [Defining Your CLI]({{ site.baseurl }}/v2/guides/03-defining-your-cli/) section.**