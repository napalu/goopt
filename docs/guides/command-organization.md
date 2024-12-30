---
layout: default
title: Command Organization
parent: Guides
nav_order: 2
---

# Command Organization

goopt offers three main approaches to organizing commands and flags:

## Flag-Centric Approach

Best for simpler CLIs with flat structure. Using the path variables allows creating commands and flags in a single line. Several paths can be specified to associate the flag with multiple commands. The value of the flag is shared across all commands that have the same path.

```go
type Options struct {
    CreateUser string `goopt:"kind:flag;path:user create;name:name;desc:Create a new user"` // path is optional, but can be used to specify the command path - this will create a command called "user create" and associate the flag with the "user create" command-context. 
    Force bool `goopt:"kind:flag;path:user create,user delete;name:force;desc:Force operation"` // shared flag for both  "user create" and "user delete" commands
}
```


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

// alternative way to define the same structure
user := Command{
    Name: "user",
    Subcommands: []Command{
        {Name: "create", Description: "User name"},
    },
}

type OtherOptions struct {
    User :  Command{
        Name: "user",
        Subcommands: []Command{
            {Name: "create", Description: "User name"},
        },
    },
   
}
```


## Mixed Approach

```go
type Options struct {
    User struct {
        Create struct {
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
    }
}
``` 