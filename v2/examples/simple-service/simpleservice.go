package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
)

// Config demonstrates hierarchical flag definition and inheritance.
type Config struct {
	// --- Root Level Flag ---
	// Defined at the top level, available to all commands unless overridden.
	LogLevel string `goopt:"name:log-level;short:l;default:INFO;desc:Global log level"`

	// --- Command Structure ---
	// 'App' serves as a container, its fields are not flags/commands themselves.
	App struct {
		// --- 'Service' Command ---
		Service struct {
			// --- 'Service' Level Flag ---
			// Specific to the 'service' command and its subcommands.
			Port int `goopt:"short:p;default:8080;desc:Service port"`

			// --- 'Start' Subcommand ---
			Start struct {
				// --- 'Start' Level Flag (Overrides Parent) ---
				// Overrides 'log-level' specifically for 'start'.
				LogLevel string `goopt:"name:log-level;short:ll;default:DEBUG;desc:Log level for start command"`
				// Specific flag for 'start'
				Workers int `goopt:"short:w;default:4;desc:Number of workers"`
			} `goopt:"kind:command;name:start;desc:Start the service"`

			// --- 'Stop' Subcommand ---
			Stop struct {
				// Inherits 'log-level' from the root level (INFO).
				// Inherits 'port' from the 'service' level (8080).
				Force bool `goopt:"short:f;desc:Force stop"`
			} `goopt:"kind:command;desc:Stop the service"`
		} `goopt:"kind:command;desc:Manage the service"`
	}
}

func main() {
	cfg := &Config{}
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing parser: %v\n", err)
		os.Exit(1)
	}

	// --- Example Command Lines ---
	// 1. ./simple-service service start                 								(LogLevel=DEBUG, Port=8080, Workers=4)
	// 2. ./simple-service stop --app.force              								(LogLevel=INFO, Port=8080, Force=true)
	// 3. ./simple-service -l WARN service start -p 9090 								(LogLevel=DEBUG, Port=9090, Workers=4) <- Global -l WARN ignored for start
	// 4.  ./simple-service service start -ll WARN -l DEBUG  -p 9090 --app.workers 255	(LogLevel=WARN, Port=9090, Workers=255) <- Global -l DEBUG used for start
	// 5. ./simple-service -l WARN service stop          								(LogLevel=WARN, Port=8080, Force=false) <- Global -l WARN used for stop

	if !parser.Parse(os.Args) {
		fmt.Fprintln(os.Stderr, "Error: Invalid command-line arguments.")
		for _, parseErr := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, " - %s\n", parseErr)
		}
		fmt.Fprintln(os.Stderr, "")
		parser.PrintUsageWithGroups(os.Stdout)
		os.Exit(1)
	}

	// --- Accessing Values (Rely on bound struct fields) ---

	if parser.HasCommand("service start") {
		fmt.Println("Executing 'service start'...")
		// Access fields directly. Goopt ensures the correct values based on hierarchy are bound.
		fmt.Printf("  Log Level (Specific to Start): %s\n", cfg.App.Service.Start.LogLevel) // Should be DEBUG or overridden
		fmt.Printf("  Port (Inherited from Service): %d\n", cfg.App.Service.Port)           // Should be 8080 or overridden
		fmt.Printf("  Workers (Specific to Start):   %d\n", cfg.App.Service.Start.Workers)  // Should be 4 or overridden
		fmt.Printf("  Global LogLevel (Not used directly here): %s\n", cfg.LogLevel)        // Shows the global value if set

	} else if parser.HasCommand("service stop") {
		fmt.Println("Executing 'service stop'...")
		// Access inherited LogLevel from the root 'Config' struct
		fmt.Printf("  Log Level (Inherited from Root): %s\n", cfg.LogLevel) // Should be INFO or overridden globally
		// Access inherited Port from the parent 'Service' struct
		fmt.Printf("  Port (Inherited from Service):   %d\n", cfg.App.Service.Port) // Should be 8080 or overridden
		// Access flag specific to Stop
		fmt.Printf("  Force Stop:                      %t\n", cfg.App.Service.Stop.Force)

	} else {
		fmt.Println("No specific service command executed.")
		// Example: Check global log level if set directly
		if parser.HasRawFlag("log-level") { // Check if --log-level was explicitly passed globally
			fmt.Printf("Global Log Level explicitly set to: %s\n", cfg.LogLevel)
		}
	}
}
