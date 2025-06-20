package goopt

import (
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"

	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func TestWithExecOnParse(t *testing.T) {

	tests := []struct {
		name        string
		args        []string
		execOnParse bool
		expectExec  bool
	}{
		{
			name:        "exec on parse enabled",
			args:        []string{"command", "sub-command"},
			execOnParse: true,
			expectExec:  true,
		},
		{
			name:        "exec on parse disabled",
			args:        []string{"command", "sub-command"},
			execOnParse: false,
			expectExec:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executed := false
			cmd := &Command{
				Name: "command",
				Subcommands: []Command{
					{
						Name: "sub-command",
						Callback: func(cmdLine *Parser, command *Command) error {
							executed = true
							return nil
						},
					},
				},
			}

			p, err := NewParserWith(
				WithCommand(cmd),
				WithExecOnParse(tt.execOnParse),
			)
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.True(t, ok)
			assert.Equal(t, tt.expectExec, executed)
		})
	}
}

func TestWithListDelimiterFunc(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		delimiter string
		expected  []string
	}{
		{
			name:      "custom delimiter semicolon",
			args:      []string{"--tags", "tag1;tag2;tag3"},
			delimiter: ";",
			expected:  []string{"tag1", "tag2", "tag3"},
		},
		{
			name:      "custom delimiter pipe",
			args:      []string{"--tags", "tag1|tag2|tag3"},
			delimiter: "|",
			expected:  []string{"tag1", "tag2", "tag3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tags []string
			p, err := NewParserWith(
				WithBindFlag("tags", &tags, NewArg(WithType(types.Chained))),
				WithListDelimiterFunc(func(matchOn rune) bool {
					for _, delimiter := range tt.delimiter {
						if matchOn == delimiter {
							return true
						}
					}
					return false
				}),
			)
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, tags)
		})
	}
}

func TestWithPosix(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		posix    bool
		expected bool
	}{
		{
			name:     "posix enabled - combined short flags",
			args:     []string{"-dl", "info"},
			posix:    true,
			expected: true,
		},
		{
			name:     "posix disabled - combined short flags",
			args:     []string{"-dl", "info"},
			posix:    false,
			expected: false,
		},
		{
			name:     "posix enabled - separate short flags",
			args:     []string{"-d", "-l", "info"},
			posix:    true,
			expected: true,
		},
		{
			name:     "posix enabled - long flags",
			args:     []string{"--debug", "--level", "info"},
			posix:    true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewParserWith(
				WithFlag("debug", NewArg(WithType(types.Standalone), WithShortFlag("d"))),
				WithFlag("level", NewArg(WithType(types.Single), WithShortFlag("l"))),
				WithPosix(tt.posix),
			)
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.Equal(t, tt.expected, ok)

			if tt.expected && tt.posix {
				// Verify flag values when parsing succeeds
				debugVal := p.GetOrDefault("debug", "")
				assert.Equal(t, "true", debugVal)

				if tt.args[0] == "-dl" {
					levelVal := p.GetOrDefault("level", "")
					assert.Equal(t, "info", levelVal)
				}
			}
		})
	}
}

func TestWithNameConverters(t *testing.T) {
	type Config struct {
		UserName string `goopt:"kind:flag"`
		Command  struct {
			SubCommand struct{} `goopt:"kind:command"`
		} `goopt:"kind:command"`
	}

	tests := []struct {
		name          string
		args          []string
		envVars       map[string]string
		flagConverter NameConversionFunc
		envConverter  NameConversionFunc
		cmdConverter  NameConversionFunc
		expectedFlag  string
		expectedCmd   string
	}{
		{
			name:          "default converters",
			args:          []string{"command", "subcommand"},
			envVars:       map[string]string{"USER_NAME": "test-user"},
			flagConverter: DefaultFlagNameConverter,
			envConverter:  DefaultFlagNameConverter,
			cmdConverter:  DefaultCommandNameConverter,
			expectedFlag:  "userName",
			expectedCmd:   "command",
		},
		{
			name:    "custom lowercase converters",
			args:    []string{"command", "subcommand"},
			envVars: map[string]string{"user_name": "test-user"},
			flagConverter: func(s string) string {
				return strings.ReplaceAll(strings.ToLower(s), "_", "")
			},
			envConverter: func(s string) string {
				return strings.ReplaceAll(strings.ToLower(s), "_", "")
			},
			cmdConverter: func(s string) string {
				return strings.ReplaceAll(strings.ToLower(s), "_", "")
			},
			expectedFlag: "username",
			expectedCmd:  "command",
		},
		{
			name:          "mixed case converters",
			args:          []string{"Command", "SubCommand"},
			envVars:       map[string]string{"USER_NAME": "test-user"},
			flagConverter: strings.ToUpper,
			envConverter: func(s string) string {
				return strings.ReplaceAll(strings.ToUpper(s), "_", "")
			},
			cmdConverter: func(s string) string {
				return cases.Title(language.Und).String(strings.ToLower(s))
			},
			expectedFlag: "USERNAME",
			expectedCmd:  "Command",
		},
		{
			name:          "kebab case converter",
			args:          []string{"command", "subcommand"},
			envVars:       map[string]string{"USER_NAME": "test-user"},
			flagConverter: strcase.ToKebab,
			envConverter: func(s string) string {
				return strings.ReplaceAll(strings.ToLower(s), "_", "-")
			},
			cmdConverter: func(s string) string {
				return strings.ReplaceAll(strings.ToLower(s), "_ ", "-") + "-name"
			},
			expectedFlag: "user-name",
			expectedCmd:  "command-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg := &Config{}
			p, err := NewParserFromStruct(cfg,
				WithFlagNameConverter(tt.flagConverter),
				WithEnvNameConverter(tt.envConverter),
				WithCommandNameConverter(tt.cmdConverter),
			)
			assert.NoError(t, err)

			// Test flag name and env var mapping
			ok := p.Parse(tt.args)
			assert.True(t, ok)
			assert.Equal(t, "test-user", cfg.UserName)

			// Test command name conversion
			cmd, found := p.getCommand(tt.expectedCmd)
			assert.True(t, found)
			assert.Equal(t, tt.expectedCmd, cmd.Name)

		})

	}
}

