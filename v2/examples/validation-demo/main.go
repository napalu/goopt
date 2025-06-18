package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/validation"
)

func main() {
	// Create parser with various validated flags
	parser, err := goopt.NewParserWith(
		// Email validation
		goopt.WithFlag("email", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("User email address"),
			goopt.WithRequired(true),
			goopt.WithValidator(validation.Email()),
		)),

		// Port validation
		goopt.WithFlag("port", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithShortFlag("p"),
			goopt.WithDescription("Server port (1-65535)"),
			goopt.WithDefaultValue("8080"),
			goopt.WithValidator(validation.Port()),
		)),

		// URL validation with specific schemes
		goopt.WithFlag("webhook", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("Webhook URL (http/https only)"),
			goopt.WithValidator(validation.URL("http", "https")),
		)),

		// Username with multiple validators
		goopt.WithFlag("username", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithShortFlag("u"),
			goopt.WithDescription("Username (3-20 chars, alphanumeric)"),
			goopt.WithRequired(true),
			goopt.WithValidators(
				validation.MinLength(3),
				validation.MaxLength(20),
				validation.AlphaNumeric(),
			),
		)),

		// Password with complex validation
		goopt.WithFlag("password", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("Password (min 8 chars, must contain uppercase, lowercase, and digit)"),
			goopt.WithSecurePrompt("Enter password: "),
			goopt.WithValidator(validation.All(
				validation.MinLength(8),
				validation.Regex(`[A-Z]`, "Must contain uppercase"), // At least one uppercase
				validation.Regex(`[a-z]`, "Must contain lowercase"), // At least one lowercase
				validation.Regex(`[0-9]`, "Must contain digit"),     // At least one digit
			)),
		)),

		// Age validation
		goopt.WithFlag("age", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("User age (18-100)"),
			goopt.WithValidators(
				validation.Integer(),
				validation.Range(18, 100),
			),
		)),

		// Config file with extension validation
		goopt.WithFlag("config", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithShortFlag("c"),
			goopt.WithDescription("Configuration file (.json, .yaml, or .toml)"),
			goopt.WithValidator(validation.FileExtension(".json", ".yaml", ".yml", ".toml")),
		)),

		// Host validation
		goopt.WithFlag("host", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("Server hostname"),
			goopt.WithDefaultValue("localhost"),
			goopt.WithValidator(validation.Hostname()),
		)),

		// IP address validation
		goopt.WithFlag("bind", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("Bind IP address"),
			goopt.WithDefaultValue("0.0.0.0"),
			goopt.WithValidator(validation.IP()),
		)),

		// Custom validation example
		goopt.WithFlag("priority", goopt.NewArg(
			goopt.WithType(types.Single),
			goopt.WithDescription("Priority level (low, medium, high)"),
			goopt.WithValidator(validation.IsOneOf("low", "medium", "high")),
		)),

		// Enable auto-help
		goopt.WithAutoHelp(true),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse command line arguments
	if !parser.Parse(os.Args) {
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "%v\n", e)
		}
		fmt.Fprintln(os.Stderr, "")
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// If we get here, all validations passed
	fmt.Println("All validations passed!")
	fmt.Println("\nConfiguration:")

	// Display validated values
	if email, _ := parser.Get("email"); email != "" {
		fmt.Printf("  Email: %s\n", email)
	}

	if username, _ := parser.Get("username"); username != "" {
		fmt.Printf("  Username: %s\n", username)
	}

	if port, _ := parser.Get("port"); port != "" {
		fmt.Printf("  Port: %s\n", port)
	}

	if webhook, _ := parser.Get("webhook"); webhook != "" {
		fmt.Printf("  Webhook URL: %s\n", webhook)
	}

	if age, _ := parser.Get("age"); age != "" {
		fmt.Printf("  Age: %s\n", age)
	}

	if config, _ := parser.Get("config"); config != "" {
		fmt.Printf("  Config file: %s\n", config)
	}

	if host, _ := parser.Get("host"); host != "" {
		fmt.Printf("  Host: %s\n", host)
	}

	if bind, _ := parser.Get("bind"); bind != "" {
		fmt.Printf("  Bind address: %s\n", bind)
	}

	if priority, _ := parser.Get("priority"); priority != "" {
		fmt.Printf("  Priority: %s\n", priority)
	}

	// Password is secure, so we just confirm it was set
	if parser.HasFlag("password") {
		fmt.Println("  Password: [SET]")
	}
}
