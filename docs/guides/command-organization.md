---
layout: default
title: Command Organization
parent: Guides
nav_order: 2
---

# Command Organization

goopt offers three main approaches to organizing commands and flags:

## Flag-Centric Approach

Best for simpler CLIs with flat structure:

```go
type Options struct {
    CreateUser string `goopt:"kind:flag;path:user create;name:name;desc:Create a new user"`
    Force bool `goopt:"kind:flag;path:user create,user delete;name:force;desc:Force operation"`
}
```

[Your current flag-centric content]

## Command-Centric Approach

Better for complex hierarchical CLIs:

```go
type Options struct {
    User struct {
        `goopt:"kind:command;name:user;desc:User management"`
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
    }
}
```

[Your current command-centric content]

## Mixed Approach

[Your current mixed approach content] 