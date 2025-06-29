package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"

	// Locale system translations
	arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"
	esLocale "github.com/napalu/goopt/v2/i18n/locales/es"
	jaLocale "github.com/napalu/goopt/v2/i18n/locales/ja"
)

// ServerConfig defines CLI options with i18n support
type ServerConfig struct {
	Port     int    `goopt:"nameKey:flag.port;short:p;default:8080;desc:Server port;descKey:desc.port;validators:port"`
	Language string `goopt:"nameKey:goopt.flag.language;short:l;default:en;desc:Language;descKey:desc.language"`
	Workers  int    `goopt:"nameKey:flag.workers;short:w;default:4;desc:Number of workers;descKey:desc.workers"`
	Config   string `goopt:"nameKey:flag.config;short:c;type:file;desc:Configuration file;descKey:desc.config"`
	Verbose  bool   `goopt:"nameKey:flag.verbose;short:v;desc:Enable verbose output;descKey:desc.verbose"`
	Help     bool   `goopt:"nameKey:goopt.flag.help;short:h;desc:Show help;descKey:desc.help"`
}

func main() {
	cfg := &ServerConfig{}
	bundle := i18n.NewEmptyBundle()

	must(addTranslations(bundle))

	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithUserBundle(bundle),
		goopt.WithSystemLocales(
			// add system locales on top of the core (en, fr, de) ones
			i18n.NewLocale(arLocale.Tag, arLocale.SystemTranslations),
			i18n.NewLocale(esLocale.Tag, esLocale.SystemTranslations),
			i18n.NewLocale(jaLocale.Tag, jaLocale.SystemTranslations),
		),
		// Disable auto-help and auto-language because we want to demonstrate
		// how to handle translated flag names manually. This is useful when:
		// - You need custom help behavior
		// - You want to use --help or --language for other purposes
		// - You're building a minimal CLI without automatic features
		goopt.WithAutoHelp(false),
		goopt.WithAutoLanguage(false),
	)
	must(err)

	// Customize suggestions based on language
	parser.SetSuggestionsFormatter(func(suggestions []string) string {
		switch cfg.Language {
		case "ja":
			return "「" + suggestions[0] + "」"
		case "ar":
			return "«" + suggestions[0] + "»"
		case "es":
			return "¿" + suggestions[0] + "?"
		default:
			return "[" + suggestions[0] + "]"
		}
	})

	// Handle language flag manually before parsing
	// This is needed because auto-language is disabled
	for i, arg := range os.Args {
		if (arg == "-l" || arg == "--language") && i+1 < len(os.Args) {
			langCode := os.Args[i+1]
			parser.SetLanguage(language.Make(langCode))
			break
		} else if strings.HasPrefix(arg, "--language=") {
			langCode := strings.TrimPrefix(arg, "--language=")
			parser.SetLanguage(language.Make(langCode))
			break
		} else if strings.HasPrefix(arg, "-l=") {
			langCode := strings.TrimPrefix(arg, "-l=")
			parser.SetLanguage(language.Make(langCode))
			break
		}
		// Also check for translated flag names
		translator := parser.GetTranslator()
		langFlag := translator.T("goopt.flag.language")
		if arg == "--"+langFlag && i+1 < len(os.Args) {
			langCode := os.Args[i+1]
			parser.SetLanguage(language.Make(langCode))
			break
		}
	}

	if !parser.Parse(os.Args) {
		// Display parsing errors
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "%v\n", e)
		}
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Check if help flag was set
	if cfg.Help {
		parser.PrintHelp(os.Stdout)
		os.Exit(0)
	}

	// Check if the requested language was available
	supported := parser.GetSupportedLanguages()
	userTag := language.Make(cfg.Language)
	actualLang := parser.GetLanguage()

	if cfg.Language != "" && userTag != actualLang {
		fmt.Fprintf(os.Stderr,
			"⚠️  Language %q not available, using %q instead. Available: %v\n",
			cfg.Language, actualLang, supported,
		)
	}

	translator := parser.GetTranslator()
	fmt.Println(translator.T("msg.language", cfg.Language))
	fmt.Println(translator.T("msg.starting", cfg.Port, cfg.Workers))

	if cfg.Verbose {
		fmt.Printf("Verbose mode enabled\n")
		fmt.Printf("Config file: %s\n", cfg.Config)
	}
}

