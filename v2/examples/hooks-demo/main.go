package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/napalu/goopt/v2"
)

// Simple in-memory auth for demo
var (
	authenticated = false
	authToken     = ""
)

func main() {
	if len(os.Args) == 1 {
		showExamples()
		return
	}

	runCLI()
}

func showExamples() {
	fmt.Println("Execution Hooks Examples")
	fmt.Println("========================\n")

	example1Logging()
	example2Authentication()
	example3Cleanup()
	example4Metrics()
	example5CommandSpecific()

	fmt.Println("\nTry running with real arguments:")
	fmt.Println("  go run main.go login")
	fmt.Println("  go run main.go protected-command")
	fmt.Println("  go run main.go db backup --output=backup.sql")
}

func example1Logging() {
	fmt.Println("Example 1: Logging Hooks")
	fmt.Println("------------------------")

	parser := goopt.NewParser()

	// Add logging hooks
	parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
		log.Printf("[START] Executing command: %s", c.Path())
		return nil
	})

	parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
		if err != nil {
			log.Printf("[ERROR] Command %s failed: %v", c.Path(), err)
		} else {
			log.Printf("[SUCCESS] Command %s completed", c.Path())
		}
		return nil
	})

	// Add test command
	parser.AddCommand(&goopt.Command{
		Name:        "test",
		Description: "Test command",
		Callback: func(p *goopt.Parser, c *goopt.Command) error {
			fmt.Println("  Executing test command...")
			return nil
		},
	})

	parser.Parse([]string{"test"})
	parser.ExecuteCommands()
	fmt.Println()
}

func example2Authentication() {
	fmt.Println("Example 2: Authentication Hooks")
	fmt.Println("-------------------------------")

	parser := goopt.NewParser()

	// Add authentication check
	parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
		// Skip auth for login command
		if c.Name == "login" {
			return nil
		}

		if !authenticated {
			return errors.New("authentication required - please login first")
		}

		fmt.Printf("  ✓ Authenticated as: %s\n", authToken)
		return nil
	})

	// Login command
	parser.AddCommand(&goopt.Command{
		Name:        "login",
		Description: "Login to the system",
		Callback: func(p *goopt.Parser, c *goopt.Command) error {
			authenticated = true
			authToken = "user@example.com"
			fmt.Println("  Login successful!")
			return nil
		},
	})

	// Protected command
	parser.AddCommand(&goopt.Command{
		Name:        "protected",
		Description: "A protected command",
		Callback: func(p *goopt.Parser, c *goopt.Command) error {
			fmt.Println("  Executing protected command...")
			return nil
		},
	})

	// Try protected without auth
	fmt.Println("Attempting protected command without auth:")
	parser.Parse([]string{"protected"})
	if errs := parser.ExecuteCommands(); errs > 0 {
		fmt.Println("  ✗ Command failed (expected)")
	}

	// Login
	fmt.Println("\nLogging in:")
	parser.Parse([]string{"login"})
	parser.ExecuteCommands()

	// Try protected with auth
	fmt.Println("\nAttempting protected command after auth:")
	parser.Parse([]string{"protected"})
	parser.ExecuteCommands()

	// Reset for other examples
	authenticated = false
	fmt.Println()
}

func example3Cleanup() {
	fmt.Println("Example 3: Cleanup Hooks")
	fmt.Println("------------------------")

	var tempFiles []string

	parser := goopt.NewParser()

	// Add cleanup hook that always runs
	parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
		if len(tempFiles) > 0 {
			fmt.Printf("  Cleaning up %d temporary files...\n", len(tempFiles))
			// In real app, would delete files here
			tempFiles = []string{}
		}
		return nil
	})

	// Command that creates temp files
	parser.AddCommand(&goopt.Command{
		Name:        "process",
		Description: "Process data",
		Callback: func(p *goopt.Parser, c *goopt.Command) error {
			fmt.Println("  Creating temporary files...")
			tempFiles = append(tempFiles, "temp1.dat", "temp2.dat", "temp3.dat")

			// Simulate failure
			return errors.New("processing failed")
		},
	})

	parser.Parse([]string{"process"})
	parser.ExecuteCommands()

	fmt.Println("  ✓ Cleanup completed even though command failed")
	fmt.Println()
}

