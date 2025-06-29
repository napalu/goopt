package main

import (
	"fmt"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
	"os"
	"strings"
	"time"
)

// Config demonstrates comprehensive i18n features including nameKey,
// locale-aware formatting, and fully translated UI
type Config struct {

	// Numeric flag to demonstrate locale formatting
	Port int `goopt:"short:p;default:8080;namekey:flag.port.name;desckey:flag.port.desc;validators:port"`

	// Large number to show thousands separators
	MaxConnections int `goopt:"default:1000000;namekey:flag.maxconn.name;desckey:flag.maxconn.desc"`

	// Commands with translated names
	Server struct {
		Workers    int    `goopt:"default:10000;namekey:flag.workers.name;desckey:flag.workers.desc;validators:range(100,100000)"`
		Timeout    int    `goopt:"default:30;namekey:flag.timeout.name;desckey:flag.timeout.desc"`
		ConfigFile string `goopt:"pos:0;namekey:pos.config.name;desckey:pos.config.desc"`
	} `goopt:"kind:command;namekey:cmd.server.name;desckey:cmd.server.desc"`

	Info struct{} `goopt:"kind:command;namekey:cmd.info.name;desckey:cmd.info.desc"`
}

func main() {
	// Create comprehensive translation bundle for user messages
	bundle := i18n.NewEmptyBundle()

	// English translations
	bundle.AddLanguage(language.English, map[string]string{
		// Flag names and descriptions
		"flag.language.name": "language",
		"flag.language.desc": "Interface language (en, de, fr)",
		"flag.port.name":     "port",
		"flag.port.desc":     "Server port number",
		"flag.maxconn.name":  "max-connections",
		"flag.maxconn.desc":  "Maximum concurrent connections",
		"flag.workers.name":  "workers",
		"flag.workers.desc":  "Number of worker threads",
		"flag.timeout.name":  "timeout",
		"flag.timeout.desc":  "Server timeout in seconds",

		// Command names and descriptions
		"cmd.server.name": "server",
		"cmd.server.desc": "Start the server",
		"cmd.info.name":   "info",
		"cmd.info.desc":   "Display system information",

		// Messages
		"msg.welcome":         "Welcome to the comprehensive i18n demo!",
		"msg.language.set":    "Language set to:",
		"msg.server.config":   "Server Configuration:",
		"msg.port":            "Port:",
		"msg.max.connections": "Max connections:",
		"msg.workers":         "Workers:",
		"msg.timeout":         "Timeout:",
		"msg.config.file":     "Config file:",
		"msg.current.time":    "Current time:",
		"msg.system.info":     "System Information",

		// Positional arguments
		"pos.config.name": "config-file",
		"pos.config.desc": "Configuration file path",
	})

	// German translations
	bundle.AddLanguage(language.German, map[string]string{
		"flag.language.name": "sprache",
		"flag.language.desc": "Schnittstellensprache (en, de, fr)",
		"flag.port.name":     "port",
		"flag.port.desc":     "Server-Portnummer",
		"flag.maxconn.name":  "max-verbindungen",
		"flag.maxconn.desc":  "Maximale gleichzeitige Verbindungen",
		"flag.workers.name":  "arbeiter",
		"flag.workers.desc":  "Anzahl der Arbeitsthreads",
		"flag.timeout.name":  "zeitlimit",
		"flag.timeout.desc":  "Server-Zeitlimit in Sekunden",

		"cmd.server.name": "server",
		"cmd.server.desc": "Server starten",
		"cmd.info.name":   "info",
		"cmd.info.desc":   "Systeminformationen anzeigen",

		"msg.welcome":         "Willkommen zur umfassenden i18n-Demo!",
		"msg.language.set":    "Sprache eingestellt auf:",
		"msg.server.config":   "Server-Konfiguration:",
		"msg.port":            "Port:",
		"msg.max.connections": "Max. Verbindungen:",
		"msg.workers":         "Arbeiter:",
		"msg.timeout":         "Zeitlimit:",
		"msg.config.file":     "Konfigurationsdatei:",
		"msg.current.time":    "Aktuelle Zeit:",
		"msg.system.info":     "Systeminformationen",

		// Positional arguments
		"pos.config.name": "konfig-datei",
		"pos.config.desc": "Konfigurationsdateipfad",
	})

	// French translations
	bundle.AddLanguage(language.French, map[string]string{
		"flag.language.name": "langue",
		"flag.language.desc": "Langue de l'interface (en, de, fr)",
		"flag.port.name":     "port",
		"flag.port.desc":     "Numéro de port du serveur",
		"flag.maxconn.name":  "connexions-max",
		"flag.maxconn.desc":  "Connexions simultanées maximales",
		"flag.workers.name":  "travailleurs",
		"flag.workers.desc":  "Nombre de threads de travail",
		"flag.timeout.name":  "délai",
		"flag.timeout.desc":  "Délai d'expiration du serveur en secondes",

		"cmd.server.name": "serveur",
		"cmd.server.desc": "Démarrer le serveur",
		"cmd.info.name":   "info",
		"cmd.info.desc":   "Afficher les informations système",

		"msg.welcome":         "Bienvenue dans la démo i18n complète!",
		"msg.language.set":    "Langue définie sur:",
		"msg.server.config":   "Configuration du serveur:",
		"msg.port":            "Port:",
		"msg.max.connections": "Connexions max:",
		"msg.workers":         "Travailleurs:",
		"msg.timeout":         "Délai:",
		"msg.config.file":     "Fichier de config:",
		"msg.current.time":    "Heure actuelle:",
		"msg.system.info":     "Informations système",

		// Positional arguments
		"pos.config.name": "fichier-config",
		"pos.config.desc": "Chemin du fichier de configuration",
	})

	fmt.Println("This demo shows how to use i18n features to create a comprehensive user interface.")
	fmt.Println("We'll exercise the interface to show the interactions of various commands in different languages")
	fmt.Println("including locale-aware formatting and translation of error messages as well as parsing of ")
	fmt.Println("fully translated flags and positional arguments.")

	var demoArgs [][]string
	demoArgs = append(demoArgs, []string{"--help"})
	demoArgs = append(demoArgs, []string{"server", "--port", "8080", "--max-connections", "20", "--workers", "10000", "--timeout", "30", "config.yaml"})
	demoArgs = append(demoArgs, []string{"server", "--prt", "8080", "--timeot", "30", "config.yaml"})
	demoArgs = append(demoArgs, []string{"-l", "de", "--help"})
	demoArgs = append(demoArgs, []string{"-l", "de", "server", "--port", "8080", "--max-verbindungen", "20", "--arbeiter", "10000", "--zeitlimit", "30", "config.yaml"})
	demoArgs = append(demoArgs, []string{"-l", "de", "servr", "--prt", "8080", "--max-verbindngen", "20", "--arbeitr", "10000", "--zetlimit", "30", "config.yaml"})
	demoArgs = append(demoArgs, []string{"-l", "fr", "--help"})
	demoArgs = append(demoArgs, []string{"-l", "fr", "serveur", "--port", "8080", "--connexions-max", "20", "--travailleurs", "10000", "--délai", "30", "config.yaml"})
	demoArgs = append(demoArgs, []string{"-l", "fr", "servur", "--port", "8080", "--connxions-max", "20", "--travilleurs", "10000", "--déla", "30", "config.yaml"})

	for _, items := range demoArgs {
		fmt.Printf("\n=========== Running demo with arguments: %s ===========\n\n", strings.Join(items, " "))
		// Create the parser with i18n
		cfg := &Config{}
		parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}
		parser.SetEndHelpFunc(func() error {
			// by default, the parser auto-exits after showing help - we override this for the demo
			return nil
		})

		success := parser.Parse(items)

		// Parse arguments
		if !success {
			// Print errors first (including "did you mean" suggestions)
			for _, err := range parser.GetErrors() {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			fmt.Fprintln(os.Stderr) // Empty line before help
			parser.PrintHelp(os.Stderr)
			continue
		}

		// If help was shown, exit
		if parser.WasHelpShown() {
			continue
		}

		// Get translator for both translation and locale-aware formatting
		translator := parser.GetTranslator()
		printer := translator.GetPrinter()

		fmt.Println(translator.T("msg.welcome"))
		printer.Printf("%s %s\n", translator.T("msg.language.set"), parser.GetLanguage())
		fmt.Println()

		// Handle commands
		if parser.HasCommand("server") {
			fmt.Println(translator.T("msg.server.config"))
			fmt.Printf("  %s %d\n", translator.T("msg.port"), cfg.Port)
			printer.Printf("  %s %d\n", translator.T("msg.max.connections"), cfg.MaxConnections)
			printer.Printf("  %s %d\n", translator.T("msg.workers"), cfg.Server.Workers)
			printer.Printf("  %s %d\n", translator.T("msg.timeout"), cfg.Server.Timeout)
			if cfg.Server.ConfigFile != "" {
				printer.Printf("  %s %s\n", translator.T("msg.config.file"), cfg.Server.ConfigFile)
			}
		} else if parser.HasCommand("info") {
			fmt.Println(translator.T("msg.system.info"))
			printer.Printf("  %s %s\n", translator.T("msg.current.time"), time.Now())
			printer.Printf("  %s %d\n", translator.T("msg.max.connections"), cfg.MaxConnections)
		}
	}

}
