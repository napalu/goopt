package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

// Example 1: Default auto-help behavior
func example1() {
	fmt.Println("\n=== Example 1: Default Auto-Help ===")

	type Config struct {
		Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
		Config  string `goopt:"short:c;desc:Configuration file path"`
		Workers int    `goopt:"short:w;desc:Number of workers;default:4"`
	}

	cfg := &Config{}
	parser, _ := goopt.NewParserFromStruct(cfg)

	// Simulate: ./app --help
	if parser.Parse([]string{"app", "--help"}) {
		if parser.WasHelpShown() {
			fmt.Println("✓ Auto-help was displayed")
		}
	}
}

// Example 2: Auto-help with different styles
func example2() {
	fmt.Println("\n=== Example 2: Auto-Help with Compact Style ===")

	// Create parser with many flags to trigger compact style
	parser, _ := goopt.NewParserWith(
		goopt.WithHelpStyle(goopt.HelpStyleCompact),
	)

	// Add many flags
	for i := 0; i < 25; i++ {
		parser.AddFlag(fmt.Sprintf("flag%d", i), &goopt.Argument{
			Description: fmt.Sprintf("Flag number %d", i),
			TypeOf:      types.Single,
		})
	}

	// Add commands
	parser.AddCommand(&goopt.Command{
		Name:        "serve",
		Description: "Start the server",
	})
	parser.AddCommand(&goopt.Command{
		Name:        "test",
		Description: "Run tests",
	})

	// Simulate: ./app --help
	parser.Parse([]string{"app", "--help"})
	fmt.Println("✓ Compact help style was used")
}

// Example 3: User-defined help flag
func example3() {
	fmt.Println("\n=== Example 3: User-Defined Help Flag ===")

	type Config struct {
		// User defines their own help flag
		Help    bool `goopt:"name:help;short:?;desc:Show usage information"`
		Verbose bool `goopt:"short:v;desc:Enable verbose output"`
		// User wants to use -h for host
		Host string `goopt:"short:h;desc:Database host;default:localhost"`
	}

	cfg := &Config{}
	parser, _ := goopt.NewParserFromStruct(cfg)

	// Simulate: ./app --help
	parser.Parse([]string{"app", "--help"})

	if cfg.Help {
		fmt.Println("✓ User's custom help flag was set")
		fmt.Println("✓ Auto-help did not trigger (user has control)")
		// User can implement their own help display
		fmt.Println("\nCustom Help:")
		fmt.Println("  This is my custom help implementation")
		fmt.Println("  Use -? for help (not -h which is for host)")
	}

	// Simulate: ./app -h myhost.com
	parser2, _ := goopt.NewParserFromStruct(&Config{})
	parser2.Parse([]string{"app", "-h", "myhost.com"})
	val, _ := parser2.Get("host")
	fmt.Printf("✓ -h flag correctly maps to host: %s\n", val)
}

// Example 4: Disable auto-help
func example4() {
	fmt.Println("\n=== Example 4: Disable Auto-Help ===")

	parser, _ := goopt.NewParserWith(
		goopt.WithAutoHelp(false),
		goopt.WithFlag("verbose", goopt.NewArg(goopt.WithShortFlag("v"))),
	)

	// Simulate: ./app --help
	if !parser.Parse([]string{"app", "--help"}) {
		fmt.Println("✓ Parse failed because --help is not defined")
		fmt.Println("✓ Auto-help is disabled")
	}
}

// Example 5: Custom help flags
func example5() {
	fmt.Println("\n=== Example 5: Custom Help Flags ===")

	parser, _ := goopt.NewParserWith(
		goopt.WithHelpFlags("ayuda", "a"), // Spanish for help
		goopt.WithFlag("verbose", goopt.NewArg(goopt.WithShortFlag("v"))),
	)

	// Simulate: ./app --ayuda
	parser.Parse([]string{"app", "--ayuda"})
	if parser.WasHelpShown() {
		fmt.Println("✓ Custom help flag '--ayuda' triggered auto-help")
	}

	// Simulate: ./app -a
	parser2, _ := goopt.NewParserWith(
		goopt.WithHelpFlags("ayuda", "a"),
		goopt.WithFlag("verbose", goopt.NewArg(goopt.WithShortFlag("v"))),
	)
	parser2.Parse([]string{"app", "-a"})
	if parser2.WasHelpShown() {
		fmt.Println("✓ Custom short help flag '-a' triggered auto-help")
	}
}

