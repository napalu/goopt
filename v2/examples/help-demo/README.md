# Help Demo for `goopt`

This demo showcases advanced help system capabilities of the [`goopt`](https://github.com/napalu/goopt/v2) CLI parser, including multilingual help, filtered views, and smart contextual error reporting.

The demo makes it easier to:
- Understand `goopt`’s smart help behavior
- See how internationalization (i18n) integrates seamlessly
- Inspect flag grouping, pattern filtering, and hierarchical layouts
- Show helpful contextual error feedback to end users


## How to Run

```bash
go run ./help-demo.go
```

## What This Demo Shows

Upon running, the demo will:

1. Parse a sample command line with required global flags
2. Execute `demo` which:
   - Iterates over multiple help scenarios
   - Sets help language (English, German, French)
   - Demonstrates various `--help` submodes and features

## Help Features Demonstrated

| #  | Feature                             | Example                                   |
|----|-------------------------------------|-------------------------------------------|
| 1  | Global Flags                        | `--help globals`                          |
| 2  | Command Tree                        | `--help commands`                         |
| 3  | Help with Defaults                  | `--help --show-defaults`                  |
| 4  | Flag Filtering                      | `--help --filter core.ldap.*`             |
| 5  | Search Help Content                 | `--help --search user`                    |
| 6  | Contextual Help for Commands        | `users --help`                            |
| 7  | Graceful Error for Invalid Commands| `invalid-cmd`                             |
| 8  | Example Commands                    | `--help examples`                         |
| 9  | Help for Nested Commands            | `--help users create`                     |
| 10 | Help System Help                    | `--help --help`                           |

## Supported Languages

- English (`en`)
- German (`de`)
- French (`fr`)

Translations are applied automatically based on `LANG`, or programmatically via `SetLanguage`.

## Requirements

- Go 1.18 or higher
- [`golang.org/x/text`](https://pkg.go.dev/golang.org/x/text)
- [`github.com/napalu/goopt`](https://github.com/napalu/goopt)

## Related Links

- [goopt on GitHub](https://github.com/napalu/goopt/v2)
- [Examples Folder](./examples)
- [Translation Files](./internal/messages)

---
