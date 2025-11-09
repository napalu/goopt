---
layout: default
title: Guides
nav_order: 3 
has_children: true
version: v2
---

# goopt Guides

Welcome to the `goopt` documentation guides. This collection provides a comprehensive learning path, from building your first application to mastering advanced features.

Whether you're a new user or an experienced developer, these guides are designed to help you get the most out of `goopt`.

## Learning Path

We recommend reading the guides in the following order.

### 1. The Basics
Start here to learn the fundamentals of building a CLI with `goopt`.

*   **[Getting Started]({{ site.baseurl }}/v2/guides/01-getting-started/):** Build your first application in 5 minutes.
*   **[Core Concepts]({{ site.baseurl }}/v2/guides/02-core-concepts/):** Understand the key building blocks and design philosophy of the library.

### 2. Structuring Your Application
These guides cover the patterns for defining your flags, commands, and arguments.

*   **[Defining Your CLI]({{ site.baseurl }}/v2/guides/03-defining-your-cli/index/):** An overview of the different ways to structure your application's interface.
  *   **[Struct Tags Reference]({{ site.baseurl }}/v2/guides/03-defining-your-cli/01-struct-tags-reference/):** A quick reference for all available `goopt:"..."` tags.
  *   **[Command Patterns]({{ site.baseurl }}/v2/guides/03-defining-your-cli/02-command-patterns/):** Learn to organize commands using nested structs, paths, and programmatic builders.
  *   **[Flag Patterns]({{ site.baseurl }}/v2/guides/03-defining-your-cli/03-flag-patterns/):** Explore patterns for namespacing and reusing flag groups.
  *   **[Positional Arguments]({{ site.baseurl }}/v2/guides/03-defining-your-cli/04-positional-arguments/):** A detailed guide on position-dependent arguments.
  *   **[Command Callbacks]({{ site.baseurl }}/v2/guides/03-defining-your-cli/05-command-callbacks/):** Learn how to attach behavior to your commands.

### 3. Advanced Features
Dive deeper into the powerful features that make `goopt` suitable for complex, production-grade applications.

*   **[Advanced Features Overview]({{ site.baseurl }}/v2/guides/04-advanced-features/index/)**
  *   **[Validation]({{ site.baseurl }}/v2//guides/04-advanced-features/01-validation/):** Ensure data correctness with built-in and custom validators.
  *   **[Execution Hooks]({{ site.baseurl }}/v2/guides/04-advanced-features/02-execution-hooks/):** Manage the command lifecycle with pre- and post-execution hooks.
  *   **[Error Handling]({{ site.baseurl }}/v2/guides/04-advanced-features/03-error-handling/):** Best practices for robust error handling during setup.
  *   **[Flag Inheritance]({{ site.baseurl }}/v2/guides/04-advanced-features/04-flag-inheritance/):** Understand how flags are resolved in nested command hierarchies.

### 4. Built-in Functionality
Learn about the powerful "batteries-included" features that come with `goopt`.

*   **[Built-in Features Overview]({{ site.baseurl }}/v2/guides/05-built-in-features/index/)**
  *   **[The Help System]({{ site.baseurl }}/v2/guides/05-built-in-features/01-help-system/):** Customize the adaptive and interactive help system.
  *   **[Version Flag Support]({{ site.baseurl }}/v2/guides/05-built-in-features/02-version-support/):** Automatically add a `--version` flag.
  *   **[Shell Completion]({{ site.baseurl }}/v2/guides/05-built-in-features/03-shell-completion/):** Generate completion scripts for popular shells.
  *   **[Environment & External Configuration]({{ site.baseurl }}/v2/guides/05-built-in-features/04-environment-config/):** Load configuration from environment variables or files.

### 5. Internationalization (i18n)
A comprehensive guide to creating multi-language CLIs.

*   **[Internationalization Overview]({{ site.baseurl }}/v2/guides/06-internationalization/index/)**
  *   **[Tooling: `goopt-i18n-gen`]({{ site.baseurl }}/v2/guides/06-internationalization/01-tooling-goopt-i18n-gen/):** A deep dive into the powerful code generation and workflow tool for i18n.