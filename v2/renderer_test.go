package goopt

import (
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

// TestCommandName tests the CommandName renderer function
func TestCommandName(t *testing.T) {
	t.Run("command with name key", func(t *testing.T) {
		// Create a bundle with translations
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.English, map[string]string{
			"cmd.test": "Translated Test Command",
		})

		p := NewParser()
		err := p.SetUserBundle(bundle)
		assert.NoError(t, err)
		cmd := &Command{
			Name:    "test",
			NameKey: "cmd.test",
		}

		renderer := p.renderer
		name := renderer.CommandName(cmd)
		assert.Equal(t, "Translated Test Command", name)
	})

	t.Run("command without name key", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name: "test",
		}

		renderer := p.renderer
		name := renderer.CommandName(cmd)
		assert.Equal(t, "test", name)
	})

	t.Run("command with invalid name key", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:    "test",
			NameKey: "nonexistent.key",
		}

		renderer := p.renderer
		name := renderer.CommandName(cmd)
		// When key is not found, i18n returns the key itself
		assert.Equal(t, "nonexistent.key", name)
	})
}

// TestCommandUsage tests the CommandUsage renderer function
func TestCommandUsage(t *testing.T) {
	t.Run("command with description", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "test",
			Description: "Test command description",
		}

		renderer := p.renderer
		usage := renderer.CommandUsage(cmd)
		assert.Equal(t, `test "Test command description"`, usage)
	})

	t.Run("command with translated name and description", func(t *testing.T) {
		// Create a new bundle instead of modifying the global default
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.English, map[string]string{
			"cmd.test":      "translated-test",
			"cmd.test.desc": "Translated description",
		})

		p := NewParser()
		// Set the user bundle instead of modifying system bundle
		err := p.SetUserBundle(bundle)
		assert.NoError(t, err)

		cmd := &Command{
			Name:           "test",
			NameKey:        "cmd.test",
			Description:    "Default description",
			DescriptionKey: "cmd.test.desc",
		}

		renderer := p.renderer
		usage := renderer.CommandUsage(cmd)
		assert.Equal(t, `translated-test "Translated description"`, usage)
	})

	t.Run("command without description", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name: "test",
		}

		renderer := p.renderer
		usage := renderer.CommandUsage(cmd)
		assert.Equal(t, `test`, usage)
	})
}

// TestGetArgumentInfoByID tests the internal getArgumentInfoByID function
func TestGetArgumentInfoByID(t *testing.T) {
	t.Run("valid argument ID", func(t *testing.T) {
		p := NewParser()

		// Add a flag with ID
		arg := NewArg(WithDescription("Test flag"))
		err := p.AddFlag("test-flag", arg)
		assert.NoError(t, err)

		// The UUID is set internally by ensureInit()
		// Test getArgumentInfoByID
		info := p.getArgumentInfoByID(arg.uniqueID)
		assert.NotNil(t, info)
		assert.Equal(t, "Test flag", info.Argument.Description)
	})

	t.Run("invalid argument ID", func(t *testing.T) {
		p := NewParser()

		info := p.getArgumentInfoByID("nonexistent-id")
		assert.Nil(t, info)
	})

	t.Run("ID in lookup but not in acceptedFlags", func(t *testing.T) {
		p := NewParser()

		// Add ID to lookup but not to acceptedFlags
		p.lookup["orphan-id"] = "nonexistent-flag"

		info := p.getArgumentInfoByID("orphan-id")
		assert.Nil(t, info)
	})

	t.Run("empty ID", func(t *testing.T) {
		p := NewParser()

		info := p.getArgumentInfoByID("")
		assert.Nil(t, info)
	})
}

