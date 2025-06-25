package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"

	// Import individual locale packages
	arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"
	esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
	heLocale "github.com/napalu/goopt/v2/i18n/locales/he"
	jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
)

// Config represents our CLI configuration
type Config struct {
	Language string `goopt:"short:l;desc:Language to use;default:en"`
	Version  bool   `goopt:"short:v;desc:Show version"`

	// Example command with validation
	Process struct {
		File   string `goopt:"short:f;desc:File to process;required:true"`
		Format string `goopt:"desc:Output format;default:json;validate:oneof(json,xml,yaml)"`
		Port   int    `goopt:"short:p;desc:Port number;default:8080;validate:port"`
	} `goopt:"kind:command;desc:Process a file"`
}

func main() {
	cfg := &Config{}

	// Create parser with extended locale support
	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithSystemLocales(
			i18n.NewLocale(arLocale.Tag, arLocale.SystemTranslations),
			i18n.NewLocale(esLocale.Tag, esLocale.SystemTranslations),
			i18n.NewLocale(heLocale.Tag, heLocale.SystemTranslations),
			i18n.NewLocale(jaLocale.Tag, jaLocale.SystemTranslations),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse arguments
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Set language if specified
	if cfg.Language != "" && cfg.Language != "en" {
		lang, err := i18n.ParseLanguageTag(cfg.Language)
		if err == nil {
			parser.SetLanguage(lang)
		}
	}

	// Handle version flag
	if cfg.Version {
		fmt.Println("locale-packages-demo v1.0.0")
		fmt.Println("Demonstrates goopt's locale package system")
		return
	}

	// Handle process command
	if parser.HasCommand("process") {
		fmt.Printf("Processing file: %s\n", cfg.Process.File)
		fmt.Printf("Output format: %s\n", cfg.Process.Format)
		fmt.Printf("Using port: %d\n", cfg.Process.Port)
	} else {
		// Show help in selected language
		parser.PrintHelp(os.Stdout)
	}
}
