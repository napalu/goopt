package main

import (
	"embed"
	"fmt"
	"github.com/napalu/goopt/v2/errs"
	"golang.org/x/text/language"
	"os"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"

	// Import individual locale packages
	arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"
	esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
	heLocale "github.com/napalu/goopt/v2/i18n/locales/he"
	jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
)

//go:embed locales/*.json
var localeFS embed.FS

// Config for i18n system extensions demo
type Config struct {
	// Global flags
	// Note: We use descKey instead of desc to enable translation of flag descriptions.
	// The descKey points to a translation key in our locale files (locales/*.json).
	// You can also use both desc and descKey - desc will be the fallback if translation is not found.
	Language string `goopt:"short:l;default:en;descKey:app.flag.language.desc"`
	Verbose  bool   `goopt:"short:v;descKey:app.flag.verbose.desc"`

	// Demo command - demonstrates loading additional language extensions
	Demo struct {
		Exec goopt.CommandFunc
	} `goopt:"kind:command;descKey:app.command.demo.desc"`
}

func executeDemo(cmdLine *goopt.Parser, command *goopt.Command) error {
	config, ok := goopt.GetStructCtxAs[*Config](cmdLine)
	if !ok {
		return errs.ErrProcessingCommand.WithArgs(command.Name)
	}

	demoLocales(cmdLine, config)
	return nil
}

func main() {
	cfg := &Config{}
	cfg.Demo.Exec = executeDemo

	// Load user translations from embedded files
	userBundle, err := i18n.NewBundleWithFS(localeFS, "locales")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading user translations: %v\n", err)
		os.Exit(1)
	}

	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithVersion("1.0.0"),
		goopt.WithSystemLocales(
			// load additional system files on top of the core French, English and German translations
			i18n.NewLocale(esLocale.Tag, esLocale.SystemTranslations),
			i18n.NewLocale(jaLocale.Tag, jaLocale.SystemTranslations),
			i18n.NewLocale(heLocale.Tag, heLocale.SystemTranslations),
			i18n.NewLocale(arLocale.Tag, arLocale.SystemTranslations),
		),
		goopt.WithUserBundle(userBundle),
		// execute commands with callbacks in registration order as soon as parse completes
		goopt.WithExecOnParseComplete(true),
	)

	// Note: The language can be set from the command line by the user using the --lang or -l flag
	// The lang or l flags work out of the box because goopt handles language switching
	// as part of its auto-help system. When a language is specified via -l or --lang,
	// goopt automatically calls SetLanguage() on the parser before processing other flags.
	// This is part of the Auto-Language Detection feature.
	// See: https://github.com/napalu/goopt/blob/main/v2/docs/_v2/guides/05-built-in-features/06-auto-language-detection.md
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	if !parser.Parse(os.Args) {
		// Display any parsing errors
		for _, err = range parser.GetErrors() {
			_, _ = parser.GetStderr().Write([]byte(err.Error() + "\n"))
		}
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}
}

func demoLocales(parser *goopt.Parser, cfg *Config) {
	currentLang := parser.GetLanguage()
	supportedLanguages := parser.GetSupportedLanguages()
	translator := parser.GetTranslator()
	langName := i18n.GetNativeLanguageName(currentLang)

	fmt.Printf("Current language: %s\n", currentLang)
	fmt.Printf("All supported languages: %v\n\n", supportedLanguages)

	// Language Info
	if cfg.Language != currentLang.String() {
		fmt.Printf("🌐 Matched Language: %s (%s) – requested: %s (%s)\n",
			langName, currentLang,
			i18n.GetNativeLanguageName(language.Make(cfg.Language)), cfg.Language)
	} else {
		fmt.Printf("🌐 Language: %s (%s)\n", langName, currentLang)
	}

	fmt.Print("🔤 Supported Languages: ")
	for i, tag := range supportedLanguages {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(i18n.GetNativeLanguageName(tag), " (", tag, ")")
	}
	fmt.Println()

	printSection("🧭 RTL Language Features", i18n.IsRTL(currentLang), func() {
		fmt.Println("  ➤ Direction: Right-to-Left")
		fmt.Println("  ➤ Note: Terminal rendering for RTL may vary across platforms\n")
	})

	printSection("🗣️ Sample Translations", true, func() {
		printTranslation("Commands", "goopt.msg.commands", translator)
		printTranslation("Global Flags", "goopt.msg.global_flags", translator)
		printTranslation("Required", "goopt.msg.required", translator)
		printTranslation("Optional", "goopt.msg.optional", translator)
		fmt.Println()
	})

	if cfg.Verbose {
		printSection("🧩 Verbose Output: Modular Translation System", true, func() {
			fmt.Println("- Each language is a separate Go package")
			fmt.Println("- Only imported languages are compiled into the binary")
			fmt.Println("- No runtime file I/O required")
			fmt.Println("- Type-safe locale access\n")
		})
	}

	fmt.Println("💡 Tip: Add your own translations in `locales/*.json`")
	fmt.Println("📖 Docs: https://napalu.github.io/goopt/v2/guides/06-internationalization/index/")
}

func printSection(title string, show bool, body func()) {
	if !show {
		return
	}
	fmt.Println("────────────────────────────────────────────")
	fmt.Println(title)
	fmt.Println("────────────────────────────────────────────")
	body()
}

func printTranslation(label, key string, tr i18n.Translator) {
	text := tr.T(key)
	if strings.HasPrefix(text, "[TODO]") {
		text += " ⚠️"
	}
	fmt.Printf("  • %-14s → %s\n", label, text)
}
