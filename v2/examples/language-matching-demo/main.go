package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

// Config for language matching demo
type Config struct {
	Language string `goopt:"short:l;default:en;desc:Language code to test (e.g., en, en-CA, de-AT)"`
	Verbose  bool   `goopt:"short:v;desc:Show detailed matching information"`
}

func main() {
	// Create a bundle with various language variants
	bundle := i18n.NewEmptyBundle()

	// English variants
	bundle.AddLanguage(language.MustParse("en-US"), map[string]string{
		"greeting":  "Hello",
		"variant":   "American English (en-US)",
		"currency":  "USD",
		"date_fmt":  "MM/DD/YYYY",
		"color":     "color",
		"center":    "center",
		"kilometer": "kilometer",
	})

	bundle.AddLanguage(language.BritishEnglish, map[string]string{
		"greeting":  "Hello",
		"variant":   "British English (en-GB)",
		"currency":  "GBP",
		"date_fmt":  "DD/MM/YYYY",
		"color":     "colour",
		"center":    "centre",
		"kilometer": "kilometre",
	})

	bundle.AddLanguage(language.MustParse("en-CA"), map[string]string{
		"greeting":  "Hello",
		"variant":   "Canadian English (en-CA)",
		"currency":  "CAD",
		"date_fmt":  "DD/MM/YYYY",
		"color":     "colour",
		"center":    "centre",
		"kilometer": "kilometre",
	})

	bundle.AddLanguage(language.MustParse("en-AU"), map[string]string{
		"greeting":  "G'day",
		"variant":   "Australian English (en-AU)",
		"currency":  "AUD",
		"date_fmt":  "DD/MM/YYYY",
		"color":     "colour",
		"center":    "centre",
		"kilometer": "kilometre",
	})

	// German variants
	bundle.AddLanguage(language.German, map[string]string{
		"greeting":  "Hallo",
		"variant":   "Standard German (de)",
		"currency":  "EUR",
		"date_fmt":  "DD.MM.YYYY",
		"color":     "Farbe",
		"center":    "Zentrum",
		"kilometer": "Kilometer",
	})

	bundle.AddLanguage(language.MustParse("de-CH"), map[string]string{
		"greeting":  "Grüezi",
		"variant":   "Swiss German (de-CH)",
		"currency":  "CHF",
		"date_fmt":  "DD.MM.YYYY",
		"color":     "Farbe",
		"center":    "Zentrum",
		"kilometer": "Kilometer",
	})

	bundle.AddLanguage(language.MustParse("de-AT"), map[string]string{
		"greeting":  "Servus",
		"variant":   "Austrian German (de-AT)",
		"currency":  "EUR",
		"date_fmt":  "DD.MM.YYYY",
		"color":     "Farbe",
		"center":    "Zentrum",
		"kilometer": "Kilometer",
	})

	// French variants
	bundle.AddLanguage(language.French, map[string]string{
		"greeting":  "Bonjour",
		"variant":   "Standard French (fr)",
		"currency":  "EUR",
		"date_fmt":  "DD/MM/YYYY",
		"color":     "couleur",
		"center":    "centre",
		"kilometer": "kilomètre",
	})

	bundle.AddLanguage(language.MustParse("fr-CA"), map[string]string{
		"greeting":  "Bonjour",
		"variant":   "Canadian French (fr-CA)",
		"currency":  "CAD",
		"date_fmt":  "YYYY-MM-DD",
		"color":     "couleur",
		"center":    "centre",
		"kilometer": "kilomètre",
	})

	bundle.AddLanguage(language.MustParse("fr-CH"), map[string]string{
		"greeting":  "Bonjour",
		"variant":   "Swiss French (fr-CH)",
		"currency":  "CHF",
		"date_fmt":  "DD.MM.YYYY",
		"color":     "couleur",
		"center":    "centre",
		"kilometer": "kilomètre",
	})

	// Spanish variants
	bundle.AddLanguage(language.Spanish, map[string]string{
		"greeting":  "Hola",
		"variant":   "European Spanish (es)",
		"currency":  "EUR",
		"date_fmt":  "DD/MM/YYYY",
		"color":     "color",
		"center":    "centro",
		"kilometer": "kilómetro",
	})

	bundle.AddLanguage(language.MustParse("es-MX"), map[string]string{
		"greeting":  "Hola",
		"variant":   "Mexican Spanish (es-MX)",
		"currency":  "MXN",
		"date_fmt":  "DD/MM/YYYY",
		"color":     "color",
		"center":    "centro",
		"kilometer": "kilómetro",
	})

	// Parse command line
	cfg := &Config{}
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	if parser.WasHelpShown() {
		os.Exit(0)
	}

	// Test language matching
	requested, err := language.Parse(cfg.Language)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid language tag '%s': %v\n", cfg.Language, err)
		os.Exit(1)
	}

	// Show matching process
	fmt.Printf("Language Matching Demo\n")
	fmt.Printf("======================\n\n")

	fmt.Printf("Requested language: %s\n", requested)

	// Find the match
	matched := bundle.MatchLanguage(requested)
	fmt.Printf("Matched language:   %s\n", matched)

	if cfg.Verbose {
		fmt.Printf("\nAvailable languages:\n")
		for _, lang := range bundle.Languages() {
			fmt.Printf("  - %s\n", lang)
		}
	}

	// Set the language and get translator
	parser.SetLanguage(requested)
	actualLang := parser.GetLanguage()
	translator := parser.GetTranslator()

	fmt.Printf("\nActual language used: %s\n", actualLang)
	fmt.Printf("Variant: %s\n", translator.T("variant"))
	fmt.Printf("\nTranslations:\n")
	fmt.Printf("  Greeting:  %s\n", translator.T("greeting"))
	fmt.Printf("  Currency:  %s\n", translator.T("currency"))
	fmt.Printf("  Date fmt:  %s\n", translator.T("date_fmt"))
	fmt.Printf("  Color:     %s\n", translator.T("color"))
	fmt.Printf("  Center:    %s\n", translator.T("center"))
	fmt.Printf("  Kilometer: %s\n", translator.T("kilometer"))

	// Show some matching examples
	if cfg.Verbose {
		fmt.Printf("\n\nLanguage Matching Examples:\n")
		fmt.Printf("===========================\n")

		testCases := []string{
			"en",    // Generic English
			"en-NZ", // New Zealand English (not in bundle)
			"en-ZA", // South African English (not in bundle)
			"de",    // Generic German
			"de-LI", // Liechtenstein German (not in bundle)
			"fr",    // Generic French
			"fr-BE", // Belgian French (not in bundle)
			"es",    // Generic Spanish
			"es-AR", // Argentinian Spanish (not in bundle)
			"pt",    // Portuguese (not in bundle at all)
			"ja",    // Japanese (not in bundle at all)
		}

		for _, test := range testCases {
			testLang, err := language.Parse(test)
			if err != nil {
				continue
			}
			matched := bundle.MatchLanguage(testLang)
			fmt.Printf("  %-10s → %s", test, matched)
			if bundle.HasLanguage(testLang) {
				fmt.Printf(" (exact match)")
			} else if matched.String() != bundle.GetDefaultLanguage().String() {
				fmt.Printf(" (closest match)")
			} else {
				fmt.Printf(" (fallback to default)")
			}
			fmt.Println()
		}
	}
}
