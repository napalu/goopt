---
layout: default
title: Flag Inheritance
parent: Advanced Features
nav_order: 4
---

# Flag Inheritance

In `goopt`, flags defined on parent commands are automatically inherited by their subcommands. This powerful feature allows you to define common flags (like `--verbose` or `--region`) once at a higher level in your command hierarchy.

## Inheritance Rules

1.  **Inheritance:** Child commands automatically have access to all flags defined on their parents, all the way up to the global scope.
2.  **Precedence & Overriding:** If a child command defines a flag with the *same name* as an inherited flag, the child's version takes precedence. This allows you to change the behavior or default value of a flag for a specific subcommand.
3.  **Accessing Values:** When accessing the value of an inherited or overridden flag from a struct, you must use the field corresponding to where that specific flag was defined.

## Example

Consider this CLI configuration for a service:

```go
type Config struct {
	// 1. Global Flag: Available to all commands.
	LogLevel string `goopt:"name:log-level;short:l;default:INFO"`

	Service struct {
		// 2. Parent Command Flag: Inherited by 'start' and 'stop'.
		Port int `goopt:"name:port;default:8080"`

		Start struct {
			// 3. Overridden Flag: 'log-level' here takes precedence over the global one.
			LogLevel string `goopt:"name:log-level;default:DEBUG"`
			Workers  int    `goopt:"name:workers;default:4"`
		} `goopt:"kind:command;name:start"`

		Stop struct {
			// This command inherits 'log-level' from global and 'port' from 'service'.
			Force bool `goopt:"name:force"`
		} `goopt:"kind:command;name:stop"`

	} `goopt:"kind:command;name:service"`
}
```

### Command-Line Scenarios

Let's see how `goopt` resolves the values:

*   **`./app service stop`**
    *   `LogLevel`: "INFO" (from global `Config.LogLevel`)
    *   `Port`: 8080 (from `Config.Service.Port`)

*   **`./app service start`**
    *   `LogLevel`: "DEBUG" (from the overridden `Config.Service.Start.LogLevel`)
    *   `Port`: 8080 (from `Config.Service.Port`)

*   **`./app -l WARN service stop`**
    *   `LogLevel`: "WARN" (the global `-l` flag is set, and `stop` inherits it)
    *   `Port`: 8080

*   **`./app -l WARN service start`**
    *   `LogLevel`: "DEBUG" (the `start` command has its own `log-level` definition, so the global `-l` flag is ignored for this command path)
    *   `Port`: 8080

This inheritance model provides a flexible way to manage common and specific options across a complex CLI.