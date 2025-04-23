---
layout: default
title: Guides
nav_order: 2
has_children: true
---

# When to use goopt

`goopt` is particularly well-suited for:

- **Flexible command definition** supporting struct-first, builder pattern, or imperative style
- **Multiple command organization approaches**: support for a variety of command organization styles
- **Type-safe configurations** with compile-time validation
- **Ordered command execution** where commands need to be processed in sequence

Feature overview:
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
  - Clear precedence: Explicit ordering for configuration sources (Default values → ENV vars → config files → CLI flags) where CLI flags have the highest precedence
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
