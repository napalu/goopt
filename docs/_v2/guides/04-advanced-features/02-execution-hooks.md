---
layout: default
title: Execution Hooks
parent: Advanced Features
nav_order: 2
---

# Execution Hooks Guide

The goopt v2 execution hooks feature provides pre and post-execution hooks for command lifecycle management, enabling cross-cutting concerns like logging, authentication, and cleanup.

## Overview

Execution hooks allow you to:
- Run code before commands execute (pre-hooks)
- Run code after commands execute (post-hooks)
- Handle errors and cleanup consistently
- Implement cross-cutting concerns without modifying command logic

## Hook Types

### Pre-Execution Hooks
Run before command execution. If a pre-hook returns an error, the command is not executed.

```go
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    // Runs before any command
    log.Printf("Executing: %s", c.Path())
    return nil
})
```

### Post-Execution Hooks
Run after command execution, even if the command fails. Receive the command's error (if any).

```go
parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, cmdErr error) error {
    // Always runs after command
    if cmdErr != nil {
        log.Printf("Command failed: %v", cmdErr)
    }
    return cleanup()
})
```

## Scope

### Global Hooks
Apply to all commands:

```go
// Global pre-hook
parser.AddGlobalPreHook(authCheck)

// Global post-hook
parser.AddGlobalPostHook(logMetrics)
```

### Command-Specific Hooks
Apply only to specific commands:

```go
// Pre-hook for "db backup" command
parser.AddCommandPreHook("db backup", validateBackupConfig)

// Post-hook for "server start" command
parser.AddCommandPostHook("server start", notifyServerStarted)
```

## Configuration

### Using With Functions

```go
parser, err := goopt.NewParserWith(
    goopt.WithGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
        return authenticate()
    }),
    goopt.WithGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
        return logExecution(c, err)
    }),
    goopt.WithCommandPreHook("deploy", validateDeployment),
    goopt.WithHookOrder(goopt.OrderGlobalFirst),
)
```

### Hook Execution Order

Control whether global or command-specific hooks run first:

```go
// Global hooks run first (default)
parser.SetHookOrder(goopt.OrderGlobalFirst)

// Command hooks run first
parser.SetHookOrder(goopt.OrderCommandFirst)
```

For cleanup, post-hooks run in reverse order of pre-hooks.

## Common Use Cases

### 1. Authentication/Authorization

```go
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    // Skip auth for public commands
    if isPublicCommand(c.Name) {
        return nil
    }
    
    // Check authentication
    if !isAuthenticated() {
        return errors.New("authentication required")
    }
    
    // Check authorization
    if !isAuthorized(c.Path()) {
        return errors.New("permission denied")
    }
    
    return nil
})
```

### 2. Logging and Telemetry

```go
// Track command execution
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    log.Printf("[START] Command: %s, User: %s", c.Path(), currentUser())
    telemetry.StartSpan(c.Path())
    return nil
})

parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
    duration := telemetry.EndSpan(c.Path())
    
    if err != nil {
        log.Printf("[ERROR] Command: %s, Error: %v, Duration: %v", 
            c.Path(), err, duration)
        metrics.IncrementErrors(c.Path())
    } else {
        log.Printf("[SUCCESS] Command: %s, Duration: %v", 
            c.Path(), duration)
        metrics.IncrementSuccess(c.Path())
    }
    
    return nil
})
```

### 3. Resource Management

```go
var dbConn *sql.DB

// Ensure database connection
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    if needsDatabase(c) {
        conn, err := connectDB()
        if err != nil {
            return fmt.Errorf("database connection failed: %w", err)
        }
        dbConn = conn
    }
    return nil
})

// Cleanup resources
parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
    if dbConn != nil {
        dbConn.Close()
        dbConn = nil
    }
    closeOpenFiles()
    releaseLocks()
    return nil
})
```

### 4. Transaction Management

```go
// Start transaction for write operations
parser.AddCommandPreHook("db update", func(p *goopt.Parser, c *goopt.Command) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    setTransaction(tx)
    return nil
})

// Commit or rollback based on result
parser.AddCommandPostHook("db update", func(p *goopt.Parser, c *goopt.Command, err error) error {
    tx := getTransaction()
    if tx == nil {
        return nil
    }
    
    if err != nil {
        return tx.Rollback()
    }
    return tx.Commit()
})
```

### 5. Environment Setup

```go
// Prepare environment
parser.AddCommandPreHook("test", func(p *goopt.Parser, c *goopt.Command) error {
    // Set up test environment
    os.Setenv("ENV", "test")
    createTempDirs()
    seedTestData()
    return nil
})

// Clean up environment
parser.AddCommandPostHook("test", func(p *goopt.Parser, c *goopt.Command, err error) error {
    // Always clean up
    removeTempDirs()
    clearTestData()
    os.Unsetenv("ENV")
    return nil
})
```

## Hook Context

