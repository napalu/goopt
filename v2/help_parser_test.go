package goopt

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/errs"

	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutput captures both stdout and stderr
type TestOutput struct {
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

// setupTestParser creates a parser with test output capture
func setupTestParser(t *testing.T) (*Parser, *TestOutput) {
	p := NewParser()

	// Set smart help behavior so error context goes to stderr
	p.SetHelpBehavior(HelpBehaviorSmart)

	output := &TestOutput{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	p.SetStdout(output.Stdout)
	p.SetStderr(output.Stderr)

	// Prevent actual exit
	p.helpEndFunc = func() error {
		return nil
	}

	return p, output
}

func TestHelpParser_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		expectStderr   bool
		expectedOutput []string
	}{
		{
			name:         "invalid command shows error",
			args:         []string{"invalid-command", "--help"},
			expectError:  true,
			expectStderr: true,
			expectedOutput: []string{
				"Error: Unknown command 'invalid-command'",
				"Available commands:",
			},
		},
		{
			name:         "valid help request uses stdout",
			args:         []string{"--help"},
			expectError:  false,
			expectStderr: false,
			expectedOutput: []string{
				"Usage:",
			},
		},
		{
			name:         "help with valid command uses stdout",
			args:         []string{"serve", "--help"},
			expectError:  false,
			expectStderr: false,
			expectedOutput: []string{
				"serve:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, output := setupTestParser(t)

			// Add some test commands
			_ = p.AddCommand(&Command{
				Name:        "serve",
				Description: "Start the server",
			})
			_ = p.AddCommand(&Command{
				Name:        "test",
				Description: "Run tests",
			})

			// Use improved help parser
			helpParser := NewHelpParser(p, p.helpConfig)
			// Override the internal parser's helpEndFunc to prevent exit in tests
			helpParser.hp.helpEndFunc = func() error {
				return nil
			}
			err := helpParser.Parse(tt.args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check output location
			if tt.expectStderr {
				stderrOutput := output.Stderr.String()
				assert.NotEmpty(t, stderrOutput, "Expected stderr output")
				for _, expected := range tt.expectedOutput {
					assert.Contains(t, stderrOutput, expected)
				}
			} else {
				stdoutOutput := output.Stdout.String()
				assert.NotEmpty(t, stdoutOutput, "Expected stdout output")
				for _, expected := range tt.expectedOutput {
					assert.Contains(t, stdoutOutput, expected)
				}
			}
		})
	}
}

func TestHelpParser_HelpModes(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput []string
		notExpected    []string
	}{
		{
			name: "globals mode shows only global flags",
			args: []string{"--help", "globals"},
			expectedOutput: []string{
				"Global flags",
			},
			notExpected: []string{
				"Commands:",
				"serve:",
			},
		},
		{
			name: "commands mode shows only commands",
			args: []string{"--help", "commands"},
			expectedOutput: []string{
				"serve",
				"test",
			},
			notExpected: []string{
				"--verbose",
			},
		},
		{
			name: "examples mode shows examples",
			args: []string{"--help", "examples"},
			expectedOutput: []string{
				"Examples:",
				"Show this help",
				"--help",
			},
		},
		{
			name: "search mode finds matching items",
			args: []string{"--help", "--search", "verbose"},
			expectedOutput: []string{
				"Search results for 'verbose':",
				"--verbose",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, output := setupTestParser(t)

			// Add test data
			_ = p.AddFlag("verbose", &Argument{
				Short:       "v",
				Description: "Enable verbose output",
				TypeOf:      types.Standalone,
			})

			_ = p.AddCommand(&Command{
				Name:        "serve",
				Description: "Start the server",
			})

			_ = p.AddCommand(&Command{
				Name:        "test",
				Description: "Run tests",
			})

			// Parse help
			helpParser := NewHelpParser(p, p.helpConfig)
			// Override the internal parser's helpEndFunc to prevent exit in tests
			helpParser.hp.helpEndFunc = func() error {
				return nil
			}
			err := helpParser.Parse(tt.args)
			require.NoError(t, err)

			stdoutOutput := output.Stdout.String()

			// Check expected content
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, stdoutOutput, expected)
			}

			// Check not expected content
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, stdoutOutput, notExpected)
			}
		})
	}
}

