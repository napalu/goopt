package goopt

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestAutoVersion(t *testing.T) {
	t.Run("simple version", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("1.2.3"),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		// Parse with --version
		ok := parser.Parse([]string{"--version"})
		assert.True(t, ok)
		assert.True(t, parser.WasVersionShown())
		assert.Equal(t, "1.2.3\n", buf.String())
	})

	t.Run("short version flag", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("2.0.0"),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		// Parse with -v
		ok := parser.Parse([]string{"-v"})
		assert.True(t, ok)
		assert.True(t, parser.WasVersionShown())
		assert.Equal(t, "2.0.0\n", buf.String())
	})

	t.Run("dynamic version", func(t *testing.T) {
		var (
			Version   = "1.0.0"
			GitCommit = "abc123"
			BuildTime = time.Now().Format(time.RFC3339)
		)

		parser, err := NewParserWith(
			WithVersionFunc(func() string {
				return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
			}),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		ok := parser.Parse([]string{"--version"})
		assert.True(t, ok)
		assert.Contains(t, buf.String(), "1.0.0")
		assert.Contains(t, buf.String(), "abc123")
		assert.Contains(t, buf.String(), "built:")
	})

	t.Run("custom formatter", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("3.0.0"),
			WithVersionFormatter(func(version string) string {
				return fmt.Sprintf("MyApp v%s\nCopyright (c) 2024 MyCompany\nLicense: MIT", version)
			}),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		ok := parser.Parse([]string{"--version"})
		assert.True(t, ok)
		assert.Contains(t, buf.String(), "MyApp v3.0.0")
		assert.Contains(t, buf.String(), "Copyright")
		assert.Contains(t, buf.String(), "MIT")
	})

	t.Run("disable auto version", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("1.0.0"),
			WithAutoVersion(false),
		)
		assert.NoError(t, err)

		// Should fail to parse --version since it's not registered
		ok := parser.Parse([]string{"--version"})
		assert.False(t, ok)
		assert.False(t, parser.WasVersionShown())
	})

	t.Run("custom version flags", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("1.0.0"),
			WithVersionFlags("ver", "V"), // Use capital V
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		// Test --ver
		ok := parser.Parse([]string{"--ver"})
		assert.True(t, ok)
		assert.True(t, parser.WasVersionShown())

		// Test -V (capital)
		parser2, _ := NewParserWith(
			WithVersion("1.0.0"),
			WithVersionFlags("ver", "V"),
		)
		parser2.SetStdout(&buf)
		buf.Reset()
		ok = parser2.Parse([]string{"-V"})
		assert.True(t, ok)
		assert.True(t, parser2.WasVersionShown())
	})

	t.Run("version in help", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("1.2.3"),
			WithShowVersionInHelp(true),
			WithFlag("verbose", NewArg(WithType(types.Standalone), WithShortFlag("v"))),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)
		parser.helpEndFunc = func() error {
			return nil
		}
		// Show help
		ok := parser.Parse([]string{"--help"})
		assert.True(t, ok)

		output := buf.String()
		// Should show version at top of help
		lines := strings.Split(output, "\n")
		assert.Contains(t, lines[0], "1.2.3")
	})

	t.Run("user defined version flag", func(t *testing.T) {
		// User defines their own version flag
		parser := NewParser()
		parser.SetVersion("1.0.0") // Set version but user will handle flag

		// User adds their own version flag
		err := parser.AddFlag("version", &Argument{
			Description: "Print version and exit",
			TypeOf:      types.Standalone,
		})
		assert.NoError(t, err)

		ok := parser.Parse([]string{"--version"})
		assert.True(t, ok)

		// Check that user's flag was set
		val, found := parser.Get("version")
		assert.True(t, found)
		assert.Equal(t, "true", val)
		assert.False(t, parser.WasVersionShown()) // Auto version didn't trigger
	})

	t.Run("user uses -v for verbose", func(t *testing.T) {
		// User wants -v for verbose, not version
		parser, err := NewParserWith(
			WithVersion("1.0.0"),
			WithFlag("verbose", NewArg(
				WithType(types.Standalone),
				WithShortFlag("v"),
				WithDescription("Enable verbose output"),
			)),
		)
		assert.NoError(t, err)

		// -v should map to verbose
		ok := parser.Parse([]string{"-v"})
		assert.True(t, ok)
		val, found := parser.Get("verbose")
		assert.True(t, found)
		assert.Equal(t, "true", val)
		assert.False(t, parser.WasVersionShown())

		// --version should still work
		var buf bytes.Buffer
		parser.SetStdout(&buf)
		ok = parser.Parse([]string{"app", "--version"})
		assert.True(t, ok)
		assert.True(t, parser.WasVersionShown())
	})

	t.Run("no version set", func(t *testing.T) {
		// No version = no auto flags
		parser := NewParser()

		// Should not have version flags
		ok := parser.Parse([]string{"--version"})
		assert.False(t, ok) // Parse fails because flag doesn't exist
	})

	t.Run("version with struct tags", func(t *testing.T) {
		type Config struct {
			Verbose bool `goopt:"short:v;desc:Enable verbose output"`
		}

		cfg := &Config{}
		parser, err := NewParserFromStruct(cfg,
			WithVersion("2.0.0"),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		// --version should work
		ok := parser.Parse([]string{"--version"})
		assert.True(t, ok)
		assert.True(t, parser.WasVersionShown())
		assert.Equal(t, "2.0.0\n", buf.String())
	})

	t.Run("version function and formatter combined", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersionFunc(func() string {
				return "1.2.3-dev"
			}),
			WithVersionFormatter(func(version string) string {
				return fmt.Sprintf("=== MyApp %s ===", version)
			}),
		)
		assert.NoError(t, err)

		var buf bytes.Buffer
		parser.SetStdout(&buf)

		ok := parser.Parse([]string{"--version"})
		assert.True(t, ok)
		assert.Equal(t, "=== MyApp 1.2.3-dev ===\n", buf.String())
	})
}

func TestVersionMethods(t *testing.T) {
	t.Run("set and get version", func(t *testing.T) {
		p := NewParser()
		p.SetVersion("1.0.0")
		assert.Equal(t, "1.0.0", p.GetVersion())
	})

	t.Run("version func overrides static version", func(t *testing.T) {
		p := NewParser()
		p.SetVersion("1.0.0")
		p.SetVersionFunc(func() string {
			return "2.0.0-dynamic"
		})
		assert.Equal(t, "2.0.0-dynamic", p.GetVersion())
	})

	t.Run("print version with no version set", func(t *testing.T) {
		p := NewParser()
		var buf bytes.Buffer
		p.PrintVersion(&buf)
		assert.Equal(t, "unknown\n", buf.String())
	})
}

func TestVersionIntegration(t *testing.T) {
	t.Run("version and help together", func(t *testing.T) {
		parser, err := NewParserWith(
			WithVersion("1.0.0"),
			WithShowVersionInHelp(true),
			WithFlag("input", NewArg(WithType(types.Single), WithShortFlag("i"))),
		)
		assert.NoError(t, err)

		// Parse once to trigger auto-registration
		parser.Parse([]string{})

		// Now both flags should be available
		_, err = parser.GetArgument("help")
		assert.NoError(t, err)

		_, err = parser.GetArgument("version")
		assert.NoError(t, err)
	})

}
