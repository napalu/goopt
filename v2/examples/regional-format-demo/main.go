package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

// Config demonstrates regional locale formatting
type Config struct {
	Amount   int     `goopt:"short:a;default:1234567;desc:Amount in CHF"`
	Price    float64 `goopt:"short:p;default:1234.56;desc:Price per unit"`
	Language string  `goopt:"short:l;default:de-CH;desc:Language and region (e.g. de-CH, de-DE, en-US, en-GB)"`
}

func main() {
	// This example demonstrates language matching with regional variants.
	// If you request a language that's not available, it will find the closest match.
	// For example:
	// - Request "en" → matches "en-US" or "en-GB"
	// - Request "en-CA" → matches "en-CA" if available, otherwise "en-US" or "en-GB"
	// - Request "de" → matches "de" (Germany)
	// - Request "de-LI" (Liechtenstein) → matches "de-CH" (Switzerland) or "de" (Germany)

	// Create bundle with translations for different German variants
	bundle := i18n.NewEmptyBundle()

	// Add base German translations
	bundle.AddLanguage(language.German, map[string]string{
		"msg.title":    "Standarddeutsch (Deutschland)",
		"msg.amount":   "Betrag: %d %s",
		"msg.price":    "Preis: %.2f %s",
		"msg.currency": "EUR",
	})

	// Add Swiss German translations
	bundle.AddLanguage(language.MustParse("de-CH"), map[string]string{
		"msg.title":    "Schweizerdeutsch",
		"msg.amount":   "Betrag: %d %s",
		"msg.price":    "Preis: %.2f %s",
		"msg.currency": "CHF",
	})

	// Add Austrian German translations
	bundle.AddLanguage(language.MustParse("de-AT"), map[string]string{
		"msg.title":    "Österreichisches Deutsch",
		"msg.amount":   "Betrag: %d %s",
		"msg.price":    "Preis: %.2f %s",
		"msg.currency": "EUR",
	})

	// Add various English regional variants
	bundle.AddLanguage(language.AmericanEnglish, map[string]string{
		"msg.title":    "American English",
		"msg.amount":   "Amount: %d %s",
		"msg.price":    "Price: %.2f %s",
		"msg.currency": "USD",
	})

	bundle.AddLanguage(language.BritishEnglish, map[string]string{
		"msg.title":    "British English",
		"msg.amount":   "Amount: %d %s",
		"msg.price":    "Price: %.2f %s",
		"msg.currency": "GBP",
	})

	bundle.AddLanguage(language.MustParse("en-CA"), map[string]string{
		"msg.title":    "Canadian English",
		"msg.amount":   "Amount: %d %s",
		"msg.price":    "Price: %.2f %s",
		"msg.currency": "CAD",
	})

	// Note: We're NOT adding generic "en" (English) - only regional variants
	// This demonstrates how language matching will pick the best available variant

	cfg := &Config{}

	// Create parser with locale support
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse with locale-aware formatting
	success := parser.Parse(os.Args)
	requestedLang, err := language.Parse(cfg.Language)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid language tag %q: %v\n", cfg.Language, err)
		requestedLang = language.German
	}

	// Set language - will use language matching to find best available match
	parser.SetLanguage(requestedLang)
	matchedLang := parser.GetLanguage()

	// Show language matching info if different from requested
	if matchedLang != requestedLang {
		fmt.Printf("Language %s not available, using closest match: %s\n", requestedLang, matchedLang)
		supportedLangs := parser.GetSupportedLanguages()
		fmt.Printf("Supported languages: %v\n\n", supportedLangs)
	}

	if !success {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	if parser.WasHelpShown() {
		// The help will show numbers formatted according to the regional variant
		os.Exit(0)
	}

	// Get formatter for the specific regional variant
	tr := parser.GetTranslator()

	// Display regional information
	fmt.Printf("%s\n\n", tr.T("msg.title"))
	fmt.Printf("Requested language: %s\n", requestedLang)
	fmt.Printf("Matched language: %s\n", matchedLang)
	fmt.Printf("Base language: %s\n", matchedLang.Parent())
	fmt.Println()

	// Show number formatting differences
	fmt.Println(tr.T("msg.amount", cfg.Amount, tr.T("msg.currency")))
	fmt.Println(tr.T("msg.price", cfg.Price, tr.T("msg.currency")))

	// Show how different regions format the same numbers
	fmt.Println("\nNumber formatting by region:")
	fmt.Printf("%-20s %s\n", "Region", "1,234,567")
	fmt.Printf("%-20s %s\n", "------", "---------")

	regions := []language.Tag{
		language.MustParse("de-DE"), // Germany: 1.234.567
		language.MustParse("de-CH"), // Switzerland: 1'234'567 (but golang.org/x/text might use spaces)
		language.MustParse("de-AT"), // Austria: 1.234.567
		language.AmericanEnglish,    // US: 1,234,567
		language.BritishEnglish,     // UK: 1,234,567
		language.French,             // France: 1 234 567
		language.MustParse("fr-CH"), // Swiss French: 1 234 567
	}

	for _, region := range regions {
		f := i18n.NewFormatter(region)
		fmt.Printf("%-20s %s\n", region, f.FormatInt(1234567))
	}
}