// TestLocaleFormattedDefaults tests that numeric default values are formatted according to locale
func TestLocaleFormattedDefaults(t *testing.T) {
	tests := []struct {
		name           string
		lang           language.Tag
		defaultValue   string
		expectedFormat string
	}{
		{
			name:           "English thousands",
			lang:           language.English,
			defaultValue:   "8080",
			expectedFormat: "8,080",
		},
		{
			name:           "French thousands",
			lang:           language.French,
			defaultValue:   "10000",
			expectedFormat: "10\u00a0000", // non-breaking space
		},
		{
			name:           "German thousands",
			lang:           language.German,
			defaultValue:   "1000000",
			expectedFormat: "1.000.000",
		},
		{
			name:           "English float",
			lang:           language.English,
			defaultValue:   "99.95",
			expectedFormat: "99.95",
		},
		{
			name:           "French float",
			lang:           language.French,
			defaultValue:   "1234.56",
			expectedFormat: "1\u00a0234,56",
		},
		{
			name:           "Non-numeric default",
			lang:           language.English,
			defaultValue:   "localhost",
			expectedFormat: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create bundle with locale
			bundle := i18n.NewEmptyBundle()
			bundle.AddLanguage(language.English, map[string]string{
				"goopt.msg.defaults_to": "defaults to",
			})
			bundle.AddLanguage(language.French, map[string]string{
				"goopt.msg.defaults_to": "par défaut",
			})
			bundle.AddLanguage(language.German, map[string]string{
				"goopt.msg.defaults_to": "Standard",
			})

			// Create parser
			p := NewParser()
			p.SetLanguage(tt.lang)
			p.SetHelpConfig(HelpConfig{
				ShowDefaults: true,
			})
			err := p.SetUserBundle(bundle)
			assert.NoError(t, err)

			// Create argument with default value
			arg := &Argument{
				DefaultValue: tt.defaultValue,
				TypeOf:       types.Single,
			}

			// Get formatted usage
			usage := p.renderer.FlagUsage(arg)

			// Check that the formatted default is included
			assert.Contains(t, usage, tt.expectedFormat)
		})
	}
}

// TestDefaultValueFormattingInHelp tests that help output shows locale-formatted defaults
func TestDefaultValueFormattingInHelp(t *testing.T) {
	// Create a config struct with numeric defaults
	type TestConfig struct {
		Port    int     `goopt:"default:8080;desc:Server port"`
		Workers int     `goopt:"default:10000;desc:Number of workers"`
		MaxConn int     `goopt:"default:1000000;desc:Maximum connections"`
		Rate    float64 `goopt:"default:99.95;desc:Success rate"`
	}

	// Test with French locale
	bundle := i18n.NewEmptyBundle()
	bundle.AddLanguage(language.English, map[string]string{
		"goopt.msg.defaults_to": "defaults to",
		"goopt.msg.optional":    "optional",
	})
	bundle.AddLanguage(language.French, map[string]string{
		"goopt.msg.defaults_to": "par défaut",
		"goopt.msg.optional":    "optionnel",
	})

	cfg := &TestConfig{}
	parser, err := NewParserFromStruct(cfg, WithUserBundle(bundle))
	assert.NoError(t, err)
	parser.SetLanguage(language.French)
	parser.SetHelpConfig(HelpConfig{
		ShowDefaults:    true,
		ShowDescription: true,
	})

	// Capture help output
	var helpOutput strings.Builder
	parser.PrintHelp(&helpOutput)
	help := helpOutput.String()

	// Verify locale-formatted numbers appear in help
	assert.Contains(t, help, "8\u00a0080")          // 8,080 with French formatting
	assert.Contains(t, help, "10\u00a0000")         // 10,000 with French formatting
	assert.Contains(t, help, "1\u00a0000\u00a0000") // 1,000,000 with French formatting
	assert.Contains(t, help, "99,95")               // 99.95 with French decimal separator
	assert.Contains(t, help, "par défaut")          // French translation
}

// TestContainsRTLRunes tests RTL character detection
func TestContainsRTLRunes(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{"Arabic text", "مرحبا", true},
		{"Hebrew text", "שלום", true},
		{"Mixed Arabic and English", "Hello مرحبا", true},
		{"Mixed Hebrew and English", "Hello שלום", true},
		{"English only", "Hello World", false},
		{"Numbers", "12345", false},
		{"Latin special chars", "café", false},
		{"Chinese", "你好", false},
		{"Japanese", "こんにちは", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			renderer := p.renderer.(*DefaultRenderer)
			assert.Equal(t, tt.expected, renderer.containsRTLRunes(tt.text))
		})
	}
}

