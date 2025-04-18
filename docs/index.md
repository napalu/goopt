---
layout: default
title: Home
nav_order: 1
---

# goopt Documentation

`goopt` is a flexible and powerful command-line option parser for Go applications that supports multiple approaches to defining your CLI interface.

## Design

goopt follows these key principles:
- **Flexibility First**: Support multiple ways to define your CLI (struct-based, builder pattern, or imperative)
- **Type Safety**: Provide compile-time guarantees about command structure
- **Clear Precedence**: Explicit ordering for configuration sources (ENV vars → config files → CLI flags)
- **Ordered Execution**: Commands execute in the order they're provided
- **Command Context**: Command-specific flags are evaluated within their command context
- **Global, shared and command-specific flags**: goopt supports all three types of flags

## Key Features

- **Multiple Definition Styles**:
  - Struct-based using tags
  - Builder pattern
  - Imperative
  - Mixed approaches
- **Flexible Command Organization**:
  - Flag-centric with base paths
  - Command-centric with struct grouping
  - Hybrid approaches
- **Rich Feature Set**:
  - Type-safe configuration
  - Ordered command execution
  - Positional arguments with flexible position and index constraints
  - Flag dependencies and validation
  - Shell completion (Bash, Zsh, Fish, PowerShell)
  - Environment variable support
  - External configuration support through `goopt.ParseWithDefaults`
  - Optional POSIX-compliant flag parsing
  - Customizable flag name and command name converters
  - Positional arguments with flexible position and index constraints
- **i18n Support**:
  - Goopt has built-in support for internalization in English, French and German
  - Built-in translations can be easily extended to support additional languages
  - Support for user-defined internalization support for flag and command usage
  - Allows overriding built-in messages if necessary

## Documentation Contents

### Guides
- [Getting Started](guides/getting-started.md) - Installation and basic usage
- [Command structure patterns](guides/command-organization.md) - Different ways to structure your CLI
- [Flag structure patterns](guides/flag-organization.md) - Different ways to structure your flags
- [Struct-First Approach](guides/struct-tags.md) - Struct-first approach
- [Advanced Features](guides/advanced-features.md) - Nested access, dependencies, validation, and more
- [Positional Arguments](guides/positional-arguments.md) - Flexible positional argument handling
- [Internationalization](guides/internationalization.md) - Internationalization support for you CLI

### Configuration
- [Environment Variables](configuration/environment.md) - ENV var support
- [External Configuration](configuration/external-config.md) - Config files and defaults

### Integration
- [Shell Completion](shell/completion.md) - Setup completion for various shells

## Need Help?

- Check [Guides](guides/) section for detailed documentation
- Visit [GitHub repository](https://github.com/napalu/goopt) for issues and updates
- See the [API Reference](https://godoc.org/github.com/napalu/goopt) for detailed API documentation 
