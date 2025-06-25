package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

// ServerConfig demonstrates full i18n support including translated flag names
type ServerConfig struct {
	// Port demonstrates a translated flag name
	Port int `goopt:"nameKey:flag.port;short:p;default:8080;desc:Server port;descKey:desc.port"`

	// Language selection flag (meta!)
	Language string `goopt:"nameKey:flag.language;short:l;default:en;desc:Language;descKey:desc.language"`

	// Workers demonstrates numeric flag with i18n
	Workers int `goopt:"nameKey:flag.workers;short:w;default:4;desc:Number of workers;descKey:desc.workers"`

	// Config file path - shows file type validation
	Config string `goopt:"nameKey:flag.config;short:c;type:file;desc:Configuration file;descKey:desc.config"`

	// Verbose mode
	Verbose bool `goopt:"nameKey:flag.verbose;short:v;desc:Enable verbose output;descKey:desc.verbose"`
}

func main() {
	// Create empty bundle for user translations
	bundle := i18n.NewEmptyBundle()

	// English translations
	err := bundle.AddLanguage(language.English, map[string]string{
		// Flag names
		"flag.port":     "port",
		"flag.language": "language",
		"flag.workers":  "workers",
		"flag.config":   "config",
		"flag.verbose":  "verbose",

		// Descriptions
		"desc.port":     "Server port number",
		"desc.language": "Interface language",
		"desc.workers":  "Number of worker threads",
		"desc.config":   "Configuration file path",
		"desc.verbose":  "Enable verbose output",

		// Additional messages
		"msg.starting": "Starting server on port %d with %d workers",
		"msg.language": "Using language: %s",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding English translations: %v\n", err)
		os.Exit(1)
	}

	// Japanese translations
	err = bundle.AddLanguage(language.Japanese, map[string]string{
		// Flag names - actual Japanese names!
		"flag.port":     "ポート",
		"flag.language": "言語",
		"flag.workers":  "ワーカー",
		"flag.config":   "設定",
		"flag.verbose":  "詳細",

		// Descriptions
		"desc.port":     "サーバーポート番号",
		"desc.language": "インターフェース言語",
		"desc.workers":  "ワーカースレッド数",
		"desc.config":   "設定ファイルパス",
		"desc.verbose":  "詳細出力を有効にする",

		// Additional messages
		"msg.starting": "ポート %d で %d ワーカーでサーバーを起動中",
		"msg.language": "使用言語: %s",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Japanese translations: %v\n", err)
		os.Exit(1)
	}

	// Arabic translations (RTL language)
	err = bundle.AddLanguage(language.Arabic, map[string]string{
		// Flag names in Arabic
		"flag.port":     "منفذ",
		"flag.language": "لغة",
		"flag.workers":  "عمال",
		"flag.config":   "تكوين",
		"flag.verbose":  "مفصل",

		// Descriptions
		"desc.port":     "رقم منفذ الخادم",
		"desc.language": "لغة الواجهة",
		"desc.workers":  "عدد خيوط العمل",
		"desc.config":   "مسار ملف التكوين",
		"desc.verbose":  "تمكين الإخراج المفصل",

		// Additional messages
		"msg.starting": "بدء الخادم على المنفذ %d مع %d عمال",
		"msg.language": "اللغة المستخدمة: %s",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Arabic translations: %v\n", err)
		os.Exit(1)
	}

	// Spanish translations
	err = bundle.AddLanguage(language.Spanish, map[string]string{
		// Flag names in Spanish
		"flag.port":     "puerto",
		"flag.language": "idioma",
		"flag.workers":  "trabajadores",
		"flag.config":   "configuración",
		"flag.verbose":  "detallado",

		// Descriptions
		"desc.port":     "Número de puerto del servidor",
		"desc.language": "Idioma de la interfaz",
		"desc.workers":  "Número de hilos de trabajo",
		"desc.config":   "Ruta del archivo de configuración",
		"desc.verbose":  "Habilitar salida detallada",

		// Additional messages
		"msg.starting": "Iniciando servidor en puerto %d con %d trabajadores",
		"msg.language": "Idioma en uso: %s",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Spanish translations: %v\n", err)
		os.Exit(1)
	}

	cfg := &ServerConfig{}
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Custom suggestions formatter for different languages
	parser.SetSuggestionsFormatter(func(suggestions []string) string {
		switch cfg.Language {
		case "ja":
			// Japanese style with 「」
			return "「" + suggestions[0] + "」"
		case "ar":
			// Arabic style
			return "«" + suggestions[0] + "»"
		case "es":
			// Spanish style with ¿?
			return "¿" + suggestions[0] + "?"
		default:
			// Default style
			return "[" + suggestions[0] + "]"
		}
	})

	// Parse with full i18n support
	success := parser.Parse(os.Args)
	langTag, err := language.Parse(cfg.Language)
	if err != nil {
		langTag = language.English
	}

	// Set the language on the parser
	parser.SetLanguage(langTag)
	if !success {
		parser.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Check if help was requested
	if parser.WasHelpShown() {
		os.Exit(0)
	}

	// Use the parsed values with localized messages
	translator := parser.GetTranslator()
	fmt.Println(translator.T("msg.language", cfg.Language))
	fmt.Println(translator.T("msg.starting", cfg.Port, cfg.Workers))

	if cfg.Verbose {
		fmt.Printf("Verbose mode enabled\n")
		fmt.Printf("Config file: %s\n", cfg.Config)
	}
}
