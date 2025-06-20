---
layout: default
title: What's New in v2
nav_order: 2
version: v2
---

{% include version-selector.html %}

# What's New in goopt v2

`goopt` v2 is a major update focused on providing powerful, out-of-the-box solutions for building professional, robust, and user-friendly command-line applications.

## New Major Features

### ✨ A Powerful Validation Engine (Replaces `accepted`)
The old `accepted` tag has been **deprecated** in favor of a completely new validation engine that is more powerful, composable, and easier to use.

- **Directly in Struct Tags:** Define complex validation rules right where you define your flag.
- **Composable Logic:** Chain built-in validators (`email`, `port`, `range`) or combine them with logical operators (`oneof`, `all`, `not`).
- **Custom Validators:** Easily write and integrate your own domain-specific validation logic.
- **Clearer Syntax:** The new `validators` tag uses a more intuitive parenthesis-based syntax (e.g., `validators:"minlength(5)"`).
- **➡️ [Read the Validation Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/)**

### Command Execution Hooks
Manage the entire lifecycle of your commands with pre- and post-execution hooks. This is perfect for handling cross-cutting concerns without cluttering your command logic.
- **Use cases:** Authentication checks, database connection management, logging, metrics, and resource cleanup.
- **Flexible scope:** Apply hooks globally to all commands or target specific commands.
- **➡️ [Read the Execution Hooks Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/02-execution-hooks/)**

### Advanced Help & Version Systems
The help and version systems are now fully automatic and highly configurable.
- **Auto-Help:** `--help` and `-h` flags are now provided by default, with an adaptive display style that suits your CLI's complexity.
- **Interactive Help:** Users can now query the help system with commands like `myapp --help --search "database"`.
- **Auto-Version:** Enable a `--version` flag with a single line of configuration, with support for dynamic build-time variables.
- **➡️ [See the Help System Guide]({{ site.baseurl }}/v2/guides/05-built-in-features/01-help-system/) and [Version Support Guide]({{ site.baseurl }}/v2/guides/05-built-in-features/02-version-support/)**

## 🏗️ Architectural Improvements

### Enhanced Internationalization (i18n)
The i18n system is now more robust and easier to use.
- **Layered Bundles:** A clearer separation between the default system bundle and your application's user bundle.
- **Improved Tooling:** The `goopt-i18n-gen` tool is more powerful than ever, with a "360° workflow" to automate adding i18n to existing projects.
- **➡️ [Read the i18n Guide]({{ site.baseurl }}/v2/guides/06-internationalization/)**

### Hierarchical Flag Inheritance
Flag handling is now fully hierarchical and more predictable.
- **Parent-child flag resolution:** Flags defined on parent commands are available to all children.
- **Clear precedence rules:** Command-specific flags correctly override inherited flags.
- **➡️ [See the Flag Inheritance Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/04-flag-inheritance/)**

### API Cleanup
The public API has been modernized and simplified.
- Deprecated methods from v1 have been removed.
- Naming has been made more consistent throughout the library.
- Error handling at initialization is more robust with the introduction of `NewArgE`.

## Breaking Changes

For a complete list of breaking changes and instructions on how to update your code from v1, please see the **[Migration Guide]({{ site.baseurl }}/v2/migration/)**.