func TestHelpParser_RuntimeOptions(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "show defaults option",
			args: []string{"--help", "--show-defaults"},
			checkFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, "(defaults to: 8080)")
			},
		},
		{
			name: "no descriptions option",
			args: []string{"--help", "--no-desc"},
			checkFunc: func(t *testing.T, output string) {
				assert.NotContains(t, output, "Server port")
			},
		},
		{
			name: "filter option",
			args: []string{"--help", "--filter", "serve.*"},
			checkFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, "--serve.port")
				assert.NotContains(t, output, "--verbose")
			},
		},
		{
			name: "show types option",
			args: []string{"--help", "--show-types"},
			checkFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, "(single)")
				assert.Contains(t, output, "(standalone)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, output := setupTestParser(t)

			// Add test flags
			_ = p.AddFlag("verbose", &Argument{
				Short:       "v",
				Description: "Enable verbose output",
				TypeOf:      types.Standalone,
			})

			_ = p.AddFlag("serve.port", &Argument{
				Short:        "p",
				Description:  "Server port",
				TypeOf:       types.Single,
				DefaultValue: "8080",
			})

			// Parse help
			helpParser := NewHelpParser(p, p.helpConfig)
			// Override the internal parser's helpEndFunc to prevent exit in tests
			helpParser.hp.helpEndFunc = func() error {
				return nil
			}
			err := helpParser.Parse(tt.args)
			require.NoError(t, err)

			stdoutOutput := output.Stdout.String()
			tt.checkFunc(t, stdoutOutput)
		})
	}
}

func TestHelpParser_SimilarCommands(t *testing.T) {
	p, output := setupTestParser(t)

	// Add commands with similar names
	_ = p.AddCommand(&Command{Name: "serve", Description: "Start server"})
	_ = p.AddCommand(&Command{Name: "server", Description: "Server management"})
	_ = p.AddCommand(&Command{Name: "service", Description: "Service control"})
	_ = p.AddCommand(&Command{Name: "test", Description: "Run tests"})

	// Try invalid command similar to existing ones
	helpParser := NewHelpParser(p, p.helpConfig)
	helpParser.SetContext(HelpContextError)

	err := helpParser.Parse([]string{"serv"})
	assert.Error(t, err)

	stderrOutput := output.Stderr.String()

	// Should suggest similar commands
	assert.Contains(t, stderrOutput, "Did you mean:")
	assert.Contains(t, stderrOutput, "serve")
	assert.Contains(t, stderrOutput, "server")
	// Should show suggestions (at least 2 since serve is very similar to serv)
	assert.True(t, strings.Count(stderrOutput, "  ") >= 2)
}

func TestHelpWithInvalidSubcommand(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectHelp    bool
		expectError   bool
		errorContains []string
	}{
		{
			name:          "valid command with invalid subcommand and help",
			args:          []string{"users", "seach", "--help"},
			expectHelp:    true, // Help should be shown with errors
			expectError:   false,
			errorContains: []string{"users seach"},
		},
		{
			name:          "invalid root command with help",
			args:          []string{"usees", "seach", "--help"},
			expectHelp:    true,  // Help should be shown (improved parser handles it)
			expectError:   false, // Main parser doesn't error on unknown root commands
			errorContains: []string{},
		},
		{
			name:        "valid command with help",
			args:        []string{"users", "--help"},
			expectHelp:  true,
			expectError: false,
		},
		{
			name:        "help only",
			args:        []string{"--help"},
			expectHelp:  true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create parser (improved help is now the default)
			p, _ := NewParserWith(
				WithAutoHelp(true),
			)

			// Override help end function to prevent os.Exit in tests
			p.helpEndFunc = func() error {
				return nil
			}

			// Register commands
			_ = p.AddCommand(&Command{
				Name:        "users",
				Description: "User management",
				Callback: func(_ *Parser, _ *Command) error {
					return nil
				},
				Subcommands: []Command{
					{
						Name:        "search",
						Description: "Search users",
						Callback: func(_ *Parser, _ *Command) error {
							return nil
						},
					},
				},
			})

			_ = p.AddCommand(&Command{
				Name:        "groups",
				Description: "Group management",
				Callback: func(_ *Parser, _ *Command) error {
					return nil
				},
			})

			// Capture output
			var stdout, stderr bytes.Buffer
			p.SetStdout(&stdout)
			p.SetStderr(&stderr)

			// Parse
			success := p.Parse(tt.args)

			// Check expectations
			if tt.expectError && success {
				t.Errorf("Expected parsing to fail, but it succeeded")
			}
			if !tt.expectError && !success {
				t.Errorf("Expected parsing to succeed, but it failed with errors: %v", p.GetErrors())
			}

			if tt.expectHelp && !p.WasHelpShown() {
				t.Errorf("Expected help to be shown, but it wasn't")
			}
			if !tt.expectHelp && p.WasHelpShown() {
				t.Errorf("Expected help not to be shown, but it was")
			}

			// Check error messages
			if tt.expectError {
				errorStr := fmt.Sprintf("%v", p.GetErrors())
				for _, expected := range tt.errorContains {
					if !strings.Contains(errorStr, expected) {
						t.Errorf("Expected error to contain %q, but got: %v", expected, p.GetErrors())
					}
				}
			}
		})
	}
}

