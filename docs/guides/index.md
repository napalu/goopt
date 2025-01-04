---
layout: default
title: Guides
nav_order: 2
has_children: true
---

# Basic Design

`goopt` follows a design that allows flexibility in how flags and commands are defined and parsed.

- **Declarative Flags via Struct Tags**: Flags can be defined using struct tags. The parser introspects the struct and automatically binds the struct fields to flags.
- **Programmatic Definition**: Commands and flags can also be defined programmatically or declaratively. This allows dynamic construction of commands based on runtime conditions.
- **Flag Grouping**: Flags can be associated with specific commands or shared across multiple commands. Global flags are available across all commands.
- **Dependency Validation**: Flags can be defined to depend on the presence or value of other flags. This validation is performed automatically after parsing.

# When to use goopt

`goopt` is particularly well-suited for:

- **Flexible command definition** supporting struct-first, builder pattern, or imperative style
- **Multiple command organization approaches**:
  - Flag-centric (using struct base path tags)
  - Command-centric (grouping via command structs)
  - Mixed approach combining both styles
- **Type-safe configurations** with compile-time validation
- **Ordered command execution** where commands need to be processed in sequence

Feature overview:
- Multiple command definition styles:
  - Struct-based using tags
  - Builder pattern
  - Imperative
  - Mixed approaches
- Flexible command organization:
  - Flag-centric with base paths
  - Command-centric with struct grouping
  - Hybrid approaches
- Nested commands with command-specific flags
- Command callbacks (explicit or automatic)
- Environment variable support
- Configurable defaults through ParseWithDefaults:
  - Load defaults from any source (JSON, YAML, etc.)
  - Implement only the configuration features you need
  - Clear precedence: ENV vars -> defaults -> CLI flags
- Ordered command execution
- Type-safe flag parsing
- Flag dependencies and validation
- Pattern matching for flag values
- Shell completion support:
  - Bash completion (flags and commands)
  - Zsh completion (rich command/flag descriptions, file type hints)
  - Fish completion (command/flag descriptions, custom suggestions)
  - PowerShell completion (parameter sets, dynamic completion)
  - Custom completion functions for dynamic values
  - Built-in completion installation commands

While [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper) provide a comprehensive configuration management solution with persistent and global flags, `goopt` offers unique flexibility in how commands and flags can be organized, along with guaranteed execution order.

Choose `goopt` when you:
- Want freedom to choose between struct tags, builder pattern, or imperative style
- Need flexibility in organizing commands (flag-centric, command-centric, or mixed)
- Need guaranteed command execution order
- Need strong compile-time guarantees about your command structure
- Need completion support across multiple shell types
- Prefer implementing specific configuration features over a full-featured solution

# Guides

A number of guides are available to help you get started with goopt.
