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

This table provides a complete reference to all available tags.

| Tag | Description | Example Syntax |
|:---|:---|:---|
| `kind` | Specifies if a struct field represents a `flag` or a `command`. Default is `flag`. | `kind:command` |
| `name` | Sets the long name for the flag or command (e.g., `--output`). | `name:output` |
| `short` | Sets a single-character short name (e.g., `-o`). | `short:o` |
| `desc` | A human-readable description shown in the help text. | `desc:"The output file path"` |
| `desckey` | An i18n key for a translatable description. | `desckey:flag.output.desc` |
| `type` | Overrides the inferred flag type. See `types.OptionType`. | `type:standalone` |
| `required` | Makes a flag mandatory. The parser will error if it's missing. | `required:true` |
| `default` | Provides a default value if the flag is not set. | `default:./output.txt` |
| `secure` | Marks a flag as a secure input (e.g., for passwords). Hides user input. | `secure:true` |
| `prompt` | Sets the prompt text to display for a `secure` flag. | `prompt:"Enter password:"` |
| `path` | Associates a flag with one or more comma-separated commands. | `path:"server start,server stop"` |
| `pos` | Defines a flag as a positional argument at a specific index. | `pos:0` |
| `capacity` | For slices of nested structs, pre-allocates the slice capacity. | `capacity:5` |
| `validators` | A comma-separated list of validation rules to apply. | `validators:"email,minlength(8)"` |
| `depends` | Defines a dependency where this flag requires another flag to be present with a specific value. | `depends:"{flag:format,values:[json]}"` |
| `accepted` | **[Deprecated]** Use the `validators` tag instead. | `accepted:"{pattern:json,desc:Format}"` |

---

## Complex Tag Formats

Some tags, like `validators` and `depends`, accept more complex values.

### `validators`

The `validators` tag accepts a comma-separated list of validation rules. Validators with arguments must use parentheses.

*   **Simple list:** `validators:"email,minlength(8),alphanumeric"`
*   **With arguments:** `validators:"range(1,100)"`
*   **Regex pattern:** `validators:"regex(^[A-Z]{3}-\d{4}$)"`
*   **Composition:** `validators:"oneof(email,regex(^USR-\d{8}$))"`

For a complete guide, see the [**Validation Guide**]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/).

### `depends`

The `depends` tag enforces that another flag must be present (and optionally have a specific value) for the current flag to be valid.

It uses a brace-enclosed format: `{flag:FLAG_NAME,values:[VALUE1,VALUE2]}`.

*   **Simple Dependency:** The `--compress` flag requires the `--output` flag to be present.
    ```go
    type Config struct {
        Output   string
        Compress bool `goopt:"depends:{flag:output}"`
    }
    ```

*   **Dependency with Specific Values:** The `--compress` flag is only valid if `--format` is either `json` or `tar`.
    ```go
    type Config struct {
        Format   string
        Compress bool `goopt:"depends:{flag:format,values:[json,tar]}"`
    }
    ```

*   **Multiple Dependencies:** You can chain multiple dependency blocks, separated by commas.
    ```go
    // --api-key is only valid if --auth=token AND --httpss=true
    APIKey string `goopt:"depends:{flag:auth,values:[token]},{flag:https,values:[true]}"`
    ```

### `accepted` (Deprecated)

The `accepted` tag is deprecated in favor of the more powerful `validators` system.

*   **Old syntax:** `accepted:"{pattern:json|yaml,desc:Output format}"`
*   **New equivalent:** `validators:"isoneof(json,yaml)"` or `validators:"regex(json|yaml)"`

---

For more detailed examples of how to use these tags to structure your application, see the [Command Patterns](./02-command-patterns.md) and [Flag Patterns](./03-flag-patterns.md) guides.