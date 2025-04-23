---
layout: default
title: Home
nav_exclude: true
---

# goopt Documentation

`goopt` is a flexible and powerful command-line option parser for Go applications. It provides a clear, type-safe approach to building command-line interfaces in Go with support for commands, subcommands, flags, and their relationships.

## Choose a Version
{% include version-selector.html %}

### Version 1.xx

The v1 is no longer in active development but receives critical bug fixes.

### Version 2.xx

The latest version with significant enhancements:

- **First-class internationalization (i18n)** - Built-in support for English, French, and German with an extensible translation system
- **Enhanced error handling** - Structured errors with context and improved testing utilities
- **Hierarchical flag inheritance** - Parent command flags are automatically available to subcommands
- **Command-specific flags** - Flags can be scoped to specific commands in the hierarchy
- **Generics support** - Type-safe binding using Go's generics
- **Simplified API** - Removal of deprecated methods and more consistent naming

**Installation:**
```bash
go get github.com/napalu/goopt/v2
```

[See what's new in v2]({{ site.baseurl }}/v2/whats-new/) or [read the migration guide]({{ site.baseurl }}/v2/migration/).