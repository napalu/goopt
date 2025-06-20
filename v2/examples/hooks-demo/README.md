# Execution Hooks Demo

This example demonstrates goopt v2's pre/post execution hooks feature for command lifecycle management.

## Features Demonstrated

1. **Global Logging** - Log all command executions
2. **Authentication** - Require auth for protected commands
3. **Cleanup** - Always run cleanup, even on errors
4. **Metrics Collection** - Track command performance
5. **Command-Specific Hooks** - Different hooks for different commands

## Running the Examples

### Interactive Demo
```bash
go run main.go
```

### Real CLI Usage

#### Authentication Flow
```bash
# Try protected command without auth (will fail)
go run main.go db status

# Login first
go run main.go login -u alice

# Now protected commands work
go run main.go db status

# Or use environment variable
AUTH_TOKEN=alice go run main.go db status
```

#### Database Operations with Hooks
```bash
# Backup with pre/post hooks
go run main.go db backup -o backup.sql

# With verbose logging
go run main.go -v db backup -o backup.sql -c

# Restore (different hooks)
go run main.go db restore -i backup.sql
```

#### Server Operations
```bash
# Start server
go run main.go server start -p 9000 -w 8

# Check status
go run main.go server status

# Stop server
go run main.go server stop --force
```

## Hook Types

### Global Hooks
Apply to all commands:
```go
parser.AddGlobalPreHook(func(p *Parser, cmd *Command) error {
    // Runs before any command
    return nil
})

parser.AddGlobalPostHook(func(p *Parser, cmd *Command, err error) error {
    // Runs after any command (even on error)
    return nil
})
```

### Command-Specific Hooks
Apply to specific commands only:
```go
parser.AddCommandPreHook("db backup", func(p *Parser, cmd *Command) error {
    // Runs before 'db backup' command
    return nil
})

parser.AddCommandPostHook("db backup", func(p *Parser, cmd *Command, err error) error {
    // Runs after 'db backup' command
    return nil
})
```

## Common Use Cases

### 1. Authentication/Authorization
```go
parser.AddGlobalPreHook(func(p *Parser, cmd *Command) error {
    if !isAuthenticated() {
        return errors.New("authentication required")
    }
    if !isAuthorized(cmd.Path()) {
        return errors.New("permission denied")
    }
    return nil
})
```

### 2. Logging/Telemetry
```go
parser.AddGlobalPreHook(func(p *Parser, cmd *Command) error {
    log.Printf("[START] %s", cmd.Path())
    return nil
})

parser.AddGlobalPostHook(func(p *Parser, cmd *Command, err error) error {
    if err != nil {
        log.Printf("[ERROR] %s: %v", cmd.Path(), err)
        metrics.RecordError(cmd.Path())
    } else {
        log.Printf("[SUCCESS] %s", cmd.Path())
        metrics.RecordSuccess(cmd.Path())
    }
    return nil
})
```

### 3. Resource Management
```go
parser.AddGlobalPostHook(func(p *Parser, cmd *Command, err error) error {
    // Always cleanup, even on errors
    closeConnections()
    releaseLocks()
    deleteTempFiles()
    return nil
})
```

### 4. Transaction Management
```go
parser.AddCommandPreHook("db update", func(p *Parser, cmd *Command) error {
    return beginTransaction()
})

parser.AddCommandPostHook("db update", func(p *Parser, cmd *Command, err error) error {
    if err != nil {
        return rollbackTransaction()
    }
    return commitTransaction()
})
```

## Hook Execution Order

You can control the order of global vs command-specific hooks:

```go
// Global hooks run first (default)
parser.SetHookOrder(goopt.OrderGlobalFirst)

// Command hooks run first
parser.SetHookOrder(goopt.OrderCommandFirst)
```

For cleanup, post-hooks run in reverse order of pre-hooks.

## Best Practices

1. **Keep hooks lightweight** - Don't do heavy processing
2. **Handle errors gracefully** - Pre-hook errors prevent execution
3. **Use post-hooks for cleanup** - They run even on failure
4. **Be careful with state** - Hooks share parser state
5. **Document hook behavior** - Make it clear what hooks do