// must panics on error (for clean main)
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// addTranslations registers user-defined i18n strings
func addTranslations(bundle *i18n.Bundle) error {
	return multi(
		bundle.AddLanguage(language.English, map[string]string{
			"flag.port":     "port",
			"flag.workers":  "workers",
			"flag.config":   "config",
			"flag.verbose":  "verbose",
			"desc.help":     "Show help",
			"desc.port":     "Server port number",
			"desc.language": "Interface language",
			"desc.workers":  "Number of worker threads",
			"desc.config":   "Configuration file path",
			"desc.verbose":  "Enable verbose output",
			"msg.starting":  "Starting server on port %d with %d workers",
			"msg.language":  "Using language: %s",
		}),
		bundle.AddLanguage(language.Japanese, map[string]string{
			"flag.port":     "ポート",
			"flag.workers":  "ワーカー",
			"flag.config":   "設定",
			"flag.verbose":  "詳細",
			"desc.help":     "ヘルプ情報を表示する",
			"desc.port":     "サーバーポート番号",
			"desc.language": "インターフェース言語",
			"desc.workers":  "ワーカースレッド数",
			"desc.config":   "設定ファイルパス",
			"desc.verbose":  "詳細出力を有効にする",
			"msg.starting":  "ポート %d で %d ワーカーでサーバーを起動中",
			"msg.language":  "使用言語: %s",
		}),
		bundle.AddLanguage(language.Arabic, map[string]string{
			"flag.port":     "منفذ",
			"flag.workers":  "عمال",
			"flag.config":   "تكوين",
			"flag.verbose":  "مفصل",
			"desc.help":     "عرض معلومات المساعدة",
			"desc.port":     "رقم منفذ الخادم",
			"desc.language": "لغة الواجهة",
			"desc.workers":  "عدد خيوط العمل",
			"desc.config":   "مسار ملف التكوين",
			"desc.verbose":  "تمكين الإخراج المفصل",
			"msg.starting":  "بدء الخادم على المنفذ %d مع %d عمال",
			"msg.language":  "اللغة المستخدمة: %s",
		}),
		bundle.AddLanguage(language.Spanish, map[string]string{
			"flag.port":     "puerto",
			"flag.workers":  "trabajadores",
			"flag.config":   "configuración",
			"flag.verbose":  "detallado",
			"desc.help":     "Mostrar información de ayuda",
			"desc.port":     "Número de puerto del servidor",
			"desc.language": "Idioma de la interfaz",
			"desc.workers":  "Número de hilos de trabajo",
			"desc.config":   "Ruta del archivo de configuración",
			"desc.verbose":  "Habilitar salida detallada",
			"msg.starting":  "Iniciando servidor en puerto %d con %d trabajadores",
			"msg.language":  "Idioma en uso: %s",
		}),
		bundle.AddLanguage(language.German, map[string]string{
			"flag.port":     "port",
			"flag.workers":  "arbeiter",
			"flag.config":   "konfiguration",
			"flag.verbose":  "ausführlich",
			"desc.help":     "Hilfe anzeigen",
			"desc.port":     "Serverportnummer",
			"desc.language": "Sprache der Benutzeroberfläche",
			"desc.workers":  "Anzahl der Arbeitsthreads",
			"desc.config":   "Pfad zur Konfigurationsdatei",
			"desc.verbose":  "Ausführliche Ausgabe aktivieren",
			"msg.starting":  "Starte Server auf Port %d mit %d Arbeitern",
			"msg.language":  "Verwendete Sprache: %s",
		}),
		bundle.AddLanguage(language.French, map[string]string{
			// Flag names
			"flag.port":     "port",
			"flag.workers":  "travailleurs",
			"flag.config":   "configuration",
			"flag.verbose":  "détaillé",
			"desc.help":     "Afficher l’aide",
			"desc.port":     "Numéro de port du serveur",
			"desc.language": "Langue de l’interface",
			"desc.workers":  "Nombre de threads de travail",
			"desc.config":   "Chemin du fichier de configuration",
			"desc.verbose":  "Activer la sortie verbeuse",
			"msg.starting":  "Démarrage du serveur sur le port %d avec %d travailleurs",
			"msg.language":  "Langue utilisée : %s",
		}),
	)
}

// multi handles multiple error returns
func multi(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func containsLang(tags []language.Tag, tag language.Tag) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
