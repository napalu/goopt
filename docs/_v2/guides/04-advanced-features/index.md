---
layout: default
title: Advanced Features
nav_order: 4
parent: Guides
has_children: true
version: v2
---

# Advanced Features

Once you are comfortable with the basics of defining your CLI, this section will guide you through some of `goopt`'s more powerful features. Here, you'll learn how to add robust validation, create sophisticated command lifecycles with execution hooks, and manage complex flag inheritance scenarios.

These features allow you to build more resilient, maintainable, and professional command-line applications.

**Topics:**

1.  **[Validation]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/):** A deep dive into the validation system, from built-in rules to creating your own custom validators.
2.  **[Execution Hooks]({{ site.baseurl }}/v2/guides/04-advanced-features/02-execution-hooks/):** Learn how to run code before and after your commands to handle cross-cutting concerns like logging, authentication, and resource cleanup.
3.  **[Error Handling]({{ site.baseurl }}/v2/guides/04-advanced-features/03-error-handling/):** Best practices for robust error handling during argument and parser setup.
4.  **[Flag Inheritance]({{ site.baseurl }}/v2/guides/04-advanced-features/04-flag-inheritance/):** Understand the rules for how flags are inherited and overridden in nested command hierarchies.