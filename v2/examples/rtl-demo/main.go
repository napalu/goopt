package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"
	heLocale "github.com/napalu/goopt/v2/i18n/locales/he"
	"golang.org/x/text/language"
)

// AppConfig demonstrates RTL support
type AppConfig struct {
	// Configuration with bilingual support
	Server   string `goopt:"nameKey:flag.server;short:s;default:localhost;desc:Server address;descKey:desc.server"`
	Port     int    `goopt:"nameKey:flag.port;short:p;default:8080;desc:Server port;descKey:desc.port"`
	Language string `goopt:"nameKey:flag.language;short:l;default:en;desc:Language;descKey:desc.language"`
	Debug    bool   `goopt:"nameKey:flag.debug;short:d;desc:Enable debug mode;descKey:desc.debug"`
}

func main() {
	// Create user translation bundle (empty to start)
	// Note: Use NewEmptyBundle() for user translations to avoid validation errors
	// when adding partial translations for new languages
	bundle := i18n.NewEmptyBundle()

	// English translations
	err := bundle.AddLanguage(language.English, map[string]string{
		// Flag names
		"flag.server":   "server",
		"flag.port":     "port",
		"flag.language": "language",
		"flag.debug":    "debug",

		// Descriptions
		"desc.server":   "Server address to connect to",
		"desc.port":     "Port number for the server",
		"desc.language": "Interface language",
		"desc.debug":    "Enable debug output",

		// Messages
		"msg.config": "Configuration:",
		"msg.server": "Server: %s:%d",
		"msg.debug":  "Debug mode: %v",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding English translations: %v\n", err)
		os.Exit(1)
	}

	// Arabic translations (RTL)
	err = bundle.AddLanguage(language.Arabic, map[string]string{
		// Flag names in Arabic
		"flag.server":   "خادم",
		"flag.port":     "منفذ",
		"flag.language": "لغة",
		"flag.debug":    "تصحيح",

		// Descriptions in Arabic
		"desc.server":   "عنوان الخادم للاتصال",
		"desc.port":     "رقم المنفذ للخادم",
		"desc.language": "لغة الواجهة",
		"desc.debug":    "تمكين وضع التصحيح",

		// Messages in Arabic
		"msg.config": "التكوين:",
		"msg.server": "الخادم: %s:%d",
		"msg.debug":  "وضع التصحيح: %v",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Arabic translations: %v\n", err)
		os.Exit(1)
	}

	// Hebrew translations (RTL)
	err = bundle.AddLanguage(language.Hebrew, map[string]string{
		// Flag names in Hebrew
		"flag.server":   "שרת",
		"flag.port":     "פורט",
		"flag.language": "שפה",
		"flag.debug":    "דיבאג",

		// Descriptions in Hebrew
		"desc.server":   "כתובת השרת להתחברות",
		"desc.port":     "מספר הפורט של השרת",
		"desc.language": "שפת הממשק",
		"desc.debug":    "הפעל מצב דיבאג",

		// Messages in Hebrew
		"msg.config": "הגדרות:",
		"msg.server": "שרת: %s:%d",
		"msg.debug":  "מצב דיבאג: %v",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Hebrew translations: %v\n", err)
		os.Exit(1)
	}

	// Create config
	cfg := &AppConfig{}

	// Create parser with user translations and system locales
	// Auto-language will handle language detection automatically!
	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithUserBundle(bundle),
		goopt.WithSystemLocales(
			i18n.NewLocale(language.Arabic, arLocale.SystemTranslations),
			i18n.NewLocale(language.Hebrew, heLocale.SystemTranslations)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Just parse! Auto-language will detect the language from:
	// 1. Command-line flags (--language, --lang, -l)
	// 2. Environment variables (LANG, GOOPT_LANG)
	// 3. System locale
	if !parser.Parse(os.Args) {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Check if help was shown
	if parser.WasHelpShown() {
		os.Exit(0)
	}

	// Display configuration
	tr := parser.GetTranslator()
	fmt.Printf("%s\n", tr.T("msg.config"))
	fmt.Printf("%s\n", tr.T("msg.server", cfg.Server, cfg.Port))
	fmt.Printf("%s\n", tr.T("msg.debug", cfg.Debug))
}
