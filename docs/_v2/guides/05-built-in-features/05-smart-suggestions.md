---
layout: default
title: Smart Suggestions
parent: Built-in Features
nav_order: 5
version: v2
---

# Smart "Did You Mean?" Suggestions

`goopt` includes an intelligent suggestion system that helps users recover from typos and discover available commands and flags. This feature significantly improves the user experience by providing helpful hints when users make mistakes.

## How It Works

When a user types an unrecognized command or flag, goopt automatically:

1. **Analyzes the input** using Levenshtein distance algorithm
2. **Finds similar options** from available commands and flags
3. **Filters suggestions** based on distance thresholds
4. **Displays helpful hints** in error messages

## Examples

### Command Suggestions

```bash
# User mistypes a command
$ myapp serverr start
Error: Unknown command "serverr". Did you mean "server"?

# Multiple suggestions for ambiguous input
$ myapp sta
Error: Unknown command "sta". Did you mean one of these?
  start
  status
  stats
```

### Flag Suggestions

```bash
# User mistypes a flag
$ myapp --verbse
Error: unknown flag: verbse. Did you mean one of these?
  --verbose
  --version

# Works with short flags too
$ myapp -hel
Error: unknown flag: hel. Did you mean "-h" (help)?
```

## Context-Aware Suggestions for i18n

One of goopt's most sophisticated features is context-aware suggestions for internationalized applications. The system detects whether the user's input is closer to the canonical (English) name or a translated name:

```bash
# German user types something close to German translation
$ myapp --max-verbindung
Fehler: unbekannter Flag: max-verbindung. Meinten Sie vielleicht eines davon?
  --max-verbindungen

# Same user types something close to English
$ myapp --max-connection  
Error: unknown flag: max-connection. Did you mean one of these?
  --max-connections
```

This works because goopt:
- Checks distances to both canonical and translated names
- Shows suggestions in the language closest to what the user typed
- Maintains consistency with the user's apparent language preference

## Customizing Suggestion Behavior

### Adjusting Sensitivity with Thresholds

You can control how fuzzy the matching should be:

```go
// Default: maximum Levenshtein distance of 2 for both
parser := goopt.NewParser()

// More permissive matching
parser.SetSuggestionThreshold(3, 3)  // Distance up to 3

// Different thresholds for flags vs commands
parser.SetSuggestionThreshold(3, 2)  // Flags: 3, Commands: 2

// Disable suggestions
parser.SetSuggestionThreshold(0, 0)  // No suggestions

// Set during parser creation
parser, _ := goopt.NewParserWith(
    goopt.WithSuggestionThreshold(2, 3),
)
```

### Conservative Filtering

goopt uses conservative filtering to avoid overwhelming users:
- If there are matches with distance 1, only those are shown
- If not, matches with distance 2 are shown
- This prevents showing "stop" as a suggestion for "start" (distance 4)

### Custom Display Formatting

You can customize how suggestions are displayed:

```go
// Default format: [--verbose, --version]
parser.SetSuggestionsFormatter(func(suggestions []string) string {
    return "[" + strings.Join(suggestions, ", ") + "]"
})

// Bullet list format
parser.SetSuggestionsFormatter(func(suggestions []string) string {
    return "\n  • " + strings.Join(suggestions, "\n  • ")
})

// Numbered list
parser.SetSuggestionsFormatter(func(suggestions []string) string {
    var result []string
    for i, s := range suggestions {
        result = append(result, fmt.Sprintf("%d. %s", i+1, s))
    }
    return "\n  " + strings.Join(result, "\n  ")
})
```

## Integration with Help System

The suggestion system is fully integrated with goopt's help system:

```bash
# Suggestions in help context
$ myapp help serverr
Error: Unknown command 'serverr'

Did you mean: 
  server

Available commands:
  server     Server management
  service    Service management
  status     Show status
```

## Best Practices

1. **Keep default thresholds** - The default distance of 2 works well for most CLIs
2. **Test with common typos** - Ensure your command/flag names are distinct enough
3. **Consider your audience** - Increase thresholds for expert users, decrease for beginners
4. **Use clear, distinct names** - Avoid similar names like "remove" and "remote"

## Implementation Details

The suggestion system:
- Runs automatically during parsing
- Has minimal performance impact
- Works with all goopt features (subcommands, namespaced flags, etc.)
- Respects the current language settings for i18n applications
- Is consistent across all error contexts

This feature requires no configuration to use - it's enabled by default and just works!