// Example 6: Complex CLI with hierarchical help
func example6() {
	fmt.Println("\n=== Example 6: Complex CLI with Hierarchical Help ===")

	type ComplexApp struct {
		Global string `goopt:"short:g;desc:Global setting"`

		Kubernetes struct {
			Config string `goopt:"short:c;desc:Kubeconfig file"`

			Pod struct {
				Create struct {
					Name  string `goopt:"short:n;desc:Pod name;required:true"`
					Image string `goopt:"short:i;desc:Container image;required:true"`
				} `goopt:"kind:command;desc:Create a new pod"`

				Delete struct {
					Name string `goopt:"short:n;desc:Pod name;required:true"`
				} `goopt:"kind:command;desc:Delete a pod"`
			} `goopt:"kind:command;desc:Manage pods"`

			Service struct {
				Create struct {
					Name string `goopt:"short:n;desc:Service name;required:true"`
					Port int    `goopt:"short:p;desc:Service port;default:80"`
				} `goopt:"kind:command;desc:Create a new service"`
			} `goopt:"kind:command;desc:Manage services"`
		} `goopt:"kind:command;name:k8s;desc:Kubernetes operations"`

		Docker struct {
			Build struct {
				Tag string `goopt:"short:t;desc:Image tag;required:true"`
			} `goopt:"kind:command;desc:Build container image"`
		} `goopt:"kind:command;desc:Docker operations"`
	}

	cfg := &ComplexApp{}
	parser, _ := goopt.NewParserFromStruct(cfg,
		goopt.WithHelpStyle(goopt.HelpStyleHierarchical),
	)

	// Simulate: ./app --help
	fmt.Println("\nShowing hierarchical help for complex CLI:")
	parser.Parse([]string{"app", "--help"})
	fmt.Println("✓ Hierarchical style shows command structure")
}

func main() {
	if len(os.Args) > 1 {
		// If running with arguments, use as a real CLI
		runAsCLI()
		return
	}

	// Run all examples
	fmt.Println("Auto-Help Feature Examples")
	fmt.Println("==========================")

	// Redirect stdout for examples to capture help output
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	example1()
	example2()
	example3()
	example4()
	example5()
	example6()

	fmt.Println("\n\nAll examples completed!")
	fmt.Println("\nTry running with arguments to see real help:")
	fmt.Println("  go run main.go --help")
	fmt.Println("  go run main.go db --help")
}

// Run as a real CLI when arguments are provided
func runAsCLI() {
	type Config struct {
		// Global flags
		Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
		Config  string `goopt:"short:c;desc:Configuration file path"`

		// Database command
		Database struct {
			Host string `goopt:"desc:Database host;default:localhost"`
			Port int    `goopt:"desc:Database port;default:5432"`

			Backup struct {
				Output string `goopt:"short:o;desc:Backup output file;required:true"`
			} `goopt:"kind:command;desc:Create database backup"`

			Restore struct {
				Input string `goopt:"short:i;desc:Backup input file;required:true"`
			} `goopt:"kind:command;desc:Restore database from backup"`
		} `goopt:"kind:command;name:db;desc:Database operations"`
	}

	cfg := &Config{}

	// Let user choose help style via environment variable
	helpStyle := goopt.HelpStyleSmart
	if style := os.Getenv("HELP_STYLE"); style != "" {
		switch strings.ToLower(style) {
		case "flat":
			helpStyle = goopt.HelpStyleFlat
		case "grouped":
			helpStyle = goopt.HelpStyleGrouped
		case "compact":
			helpStyle = goopt.HelpStyleCompact
		case "hierarchical":
			helpStyle = goopt.HelpStyleHierarchical
		}
	}

	// Create parser with chosen help style
	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithHelpStyle(helpStyle),
		goopt.WithFlagNameConverter(goopt.ToKebabCase),
		goopt.WithCommandNameConverter(goopt.ToKebabCase),
		goopt.WithEnvNameConverter(goopt.ToKebabCase),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse command line
	if !parser.Parse(os.Args) {
		// If parsing failed, show errors
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "Error: %v\n", e)
		}
		os.Exit(1)
	}

	// Check if help was shown automatically
	if parser.WasHelpShown() {
		// Help was displayed, exit cleanly
		os.Exit(0)
	}

	// Normal program execution
	fmt.Println("Program executing normally...")

	if parser.HasCommand("db backup") {
		fmt.Printf("Backing up database from %s:%d to %s\n",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Backup.Output)
	} else if parser.HasCommand("db restore") {
		fmt.Printf("Restoring database to %s:%d from %s\n",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Restore.Input)
	}
}