func TestHelpBehavior(t *testing.T) {
	tests := []struct {
		name         string
		behavior     HelpBehavior
		isError      bool
		expectStderr bool
	}{
		{
			name:         "default behavior uses stdout",
			behavior:     HelpBehaviorStdout,
			isError:      true,
			expectStderr: false,
		},
		{
			name:         "smart behavior uses stdout for normal help",
			behavior:     HelpBehaviorSmart,
			isError:      false,
			expectStderr: false,
		},
		{
			name:         "smart behavior uses stderr for error help",
			behavior:     HelpBehaviorSmart,
			isError:      true,
			expectStderr: true,
		},
		{
			name:         "stderr behavior always uses stderr",
			behavior:     HelpBehaviorStderr,
			isError:      false,
			expectStderr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, output := setupTestParser(t)
			p.SetHelpBehavior(tt.behavior)

			// Add a flag for content
			_ = p.AddFlag("test", &Argument{Description: "Test flag"})

			// Print help with context
			p.PrintHelpWithContext(tt.isError)

			if tt.expectStderr {
				assert.NotEmpty(t, output.Stderr.String())
				assert.Empty(t, output.Stdout.String())
			} else {
				assert.NotEmpty(t, output.Stdout.String())
				assert.Empty(t, output.Stderr.String())
			}
		})
	}
}

func TestHelpParser_CommandHierarchy(t *testing.T) {
	p, output := setupTestParser(t)

	// Create command hierarchy
	k8s := &Command{
		Name:        "k8s",
		Description: "Kubernetes operations",
		Subcommands: []Command{
			{
				Name:        "pod",
				Description: "Manage pods",
				Subcommands: []Command{
					{
						Name:        "create",
						Description: "Create a pod",
					},
				},
			},
		},
	}
	_ = p.AddCommand(k8s)

	// Add flags at different levels
	_ = p.AddFlag("namespace", &Argument{
		Short:       "n",
		Description: "Kubernetes namespace",
	}, "k8s")

	_ = p.AddFlag("name", &Argument{
		Description: "Pod name",
		Required:    true,
	}, "k8s", "pod", "create")

	// Test hierarchical help
	helpParser := NewHelpParser(p, p.helpConfig)
	err := helpParser.Parse([]string{"k8s", "pod", "--help"})
	require.NoError(t, err)

	stdoutOutput := output.Stdout.String()

	// Should show hierarchy
	assert.Contains(t, stdoutOutput, "k8s: Kubernetes operations")
	assert.Contains(t, stdoutOutput, "pod: Manage pods")
	assert.Contains(t, stdoutOutput, "all parent flags")
	assert.Contains(t, stdoutOutput, "create")
}

func TestHelpParser_DepthControl(t *testing.T) {
	p, output := setupTestParser(t)

	// Create deep command hierarchy
	_ = p.AddCommand(&Command{
		Name:        "level1",
		Description: "Level 1",
		Subcommands: []Command{
			{
				Name:        "level2",
				Description: "Level 2",
				Subcommands: []Command{
					{
						Name:        "level3",
						Description: "Level 3",
					},
				},
			},
		},
	})

	// Test with depth limit
	helpParser := NewHelpParser(p, p.helpConfig)
	err := helpParser.Parse([]string{"level1", "--help", "--depth", "1"})
	require.NoError(t, err)

	stdoutOutput := output.Stdout.String()

	// Should show level2 but not level3
	assert.Contains(t, stdoutOutput, "level2")
	assert.NotContains(t, stdoutOutput, "level3")
}

