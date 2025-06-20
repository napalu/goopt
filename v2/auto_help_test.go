package goopt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestAutoHelp(t *testing.T) {
	tests := []struct {
		name           string
		setupParser    func() *Parser
		args           []string
		expectedHelp   bool
		expectedOutput string
	}{
		{
			name: "default auto-help with --help",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("verbose", &Argument{
					Short:       "v",
					Description: "Enable verbose output",
					TypeOf:      types.Standalone,
				})
				p.helpEndFunc = func() error {
					return nil
				}
				return p
			},
			args:           []string{"--help"},
			expectedHelp:   true,
			expectedOutput: "Show help information",
		},
		{
			name: "default auto-help with -h",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("verbose", &Argument{
					Short:       "v",
					Description: "Enable verbose output",
					TypeOf:      types.Standalone,
				})
				return p
			},
			args:           []string{"-h"},
			expectedHelp:   true,
			expectedOutput: "Show help information",
		},
		{
			name: "auto-help disabled",
			setupParser: func() *Parser {
				p, _ := NewParserWith(WithAutoHelp(false))
				_ = p.AddFlag("verbose", &Argument{
					Short:       "v",
					Description: "Enable verbose output",
					TypeOf:      types.Standalone,
				})
				return p
			},
			args:         []string{"--help"},
			expectedHelp: false,
		},
		{
			name: "custom help flags",
			setupParser: func() *Parser {
				p, _ := NewParserWith(WithHelpFlags("help", "?"))
				_ = p.AddFlag("verbose", &Argument{
					Short:       "v",
					Description: "Enable verbose output",
					TypeOf:      types.Standalone,
				})
				return p
			},
			args:           []string{"-?"},
			expectedHelp:   true,
			expectedOutput: "Show help information",
		},
		{
			name: "user-defined help flag takes precedence",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("host", &Argument{
					Short:       "h",
					Description: "Database host",
					TypeOf:      types.Single,
				})
				return p
			},
			args:         []string{"-h", "localhost"},
			expectedHelp: false,
		},
		{
			name: "help still works with long form when short is taken",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("host", &Argument{
					Short:       "h",
					Description: "Database host",
					TypeOf:      types.Single,
				})
				return p
			},
			args:           []string{"--help"},
			expectedHelp:   true,
			expectedOutput: "Show help information",
		},
		{
			name: "no help shown on parsing errors",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("port", &Argument{
					Description: "Port number",
					TypeOf:      types.Single,
					Required:    true,
				})
				return p
			},
			args:         []string{"--invalid-flag"},
			expectedHelp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setupParser()

			// Capture output
			var buf bytes.Buffer
			p.SetStdout(&buf)
			p.helpEndFunc = func() error {
				return nil
			}
			// Parse
			ok := p.Parse(tt.args)

			// Check if help was shown
			assert.Equal(t, tt.expectedHelp, p.WasHelpShown())

			// Check output contains expected text
			if tt.expectedOutput != "" {
				assert.Contains(t, buf.String(), tt.expectedOutput)
			}

			// If help was shown, parsing should still return true
			if tt.expectedHelp {
				assert.True(t, ok)
			}
		})
	}
}

func TestIsHelpRequested(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *Parser
		args     []string
		expected bool
	}{
		{
			name:     "help requested with --help",
			setup:    NewParser,
			args:     []string{"--help"},
			expected: true,
		},
		{
			name:     "help requested with -h",
			setup:    NewParser,
			args:     []string{"-h"},
			expected: true,
		},
		{
			name:     "help not requested",
			setup:    NewParser,
			args:     []string{"--verbose"},
			expected: false,
		},
		{
			name: "help not requested when auto-help disabled",
			setup: func() *Parser {
				p := NewParser()
				p.SetAutoHelp(false)
				return p
			},
			args:     []string{"--help"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setup()
			p.helpEndFunc = func() error {
				return nil
			}
			_ = p.Parse(tt.args)
			assert.Equal(t, tt.expected, p.IsHelpRequested())
		})
	}
}

