---
layout: default
title: Defining Your CLI
nav_order: 6
parent: Guides
has_children: true
version: v2
---

# Defining Your CLI

This section covers the fundamental patterns for structuring your command-line application with `goopt`. You will learn how to define flags, organize commands, and handle positional arguments using `goopt`'s flexible struct-first and programmatic APIs.

Choose a topic to begin:

1.  **[Struct Tags Reference]({{ site.baseurl }}/v2/guides/03-defining-your-cli/01-struct-tags-reference/):** A quick reference guide to all available `goopt:"..."` struct tags.
2.  **[Command Patterns]({{ site.baseurl }}/v2/guides/03-defining-your-cli/02-command-patterns/):** Learn different ways to organize your commands and subcommands, from simple flat structures to complex nested hierarchies.
3.  **[Flag Patterns]({{ site.baseurl }}/v2/guides/03-defining-your-cli/03-flag-patterns/):** Explore patterns for organizing your flags, including namespacing with nested structs and creating reusable flag groups.
4.  **[Positional Arguments]({{ site.baseurl }}/v2/guides/03-defining-your-cli/04-positional-arguments/):** A detailed guide on defining and using arguments that rely on their position in the command line.
5.  **[Command Callbacks]({{ site.baseurl }}/v2/guides/03-defining-your-cli/05-command-callbacks/):** Learn how to add behavior to your commands and access parsed data from your logic.