func TestHelpParser_Styles(t *testing.T) {
	tests := []struct {
		name           string
		style          string
		expectedOutput []string
		notExpected    []string
	}{
		{
			name:  "grouped style",
			style: "grouped",
			expectedOutput: []string{
				"Global Flags:", // Note: capital F
				"--verbose",
				"Commands:", // grouped style shows commands too
				"serve",
			},
			notExpected: []string{
				"notarealcommand",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, output := setupTestParser(t)

			// Add test data
			_ = p.AddFlag("verbose", &Argument{
				Short:       "v",
				Description: "Enable verbose output",
				TypeOf:      types.Standalone,
			})

			_ = p.AddCommand(&Command{
				Name:        "serve",
				Description: "Start the server",
			})

			// Parse with specific style
			helpParser := NewHelpParser(p, p.helpConfig)
			err := helpParser.Parse([]string{"--help", "--style", tt.style})
			require.NoError(t, err)

			stdoutOutput := output.Stdout.String()

			// Check expected content
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, stdoutOutput, expected, "Should contain: %s", expected)
			}

			// Check not expected content
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, stdoutOutput, notExpected, "Should not contain: %s", notExpected)
			}
		})
	}
}

func TestHelpParser_HelpModes_Extended(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput []string
		notExpected    []string
	}{
		{
			name: "flags mode shows only flags",
			args: []string{"--help", "flags"},
			expectedOutput: []string{
				"--verbose",
				"--port",
			},
			notExpected: []string{
				"Commands:",
				"serve",
			},
		},
		{
			name: "all mode shows everything",
			args: []string{"--help", "all"},
			expectedOutput: []string{
				"Global Flags:", // Capital F and colon
				"--verbose",
				"--port",
				"Commands:",
				"serve",
				"Examples:",
				"Show this help", // Examples section
			},
		},
		{
			name: "help for help mode",
			args: []string{"--help", "--help"}, // Double help shows help options
			expectedOutput: []string{
				"--show-descriptions", // Help options
				"--show-defaults",
				"--style",
				"--filter",
				"--search",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, output := setupTestParser(t)

			// Add test data
			_ = p.AddFlag("verbose", &Argument{
				Short:       "v",
				Description: "Enable verbose output",
				TypeOf:      types.Standalone,
			})

			_ = p.AddFlag("port", &Argument{
				Short:        "p",
				Description:  "Server port",
				TypeOf:       types.Single,
				DefaultValue: "8080",
			})

			_ = p.AddCommand(&Command{
				Name:        "serve",
				Description: "Start the server",
			})

			// Parse help
			helpParser := NewHelpParser(p, p.helpConfig)
			// Override the internal parser's helpEndFunc to prevent exit in tests
			helpParser.hp.helpEndFunc = func() error {
				return nil
			}
			err := helpParser.Parse(tt.args)
			require.NoError(t, err)

			stdoutOutput := output.Stdout.String()

			// Check expected content
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, stdoutOutput, expected, "Should contain: %s", expected)
			}

			// Check not expected content
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, stdoutOutput, notExpected, "Should not contain: %s", notExpected)
			}
		})
	}
}

