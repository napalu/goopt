---
layout: default
title: Command Patterns
parent: Defining Your CLI
nav_order: 2
version: v2
---

# Command Patterns

`goopt` offers several powerful patterns for defining your CLI's command structure. Choosing the right pattern depends on the complexity of your application and your preferred coding style.

## 1. Struct-Based Commands (Nested Structs)

This is the most common and intuitive approach for creating a clear command hierarchy. You define commands and their subcommands by nesting structs and marking them with `goopt:"kind:command"`.

```go
type CLI struct {
    // 'server' is a command
    Server struct {
        // 'start' is a subcommand of 'server'
        Start struct {
            // Flags for the 'start' command go here
            Port int `goopt:"default:8080"`
        } `goopt:"kind:command;desc:Start the server"`

        // 'stop' is another subcommand
        Stop struct {
            Force bool `goopt:"short:f"`
        } `goopt:"kind:command;desc:Stop the server"`

    } `goopt:"kind:command;desc:Manage the server"`
}
```
**Usage:**
```bash
./my-cli server start --port 9090
./my-cli server stop --force
```

**Advantages:**
- **Clear Hierarchy:** The code structure directly mirrors the command-line structure.
- **Scoped Flags:** Flags are naturally scoped to their command.
- **Type-Safe:** The entire structure is validated at compile time.

## 2. Path-Based Commands (Declarative)

For CLIs where many commands share flags, the `path` tag offers a more declarative, "flag-centric" approach. Instead of nesting structs for commands, you define a flat struct of flags and use the `path` tag to assign them to dynamically created commands.

```go
type CLI struct {
      // This flag is associated with two different command paths.
      // goopt creates the command hierarchy automatically.
      SharedFlag string `goopt:"short:s;path:create user,create group"`
      
      // This flag is specific to the 'create user' command.
      UserEmail string `goopt:"short:e;path:create user"`
}
```

**Usage:**
```bash
./my-cli create user -s "common" -e "user@example.com"
./my-cli create group -s "common"
```

**Advantages:**
- **DRY:** Excellent for sharing a single flag field across multiple, distinct commands.
- **Flat Structure:** Avoids deep struct nesting.
- **Dynamic:** Commands are created implicitly from the paths you define.

**Trade-offs:**
- The command structure is less visible in the code's hierarchy.
- Can be harder to manage for very complex CLIs with deep nesting.

## 3. Programmatic Commands (Builder Pattern)

For maximum flexibility, you can define your commands and subcommands programmatically. This is useful when the command structure is not known at compile time.

```go
parser := goopt.NewParser()

parser.AddCommand(
    goopt.NewCommand(
        goopt.WithName("server"),
        goopt.WithCommandDescription("Manage the server"),
        goopt.WithSubcommands(
            goopt.NewCommand(goopt.WithName("start")),
            goopt.NewCommand(goopt.WithName("stop")),
        ),
    ),
)
```

**Advantages:**
- **Dynamic:** Perfect for building CLIs based on runtime configuration, plugins, or other dynamic sources.
- **Explicit:** The structure is built step-by-step in code.

## Best Practices

*   **Start with Structs:** For most applications, the **Struct-Based** approach is the cleanest and most maintainable.
*   **Use `path` for Sharing:** When you have a few shared flags across many different commands, the **Path-Based** approach is a great way to reduce code duplication.
*   **Mix and Match:** Don't be afraid to use a hybrid approach. You can define your main structure with structs and then programmatically add a dynamic command if needed.