func example4Metrics() {
	fmt.Println("Example 4: Metrics Collection")
	fmt.Println("-----------------------------")

	type metric struct {
		command  string
		duration time.Duration
		success  bool
	}

	var metrics []metric
	startTimes := make(map[string]time.Time)

	parser := goopt.NewParser()

	// Pre-hook: record start time
	parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
		startTimes[c.Path()] = time.Now()
		return nil
	})

	// Post-hook: record metrics
	parser.AddGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
		duration := time.Since(startTimes[c.Path()])
		metrics = append(metrics, metric{
			command:  c.Path(),
			duration: duration,
			success:  err == nil,
		})
		delete(startTimes, c.Path())
		return nil
	})

	// Add some commands
	parser.AddCommand(&goopt.Command{
		Name: "fast",
		Callback: func(p *goopt.Parser, c *goopt.Command) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	})

	parser.AddCommand(&goopt.Command{
		Name: "slow",
		Callback: func(p *goopt.Parser, c *goopt.Command) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	})

	// Execute commands
	parser.Parse([]string{"fast"})
	parser.ExecuteCommands()

	parser.Parse([]string{"slow"})
	parser.ExecuteCommands()

	// Show metrics
	fmt.Println("  Command Metrics:")
	for _, m := range metrics {
		status := "✓"
		if !m.success {
			status = "✗"
		}
		fmt.Printf("    %s %s: %v\n", status, m.command, m.duration)
	}
	fmt.Println()
}

func example5CommandSpecific() {
	fmt.Println("Example 5: Command-Specific Hooks")
	fmt.Println("---------------------------------")

	parser := goopt.NewParser()

	// Database command group
	dbCmd := &goopt.Command{
		Name:        "db",
		Description: "Database operations",
		Subcommands: []goopt.Command{
			{
				Name:        "backup",
				Description: "Backup database",
				Callback: func(p *goopt.Parser, c *goopt.Command) error {
					fmt.Println("    Creating backup...")
					return nil
				},
			},
			{
				Name:        "restore",
				Description: "Restore database",
				Callback: func(p *goopt.Parser, c *goopt.Command) error {
					fmt.Println("    Restoring database...")
					return nil
				},
			},
		},
	}
	parser.AddCommand(dbCmd)

	// Add specific hooks for backup
	parser.AddCommandPreHook("db backup", func(p *goopt.Parser, c *goopt.Command) error {
		fmt.Println("  [backup] Checking disk space...")
		fmt.Println("  [backup] Acquiring database lock...")
		return nil
	})

	parser.AddCommandPostHook("db backup", func(p *goopt.Parser, c *goopt.Command, err error) error {
		fmt.Println("  [backup] Releasing database lock...")
		if err == nil {
			fmt.Println("  [backup] Backup completed successfully")
		}
		return nil
	})

	// Add specific hooks for restore
	parser.AddCommandPreHook("db restore", func(p *goopt.Parser, c *goopt.Command) error {
		fmt.Println("  [restore] Backing up current database...")
		fmt.Println("  [restore] Validating restore file...")
		return nil
	})

	// Execute backup
	fmt.Println("Executing backup:")
	parser.Parse([]string{"db", "backup"})
	parser.ExecuteCommands()

	fmt.Println("\nExecuting restore (no specific post-hook):")
	parser.Parse([]string{"db", "restore"})
	parser.ExecuteCommands()
	fmt.Println()
}