func TestHelpParser(t *testing.T) {
	// Create parser
	p := NewParser()

	// Set up output capture
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	p.SetStdout(stdout)
	p.SetStderr(stderr)

	// Add a simple command
	cmd := &Command{
		Name:        "test",
		Description: "Test command",
	}
	err := p.AddCommand(cmd)
	require.NoError(t, err)

	// Test basic help
	helpParser := NewHelpParser(p, p.helpConfig)
	err = helpParser.Parse([]string{"--help"})
	assert.NoError(t, err)

	// Should output to stdout
	assert.NotEmpty(t, stdout.String())
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestHelpParser_InvalidCommand(t *testing.T) {
	// Create parser
	p := NewParser()

	// Set smart help behavior so error context goes to stderr
	p.SetHelpBehavior(HelpBehaviorSmart)

	// Set up output capture
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	p.SetStdout(stdout)
	p.SetStderr(stderr)

	// Add a command
	err := p.AddCommand(&Command{
		Name:        "valid",
		Description: "Valid command",
	})
	require.NoError(t, err)

	// Test invalid command
	helpParser := NewHelpParser(p, p.helpConfig)
	err = helpParser.Parse([]string{"invalid"})

	// Should have error
	assert.Error(t, err)
	assert.ErrorIs(t, errs.ErrCommandNotFound, err)

	// Should output to stderr due to error context
	assert.NotEmpty(t, stderr.String())
	assert.Contains(t, stderr.String(), "Error: Unknown command 'invalid'")
	assert.Contains(t, stderr.String(), "Available commands:")
}

func TestHelpParser_CommandHelp(t *testing.T) {
	// Create parser
	p := NewParser()

	// Set up output capture
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	p.SetStdout(stdout)
	p.SetStderr(stderr)

	// Add command
	cmd := &Command{
		Name:        "serve",
		Description: "Start the server",
	}
	err := p.AddCommand(cmd)
	require.NoError(t, err)

	// Add flag to command
	err = p.AddFlag("port", &Argument{
		Short:       "p",
		Description: "Server port",
		TypeOf:      types.Single,
	}, "serve")
	require.NoError(t, err)

	// Test command help
	helpParser := NewHelpParser(p, p.helpConfig)
	err = helpParser.Parse([]string{"serve", "--help"})
	assert.NoError(t, err)

	// Should show command help
	output := stdout.String()
	assert.Contains(t, output, "serve: Start the server")
	assert.Contains(t, output, "--port")
	assert.Contains(t, output, "Server port")
}

func TestHelpBehavior_Integration(t *testing.T) {
	// Test 1: Normal help request -> stdout
	t.Run("normal help to stdout", func(t *testing.T) {
		p := NewParser()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)
		p.SetHelpBehavior(HelpBehaviorSmart)

		// Add a flag
		_ = p.AddFlag("test", &Argument{Description: "Test flag"})

		// Request help normally
		p.PrintHelpWithContext(false)

		// Should go to stdout
		assert.NotEmpty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	// Test 2: Error help request -> stderr with smart behavior
	t.Run("error help to stderr", func(t *testing.T) {
		p := NewParser()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)
		p.SetHelpBehavior(HelpBehaviorSmart)

		// Add a flag
		_ = p.AddFlag("test", &Argument{Description: "Test flag"})

		// Request help with error context
		p.PrintHelpWithContext(true)

		// Should go to stderr
		assert.Empty(t, stdout.String())
		assert.NotEmpty(t, stderr.String())
	})

	// Test 3: ShowErrorHelp
	t.Run("show error help", func(t *testing.T) {
		p := NewParser()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)
		p.SetHelpBehavior(HelpBehaviorSmart)

		// Add a flag
		_ = p.AddFlag("test", &Argument{Description: "Test flag"})
		p.helpEndFunc = func() error { return nil }
		success := p.ParseString("anError")
		p.PrintHelpWithContext(success != false)

		assert.Contains(t, stderr.String(), "Usage:")
		assert.Contains(t, stderr.String(), "Test flag")

		// Nothing on stdout
		assert.Empty(t, stdout.String())
	})
}