func TestAutoHelpIntegration(t *testing.T) {
	// Test with struct-based parser
	type Config struct {
		Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
		Config  string `goopt:"short:c;desc:Configuration file"`

		Service struct {
			Start struct {
				Port int `goopt:"short:p;default:8080;desc:Service port"`
			} `goopt:"kind:command;desc:Start the service"`
		} `goopt:"kind:command;desc:Service management"`
	}

	cfg := &Config{}
	parser, err := NewParserFromStruct(cfg)
	assert.NoError(t, err)

	// Capture output
	var buf bytes.Buffer
	parser.SetStdout(&buf)
	parser.helpEndFunc = func() error {
		return nil
	}

	// Test help is shown
	ok := parser.Parse([]string{"--help"})
	assert.True(t, ok)
	assert.True(t, parser.WasHelpShown())

	output := buf.String()
	assert.Contains(t, output, "Enable verbose output")
	assert.Contains(t, output, "Configuration file")
	assert.Contains(t, output, "Service management")
}

func TestAutoHelpWithCommands(t *testing.T) {
	p := NewParser()

	// Add some commands
	_ = p.AddCommand(&Command{
		Name:        "serve",
		Description: "Start the server",
		Callback: func(p *Parser, c *Command) error {
			return nil
		},
	})

	_ = p.AddFlag("port", &Argument{
		Short:       "p",
		Description: "Server port",
		TypeOf:      types.Single,
	}, "serve")

	// Test help for specific command
	var buf bytes.Buffer
	p.SetStdout(&buf)
	p.helpEndFunc = func() error {
		return nil
	}
	ok := p.Parse([]string{"serve", "--help"})
	assert.True(t, ok)
	assert.True(t, p.WasHelpShown())

	// Should show help with serve command context
	output := buf.String()
	assert.Contains(t, output, "Server port")
}

func TestAutoHelpRespectsHelpStyle(t *testing.T) {
	tests := []struct {
		name           string
		helpStyle      HelpStyle
		expectedOutput []string
	}{
		{
			name:      "flat style",
			helpStyle: HelpStyleFlat,
			expectedOutput: []string{
				"--verbose",
				"--debug",
				"--config",
			},
		},
		{
			name:      "compact style",
			helpStyle: HelpStyleCompact,
			expectedOutput: []string{
				"Global Flags:",
				"--verbose, -v",
			},
		},
		{
			name:      "hierarchical style",
			helpStyle: HelpStyleHierarchical,
			expectedOutput: []string{
				"Global Flags:",
				"Examples:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewParserWith(
				WithHelpStyle(tt.helpStyle),
				WithFlag("verbose", NewArg(
					WithShortFlag("v"),
					WithDescription("Enable verbose output"),
				)),
				WithFlag("debug", NewArg(
					WithShortFlag("d"),
					WithDescription("Enable debug mode"),
				)),
				WithFlag("config", NewArg(
					WithShortFlag("c"),
					WithDescription("Configuration file"),
				)),
			)
			assert.NoError(t, err)

			var buf bytes.Buffer
			parser.SetStdout(&buf)
			parser.helpEndFunc = func() error {
				return nil
			}
			ok := parser.Parse([]string{"--help"})
			assert.True(t, ok)
			assert.True(t, parser.WasHelpShown())

			output := buf.String()
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestUserDefinedHelpFlag(t *testing.T) {
	tests := []struct {
		name           string
		setupParser    func() *Parser
		args           []string
		expectAutoHelp bool
		expectUserHelp bool
	}{
		{
			name: "user defines help without short flag",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("help", &Argument{
					Description: "Show my custom help",
					TypeOf:      types.Standalone,
				})
				return p
			},
			args:           []string{"--help"},
			expectAutoHelp: false,
			expectUserHelp: true,
		},
		{
			name: "user defines help with different short flag",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("help", &Argument{
					Short:       "?",
					Description: "Show my custom help",
					TypeOf:      types.Standalone,
				})
				return p
			},
			args:           []string{"-?"},
			expectAutoHelp: false,
			expectUserHelp: true,
		},
		{
			name: "user defines different flag with -h short",
			setupParser: func() *Parser {
				p := NewParser()
				_ = p.AddFlag("host", &Argument{
					Short:       "h",
					Description: "Database host",
					TypeOf:      types.Single,
				})
				return p
			},
			args:           []string{"--help"},
			expectAutoHelp: true,
			expectUserHelp: false,
		},
		{
			name: "struct tags define help flag",
			setupParser: func() *Parser {
				type Config struct {
					Help bool `goopt:"name:help;short:h;desc:Custom help message"`
				}
				cfg := &Config{}
				p, _ := NewParserFromStruct(cfg)
				return p
			},
			args:           []string{"--help"},
			expectAutoHelp: false,
			expectUserHelp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setupParser()

			var buf bytes.Buffer
			p.SetStdout(&buf)

			p.helpEndFunc = func() error {
				return nil
			}
			p.Parse(tt.args)

			// Check if auto-help was triggered
			assert.Equal(t, tt.expectAutoHelp, p.WasHelpShown())

			// Check if user help flag was set
			if tt.expectUserHelp {
				val, found := p.Get("help")
				assert.True(t, found)
				assert.Equal(t, "true", val)
			}

			// If auto-help was triggered, check output
			if tt.expectAutoHelp {
				assert.Contains(t, buf.String(), "Show help information")
			}
		})
	}
}

