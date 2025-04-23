---
layout: default
title: Migration Guide
nav_order: 3
version: v2
---

{% include version-selector.html %}

# Migrating from v1 to v2

This guide will help you migrate your application from goopt v1 to v2.

## Import Path Changes

First, update your imports:

```go
// Before
import "github.com/napalu/goopt"

// After
import "github.com/napalu/goopt/v2"
```

## Breaking Changes

### Removed APIs

The following APIs have been removed:

- `Parser.Clear()` and `Parser.ClearAll()` - Use `Parser.ClearErrors()` instead
- `NewArgument` - Use `NewArg` instead
- `NewCmdLineOption`, `NewCmdLineFromStruct`, `NewCmdLineFromStructWithLevel` - Use `NewParser`, `NewParserFromStruct` and `NewParserFromStructWithLevel` instead
- `BindFlagToCmdLine`, `CustomBindFlagToCmdLine` - Use `BindFlagToParser` and `CustomBindFlagToParser` instead
- `GetConsistencyWarnings` - Use `GetWarnings` instead
- `SetRequired` - Use `WithRequired` instead
- `SetRequiredIf` - Use `WithRequiredIf` instead
- `SetSecure`, `SetSecurePrompt` - Use `WithSecurePrompt` instead
- `WithDependentValueFlags` - Use `WithDependencyMap` instead

### Removed types
- `CmdLineOption` - Use `Parser`instead
- `DependsOn` - Use `DependencyMap` instead
- `OfValue` - Use `DependencyMap` instead
- Single struct-tags were removed in favour of namespaced `goopt` struct-tags. See [Struct-Tags]({{ site.baseurl }}/v2/guides/struct-tags/) for details.


### Changed Behaviors
-   **Flag Inheritance:** Flags defined on parent commands are now automatically inherited by and available to their subcommands in v2. Command-specific flags still override inherited flags with the same name. See [Advanced Features - Flag Inheritance]({{ site.baseurl }}/v2/guides/advanced-features/#flag-inheritance) for details.
-   **Flag vs. Positional Argument Precedence:** In v2, if a configuration item can be set both via a named flag (e.g., `--output file.txt`) and positionally (e.g., via `pos:1`), the value provided by the **explicit named flag always takes precedence**. The positional value will be ignored for that specific binding, ensuring more predictable behavior consistent with common CLI conventions.


[Back to v2 Documentation]({{ site.baseurl }}/v2/index/) | [What's New]({{ site.baseurl }}/v2/whats-new/)