// TestFlagUsageRTL tests flag usage formatting for RTL languages
func TestFlagUsageRTL(t *testing.T) {
	t.Run("Arabic flag usage", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		// Add English first
		bundle.AddLanguage(language.English, map[string]string{
			"flag.server":           "server",
			"desc.server":           "Server address",
			"goopt.msg.defaults_to": "defaults to",
			"goopt.msg.required":    "required",
			"goopt.msg.optional":    "optional",
			"goopt.msg.conditional": "conditional",
		})
		bundle.AddLanguage(language.Arabic, map[string]string{
			"flag.server":           "خادم",
			"desc.server":           "عنوان الخادم",
			"goopt.msg.defaults_to": "الافتراضي",
			"goopt.msg.required":    "مطلوب",
			"goopt.msg.optional":    "اختياري",
			"goopt.msg.conditional": "مشروط",
		})

		p := NewParser()

		p.SetHelpConfig(HelpConfig{
			ShowShortFlags:  true,
			ShowDescription: true,
			ShowDefaults:    true,
			ShowRequired:    true,
		})
		err := p.SetUserBundle(bundle)
		p.SetLanguage(language.Arabic)
		assert.NoError(t, err)
		p.SetHelpConfig(HelpConfig{
			ShowShortFlags:  true,
			ShowDescription: true,
			ShowDefaults:    true,
			ShowRequired:    true,
		})
		err = p.SetUserBundle(bundle)
		assert.NoError(t, err)

		arg := &Argument{
			Short:          "s",
			NameKey:        "flag.server",
			Description:    "Server address",
			DescriptionKey: "desc.server",
			DefaultValue:   "localhost",
			TypeOf:         types.Single,
		}

		usage := p.renderer.FlagUsage(arg)
		// In RTL, description comes first, then the flag
		assert.Contains(t, usage, "عنوان الخادم")
		assert.Contains(t, usage, "--خادم / -s")
		assert.Contains(t, usage, "الافتراضي: localhost")
	})

	t.Run("English flag usage", func(t *testing.T) {
		p := NewParser()
		p.SetHelpConfig(HelpConfig{
			ShowShortFlags:  true,
			ShowDescription: true,
			ShowDefaults:    true,
			ShowRequired:    true,
		})

		arg := &Argument{
			Short:        "s",
			Description:  "Server address",
			DefaultValue: "localhost",
			TypeOf:       types.Single,
		}

		usage := p.renderer.FlagUsage(arg)
		// In LTR, flag comes first, then description
		assert.Contains(t, usage, "--")
		assert.Contains(t, usage, " or -s")
		assert.Contains(t, usage, "Server address")
		assert.Contains(t, usage, "defaults to: localhost")
	})

	t.Run("Hebrew flag with translated name", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		// Add English first
		bundle.AddLanguage(language.English, map[string]string{
			"flag.port": "port",
			"desc.port": "Port number",
		})
		bundle.AddLanguage(language.Hebrew, map[string]string{
			"flag.port": "פורט",
			"desc.port": "מספר הפורט",
		})

		p := NewParser()

		p.SetHelpConfig(HelpConfig{
			ShowShortFlags:  true,
			ShowDescription: true,
		})
		err := p.SetUserBundle(bundle)
		p.SetLanguage(language.Hebrew)
		assert.NoError(t, err)

		arg := &Argument{
			Short:          "p",
			NameKey:        "flag.port",
			DescriptionKey: "desc.port",
			TypeOf:         types.Single,
		}

		usage := p.renderer.FlagUsage(arg)
		// Should use RTL format
		assert.Contains(t, usage, "מספר הפורט")
		assert.Contains(t, usage, "--פורט / -p")
	})
}

