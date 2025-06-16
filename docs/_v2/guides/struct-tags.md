---
layout: default
title: Struct Tags
parent: Guides
nav_order: 5
---

# Struct Tag Reference

## Tag Format

The `goopt` tag format uses semicolon-separated key:value pairs. All options are defined under the `goopt` namespace:

```go
type Config struct {
    Output string `goopt:"name:output;desc:Output file;required:true"`
}
```

## Available Tags

| Tag        | Description                                                   | Example                                     | 
|------------|---------------------------------------------------------------|---------------------------------------------|
| `kind`     | Specifies if it's a flag or command (default: flag)           | `kind:flag\|command`                        |
| `name`     | Long name for the flag/command                                | `name:output`                               |
| `short`    | Short name (single-char for POSIX mode)                       | `short:o`                                   |
| `desc`     | Description shown in help                                     | `desc:Output file`                          |
| `type`     | Flag type                                                     | `type:single\|standalone\|chained\|file`    |
| `required` | Whether flag is required                                      | `required:true\|false`                      |
| `default`  | Default value                                                 | `default:stdout`                            |
| `secure`   | For password input                                            | `secure:true\|false`                        |
| `prompt`   | Prompt text for secure input                                  | `prompt:Password:`                          |
| `capacity` | Slice capacity for nested structs                             | `capacity:3`                                |
| `path`     | Associates a command with one or more (comma-delimited paths) | `path:create user,create group`             |
| `pos`      | Position requirements                                         | `pos:0`                                     |
| `accepted` | Accepted values/patterns (deprecated, use validators)         | `accepted:{pattern:json\|yaml,desc:Format}` |
| `depends`  | Flag dependencies                                             | `depends:{flag:output,values:[json]}`       |
| `validators`| Value validators (recommended over accepted)                 | `validators:email,minlength:5`              |

## Path Tag

The `path` attribbute allows associating a flag with one or several commands, meaning that the flag is evaluated in the command context it is
associated with. This allows sharing a flag with several commands at the same time (the values will be shared). It is also a useful
shorthand to create commands dynamically since a command will be created when specified in the `path` attribute.

```go
package main

import (
	"github.com/napalu/goopt/v2"
	"log"
	"os"
)

type CLI struct {
	SharedFlag string `goopt:"short:s;desc:this flag is shared;path:create user,create group"`
}

func main() {
	options := &CLI{}
	// SharedFlag path directives will create commands 'create user' and 'create group' and its value will be shared with both
	parser, err := goopt.NewParserFromStruct(options)
	if err != nil {
		log.Fatalf("failed to create parser: %w", err)
	}

	ok := parser.Parse(os.Args)
	if !ok {
		parser.PrintUsage(os.Stderr)
    }
}
```

## Position Tag

The `pos` tag allows specifying position requirements for arguments:

```go
type Config struct {
    // Must be first argument
    Source string `goopt:"name:source;pos:0"`
        // Second argument from start
    Profile string `goopt:"name:profile;pos:1"`
    // Must be last argument
    Dest string `goopt:"name:dest;pos:2"`
    
}
```

## Validators Tag

The `validators` tag provides a powerful and composable way to validate input values. Multiple validators can be combined with commas. Note that commas within validator arguments must be escaped with `\\,`.

### Common Validators

```go
type Config struct {
    // Built-in validators
    Email    string `goopt:"validators:email"`
    Port     int    `goopt:"validators:port"`
    Count    int    `goopt:"validators:integer,min:1,max:100"`
    Username string `goopt:"validators:alphanumeric,minlength:3,maxlength:20"`
    
    // Regex validators with quantifiers (note escaped commas in the desc field)
    ID       string `goopt:"validators:regex:{pattern:^[A-Z]{2}\\d{4}$\\,desc:2 letters\\, 4 digits}"`
    Phone    string `goopt:"validators:regex:{pattern:^\\d{3,4}-\\d{3}-\\d{4}$\\,desc:Phone (3-4 digit area code)}"`
    
    // Regex with translation key (see [Internationalization guide](internationalization.md))
    License  string `goopt:"validators:regex:{pattern:^[A-Z]{3}-\\d{6}$\\,desc:validation.license.format}"`
    
    // Multiple choice validators  
    Color    string `goopt:"validators:isoneof:red:green:blue"`
    Format   string `goopt:"validators:isoneof:json:yaml:xml"`
}
```

### Validator Composition

Validators are applied with AND logic by default. For OR logic, use programmatic validators:

```go
parser.AddFlagValidator("zip", validation.OneOf(
    validation.Regex("^\\d{5}$", "5-digit ZIP"),
    validation.Regex("^\\d{5}-\\d{4}$", "ZIP+4 format"),
))
```

## Complex Tag Examples

### Multiple Accepted Values (Deprecated - Use Validators)
```go
type Config struct {
    // Old way (still works but deprecated)
    Format string `goopt:"name:format;accepted:{pattern:json|yaml,desc:Format},{pattern:text|binary,desc:Type}"`
    
    // New way using validators
    Format string `goopt:"name:format;validators:isoneof:json:yaml:text:binary"`
}
```

### Multiple Dependencies
```go
type Config struct {
    Compress bool `goopt:"name:compress;depends:{flag:format,values:[json]},{flag:output,values:[file]}"`
}
```

## Flag Namespacing with Nested Structs:

You can organize flags by nesting structs. Field names of regular (non-command) nested structs contribute to a dot-notation prefix for the long flag names invoked on the command line.
The prefix is derived using the configured FlagNameConverter.

```go
type Config struct {
    // Non-command struct acts as a namespace container
    Database struct {
        Host string `goopt:"name:host"` // Invoked as --database.host
        Port int    `goopt:"name:port"` // Invoked as --database.port
    }
}
```

**Important**: Structs marked with `kind:command` define commands and scopes for flag resolution, but their field names do not add to the dot-notation prefix for invoking flags defined within them. 
The prefix is determined only by the nesting of non-command structs.

```go
type Config struct {
    App struct { // 'App' is NOT a command, provides 'app.' prefix
        Service struct { // 'Service' IS a command, defines path 'service', NO 'service.' prefix
            Port int `goopt:"name:port;short:p"` // Invoked as --app.port or -p

            Stop struct { // 'Stop' IS a command, defines path 'service stop', NO 'stop.' prefix
                Force bool `goopt:"name:force;short:f"` // Invoked as --app.force or -f
            } `goopt:"kind:command"`
        } `goopt:"kind:command"`
    }
}
```