Hooks have access to:
- Parser state (flags, arguments)
- Command information
- Previous errors (in post-hooks)

```go
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    // Access flags
    verbose, _ := p.Get("verbose")
    if verbose == "true" {
        log.SetLevel(log.DebugLevel)
    }
    
    // Access command info
    fmt.Printf("Executing: %s (%s)\n", c.Name, c.Description)
    
    // Access positional args
    args := p.GetPositionalArgs()
    validateArgs(args)
    
    return nil
})
```

## Error Handling

### Pre-Hook Errors
- Prevent command execution
- Post-hooks still run (for cleanup)
- Error is returned to caller

### Post-Hook Errors
- Don't affect command result (unless command succeeded)
- All post-hooks run regardless
- Last error is returned

### Example

```go
// Pre-hook error prevents execution
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    if !hasPermission(c) {
        return errors.New("permission denied") // Command won't run
    }
    return nil
})

// Post-hook always runs
parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
    // This runs even if pre-hook failed
    audit.Log(c.Path(), err)
    return nil
})
```

## Best Practices

### 1. Keep Hooks Lightweight
```go
// Good: Quick checks
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    if !isAuthenticated() {
        return errors.New("not authenticated")
    }
    return nil
})

// Avoid: Heavy processing in hooks
parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
    // Don't do this - move to command logic
    processLargeFile()
    return nil
})
```

### 2. Use Post-Hooks for Cleanup
```go
parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
    // Always runs, perfect for cleanup
    defer closeConnections()
    defer releaseResources()
    
    // Log regardless of success/failure
    logCommandExecution(c, err)
    return nil
})
```

### 3. Order Matters
```go
// Authentication should run before authorization
parser.AddGlobalPreHook(authenticate)
parser.AddGlobalPreHook(authorize) // Runs second

// Cleanup in reverse order
parser.AddGlobalPostHook(closeDatabase)  // Runs second
parser.AddGlobalPostHook(closeNetwork)   // Runs first
```

### 4. Handle Hook Errors
```go
if errs := parser.ExecuteCommands(); errs > 0 {
    // Check specific command errors
    for _, cmd := range executedCommands {
        if err := parser.GetCommandExecutionError(cmd); err != nil {
            handleError(cmd, err)
        }
    }
}
```

### 5. Document Hook Behavior
```go
// Package auth provides authentication hooks for CLI commands.
// 
// All commands except "login" and "help" require authentication.
// Set AUTH_TOKEN environment variable or login first.
package auth

import (
	"github.com/napalu/goopt/v2"
)

func AuthenticationHook(p *goopt.Parser, c *goopt.Command) error {
    // Skip auth for public commands
    if c.Name == "login" || c.Name == "help" {
        return nil
    }
    // ... authentication logic
}
```

## Complete Example

```go
package main

import (
    "errors"
	"os"
    "fmt"
    "log"
    "time"
    
    "github.com/napalu/goopt/v2"
)

func confirmProduction() bool {
	// something happens
	return true
}
func validateDeploymentConfig()error {
	// something happens
	return nil
}

func rollbackDeployment() {
	// something happens
}

func notifyDeploymentSuccess() {
	// something happens
}

func cleanupTempFiles() {
	// something happens
}

func main() {
    parser, err := goopt.NewParserWith(
        // Global logging
        goopt.WithGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
            log.Printf("[%s] Starting: %s", time.Now().Format(time.RFC3339), c.Path())
            return nil
        }),
        goopt.WithGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
            status := "SUCCESS"
            if err != nil {
                status = "FAILED"
            }
            log.Printf("[%s] %s: %s", time.Now().Format(time.RFC3339), status, c.Path())
            return nil
        }),
        
        // Command-specific validation
        goopt.WithCommandPreHook("deploy production", func(p *goopt.Parser, c *goopt.Command) error {
            if !confirmProduction() {
                return errors.New("production deployment cancelled")
            }
            return validateDeploymentConfig()
        }),
        
        // Cleanup hook
        goopt.WithCommandPostHook("deploy production", func(p *goopt.Parser, c *goopt.Command, err error) error {
            if err != nil {
                rollbackDeployment()
            } else {
                notifyDeploymentSuccess()
            }
            cleanupTempFiles()
            return nil
        }),
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    // Add commands...
    
    if !parser.Parse(os.Args) {
        parser.PrintHelp(os.Stderr)
        os.Exit(1)
    }
    
    if errs := parser.ExecuteCommands(); errs > 0 {
        os.Exit(1)
    }
}
```

## Integration with Other Features

Hooks work seamlessly with:
- **Auto-help**: Help display bypasses hooks
- **Version**: Version display bypasses hooks
- **Struct tags**: Hooks apply to struct-based commands
- **Nested commands**: Hooks see full command path
- **Internationalization**: Hook errors can use i18n

The execution hooks feature provides a powerful way to implement cross-cutting concerns in your CLI applications without cluttering command logic.