// TestCommandUsageRTL tests command usage formatting for RTL languages
func TestCommandUsageRTL(t *testing.T) {
	t.Run("Arabic command usage", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		// Add English first
		bundle.AddLanguage(language.English, map[string]string{
			"cmd.start":      "start",
			"cmd.start.desc": "Start the server",
		})
		bundle.AddLanguage(language.Arabic, map[string]string{
			"cmd.start":      "ابدأ",
			"cmd.start.desc": "بدء الخادم",
		})

		p := NewParser()

		p.SetHelpConfig(HelpConfig{
			ShowDescription: true,
		})
		err := p.SetUserBundle(bundle)
		p.SetLanguage(language.Arabic)
		assert.NoError(t, err)

		cmd := &Command{
			Name:           "start",
			NameKey:        "cmd.start",
			DescriptionKey: "cmd.start.desc",
		}

		usage := p.renderer.CommandUsage(cmd)
		// In RTL, description comes first
		assert.Equal(t, "بدء الخادم :ابدأ", usage)
	})

	t.Run("English command usage", func(t *testing.T) {
		p := NewParser()
		p.SetHelpConfig(HelpConfig{
			ShowDescription: true,
		})

		cmd := &Command{
			Name:        "start",
			Description: "Start the server",
		}

		usage := p.renderer.CommandUsage(cmd)
		// In LTR, command comes first (with quotes for backward compatibility)
		assert.Equal(t, "start \"Start the server\"", usage)
	})
}

// TestCommandUsageWithPositionals tests CommandUsage with positional arguments
func TestCommandUsageWithPositionals(t *testing.T) {
	t.Run("command with single required positional", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "keygen",
			Description: "Generate keypair",
			path:        "keygen",
		}

		// Add a positional argument
		err := p.AddFlag("prefix", NewArg(
			WithType(types.Single),
			WithDescription("Key prefix"),
			WithRequired(true),
			WithPosition(0),
		), "keygen")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		assert.Contains(t, usage, "keygen <prefix>")
		assert.Contains(t, usage, "Generate keypair")
	})

	t.Run("command with single optional positional", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "deploy",
			Description: "Deploy application",
			path:        "deploy",
		}

		// Add an optional positional argument
		err := p.AddFlag("config", NewArg(
			WithType(types.Single),
			WithDescription("Config file"),
			WithRequired(false),
			WithPosition(0),
		), "deploy")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		assert.Contains(t, usage, "deploy [config]")
		assert.Contains(t, usage, "Deploy application")
	})

	t.Run("command with multiple positionals", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "verify",
			Description: "Verify signature",
			path:        "verify",
		}

		// Add multiple positional arguments
		err := p.AddFlag("message", NewArg(
			WithType(types.Single),
			WithDescription("Message"),
			WithRequired(true),
			WithPosition(0),
		), "verify")
		assert.NoError(t, err)

		err = p.AddFlag("signature", NewArg(
			WithType(types.Single),
			WithDescription("Signature"),
			WithRequired(true),
			WithPosition(1),
		), "verify")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		assert.Contains(t, usage, "verify <message> <signature>")
		assert.Contains(t, usage, "Verify signature")
	})

	t.Run("command with mixed required and optional positionals", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "copy",
			Description: "Copy files",
			path:        "copy",
		}

		// Add mixed positionals
		err := p.AddFlag("source", NewArg(
			WithType(types.Single),
			WithRequired(true),
			WithPosition(0),
		), "copy")
		assert.NoError(t, err)

		err = p.AddFlag("dest", NewArg(
			WithType(types.Single),
			WithRequired(true),
			WithPosition(1),
		), "copy")
		assert.NoError(t, err)

		err = p.AddFlag("options", NewArg(
			WithType(types.Single),
			WithRequired(false),
			WithPosition(2),
		), "copy")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		assert.Contains(t, usage, "copy <source> <dest> [options]")
		assert.Contains(t, usage, "Copy files")
	})

	t.Run("command with positionals strips command path from flag name", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "process",
			Description: "Process data",
			path:        "process",
		}

		// The flag gets stored as "input@process" internally
		err := p.AddFlag("input", NewArg(
			WithType(types.Single),
			WithRequired(true),
			WithPosition(0),
		), "process")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		// Should show "input" not "input@process"
		assert.Contains(t, usage, "process <input>")
		assert.NotContains(t, usage, "@process")
	})

	t.Run("command without positionals shows just name", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "status",
			Description: "Show status",
			path:        "status",
		}

		usage := p.renderer.CommandUsage(cmd)
		assert.Equal(t, `status "Show status"`, usage)
	})

	t.Run("positionals are sorted by position", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "build",
			Description: "Build project",
			path:        "build",
		}

		// Add positionals in non-sequential order
		err := p.AddFlag("output", NewArg(
			WithType(types.Single),
			WithRequired(true),
			WithPosition(2),
		), "build")
		assert.NoError(t, err)

		err = p.AddFlag("input", NewArg(
			WithType(types.Single),
			WithRequired(true),
			WithPosition(0),
		), "build")
		assert.NoError(t, err)

		err = p.AddFlag("config", NewArg(
			WithType(types.Single),
			WithRequired(false),
			WithPosition(1),
		), "build")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		// Should be sorted: input (0), config (1), output (2)
		assert.Contains(t, usage, "build <input> [config] <output>")
	})

	t.Run("command with positionals in RTL", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.Arabic, map[string]string{})

		p := NewParser()
		err := p.SetUserBundle(bundle)
		assert.NoError(t, err)
		p.SetLanguage(language.Arabic)

		cmd := &Command{
			Name:        "ابدأ",
			Description: "بدء الخادم",
			path:        "ابدأ",
		}

		err = p.AddFlag("ملف", NewArg(
			WithType(types.Single),
			WithRequired(true),
			WithPosition(0),
		), "ابدأ")
		assert.NoError(t, err)

		usage := p.renderer.CommandUsage(cmd)
		// In RTL, description comes first
		assert.Contains(t, usage, "بدء الخادم")
		assert.Contains(t, usage, "ابدأ")
		assert.Contains(t, usage, "<ملف>")
	})
}