func TestHelpIntegration(t *testing.T) {
	// Store original os.Args for all tests
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	t.Run("basic help works", func(t *testing.T) {
		// Set os.Args to match what we pass to Parse
		os.Args = []string{"app"}

		// Create parser (help is now the default)
		p, _ := NewParserWith(
			WithFlag("verbose", NewArg(
				WithShortFlag("v"),
				WithDescription("Enable verbose output"),
			)),
		)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		p.SetStdout(stdout)

		// Parse with just help
		ok := p.Parse([]string{"app", "--help"})
		assert.True(t, ok)
		assert.True(t, p.WasHelpShown())

		// Should show basic help
		output := stdout.String()
		assert.Contains(t, output, "verbose")
	})

	t.Run("help parser is used when enabled", func(t *testing.T) {
		// Set os.Args to match
		os.Args = []string{"app"}

		// Create parser (help is now the default)
		p, _ := NewParserWith(
			WithFlag("verbose", NewArg(
				WithShortFlag("v"),
				WithDescription("Enable verbose output"),
			)),
			WithCommand(&Command{
				Name:        "test",
				Description: "Test command",
			}),
		)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)

		// Parse with help sub-command (only available in parser)
		ok := p.Parse([]string{"app", "--help", "globals"})
		assert.True(t, ok)
		assert.True(t, p.WasHelpShown())

		// Should show only global flags
		output := stdout.String()
		assert.Contains(t, output, "Global flags")
		assert.Contains(t, output, "--verbose")
		assert.NotContains(t, output, "Commands:")
	})

	t.Run("help parser handles invalid commands", func(t *testing.T) {
		// Set os.Args to match
		os.Args = []string{"app"}

		// Create parser (help is now the default)
		p, _ := NewParserWith(
			WithCommand(&Command{
				Name:        "serve",
				Description: "Start server",
			}),
		)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)

		// Parse with invalid command and help flag to trigger help parser
		ok := p.Parse([]string{"app", "serv", "--help"})
		assert.True(t, ok) // Help was shown successfully (parser handles invalid command)

		// The help parser should handle the invalid command
		// Error output should be in stderr due to error context
		stderrOutput := stderr.String()
		stdoutOutput := stdout.String()

		// Log outputs for debugging
		t.Logf("stdout: %q", stdoutOutput)
		t.Logf("stderr: %q", stderrOutput)
		t.Logf("help shown: %v", p.WasHelpShown())

		// Check both outputs
		output := stderrOutput
		if output == "" {
			output = stdoutOutput
		}
		assert.NotEmpty(t, output)

		// Should show error about unknown command
		assert.Contains(t, output, "Unknown command 'serv'")

		// Should suggest the similar command
		assert.Contains(t, output, "Did you mean:")
		assert.Contains(t, output, "serve")
	})

	t.Run("help parser runtime options", func(t *testing.T) {
		// Set os.Args to match
		os.Args = []string{"app"}

		// Create parser (help is now the default)
		p, _ := NewParserWith(
			WithFlag("port", NewArg(
				WithShortFlag("p"),
				WithDescription("Server port"),
				WithDefaultValue("8080"),
			)),
		)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)

		// Parse with show-defaults option
		ok := p.Parse([]string{"app", "--help", "--show-defaults"})
		assert.True(t, ok)

		// Should show default value
		output := stdout.String()
		if output == "" {
			output = stderr.String()
		}
		// Look for the default value in the output
		assert.Contains(t, output, "8080")
	})

	t.Run("search functionality", func(t *testing.T) {
		// Set os.Args to match
		os.Args = []string{"app"}

		// Create parser (help is now the default)
		p, _ := NewParserWith(
			WithFlag("database-host", NewArg(
				WithDescription("Database server hostname"),
			)),
			WithFlag("database-port", NewArg(
				WithDescription("Database server port"),
			)),
			WithFlag("verbose", NewArg(
				WithDescription("Enable verbose logging"),
			)),
		)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)

		// Search for "database"
		ok := p.Parse([]string{"app", "--help", "--search", "database"})
		assert.True(t, ok)

		// Should show search results
		// Search results should be in stdout (not an error context)
		output := stdout.String()
		if output == "" {
			// If not in stdout, check stderr
			output = stderr.String()
		}
		assert.Contains(t, output, "database-host")
		assert.Contains(t, output, "database-port")
		assert.NotContains(t, output, "verbose")
	})
}

func TestHelpBehaviorIntegration(t *testing.T) {
	// Store original os.Args for all tests
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	t.Run("smart behavior with errors", func(t *testing.T) {
		// Set os.Args to match
		os.Args = []string{"app"}

		// Create parser with smart help behavior
		p, _ := NewParserWith(
			WithHelpBehavior(HelpBehaviorSmart),
			WithFlag("required", NewArg(
				WithRequired(true),
				WithDescription("Required flag"),
			)),
		)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		p.SetStdout(stdout)
		p.SetStderr(stderr)

		// Parse without required flag
		ok := p.Parse([]string{"app"})
		assert.False(t, ok)

		// Request help after error
		p.PrintHelpWithContext(true)

		// Help should go to stderr because of error context
		assert.NotEmpty(t, stderr.String())
		assert.Empty(t, stdout.String())
	})
}

func TestStructBasedHelp(t *testing.T) {
	// Store original os.Args for all tests
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	type App struct {
		// Global flags
		Verbose bool `goopt:"short:v;desc:Enable verbose output"`

		// Commands
		Serve struct {
			Port int `goopt:"short:p;desc:Server port;default:8080"`
		} `goopt:"kind:command;desc:Start the server"`

		Database struct {
			Host string `goopt:"desc:Database host;default:localhost"`
			Port int    `goopt:"desc:Database port;default:5432"`

			Backup struct {
				Output string `goopt:"short:o;desc:Output file;required:true"`
			} `goopt:"kind:command;desc:Create database backup"`
		} `goopt:"kind:command;desc:Database operations"`
	}

	t.Run("struct-based parser with help", func(t *testing.T) {
		// Set os.Args to match
		os.Args = []string{"app"}

		app := &App{}
		p, err := NewParserFromStruct(app)
		require.NoError(t, err)

		// Override help end function to prevent os.Exit in tests
		p.helpEndFunc = func() error { return nil }

		// Capture output
		stdout := &bytes.Buffer{}
		p.SetStdout(stdout)

		// Show command help
		ok := p.Parse([]string{"app", "database", "--help"})
		assert.True(t, ok)

		output := stdout.String()
		assert.Contains(t, output, "database: Database operations")
		assert.Contains(t, output, "--host")
		assert.Contains(t, output, "backup")
	})
}
