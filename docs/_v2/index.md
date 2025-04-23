---
layout: default
title: Home
nav_order: 1
---

{% include version-selector.html %}

# goopt Documentation

`goopt` is a flexible and powerful command-line option parser for Go applications that supports multiple approaches to defining your CLI interface.

## Version 2.0

This is the documentation for goopt v2, which includes significant enhancements:

- **First-class internationalization (i18n)**
- **Enhanced error handling with structured errors**
- **Improved flag inheritance in command hierarchies**
- **Cleaner API with deprecated functionality removed**

[See what's new in v2]({% link _v2/whats-new.md %}) or [read the migration guide]({% link _v2/migration.md %}) if upgrading from v1.

## Design

goopt follows these key principles:
- **Flexibility First**: Support multiple ways to define your CLI (struct-based, builder pattern, or imperative)
- **Type Safety**: Provide compile-time guarantees about command structure
- **Clear Precedence**: Explicit ordering for configuration sources (CLI flags → config files via `ParseWithDefaults` → ENV vars → Default values) where CLI flags have the highest precedence and Default values the lowest
- **Ordered Execution**: Commands execute in the order they're provided
- **Command Context**: Command-specific flags are evaluated within their command context
- **Global, shared, and command-specific flags**: goopt supports all three types of flags

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
  - Flag name converters to ensure consistent flag and command names (default to lowerCamelCase)
- **i18n Support**:
  - Goopt has built-in support for internalization in English, French, and German
  - Built-in translations can be easily extended to support additional languages
  - Support for user-defined internalization support for flag and command usage
  - Allows overriding built-in messages if necessary
- **Enhanced Error Handling**:
  - Structured, translatable errors with context
  - Improved error testing utilities
  - Consistent error wrapping and chaining
- **Shared flag command support**:
  - Hierachical flags 
  - flags set on parent commands are accessible to sub-commands
  - Full hierarchical support for long and short flags

## Documentation Contents

### Guides
- [Getting Started]({% link _v2/guides/getting-started.md %}) - Installation and basic usage
- [Command structure patterns]({% link _v2/guides/command-organization.md %}) - Different ways to structure your CLI
- [Flag structure patterns]({% link _v2/guides/flag-organization.md %}) - Different ways to structure your flags
- [Struct-First Approach]({% link _v2/guides/struct-tags.md %}) - Struct-first approach
- [Advanced Features]({% link _v2/guides/advanced-features.md %}) - Nested access, dependencies, validation, and more
- [Positional Arguments]({% link _v2/guides/positional-arguments.md %}) - Flexible positional argument handling
- [Internationalization]({% link _v2/guides/internationalization.md %}) - Internationalization support for your CLI

### Configuration
- [Environment Variables]({% link _v2/configuration/environment.md %}) - ENV var support
- [External Configuration]({% link _v2/configuration/external-config.md %}) - Config files and defaults

### Integration
- [Shell Completion]({% link _v2/shell/completion.md %}) - Setup completion for various shells

### Migration
- [What's New in v2]({% link _v2/whats-new.md %}) - Overview of v2 features
- [Migration Guide]({% link _v2/migration.md %}) - Guide for upgrading from v1 to v2

## Need Help?

- Check [Guides]({% link _v2/guides/index.md %}) section for detailed documentation
- Visit [GitHub repository](https://github.com/napalu/goopt) for issues and updates
- See the [API Reference](https://pkg.go.dev/github.com/napalu/goopt/v2) for detailed API documentation