func TestHelpFlagPrecedence(t *testing.T) {
	// Test that user can override help flags completely
	type Config struct {
		Help    bool `goopt:"name:help;short:h;desc:My custom help"`
		Verbose bool `goopt:"short:v;desc:Verbose mode"`
	}

	cfg := &Config{}
	parser, err := NewParserFromStruct(cfg)
	assert.NoError(t, err)

	// Parse with help flag
	ok := parser.Parse([]string{"--help"})
	assert.True(t, ok)

	// Custom help flag should be set, auto-help should not trigger
	assert.True(t, cfg.Help)
	assert.False(t, parser.WasHelpShown())
}

func TestDisableAutoHelp(t *testing.T) {
	// Create parser with auto-help disabled
	parser, err := NewParserWith(
		WithAutoHelp(false),
		WithFlag("verbose", NewArg(WithShortFlag("v"))),
	)
	assert.NoError(t, err)

	// Try to use help flag
	ok := parser.Parse([]string{"--help"})
	assert.False(t, ok) // Should fail because --help is not defined
	assert.False(t, parser.WasHelpShown())

	// Check that help flag was not auto-registered
	_, err = parser.GetArgument("help")
	assert.Error(t, err)
}

func TestCustomHelpOutput(t *testing.T) {
	// Test that help respects the configured help style
	parser, err := NewParserWith(
		WithHelpStyle(HelpStyleCompact),
		WithFlag("verbose", NewArg(WithShortFlag("v"), WithDescription("Verbose output"))),
		WithFlag("debug", NewArg(WithShortFlag("d"), WithDescription("Debug mode"))),
	)
	assert.NoError(t, err)

	var buf bytes.Buffer
	parser.SetStdout(&buf)
	parser.helpEndFunc = func() error {
		return nil
	}
	ok := parser.Parse([]string{"--help"})
	assert.True(t, ok)
	assert.True(t, parser.WasHelpShown())

	// Should use compact style
	output := buf.String()
	assert.Contains(t, output, "Global Flags:")
	// In compact mode, should show condensed output
	lines := strings.Split(output, "\n")
	assert.Greater(t, len(lines), 3) // Should have multiple lines but be compact
}
