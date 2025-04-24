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

See [Internationalization]({{ site.baseurl }}/v2/guides/internationalization/) for details.

## Enhanced Error Handling
Note: the error system has been backported to v1.

The error system has been completely overhauled:

- **Structured errors** with detailed context information
- **Error chaining** with proper cause tracking
- **Translatable error messages** that adapt to the configured language
- **Improved error testing utilities** for better test coverage
- **Standard errors package** (`errs`) with consistent error types


## Hierarchical Flag Inheritance

Flag handling is now fully hierarchical:

- **Parent-child flag resolution** - flags defined on parent commands are available to children
- **Precedence rules** - command-specific flags take precedence over inherited flags
- **Short flag resolution** - proper resolution of short flags in command hierarchies
- **Context-aware parsing** - flag values are evaluated in the proper command context

See [Hierarchical flags]({{ site.baseurl }}/v2/guides/advanced-features/#flag-inheritance) for details.

## API Cleanup

The API has been cleaned up and modernized:

- **Removal of deprecated methods**
- **More consistent naming** throughout the codebase
- **Better documentation** with examples and usage patterns
- **Simplified initialization** with functional options

## Breaking Changes

See the [Migration Guide]({{ site.baseurl }}/v2/migration/) for a complete list of breaking changes and how to update your code.

[Back to v2 Documentation]({{ site.baseurl }}/v2/index/) | [Migration Guide]({{ site.baseurl }}/v2/migration/)