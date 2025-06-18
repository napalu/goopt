---
layout: default
title: Struct Tags Reference
parent: Defining Your CLI
nav_order: 1
---

# Struct Tag Reference

When using the struct-first approach, `goopt` uses struct tags to define commands, flags, and their behavior. All options are defined within the `goopt:"..."` tag, with key-value pairs separated by semicolons.

```go
type Config struct {
    Output string `goopt:"name:output;short:o;desc:Output file;required:true"`
}
```

## Available Tags

This table provides a quick reference to all available tags and their purpose.

| Tag        | Description                                                   | Example                                     |
|:-----------|:--------------------------------------------------------------|:--------------------------------------------|
| `kind`     | Specifies if a struct field represents a `flag` or a `command`. Default is `flag`. | `kind:command` |
| `name`     | Sets the long name for the flag or command (e.g., `--output`). | `name:output` |
| `short`    | Sets a single-character short name (e.g., `-o`). | `short:o` |
| `desc`     | A human-readable description shown in the help text. | `desc:"The output file path"` |
| `desckey`  | An i18n key for a translatable description. | `desckey:flag.output.desc` |
| `type`     | Overrides the inferred flag type. See `types.OptionType`. | `type:standalone` |
| `required` | Makes a flag mandatory. The parser will error if it's missing. | `required:true` |
| `default`  | Provides a default value if the flag is not set. | `default:./output.txt` |
| `secure`   | Marks a flag as a secure input (e.g., for passwords). Hides user input. | `secure:true` |
| `prompt`   | Sets the prompt text to display for a `secure` flag. | `prompt:"Enter password:"` |
| `path`     | Associates a flag with one or more commands declaratively. | `path:"server start,server stop"` |
| `pos`      | Defines a flag as a positional argument at a specific index. | `pos:0` |
| `capacity` | For slices of nested structs, pre-allocates the slice capacity. | `capacity:5` |
| `validators`| A comma-separated list of validation rules to apply to the flag's value. | `validators:"email,minlength(8)"` |
| `depends`  | Defines a dependency on another flag. | `depends:"{flag:format,values:[json]}"` |

For more detailed examples of how to use these tags to structure your application, see the [Command Patterns](./02-command-patterns.md) and [Flag Patterns](./03-flag-patterns.md) guides.