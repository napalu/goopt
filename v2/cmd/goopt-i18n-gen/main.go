package main

//go:generate go run . -i "locales/*.json" validate -s "*.go" -g
//go:generate go run . -i "locales/*.json" generate -o messages/messages.go -p messages

import (
	"embed"
	"fmt"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/translations"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
	"log"
	"os"
	"strings"
)

//go:embed locales/*.json
var localesFS embed.FS

func main() {
	cfg := &options.AppConfig{}

	// Assign command functions
	cfg.Generate.Exec = translations.Generate
	cfg.Validate.Exec = translations.Validate
	cfg.Audit.Exec = translations.Audit
	cfg.Init.Exec = translations.Init
	cfg.Add.Exec = translations.Add
	cfg.Extract.Exec = translations.Extract
	cfg.Sync.Exec = translations.Sync

	// Create i18n bundle
	bundle, err := i18n.NewBundleWithFS(localesFS, "locales")
	if err != nil {
		log.Fatalf("Failed to create i18n bundle: %v", err)
	}

	// Set up translator for the app
	cfg.TR = bundle

	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithFlagNameConverter(goopt.ToKebabCase),
		goopt.WithEnvNameConverter(goopt.ToKebabCase),
		goopt.WithCommandNameConverter(goopt.ToKebabCase),
		goopt.WithUserBundle(bundle))
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	// Parse command line arguments
	success := parser.Parse(os.Args)

	// Handle language switching
	if cfg.Language != "" && cfg.Language != bundle.GetDefaultLanguage().String() {
		lang := parseLanguage(cfg.Language)
		if lang != language.Und {
			bundle.SetDefaultLanguage(lang)
			// update goopt system bundle (important for goopt error and message translations)
			i18n.Default().SetDefaultLanguage(lang)
		}
	}

	if cfg.Help {
		parser.PrintUsageWithGroups(os.Stdout)
		os.Exit(0)
	}

	if !success {
		for _, err := range parser.GetErrors() {
			fmt.Fprintln(os.Stderr, cfg.TR.T(messages.Keys.AppError.ParseError, err))
			fmt.Fprintln(os.Stderr)
		}
		parser.PrintUsageWithGroups(os.Stderr)
		os.Exit(1)
	}

	// Execute commands
	errCount := parser.ExecuteCommands()
	if errCount > 0 {
		for _, cmdErr := range parser.GetCommandExecutionErrors() {
			fmt.Fprintln(os.Stderr, cfg.TR.T(messages.Keys.AppError.CommandFailed, cmdErr.Key, cmdErr.Value))
			fmt.Fprintln(os.Stderr)
		}
		os.Exit(1)
	}
}

func parseLanguage(lang string) language.Tag {
	switch strings.ToLower(lang) {
	case "en":
		return language.English
	case "de":
		return language.German
	case "fr":
		return language.French
	default:
		return language.Und
	}
}
