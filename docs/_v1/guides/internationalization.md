---
layout: default
title: Internationalization
parent: Guides
nav_order: 5
---

# Internationalization (i18n) Guide

## Overview
Introduction to how internationalization works in goopt.

## Basic Usage
How to add translations to your CLI application:

```go
// Create a parser
parser := goopt.NewParser()

// Add translations for a new language
err := parser.AddTranslations(language.German, map[string]string{
    "my.command.description": "Befehlsbeschreibung",
    "my.flag.description": "Flaggenbeschreibung"
})
```

## Translation Key Organization
Recommended patterns for organizing translation keys:
- Command keys: `app.commands.{command_name}`
- Flag keys: `app.flags.{flag_name}`
- Error messages: `app.errors.{error_name}`

## Advanced Usage
- Command-specific translations
- Replacing the default bundle
- Loading translations from files

## Complete Example
A full example showing a CLI application with i18n support.