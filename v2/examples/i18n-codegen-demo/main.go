package main

import (
	"fmt"
	"log"
	"os"

	"github.com/napalu/goopt/v2"
)

// AppConfig demonstrates the goopt-i18n-gen workflow
// Step 1: Define your configuration struct with goopt tags but WITHOUT descKey tags
// Step 2: Run goopt-i18n-gen to generate descKey suggestions and translations
// Step 3: Add the suggested descKey tags to your struct
// Step 4: Generate the translation constants with goopt-i18n-gen
type AppConfig struct {
	// Global flags - no descKey tags yet!
	Help    bool   `goopt:"short:h;desc:Display help information"`
	Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
	Output  string `goopt:"short:o;desc:Output file path"`
	Workers int    `goopt:"short:w;desc:Number of worker threads;default:4"`

	// Commands - also no descKey tags
	Process struct {
		InputFile string `goopt:"short:i;desc:Input file to process;required:true"`
		Format    string `goopt:"short:f;desc:Output format (json, xml, csv);default:json"`
		Compress  bool   `goopt:"short:c;desc:Compress output"`
		Exec      goopt.CommandFunc

		// Nested subcommand
		Validate struct {
			Strict bool   `goopt:"short:s;desc:Enable strict validation"`
			Schema string `goopt:"desc:Schema file for validation"`
			Exec   goopt.CommandFunc
		} `goopt:"kind:command;name:validate;desc:Validate the processed data"`
	} `goopt:"kind:command;name:process;desc:Process input files"`

	Convert struct {
		From string `goopt:"short:f;desc:Source format;required:true"`
		To   string `goopt:"short:t;desc:Target format;required:true"`
		Exec goopt.CommandFunc
	} `goopt:"kind:command;name:convert;desc:Convert between formats"`
}

// 360° Workflow go:generate directives:

// Option 1: FULLY AUTOMATED - analyze and auto-update source files
//go:generate ../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json -o messages/messages.go -p messages -s main.go -d -g -u --key-prefix app

// Option 2: MANUAL - just show suggestions
//go:generate ../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json -o messages/messages.go -p messages -s main.go -d -g --key-prefix app

// Step 2: Validate all descKeys have translations (for development)
//go:generate ../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json -o messages/messages.go -p messages -s main.go -v

// Step 3: Final generation - just generate the constants file
//go:generate ../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json -o messages/messages.go -p messages

// Step 4: CI/CD validation - ensure all descKeys have translations (strict mode)
//go:generate ../../cmd/goopt-i18n-gen/goopt-i18n-gen -i locales/en.json -o messages/messages.go -p messages -s main.go -v --strict

func main() {
	cfg := AppConfig{}
	// Process is non-terminal (has subcommands), so no Exec
	cfg.Process.Validate.Exec = validateCmd
	cfg.Convert.Exec = convertCmd

	// Parse command line
	parser, err := goopt.NewParserFromStruct(&cfg)
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	if !parser.Parse(os.Args) {
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "Error: %v\n", e)
		}
		parser.PrintUsageWithGroups(os.Stderr)
		os.Exit(1)
	}

	// Handle help
	if cfg.Help {
		parser.PrintUsageWithGroups(os.Stdout)
		os.Exit(0)
	}

	// Execute commands
	errCount := parser.ExecuteCommands()
	if errCount > 0 {
		for _, cmdErr := range parser.GetCommandExecutionErrors() {
			fmt.Fprintf(os.Stderr, "Command %s failed: %v\n", cmdErr.Key, cmdErr.Value)
		}
		os.Exit(1)
	}
}

func validateCmd(parser *goopt.Parser, _ *goopt.Command) error {
	config, ok := goopt.GetStructCtxAs[*AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println("Validating processed data...")
	if config.Process.Validate.Strict {
		fmt.Println("Strict validation enabled")
	}
	if config.Process.Validate.Schema != "" {
		fmt.Printf("Using schema: %s\n", config.Process.Validate.Schema)
	}
	return nil
}

func convertCmd(parser *goopt.Parser, _ *goopt.Command) error {
	config, ok := goopt.GetStructCtxAs[*AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Printf("Converting from %s to %s\n", config.Convert.From, config.Convert.To)
	return nil
}
