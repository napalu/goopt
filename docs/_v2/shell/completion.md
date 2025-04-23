---
layout: default
title: Shell Completion
nav_order: 4
---

# Shell Completion

goopt provides completion support for:
- Bash
- Zsh
- Fish
- PowerShell

## Installation

### All shells
```go
package main

import (
    "os"
    "log"
    "fmt"
    "github.com/napalu/goopt/v2"
    c "github.com/napalu/goopt/v2/completion"
)

func main() {
    // ... parser setup  ...
    
    // Add completion support
    exec, err := os.Executable()
    if err != nil {
        log.Fatal(err)
    }

    completionData := parser.GetCompletionData()
    // Generate completion scripts for all supported shells or a specific shell
    wantedShells := []string{"bash", "zsh", "fish", "powershell"}
    for _, shell := range wantedShells {
        manager, err := c.NewManager(shell, exec)
        if err != nil {
            log.Fatal(err)
        }

        err = manager.Accept(completionData)
        if err != nil {
            log.Fatal(err)
        }
    
        path, err := manager.Save()
        if err != nil {
            log.Fatal(err)
        }

        fmt.Printf("%s completion script saved. Depending on your shell, you may need to `source %s` the completion script.\n", shell, path)
    }
}
```
