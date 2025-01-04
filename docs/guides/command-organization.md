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
    // Commands defined by path in flag structs
    CreateUser string `goopt:"kind:flag;path:user create;name:name;desc:Create a new user"`
    CreateRole string `goopt:"kind:flag;path:role create;name:name;desc:Create a new role"`
    DeleteUser string `goopt:"kind:flag;path:user delete;name:name;desc:Delete a user"`
    
    // Shared flag across specific commands
    Force bool `goopt:"kind:flag;path:user create,user delete,role create;name:force;desc:Force operation"`
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

```go
type Options struct {
    User struct {
        `goopt:"kind:command;name:user;desc:User management"`
        
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
        Delete struct {
            `goopt:"kind:command;name:delete;desc:Delete a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
    }
    Role struct {
        `goopt:"kind:command;name:role;desc:Role management"`
        
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a role"`
            Name string `goopt:"kind:flag;name:name;desc:Role name"`
        }
    }
    
    // Shared flag across specific commands
    Force bool `goopt:"kind:flag;path:user create,user delete,role create;name:force;desc:Force operation"`
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

```go
type Options struct {
    // Command-centric for user management
    User struct {
        `goopt:"kind:command;name:user;desc:User management"`
        
        Create struct {
            `goopt:"kind:command;name:create;desc:Create a user"`
            Name string `goopt:"kind:flag;name:name;desc:User name"`
        }
    }
    
    // Flag-centric for role management
    CreateRole string `goopt:"kind:flag;path:role create;name:name;desc:Create a new role"`
    DeleteRole string `goopt:"kind:flag;path:role delete;name:name;desc:Delete a role"`
    
    // Shared flag across specific commands
    Force bool `goopt:"kind:flag;path:user create,role create;name:force;desc:Force operation"`
}
```

--- 
Each approach has its benefits:
- Flag-centric is flatter and good for simpler CLIs
- Command-centric provides clear structure for complex command hierarchies
- Mixed approach allows flexibility where needed