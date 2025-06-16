package goopt

import (
	"bytes"
	"github.com/napalu/goopt/v2/types"
	"strings"
	"testing"
)

func TestHelpConfigRespected(t *testing.T) {
	tests := []struct {
		name        string
		config      HelpConfig
		setupParser func(*Parser)
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "ShowDescription false",
			config: HelpConfig{
				Style:           HelpStyleFlat,
				ShowDescription: false,
				ShowDefaults:    true,
				ShowShortFlags:  true,
				ShowRequired:    true,
			},
			setupParser: func(p *Parser) {
				p.AddFlag("verbose", &Argument{
					Short:        "v",
					Description:  "Enable verbose output",
					DefaultValue: "false",
					TypeOf:       types.Standalone,
				})
			},
			checkOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "Enable verbose output") {
					t.Error("Description should not be shown when ShowDescription is false")
				}
				if !strings.Contains(output, "--verbose") {
					t.Error("Flag name should be shown")
				}
			},
		},
		{
			name: "ShowDefaults false",
			config: HelpConfig{
				Style:           HelpStyleFlat,
				ShowDescription: true,
				ShowDefaults:    false,
				ShowShortFlags:  true,
				ShowRequired:    true,
			},
			setupParser: func(p *Parser) {
				p.AddFlag("port", &Argument{
					Short:        "p",
					Description:  "Server port",
					DefaultValue: "8080",
					TypeOf:       types.Single,
				})
			},
			checkOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "defaults to: 8080") {
					t.Error("Default value should not be shown when ShowDefaults is false")
				}
				if !strings.Contains(output, "Server port") {
					t.Error("Description should be shown")
				}
			},
		},
		{
			name: "ShowShortFlags false",
			config: HelpConfig{
				Style:           HelpStyleFlat,
				ShowDescription: true,
				ShowDefaults:    true,
				ShowShortFlags:  false,
				ShowRequired:    true,
			},
			setupParser: func(p *Parser) {
				p.AddFlag("config", &Argument{
					Short:       "c",
					Description: "Config file",
					TypeOf:      types.Single,
				})
			},
			checkOutput: func(t *testing.T, output string) {
				if strings.Contains(output, " -c") || strings.Contains(output, "or -c") {
					t.Error("Short flag should not be shown when ShowShortFlags is false")
				}
				if !strings.Contains(output, "--config") {
					t.Error("Long flag should be shown")
				}
			},
		},
		{
			name: "ShowRequired false",
			config: HelpConfig{
				Style:           HelpStyleFlat,
				ShowDescription: true,
				ShowDefaults:    true,
				ShowShortFlags:  true,
				ShowRequired:    false,
			},
			setupParser: func(p *Parser) {
				p.AddFlag("apikey", &Argument{
					Description: "API key",
					Required:    true,
					TypeOf:      types.Single,
				})
			},
			checkOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "(required)") {
					t.Error("Required indicator should not be shown when ShowRequired is false")
				}
			},
		},
		{
			name: "MaxGlobals limit",
			config: HelpConfig{
				Style:           HelpStyleFlat,
				ShowDescription: true,
				ShowDefaults:    true,
				ShowShortFlags:  true,
				ShowRequired:    true,
				MaxGlobals:      2,
			},
			setupParser: func(p *Parser) {
				// Add more than MaxGlobals flags
				for i := 0; i < 5; i++ {
					p.AddFlag(strings.ToLower(string(rune('a'+i)))+"flag", &Argument{
						Description: "Test flag",
						TypeOf:      types.Single,
					})
				}
			},
			checkOutput: func(t *testing.T, output string) {
				// This test would need PrintGlobalFlags to be called
				// For now, we'll skip this specific check
			},
		},
		{
			name: "Commands respect ShowDescription",
			config: HelpConfig{
				Style:           HelpStyleFlat,
				ShowDescription: false,
				ShowDefaults:    true,
				ShowShortFlags:  true,
				ShowRequired:    true,
			},
			setupParser: func(p *Parser) {
				p.AddCommand(&Command{
					Name:        "test",
					Description: "Test command description",
				})
			},
			checkOutput: func(t *testing.T, output string) {
				if strings.Contains(output, "Test command description") {
					t.Error("Command description should not be shown when ShowDescription is false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			parser.SetHelpConfig(tt.config)
			tt.setupParser(parser)

			// Test PrintUsage
			var buf bytes.Buffer
			parser.PrintUsage(&buf)
			output := buf.String()
			tt.checkOutput(t, output)

			// Test PrintHelp
			buf.Reset()
			parser.PrintHelp(&buf)
			output = buf.String()
			tt.checkOutput(t, output)
		})
	}
}

func TestPrintGlobalFlagsRespectsMaxGlobals(t *testing.T) {
	parser := NewParser()
	parser.SetHelpConfig(HelpConfig{
		Style:           HelpStyleFlat,
		ShowDescription: true,
		ShowDefaults:    true,
		ShowShortFlags:  true,
		ShowRequired:    true,
		MaxGlobals:      3,
	})

	// Add more flags than MaxGlobals
	for i := 0; i < 10; i++ {
		parser.AddFlag(strings.ToLower(string(rune('a'+i)))+"flag", &Argument{
			Description: "Test flag " + string(rune('A'+i)),
			TypeOf:      types.Single,
		})
	}

	var buf bytes.Buffer
	parser.PrintGlobalFlags(&buf)
	output := buf.String()

	// Count how many flags are shown
	flagCount := strings.Count(output, "--")
	if flagCount > 3 {
		t.Errorf("Expected at most 3 flags, but got %d", flagCount)
	}

	// Check for "more" indicator
	if !strings.Contains(output, "more") {
		t.Error("Expected 'more' indicator when flags are truncated")
	}
}
