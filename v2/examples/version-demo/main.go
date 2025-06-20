package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/napalu/goopt/v2"
)

// Build-time variables (set with ldflags)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {

	if len(os.Args) == 1 {
		// Show examples when run without arguments
		showExamples()
		return
	}

	// Create a real CLI with version support
	runCLI()
}

func showExamples() {
	fmt.Println("Version Support Examples")
	fmt.Println("========================\n")

	example1SimpleVersion()
	example2DynamicVersion()
	example3CustomFormatter()
	example4VersionInHelp()
	example5CustomFlags()

	fmt.Println("\nTry running with arguments:")
	fmt.Println("  go run main.go --version")
	fmt.Println("  go run main.go --help")
}

func example1SimpleVersion() {
	fmt.Println("Example 1: Simple Static Version")
	fmt.Println("--------------------------------")

	parser, _ := goopt.NewParserWith(
		goopt.WithVersion("1.2.3"),
	)

	// Simulate: ./app --version
	parser.Parse([]string{"app", "--version"})

	if parser.WasVersionShown() {
		fmt.Println("✓ Version was displayed\n")
	}
}

func example2DynamicVersion() {
	fmt.Println("Example 2: Dynamic Version with Build Info")
	fmt.Println("------------------------------------------")

	// Simulate build variables
	version := "2.0.0"
	commit := "abc123def"
	buildTime := time.Now().Format(time.RFC3339)

	parser, _ := goopt.NewParserWith(
		goopt.WithVersionFunc(func() string {
			return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime)
		}),
	)

	parser.Parse([]string{"app", "--version"})
	fmt.Println()
}

func example3CustomFormatter() {
	fmt.Println("Example 3: Custom Version Formatter")
	fmt.Println("-----------------------------------")

	parser, _ := goopt.NewParserWith(
		goopt.WithVersion("3.0.0"),
		goopt.WithVersionFormatter(func(version string) string {
			return fmt.Sprintf(`MyAwesomeApp v%s

Copyright (c) 2024 MyCompany, Inc.
License: MIT
Homepage: https://github.com/mycompany/myapp

Built with: %s
Platform: %s/%s`,
				version,
				runtime.Version(),
				runtime.GOOS,
				runtime.GOARCH)
		}),
	)

	parser.Parse([]string{"app", "--version"})
	fmt.Println()
}

func example4VersionInHelp() {
	fmt.Println("Example 4: Show Version in Help Header")
	fmt.Println("--------------------------------------")

	parser, _ := goopt.NewParserWith(
		goopt.WithVersion("4.0.0"),
		goopt.WithShowVersionInHelp(true),
		goopt.WithFlag("config", goopt.NewArg(
			goopt.WithShortFlag("c"),
			goopt.WithDescription("Configuration file"),
		)),
	)

	fmt.Println("When --help is used, version appears at the top:")
	parser.Parse([]string{"app", "--help"})
	fmt.Println()
}

func example5CustomFlags() {
	fmt.Println("Example 5: Custom Version Flags")
	fmt.Println("-------------------------------")

	parser, _ := goopt.NewParserWith(
		goopt.WithVersion("5.0.0"),
		goopt.WithVersionFlags("ver", "V"), // Use --ver and -V (capital)
	)

	fmt.Println("Using custom flag --ver:")
	parser.Parse([]string{"app", "--ver"})

	fmt.Println("\nUsing custom short flag -V:")
	parser2, _ := goopt.NewParserWith(
		goopt.WithVersion("5.0.0"),
		goopt.WithVersionFlags("ver", "V"),
	)
	parser2.Parse([]string{"app", "-V"})
	fmt.Println()
}

func runCLI() {
	// Example of a real CLI with comprehensive version support
	type Config struct {
		// Global options
		Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
		Config  string `goopt:"short:c;desc:Configuration file"`

		// Commands
		Server struct {
			Port int `goopt:"short:p;default:8080;desc:Server port"`

			Start struct {
				Workers int `goopt:"short:w;default:4;desc:Number of workers"`
				Exec    goopt.CommandFunc
			} `goopt:"kind:command;desc:Start the server"`

			Stop struct {
				Exec goopt.CommandFunc
			} `goopt:"kind:command;desc:Stop the server"`
		} `goopt:"kind:command;desc:Server management"`

		Build struct {
			Output string `goopt:"short:o;default:./build;desc:Output directory"`
			Clean  bool   `goopt:"desc:Clean before building"`
			Exec   goopt.CommandFunc
		} `goopt:"kind:command;desc:Build the project"`
	}

	cfg := &Config{}

	// Create parser with dynamic version info
	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithVersionFunc(func() string {
			// In a real app, these would be set via ldflags:
			// go build -ldflags "-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)"
			if Version == "dev" {
				return fmt.Sprintf("dev (commit: %s, built: %s, go: %s)",
					GitCommit,
					BuildTime,
					runtime.Version())
			}
			return fmt.Sprintf("%s (commit: %s, built: %s)",
				Version,
				GitCommit,
				BuildTime)
		}),
		goopt.WithVersionFormatter(func(version string) string {
			return fmt.Sprintf(`myapp %s

A demo application showing goopt v2 version support.
https://github.com/napalu/goopt

%s`, version, version)
		}),
		goopt.WithShowVersionInHelp(true),
		// Keep -v for verbose since that's common
		goopt.WithVersionFlags("version"), // Only long flag
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Set command callbacks
	cfg.Server.Start.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Printf("Starting server on port %d with %d workers...\n",
			cfg.Server.Port, cfg.Server.Start.Workers)
		return nil
	}

	cfg.Server.Stop.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Println("Stopping server...")
		return nil
	}

	cfg.Build.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		if cfg.Build.Clean {
			fmt.Println("Cleaning build directory...")
		}
		fmt.Printf("Building to %s...\n", cfg.Build.Output)
		return nil
	}

	commandExecuted := false
	parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		fmt.Println(c.Path(), "completed")
		commandExecuted = true

		return nil
	})

	// Parse and execute
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Execute commands if any
	if countErr := parser.ExecuteCommands(); countErr > 0 {
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "Error: %v\n", e)
		}
		os.Exit(1)
	} else if !commandExecuted && !parser.WasVersionShown() && !parser.WasHelpShown() {
		// No command executed and no version/help shown
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}
}
