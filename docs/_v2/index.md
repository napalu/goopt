---
layout: default
title: Home
nav_order: 1
version: v2
---

{% include version-selector.html %}

# goopt: A Flexible, Feature-Rich CLI Parser for Go

`goopt` is a powerful command-line option parser for Go, designed to be intuitive for simple tools and scalable for complex, enterprise-grade applications. It provides a uniquely flexible, struct-first approach to building CLIs that are robust, maintainable, and user-friendly.

[Get started in 5 minutes]({{ site.baseurl }}/v2/guides/01-getting-started/) or see the [API Reference](https://pkg.go.dev/github.com/napalu/goopt/v2).

---

## Why goopt? More Than Just Parsing

`goopt` stands out by providing built-in solutions for the complex challenges of modern CLI development.

### 🌐 Low-Config Internationalization (i18n)
Ship a single binary that speaks your users' language. `goopt` provides out-of-the-box i18n for all system messages and a powerful code generation tool (`goopt-i18n-gen`) to automate the entire translation workflow for your application.
<br/>*[Read the Internationalization Guide]({{ site.baseurl }}/v2/guides/06-internationalization/)*

### 🚀 An Advanced, Interactive Help System
Stop writing boilerplate help text. `goopt` features an **adaptive help system** that automatically chooses the best display style for your CLI's complexity. Its interactive parser lets users search and filter help (`--help --search "db"`), providing a superior experience.
<br/>*[Learn more about the Help System]({{ site.baseurl }}/v2/guides/05-built-in-features/01-help-system/)*

### ✅ Powerful, Composable Validation
Define complex validation rules directly in your struct tags. Chain built-in validators like `email`, `port`, and `range`, or compose them with logical operators (`oneof`, `all`, `not`) to ensure your application receives correct data.
<br/>*[See the Validation Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/)*

### 훅 Command Lifecycle Hooks
Implement cross-cutting concerns cleanly with a powerful pre- and post-execution hook system. Manage authentication, database connections, logging, and resource cleanup with ease by attaching logic to the command lifecycle.
<br/>*[Explore Execution Hooks]({{ site.baseurl }}/v2/guides/04-advanced-features/02-execution-hooks/)*

---

## Key Features at a Glance

*   **Flexible Definition:** Build your CLI with a **declarative struct-first approach**, a programmatic builder, or a hybrid of both.
*   **Hierarchical Structure:** Natively supports nested commands, command-specific flags, and intelligent flag inheritance.
*   **Advanced Flag Handling:** Includes support for positional arguments, repeated flags (`-v -v -v`), and flag dependencies.
*   **Automatic Conveniences:** Zero-config support for `--help` and `--version` flags.
*   **Broad Shell Support:** Generate completion scripts for Bash, Zsh, Fish, and PowerShell.
*   **Extensive Configuration:** Load options from environment variables and external config files with a clear precedence order.

## Where to Next?

*   **New to `goopt`?** Follow our [**Getting Started**]({{ site.baseurl }}/v2/guides/01-getting-started/) guide.
*   **Want to see the patterns?** Check out how to [**Define Your CLI**]({{ site.baseurl }}/v2/guides/03-defining-your-cli/).
*   **Upgrading from v1?** Read the [**Migration Guide**]({{ site.baseurl }}/v2/migration/).

## Need Help?

- Check [Guides]({{ site.baseurl }}/v2/guides/index/) section for detailed documentation
- Visit [GitHub repository](https://github.com/napalu/goopt/v2) for issues and updates
- See the [API Reference](https://pkg.go.dev/github.com/napalu/goopt/v2) for detailed API documentation