func TestWithPosixCompatibility(t *testing.T) {
	type Config struct {
		A bool   `goopt:"kind:flag;short:a"`
		B bool   `goopt:"kind:flag;short:b"`
		C string `goopt:"kind:flag;short:c"`
	}

	tests := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "posix style combined flags",
			args: []string{"-abc", "value"},
			want: map[string]string{
				"a": "true",
				"b": "true",
				"c": "value",
			},
		},
		{
			name: "regular style flags",
			args: []string{"-a", "-b", "-c", "value"},
			want: map[string]string{
				"a": "true",
				"b": "true",
				"c": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg, WithPosix(true))
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.True(t, ok)

			for flag, expectedValue := range tt.want {
				value := p.GetOrDefault(flag, "")
				assert.Equal(t, expectedValue, value)
			}
		})
	}
}

func TestWithPrefixes(t *testing.T) {
	type Config struct {
		Flag1 string `goopt:"kind:flag"`
		Flag2 bool   `goopt:"kind:flag"`
	}

	tests := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "custom prefix flags",
			args: []string{"+flag1", "value", "/flag2"},
			want: map[string]string{
				"flag1": "value",
				"flag2": "true",
			},
		},
		{
			name: "mixed prefix usage",
			args: []string{"+flag1", "value1", "--flag2"},
			want: map[string]string{
				"flag1": "value1",
				"flag2": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg, WithArgumentPrefixes([]rune{'+', '/', '-'}))
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.True(t, ok)

			for flag, expectedValue := range tt.want {
				value := p.GetOrDefault(flag, "")
				assert.Equal(t, expectedValue, value)
			}
		})
	}
}

func TestWithListDelimiter(t *testing.T) {
	type Config struct {
		Tags []string `goopt:"kind:flag;type:chained"`
		Nums []int    `goopt:"kind:flag;type:chained"`
	}

	tests := []struct {
		name     string
		args     []string
		want     map[string][]string
		wantBool bool
	}{
		{
			name: "semicolon delimited lists",
			args: []string{"--tags", "a;b;c", "--nums", "1;2;3"},
			want: map[string][]string{
				"tags": {"a", "b", "c"},
				"nums": {"1", "2", "3"},
			},
			wantBool: true,
		},
		{
			name: "single values",
			args: []string{"--tags", "only", "--nums", "42"},
			want: map[string][]string{
				"tags": {"only"},
				"nums": {"42"},
			},
			wantBool: true,
		},
		{
			name: "empty values",
			args: []string{"--tags", "", "--nums", ""},
			want: map[string][]string{
				"tags": {""},
				"nums": {""},
			},
			wantBool: false,
		},
		{
			name: "semicolon delimited lists with valid numbers",
			args: []string{"--tags", "a;b;c", "--nums", "1;2;3"},
			want: map[string][]string{
				"tags": {"a", "b", "c"},
				"nums": {"1", "2", "3"},
			},
			wantBool: true,
		},
		{
			name:     "mixed delimiters invalid numbers",
			args:     []string{"--tags", "a,b;c", "--nums", "1,2;3"},
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg, WithListDelimiterFunc(func(matchOn rune) bool {
				return matchOn == ';'
			}))
			assert.NoError(t, err)

			ok := p.Parse(tt.args)
			assert.Equal(t, tt.wantBool, ok, "Parse() result mismatch")

			for flag, expectedValues := range tt.want {
				values := strings.Split(p.GetOrDefault(flag, ""), ";")
				assert.Equal(t, expectedValues, values, "Value mismatch for flag %s", flag)
			}
		})
	}
}

