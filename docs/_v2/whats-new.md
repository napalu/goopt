---
layout: default
title: What's New in v2
nav_order: 2
version: v2
---

{% include version-selector.html %}

# What's New in goopt v2

goopt v2 introduces several major improvements:

## Internationalization (i18n)
Note: i18n has been backported to v1.

goopt v2 comes with comprehensive internationalization support:

- **Built-in language support** for English, French, and German
- **Extensible translation system** for adding custom languages
- **Translatable error messages** for better user experience
- **User-defined message bundles** to override built-in translations

```go
// Using the default bundle (English)
parser := goopt.NewParser()

// Setting a user-specific bundle
userBundle := i18n.NewBundle("fr")
parser.SetUserBundle(userBundle)

// Completely replacing the default bundle
customBundle := i18n.NewBundle("de")
parser.ReplaceDefaultBundle(customBundle)
```

## Enhanced Error Handling
Note: the error system has been backported to v1.

The error system has been completely overhauled:

- **Structured errors** with detailed context information
- **Error chaining** with proper cause tracking
- **Translatable error messages** that adapt to the configured language
- **Improved error testing utilities** for better test coverage
- **Standard errors package** (`errs`) with consistent error types

```go
// Error handling example
if !parser.Parse(os.Args) {
    fmt.Fprintln(os.Stderr, "Error parsing arguments:")
    for _, err := range parser.GetErrors() {
        fmt.Fprintf(os.Stderr, "  - %s\n", err)
    }
    os.Exit(1)
}
```

## Hierarchical Flag Inheritance

Flag handling is now fully hierarchical:

- **Parent-child flag resolution** - flags defined on parent commands are available to children
- **Precedence rules** - command-specific flags take precedence over inherited flags
- **Short flag resolution** - proper resolution of short flags in command hierarchies
- **Context-aware parsing** - flag values are evaluated in the proper command context

```go
// Command with hierarchical flags example
rootCmd := goopt.NewCommand(goopt.WithName("app"))
rootCmd.AddFlag("verbose", goopt.NewArg(goopt.WithShort("v")))

subCmd := rootCmd.AddCommand(goopt.NewCommand(goopt.WithName("sub")))

// "sub" command inherits "verbose" flag from parent
```

## API Cleanup

The API has been cleaned up and modernized:

- **Removal of deprecated methods** like `Clear()` and `ClearAll()`
- **More consistent naming** throughout the codebase
- **Better documentation** with examples and usage patterns
- **Simplified initialization** with functional options

## Breaking Changes

See the [Migration Guide]({% link _v2/migration.md %}) for a complete list of breaking changes and how to update your code.

[Back to v2 Documentation]({% link _v2/index.md %}) | [Migration Guide]({% link _v2/migration.md %})