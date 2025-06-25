package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
	"golang.org/x/text/language"
)

type Config struct {
	Language string `goopt:"short:l;default:en;desc:Interface language (en, de, fr, es)"`
	Name     string `goopt:"short:n;desc:Your name;required:true"`
	Verbose  bool   `goopt:"short:v;desc:Enable verbose output"`
}

func main() {
	cfg := &Config{}

	// Create parser with extended Spanish support
	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithSystemLocales(
			i18n.NewLocale(esLocale.Tag, esLocale.SystemTranslations),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Parse to get language preference
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Set language based on user preference
	switch cfg.Language {
	case "es":
		parser.SetLanguage(language.Spanish)
	case "de":
		parser.SetLanguage(language.German)
	case "fr":
		parser.SetLanguage(language.French)
	default:
		// English is default
	}

	// If help was shown, exit
	if parser.WasHelpShown() {
		os.Exit(0)
	}

	// Show output
	fmt.Printf("Hello, %s!\n", cfg.Name)
	if cfg.Verbose {
		fmt.Printf("Language: %s\n", cfg.Language)
	}
}
