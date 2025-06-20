---
layout: default
title: Migrating from v1 to v2
nav_order: 5
version: v2
---

{% include version-selector.html %}

# Migrating from v1 to v2

This guide will help you migrate your application from `goopt` v1 to v2. The update introduces powerful new features like a composable validation engine, command lifecycle hooks, and an advanced help system. While there are breaking changes, the migration path is straightforward.

For a full list of new features, see [What's New in v2]({{ site.baseurl }}/v2/whats-new/).

## The Big Picture: Conceptual Changes

Before diving into the code changes, it's helpful to understand the main philosophical shifts in v2:

1.  **Validation is a Core Feature:** The old `accepted` tag is deprecated in favor of a much more powerful `validators` tag and a full programmatic validation engine. This is one of the most significant and beneficial changes.
2.  **Help and Version are Automatic:** The new `auto-help` and `auto-version` systems handle help text and version flags by default. You no longer need to manage a manual `--help` flag in your config struct.
3.  **The API is Parser-Centric:** The central `CmdLineOption` type has been renamed to `Parser`, and all related functions have been updated to reflect this clearer naming.

## Step-by-Step Migration Guide

### Step 1: Update Import Paths
First, update your Go import paths from `v1` to `v2`:

```go
// Before
import "github.com/napalu/goopt"

// After
import "github.com/napalu/goopt/v2"
```

### Step 2: Rename `CmdLineOption` to `Parser`
This is the most widespread change. You'll need to perform a search-and-replace in your project for the type name and its associated functions.

*   `goopt.CmdLineOption` → `goopt.Parser`
*   `goopt.NewCmdLineFromStruct()` → `goopt.NewParserFromStruct()`
*   `goopt.NewCmdLineOption()` → `goopt.NewParser()`

### Step 3: Migrate from `accepted` to `validators`

The `accepted` struct tag and its related functions are now deprecated. The new `validators` tag provides a more powerful and flexible replacement.

#### Migrating Simple Choices

**Before (v1):**
```go
type Config struct {
    Format string `goopt:"accepted:{pattern:json|yaml|csv,desc:Output format}"`
}
```

**After (v2):**
```go
type Config struct {
    Format string `goopt:"validators:isoneof(json,yaml,csv)"`
}
```

#### Migrating Regex Patterns

**Before (v1):**
```go
type Config struct {
    License string `goopt:"accepted:{pattern:^[A-Z]{3}-\\d{4}$,desc:License format}"`
}
```

**After (v2):**
The `regex` validator can take the pattern directly.
```go
type Config struct {
    License string `goopt:"validators:regex(^[A-Z]{3}-\\d{4}$)"`
}
```

For more complex scenarios, please see the complete **[Validation Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/)**.

### Step 4: Update Struct Tags
There are two main changes to struct tags:

1.  **Migrate from `accepted` to `validators`:**
    The `accepted` tag is now deprecated. You should replace it with the `validators` tag, which is more powerful.

    **Before:**
    ```go
    type Config struct {
        Format string `goopt:"name:format;accepted:{pattern:json|yaml|csv,desc:Output format}"`
        Port   int    `goopt:"name:port;accepted:{pattern:^\\d{4}$,desc:4-digit port}"`
    }
    ```
    **After:**
    ```go
    type Config struct {
        Format string `goopt:"name:format;validators:isoneof(json,yaml,csv)"`
        Port   int    `goopt:"name:port;validators:range(1024,65535)"`
    }
    ```
    See the [Validation Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/) for a full list of available validators.

2.  **Ensure `goopt:` Namespace:**
    While v1 supported single struct tags (e.g., `short:"v"`), v2 requires all tags to be within the `goopt:"..."` namespace.

    **Before:**
    ```go
    type Config struct {
        Verbose bool `short:"v" desc:"Enable verbose output"`
    }
    ```
    **After:**
    ```go
    type Config struct {
        Verbose bool `goopt:"short:v;desc:Enable verbose output"`
    }
    ```
    See the [Struct Tags Reference]({{ site.baseurl }}/v2/guides/03-defining-your-cli/01-struct-tags-reference/) for details.

### Step 5: Update Help and Version Handling
With the new `auto-help` and `auto-version` systems, you should remove any manual `Help` or `Version` boolean flags from your config structs.

**Before:**
```go
type Config struct {
    Help bool `goopt:"short:h;desc:Show this help message"`
}

func main() {
    // ...
    parser.Parse(os.Args)
    if cfg.Help {
        parser.PrintUsage(os.Stdout)
        os.Exit(0)
    }
}```

**After:**
```go
type Config struct {
    // No Help flag needed!
}

func main() {
    // ...
    parser.Parse(os.Args)
    
    // Check if goopt's auto-help system was triggered.
    if parser.WasHelpShown() {
        os.Exit(0)
    }
}
```
See the [Help System Guide]({{ site.baseurl }}/v2/guides/05-built-in-features/01-help-system/) for more details.

---

## Detailed API Change Reference

For reference, here is a complete list of removed and renamed APIs.

### Removed APIs & Types
The following have been removed and replaced with the `With...()` functional option pattern or have been renamed.

*   `Parser.Clear()`, `Parser.ClearAll()` → Use **`Parser.ClearErrors()`** instead.
*   `NewArgument` → Use **`NewArg`** instead.
*   `GetConsistencyWarnings()` → Use **`GetWarnings()`** instead.
*   `SetRequired()` → Use **`WithRequired()`** when creating an `Argument`.
*   `SetRequiredIf()` → Use **`WithRequiredIf()`**.
*   `SetSecure()`, `SetSecurePrompt()` → Use **`WithSecurePrompt()`**.
*   `WithDependentValueFlags()` → Use **`WithDependencyMap()`**.
*   `CmdLineOption` (type) → Use **`Parser`** instead.
*   `DependsOn`, `OfValue` (types) → Use `WithDependencyMap()` instead.

### Renamed APIs
All functions related to `CmdLineOption` have been renamed to use `Parser`.

*   `NewCmdLineFromStruct` → `NewParserFromStruct`
*   `BindFlagToCmdLine` → `BindFlagToParser`
*   `CustomBindFlagToCmdLine` → `CustomBindFlagToParser`