package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/validation"
	"golang.org/x/text/language"
)

type Config struct {
	Port    int    `goopt:"short:p;desc:Server port;validate:range(1000,65535)"`
	Workers int    `goopt:"short:w;desc:Number of workers;validate:min(100)"`
	Memory  int    `goopt:"short:m;desc:Memory limit in MB;validate:max(8192)"`
	Locale  string `goopt:"short:l;default:en;desc:Locale (en, fr, de, de-CH)"`
}

func main() {
	// Create initial parser to get locale preference
	tempCfg := &Config{}
	tempParser, err := goopt.NewParserFromStruct(tempCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}
	tempParser.Parse(os.Args)

	// Create i18n bundle
	bundle := i18n.NewEmptyBundle()

	// Add English messages
	bundle.AddLanguage(language.English, map[string]string{
		"validation.value_between":   "must be between %s and %s",
		"validation.value_at_least":  "must be at least %s",
		"validation.value_at_most":   "must be at most %s",
		"demo.title":                 "Validation with Locale-Aware Error Formatting",
		"demo.parsing":               "Parsing with locale: %s",
		"demo.errors_found":          "Validation errors found:",
		"demo.no_errors":             "All values are valid!",
		"demo.port_info":             "Port: %d (valid range: 1,000 - 65,535)",
		"demo.workers_info":          "Workers: %d (minimum: 100)",
		"demo.memory_info":           "Memory: %dMB (maximum: 8,192MB)",
		"goopt.msg.defaults_to":      "defaults to",
	})

	// Add French messages
	bundle.AddLanguage(language.French, map[string]string{
		"validation.value_between":   "doit être entre %s et %s",
		"validation.value_at_least":  "doit être au moins %s",
		"validation.value_at_most":   "doit être au maximum %s",
		"demo.title":                 "Validation avec formatage d'erreur adapté aux paramètres régionaux",
		"demo.parsing":               "Analyse avec les paramètres régionaux: %s",
		"demo.errors_found":          "Erreurs de validation trouvées:",
		"demo.no_errors":             "Toutes les valeurs sont valides!",
		"demo.port_info":             "Port: %d (plage valide: 1 000 - 65 535)",
		"demo.workers_info":          "Travailleurs: %d (minimum: 100)",
		"demo.memory_info":           "Mémoire: %dMo (maximum: 8 192Mo)",
		"goopt.msg.defaults_to":      "par défaut",
	})

	// Add German messages
	bundle.AddLanguage(language.German, map[string]string{
		"validation.value_between":   "muss zwischen %s und %s liegen",
		"validation.value_at_least":  "muss mindestens %s sein",
		"validation.value_at_most":   "darf höchstens %s sein",
		"demo.title":                 "Validierung mit gebietsschemabewusster Fehlerformatierung",
		"demo.parsing":               "Parsing mit Gebietsschema: %s",
		"demo.errors_found":          "Validierungsfehler gefunden:",
		"demo.no_errors":             "Alle Werte sind gültig!",
		"demo.port_info":             "Port: %d (gültiger Bereich: 1.000 - 65.535)",
		"demo.workers_info":          "Arbeiter: %d (Minimum: 100)",
		"demo.memory_info":           "Speicher: %dMB (Maximum: 8.192MB)",
		"goopt.msg.defaults_to":      "Standard",
	})

	// Add Swiss German messages
	swissGerman := language.MustParse("de-CH")
	bundle.AddLanguage(swissGerman, map[string]string{
		"validation.value_between":   "muss zwischen %s und %s liegen",
		"validation.value_at_least":  "muss mindestens %s sein",
		"validation.value_at_most":   "darf höchstens %s sein",
		"demo.title":                 "Validierung mit gebietsschemabewusster Fehlerformatierung (Schweiz)",
		"demo.parsing":               "Parsing mit Schweizer Gebietsschema: %s",
		"demo.errors_found":          "Validierungsfehler gefunden:",
		"demo.no_errors":             "Alle Werte sind gültig!",
		"demo.port_info":             "Port: %d (gültiger Bereich: 1'000 - 65'535)",
		"demo.workers_info":          "Arbeiter: %d (Minimum: 100)",
		"demo.memory_info":           "Speicher: %dMB (Maximum: 8'192MB)",
		"goopt.msg.defaults_to":      "Standard",
	})

	// Set locale based on user preference
	var langTag language.Tag
	switch tempCfg.Locale {
	case "fr":
		langTag = language.French
	case "de":
		langTag = language.German
	case "de-CH":
		langTag = swissGerman
	default:
		langTag = language.English
	}
	bundle.SetDefaultLanguage(langTag)

	// Create the real parser with locale
	cfg := &Config{}
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Override validation for clearer demo
	parser.SetArgument("port", nil, goopt.WithValidators(
		validation.IntRange(1000, 65535),
	))
	parser.SetArgument("workers", nil, goopt.WithValidators(
		validation.Min(100),
	))
	parser.SetArgument("memory", nil, goopt.WithValidators(
		validation.Max(8192),
	))

	// Get the translator
	tr := parser.GetTranslator()

	fmt.Println(tr.T("demo.title"))
	fmt.Println()
	fmt.Printf("%s\n\n", tr.T("demo.parsing", langTag))

	// Parse with potential errors
	parser.Parse(os.Args)

	// Display parsed values with locale formatting
	if cfg.Port != 0 {
		fmt.Println(tr.T("demo.port_info", cfg.Port))
	}
	if cfg.Workers != 0 {
		fmt.Println(tr.T("demo.workers_info", cfg.Workers))
	}
	if cfg.Memory != 0 {
		fmt.Println(tr.T("demo.memory_info", cfg.Memory))
	}
	fmt.Println()

	// Check for errors
	errors := parser.GetErrors()
	if len(errors) > 0 {
		fmt.Println(tr.T("demo.errors_found"))
		for _, err := range errors {
			// The error is already formatted with locale-aware numbers
			fmt.Printf("  • %v\n", err)
		}
		os.Exit(1)
	} else if cfg.Port != 0 || cfg.Workers != 0 || cfg.Memory != 0 {
		fmt.Println(tr.T("demo.no_errors"))
	}

	// If help was shown, exit
	if parser.WasHelpShown() {
		os.Exit(0)
	}
}