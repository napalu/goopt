---
layout: default
title: Shell Completion
parent: Built-in Features
nav_order: 3
version: v2
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
    parser := goopt.NewParser()
	// ... parser setup  ...

	// In your CLI definition, add a 'completion' command.
	parser.AddCommand(goopt.NewCommand(
		goopt.WithName("completion"),
		goopt.WithCommandDescription("Generate shell completion script"),
		goopt.WithCallback(func(p *goopt.Parser, c *goopt.Command) error {
			// In a real app, you'd let the user specify the shell
			// as an argument to this command (e.g., 'completion bash').
			shell := "bash"

			exec, err := os.Executable()
			if err != nil {
				return err
			}

			manager, err := c.NewManager(shell, exec)
			if err != nil {
				return err
			}

			// Provide the completion data from your main parser.
			manager.Accept(p.GetCompletionData())

			// Print the script to stdout. The user can then pipe it to a file.
			// e.g., ./myapp completion > /etc/bash_completion.d/myapp
			path, err := manager.Save()
			if err != nil {
				return err
			}

			fmt.Printf("%s completion script saved. Depending on your shell, you may need to `source %s` the completion script.\n", shell, path)
			return nil
		}),
	))
}
```
