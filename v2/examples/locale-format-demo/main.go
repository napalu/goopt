package main

import (
	"fmt"
	"os"
	"time"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

// Config demonstrates locale-aware formatting
type Config struct {
	// Numeric defaults that will be formatted according to locale
	Port      int     `goopt:"short:p;default:8080;desc:Server port"`
	Workers   int     `goopt:"short:w;default:10000;desc:Number of workers"`
	MaxConn   int     `goopt:"short:m;default:1000000;desc:Maximum connections"`
	Threshold float64 `goopt:"short:t;default:99.95;desc:Success threshold percentage"`
	Language  string  `goopt:"short:l;default:en;desc:Display language (en,fr,de,es)"`
}

func main() {
	// Create bundle with translations
	bundle, err := i18n.NewBundle()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating bundle: %v\n", err)
		os.Exit(1)
	}

	// Add translations for different languages with format verbs
	bundle.AddLanguage(language.French, map[string]string{
		"msg.config":    "Configuration :",
		"msg.port":      "Port : %d",
		"msg.workers":   "Travailleurs : %d",
		"msg.maxconn":   "Connexions max : %d",
		"msg.threshold": "Seuil : %.2f%%",
		"msg.date":      "Date : %s",
		"msg.range":     "Plage valide : %d - %d",
		"msg.example":   "Exemple de grand nombre : %d",
	})

	bundle.AddLanguage(language.German, map[string]string{
		"msg.config":    "Konfiguration:",
		"msg.port":      "Port: %d",
		"msg.workers":   "Arbeiter: %d",
		"msg.maxconn":   "Max. Verbindungen: %d",
		"msg.threshold": "Schwellenwert: %.2f%%",
		"msg.date":      "Datum: %s",
		"msg.range":     "Gültiger Bereich: %d - %d",
		"msg.example":   "Beispiel große Zahl: %d",
	})

	bundle.AddLanguage(language.Spanish, map[string]string{
		"msg.config":    "Configuración:",
		"msg.port":      "Puerto: %d",
		"msg.workers":   "Trabajadores: %d",
		"msg.maxconn":   "Conexiones máx: %d",
		"msg.threshold": "Umbral: %.2f%%",
		"msg.date":      "Fecha: %s",
		"msg.range":     "Rango válido: %d - %d",
		"msg.example":   "Ejemplo número grande: %d",
	})

	// English (default)
	bundle.AddLanguage(language.English, map[string]string{
		"msg.config":    "Configuration:",
		"msg.port":      "Port: %d",
		"msg.workers":   "Workers: %d",
		"msg.maxconn":   "Max connections: %d",
		"msg.threshold": "Threshold: %.2f%%",
		"msg.date":      "Date: %s",
		"msg.range":     "Valid range: %d - %d",
		"msg.example":   "Example large number: %d",
	})

	cfg := &Config{}

	// First pass - get language preference
	tempParser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}
	tempParser.Parse(os.Args)

	// Create parser with locale support
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse with locale-aware formatting
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	if parser.WasHelpShown() {
		os.Exit(0)
	}

	// Set language based on user input
	langTag, err := language.Parse(cfg.Language)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing language: %v\n", err)
		os.Exit(1)
	}
	parser.SetLanguage(langTag)

	// Get translator and printer for locale-aware operations
	translator := parser.GetTranslator()
	printer := translator.GetPrinter()

	// Display configuration with locale-aware formatting
	fmt.Println(translator.T("msg.config"))
	fmt.Println()

	// Technical value - port should NOT be locale formatted
	// Using fmt.Printf to avoid locale formatting
	fmt.Printf("Port: %d\n", cfg.Port)

	// Display values - these SHOULD be locale formatted
	// Now using T() with args for automatic locale-aware formatting
	fmt.Println(translator.T("msg.workers", cfg.Workers))
	fmt.Println(translator.T("msg.maxconn", cfg.MaxConn))
	fmt.Println(translator.T("msg.threshold", cfg.Threshold))

	// Show date formatting
	now := time.Now()
	fmt.Println(translator.T("msg.date", now.Format("2006-01-02 15:04:05")))

	// Show locale-aware number range
	fmt.Println(translator.T("msg.range", 1024, 65535))

	// Demonstrate the automatic locale formatting
	fmt.Println()
	fmt.Println(translator.T("msg.example", 1234567))

	// Show explicit use of printer when needed
	fmt.Println("\nExplicit printer usage (when needed):")
	fmt.Printf("  Direct formatting: ")
	printer.Printf("%d\n", 9876543)
}