// TestReplaceDefaultBundle tests the ReplaceDefaultBundle function
func TestReplaceDefaultBundle(t *testing.T) {
	t.Run("replace with valid bundle", func(t *testing.T) {
		p := NewParser()

		// Save the original bundle to restore it after the test
		originalBundle := p.GetSystemBundle()
		defer func() {
			// Restore the original bundle to avoid affecting other tests
			if originalBundle != nil {
				_ = p.ReplaceDefaultBundle(originalBundle)
			}
		}()

		// Create a new bundle with custom translations
		bundle := i18n.NewEmptyBundle()
		_ = bundle.AddLanguage(language.English, map[string]string{
			"test.key": "Test Value",
		})

		err := p.ReplaceDefaultBundle(bundle)
		assert.NoError(t, err)

		// Verify the bundle was replaced
		assert.Equal(t, bundle, p.GetSystemBundle())
	})

	t.Run("replace with nil bundle returns error", func(t *testing.T) {
		p := NewParser()

		err := p.ReplaceDefaultBundle(nil)
		assert.Error(t, err)
		assert.ErrorIs(t, errs.ErrNilPointer, err)
	})
}

// TestWithLanguage tests the WithLanguage configuration function
func TestWithLanguage(t *testing.T) {
	t.Run("set language to German", func(t *testing.T) {
		p, err := NewParserWith(WithLanguage(language.German))
		assert.NoError(t, err)

		// The parser should have German as the default language
		bundle := p.GetSystemBundle()
		assert.Equal(t, language.German, bundle.GetDefaultLanguage())
	})

	t.Run("set language to French", func(t *testing.T) {
		p, err := NewParserWith(WithLanguage(language.French))
		assert.NoError(t, err)

		bundle := p.GetSystemBundle()
		assert.Equal(t, language.French, bundle.GetDefaultLanguage())
	})
}

// TestWithUserBundle tests the WithUserBundle configuration function
func TestWithUserBundle(t *testing.T) {
	t.Run("set valid user bundle", func(t *testing.T) {
		userBundle := i18n.NewEmptyBundle()
		userBundle.AddLanguage(language.English, map[string]string{
			"custom.key": "Custom Value",
		})

		p, err := NewParserWith(WithUserBundle(userBundle))
		assert.NoError(t, err)

		// Verify the user bundle was set
		assert.Equal(t, userBundle, p.GetUserBundle())
	})

	t.Run("set nil user bundle", func(t *testing.T) {
		// WithUserBundle with nil should return an error
		p, err := NewParserWith(WithUserBundle(nil))
		assert.Error(t, err)
		assert.Nil(t, p)
	})
}

// TestWithReplaceBundle tests the WithReplaceBundle configuration function
func TestWithReplaceBundle(t *testing.T) {
	t.Run("replace bundle during parser creation", func(t *testing.T) {
		customBundle := i18n.NewEmptyBundle()
		customBundle.AddLanguage(language.English, map[string]string{
			"replaced.key": "Replaced Value",
		})

		// Save the original bundle to restore it after the test
		originalBundle := i18n.Default()

		p, err := NewParserWith(WithReplaceBundle(customBundle))
		defer func() {
			// Restore the original bundle to avoid affecting other tests
			if originalBundle != nil {
				p.ReplaceDefaultBundle(originalBundle)
			}
		}()
		assert.NoError(t, err)

		// Verify the bundle was replaced
		assert.Equal(t, customBundle, p.GetSystemBundle())
	})

	t.Run("replace with nil bundle returns error", func(t *testing.T) {
		// WithReplaceBundle should handle the error internally
		// The parser creation might fail or use default bundle
		p, err := NewParserWith(WithReplaceBundle(nil))
		assert.Error(t, err)

		// Parser should not be created
		assert.Nil(t, p)
	})
}

// TestWithExecOnParseComplete tests the WithExecOnParseComplete configuration function
func TestWithExecOnParseComplete(t *testing.T) {
	t.Run("execute callbacks after parse complete", func(t *testing.T) {
		executed := false

		cmd := NewCommand(
			WithName("test"),
			WithCallback(func(p *Parser, c *Command) error {
				executed = true
				return nil
			}),
		)

		p, err := NewParserWith(
			WithCommand(cmd),
			WithExecOnParseComplete(true),
		)
		assert.NoError(t, err)

		// WithExecOnParseComplete automatically executes commands after successful parse
		success := p.Parse([]string{"test"})
		assert.True(t, success)

		// Callback should have been executed automatically
		assert.True(t, executed)
	})

	t.Run("interaction with WithExecOnParse", func(t *testing.T) {
		executed := false

		cmd := NewCommand(
			WithName("test"),
			WithCallback(func(p *Parser, c *Command) error {
				executed = true
				return nil
			}),
		)

		// WithExecOnParse takes precedence
		p, err := NewParserWith(
			WithCommand(cmd),
			WithExecOnParse(true),
			WithExecOnParseComplete(true), // This should have no effect
		)
		assert.NoError(t, err)

		success := p.Parse([]string{"test"})
		assert.True(t, success)

		// Callback should have executed during parse due to WithExecOnParse
		assert.True(t, executed)
	})
}
