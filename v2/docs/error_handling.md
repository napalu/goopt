# Error Handling in goopt v2

## Creating Arguments with Error Handling

When creating arguments with patterns or other configurations that might fail, goopt v2 provides two approaches:

### Using NewArg (backward compatible)

The `NewArg` function ignores configuration errors for backward compatibility:

```go
// This will create an Argument but ignore any regex compilation errors
arg := goopt.NewArg(
    goopt.WithType(types.Single),
    goopt.WithAcceptedValues([]types.PatternValue{
        {Pattern: "[invalid regex", Description: "This regex is invalid"},
    }),
)

// The argument is created but AcceptedValues[0].Compiled will be nil
// This can lead to runtime issues later
```

### Using NewArgE (recommended)

The `NewArgE` function returns an error if configuration fails:

```go
arg, err := goopt.NewArgE(
    goopt.WithType(types.Single),
    goopt.WithAcceptedValues([]types.PatternValue{
        {Pattern: "[invalid regex", Description: "This regex is invalid"},
    }),
)
if err != nil {
    // Handle the error appropriately
    log.Fatalf("Failed to create argument: %v", err)
}
```

### Using Argument.Set

For more control, you can create an argument and configure it separately:

```go
arg := &goopt.Argument{}
err := arg.Set(
    goopt.WithType(types.Single),
    goopt.WithAcceptedValues([]types.PatternValue{
        {Pattern: `^[0-9]+$`, Description: "Numbers only"},
    }),
)
if err != nil {
    // Handle configuration error
}
```

## Best Practices

1. **Use `NewArgE` for new code** - It provides proper error handling and prevents silent failures
2. **Check errors from `AddFlag`** - The `AddFlag` method returns errors for duplicate flags and other issues
3. **Validate patterns early** - Catch regex compilation errors at initialization rather than runtime

## Example: Safe Argument Creation

```go
parser := goopt.NewParser()

// Create argument with error handling
arg, err := goopt.NewArgE(
    goopt.WithType(types.Single),
    goopt.WithDescription("Output format"),
    goopt.WithAcceptedValues([]types.PatternValue{
        {Pattern: `^(json|xml|yaml)$`, Description: "output.format.desc"},
    }),
    goopt.WithValidators(validation.Required()),
)
if err != nil {
    log.Fatalf("Invalid argument configuration: %v", err)
}

// Add to parser
if err := parser.AddFlag("format", arg); err != nil {
    log.Fatalf("Failed to add flag: %v", err)
}
```

## Migration Guide

To migrate from `NewArg` to `NewArgE`:

1. Replace `NewArg(` with `NewArgE(`
2. Capture the returned error
3. Handle the error appropriately
4. Test with invalid patterns to ensure errors are caught

```go
// Before
arg := goopt.NewArg(configs...)
parser.AddFlag("flag", arg)

// After
arg, err := goopt.NewArgE(configs...)
if err != nil {
    return fmt.Errorf("invalid flag configuration: %w", err)
}
if err := parser.AddFlag("flag", arg); err != nil {
    return fmt.Errorf("failed to add flag: %w", err)
}
```