func runCLI() {
	type Config struct {
		Verbose bool `goopt:"short:v;desc:Enable verbose output"`

		// Authentication
		Login struct {
			Username string `goopt:"short:u;desc:Username;required:true"`
			Exec     goopt.CommandFunc
		} `goopt:"kind:command;desc:Login to the system"`

		Logout struct {
			Exec goopt.CommandFunc
		} `goopt:"kind:command;desc:Logout from the system"`

		// Database operations
		DB struct {
			Backup struct {
				Output   string `goopt:"short:o;desc:Output file;required:true"`
				Compress bool   `goopt:"short:c;desc:Compress backup"`
				Exec     goopt.CommandFunc
			} `goopt:"kind:command;desc:Backup database"`

			Restore struct {
				Input string `goopt:"short:i;desc:Input file;required:true"`
				Exec  goopt.CommandFunc
			} `goopt:"kind:command;desc:Restore database"`

			Status struct {
				Exec goopt.CommandFunc
			} `goopt:"kind:command;desc:Show database status"`
		} `goopt:"kind:command;desc:Database operations"`

		// Server operations
		Server struct {
			Start struct {
				Port    int `goopt:"short:p;default:8080;desc:Server port"`
				Workers int `goopt:"short:w;default:4;desc:Number of workers"`
				Exec    goopt.CommandFunc
			} `goopt:"kind:command;desc:Start server"`

			Stop struct {
				Force bool `goopt:"short:f;desc:Force stop"`
				Exec  goopt.CommandFunc
			} `goopt:"kind:command;desc:Stop server"`

			Status struct {
				Exec goopt.CommandFunc
			} `goopt:"kind:command;desc:Show server status"`
		} `goopt:"kind:command;desc:Server operations"`
	}

	cfg := &Config{}

	// Create parser with hooks
	parser, err := goopt.NewParserFromStruct(cfg,
		// Global logging
		goopt.WithGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
			if cfg.Verbose {
				log.Printf("[VERBOSE] Starting: %s", c.Path())
			}
			return nil
		}),
		goopt.WithGlobalPostHook(func(p *goopt.Parser, c *goopt.Command, err error) error {
			if cfg.Verbose {
				if err != nil {
					log.Printf("[VERBOSE] Failed: %s - %v", c.Path(), err)
				} else {
					log.Printf("[VERBOSE] Completed: %s", c.Path())
				}
			}
			return nil
		}),
		// Command-specific hooks
		goopt.WithCommandPreHook("db backup", func(p *goopt.Parser, c *goopt.Command) error {
			fmt.Println("Preparing database backup...")
			return nil
		}),
		goopt.WithCommandPostHook("db backup", func(p *goopt.Parser, c *goopt.Command, err error) error {
			if err == nil {
				fmt.Printf("Backup saved to: %s\n", cfg.DB.Backup.Output)
			}
			return nil
		}),
		goopt.WithHookOrder(goopt.OrderGlobalFirst),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Global authentication hook (except for login)
	parser.AddGlobalPreHook(func(p *goopt.Parser, c *goopt.Command) error {
		// Skip auth for login command
		if strings.HasPrefix(c.Path(), "login") {
			return nil
		}

		// Check if user is authenticated
		if authToken == "" {
			// Check env var as fallback
			if token := os.Getenv("AUTH_TOKEN"); token != "" {
				authToken = token
				return nil
			}
			return errors.New("not authenticated - please login first or set AUTH_TOKEN")
		}
		return nil
	})

	// Set command callbacks
	cfg.Login.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		authToken = cfg.Login.Username
		fmt.Printf("Logged in as: %s\n", authToken)
		fmt.Println("(In a real app, this would validate credentials)")
		return nil
	}

	cfg.Logout.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Printf("Logged out user: %s\n", authToken)
		authToken = ""
		return nil
	}

	cfg.DB.Backup.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Printf("Backing up database to %s", cfg.DB.Backup.Output)
		if cfg.DB.Backup.Compress {
			fmt.Print(" (compressed)")
		}
		fmt.Println("...")
		// Simulate backup
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	cfg.DB.Restore.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Printf("Restoring database from %s...\n", cfg.DB.Restore.Input)
		// Simulate restore
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	cfg.DB.Status.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Println("Database Status:")
		fmt.Println("  Connection: Active")
		fmt.Println("  Size: 1.2GB")
		fmt.Println("  Tables: 42")
		return nil
	}

	cfg.Server.Start.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Printf("Starting server on port %d with %d workers...\n",
			cfg.Server.Start.Port, cfg.Server.Start.Workers)
		return nil
	}

	cfg.Server.Stop.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		if cfg.Server.Stop.Force {
			fmt.Println("Force stopping server...")
		} else {
			fmt.Println("Gracefully stopping server...")
		}
		return nil
	}

	cfg.Server.Status.Exec = func(cmdLine *goopt.Parser, command *goopt.Command) error {
		fmt.Println("Server Status:")
		fmt.Println("  Status: Running")
		fmt.Println("  Uptime: 2h 34m")
		fmt.Println("  Requests: 12,345")
		return nil
	}

	// Parse and execute
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Execute commands
	if errs := parser.ExecuteCommands(); errs > 0 {
		os.Exit(1)
	}
}
