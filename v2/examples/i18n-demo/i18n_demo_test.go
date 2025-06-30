package main

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
	"testing"
)

func TestI18nDemoPositional(t *testing.T) {
	// Exact structure from i18n-full-demo
	type Config struct {
		Port           int `goopt:"short:p;default:8080;namekey:flag.port.name;desckey:flag.port.desc;validators:port"`
		MaxConnections int `goopt:"default:1000000;namekey:flag.maxconn.name;desckey:flag.maxconn.desc"`

		Server struct {
			Workers    int    `goopt:"default:10000;namekey:flag.workers.name;desckey:flag.workers.desc;validators:range(100,100000)"`
			Timeout    int    `goopt:"default:30;namekey:flag.timeout.name;desckey:flag.timeout.desc"`
			ConfigFile string `goopt:"pos:0;namekey:pos.config.name;desckey:pos.config.desc"`
		} `goopt:"kind:command;namekey:cmd.server.name;desckey:cmd.server.desc"`

		Info struct{} `goopt:"kind:command;namekey:cmd.info.name;desckey:cmd.info.desc"`
	}

	// Create bundle with translations
	bundle := i18n.NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"flag.port.name":    "port",
		"flag.port.desc":    "Server port number",
		"flag.maxconn.name": "max-connections",
		"flag.maxconn.desc": "Maximum concurrent connections",
		"flag.workers.name": "workers",
		"flag.workers.desc": "Number of worker threads",
		"flag.timeout.name": "timeout",
		"flag.timeout.desc": "Server timeout in seconds",
		"cmd.server.name":   "server",
		"cmd.server.desc":   "Start the server",
		"cmd.info.name":     "info",
		"cmd.info.desc":     "Display system information",
		"pos.config.name":   "config-file",
		"pos.config.desc":   "Configuration file path",
	})

	cfg := &Config{}
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Exact args from demo
	args := []string{"server", "--port", "8080", "--max-connections", "20", "--workers", "10000", "--timeout", "30", "config.yaml"}

	success := parser.Parse(args)
	if !success {
		t.Fatalf("Parse failed: %v", parser.GetErrors())
	}

	// Check all values
	if cfg.Port != 8080 {
		t.Errorf("Expected Port=8080, got %d", cfg.Port)
	}
	if cfg.MaxConnections != 20 {
		t.Errorf("Expected MaxConnections=20, got %d", cfg.MaxConnections)
	}
	if cfg.Server.Workers != 10000 {
		t.Errorf("Expected Workers=10000, got %d", cfg.Server.Workers)
	}
	if cfg.Server.Timeout != 30 {
		t.Errorf("Expected Timeout=30, got %d", cfg.Server.Timeout)
	}
	if cfg.Server.ConfigFile != "config.yaml" {
		t.Errorf("Expected ConfigFile='config.yaml', got '%s'", cfg.Server.ConfigFile)
	}

}
