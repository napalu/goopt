package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	deLocale "github.com/napalu/goopt/v2/i18n/locales/de"
	frLocale "github.com/napalu/goopt/v2/i18n/locales/fr"
	"golang.org/x/text/language"
)

type Config struct {
	Language string `goopt:"short:l;long:lang;default:en;desc:Interface language (en, de, fr)"`
	Verbose  bool   `goopt:"short:v;desc:Verbose output"`
	Port     int    `goopt:"short:p;default:8080;desc:Port number"`
	File     string `goopt:"short:f;required;desc:Input file"`
}

func main() {
	// Create config
	cfg := &Config{}

	// First pass - parse just to get the language preference
	tempParser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}
	tempParser.Parse(os.Args)

	// Parse the language
	langTag, err := language.Parse(cfg.Language)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing language '%s': %v\n", cfg.Language, err)
		os.Exit(1)
	}

	// Create parser with system locales
	// Note: System locales must be complete - they need all goopt system messages
	var opts []goopt.ConfigureCmdLineFunc

	switch langTag {
	case language.German:
		opts = append(opts, goopt.WithSystemLocales(i18n.NewLocale(deLocale.Tag, deLocale.SystemTranslations)))
	case language.French:
		opts = append(opts, goopt.WithSystemLocales(i18n.NewLocale(frLocale.Tag, frLocale.SystemTranslations)))
	}

	// Create the real parser
	parser, err := goopt.NewParserFromStruct(cfg, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Set the language
	parser.SetLanguage(langTag)

	// Parse with the correct language
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Check if help was requested
	if parser.WasHelpShown() {
		os.Exit(0)
	}

	// Get translator for user messages
	translator := parser.GetTranslator()

	// Show configuration
	fmt.Printf("%s\n", translator.T("goopt.msg.flags_header"))
	fmt.Printf("- %s: %s\n", translator.T("goopt.msg.flags"), cfg.Language)
	fmt.Printf("- Port: %d\n", cfg.Port)
	fmt.Printf("- File: %s\n", cfg.File)
	if cfg.Verbose {
		fmt.Printf("- Verbose: %v\n", cfg.Verbose)
	}
}