// TestFlagUsageWithRequiredIf tests FlagUsage with conditional requirements
func TestFlagUsageWithRequiredIf(t *testing.T) {
	t.Run("flag with RequiredIf shows conditional", func(t *testing.T) {
		p := NewParser()
		p.SetHelpConfig(HelpConfig{
			ShowShortFlags:  false,
			ShowDescription: true,
			ShowDefaults:    false,
			ShowRequired:    true,
		})

		// Create a flag with RequiredIf condition
		arg := &Argument{
			Description: "Config file path",
			TypeOf:      types.Single,
			RequiredIf: func(cmdLine *Parser, optionName string) (bool, string) {
				return true, "production flag is set"
			},
		}

		usage := p.renderer.FlagUsage(arg)
		// Should show "conditional" instead of "required" or "optional"
		assert.Contains(t, usage, "conditional")
		assert.Contains(t, usage, "Config file path")
	})

	t.Run("flag with RequiredIf in different locale", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.German, map[string]string{
			"goopt.msg.conditional": "bedingt",
		})

		p := NewParser()
		err := p.SetUserBundle(bundle)
		assert.NoError(t, err)
		p.SetLanguage(language.German)
		p.SetHelpConfig(HelpConfig{
			ShowShortFlags:  false,
			ShowDescription: true,
			ShowDefaults:    false,
			ShowRequired:    true,
		})

		arg := &Argument{
			Description: "Konfigurationsdatei",
			TypeOf:      types.Single,
			RequiredIf: func(cmdLine *Parser, optionName string) (bool, string) {
				return true, "production flag is set"
			},
		}

		usage := p.renderer.FlagUsage(arg)
		// Should show German translation of "conditional"
		assert.Contains(t, usage, "bedingt")
		assert.Contains(t, usage, "Konfigurationsdatei")
	})
}

// TestMixedRTLContent tests handling of mixed RTL/LTR content
func TestMixedRTLContent(t *testing.T) {
	t.Run("RTL flag name with Latin short flag", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		// Add English first
		bundle.AddLanguage(language.English, map[string]string{
			"flag.verbose": "verbose",
		})
		bundle.AddLanguage(language.Arabic, map[string]string{
			"flag.verbose": "مفصل",
		})

		p := NewParser()

		p.SetHelpConfig(HelpConfig{
			ShowShortFlags: true,
		})
		err := p.SetUserBundle(bundle)
		p.SetLanguage(language.Arabic)
		assert.NoError(t, err)

		arg := &Argument{
			Short:   "v", // Latin character
			NameKey: "flag.verbose",
			TypeOf:  types.Standalone,
		}

		usage := p.renderer.FlagUsage(arg)
		// Should handle mixed content appropriately
		assert.Contains(t, usage, "--مفصل / -v")
	})
}
