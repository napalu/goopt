package goopt

import (
	"os"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/i18n/locales/es"
	"github.com/napalu/goopt/v2/i18n/locales/ja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestAutoLanguageDetection(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		envLang        string
		envGooptLang   string
		setupFunc      func(*Parser)
		expectedLang   language.Tag
		acceptableLang language.Tag // Alternative acceptable language (for fallback matching)
		expectHelp     bool
	}{
		{
			name:         "no language flag uses default",
			args:         []string{"--help"},
			envLang:      "", // Explicitly unset LANG
			expectedLang: language.English,
			expectHelp:   true,
		},
		{
			name:         "language flag with value",
			args:         []string{"--lang", "fr", "--help"},
			expectedLang: language.French,
			expectHelp:   true,
		},
		{
			name: "language flag with equals",
			args: []string{"--lang=es", "--help"},
			setupFunc: func(p *Parser) {
				// Add Spanish to the system bundle
				locale := i18n.NewLocale(language.Spanish, es.SystemTranslations)
				p.SetSystemLocales(locale)
			},
			expectedLang: language.Spanish,
			expectHelp:   true,
		},
		{
			name:         "short language flag",
			args:         []string{"-l", "de", "--help"},
			expectedLang: language.German,
			expectHelp:   true,
		},
		{
			name:         "language flag after help",
			args:         []string{"--help", "--lang", "fr"},
			expectedLang: language.French,
			expectHelp:   true,
		},
		{
			name: "language flag after help with equals",
			args: []string{"--help", "--lang=ja"},
			setupFunc: func(p *Parser) {
				// Add Japanese to the system bundle
				locale := i18n.NewLocale(language.Japanese, ja.SystemTranslations)
				p.SetSystemLocales(locale)
			},
			expectedLang: language.Japanese,
			expectHelp:   true,
		},
		{
			name:         "LANG environment variable not checked by default",
			args:         []string{"--help"},
			envLang:      "fr_FR.UTF-8",
			expectedLang: language.English, // LANG is ignored without WithCheckSystemLocale
			expectHelp:   true,
		},
		{
			name:         "GOOPT_LANG environment variable override",
			args:         []string{"--help"},
			envLang:      "fr_FR.UTF-8",
			envGooptLang: "es",
			setupFunc: func(p *Parser) {
				// Add Spanish to the system bundle
				locale := i18n.NewLocale(language.Spanish, es.SystemTranslations)
				p.SetSystemLocales(locale)
			},
			expectedLang: language.Spanish,
			expectHelp:   true,
		},
		{
			name:         "command line overrides GOOPT_LANG",
			args:         []string{"--lang", "de", "--help"},
			envGooptLang: "fr",
			expectedLang: language.German,
			expectHelp:   true,
		},
		{
			name:         "invalid language falls back to default",
			args:         []string{"--lang", "invalid", "--help"},
			expectedLang: language.English,
			expectHelp:   true,
		},
		{
			name: "auto language disabled",
			args: []string{"--lang", "fr", "--help"},
			setupFunc: func(p *Parser) {
				p.SetAutoLanguage(false)
			},
			expectedLang: language.English,
			expectHelp:   true,
		},
		{
			name: "custom language flags",
			args: []string{"--idioma", "es", "--help"},
			setupFunc: func(p *Parser) {
				p.SetLanguageFlags([]string{"idioma", "i"})
				// Add Spanish to the system bundle
				locale := i18n.NewLocale(language.Spanish, es.SystemTranslations)
				p.SetSystemLocales(locale)
			},
			expectedLang: language.Spanish,
			expectHelp:   true,
		},
		{
			name:         "language detection without help",
			args:         []string{"--lang", "fr"},
			expectedLang: language.French,
			expectHelp:   false,
		},
		{
			name:         "multiple language flags uses last one",
			args:         []string{"--lang", "fr", "--lang", "de", "--help"},
			expectedLang: language.German,
			expectHelp:   true,
		},
		{
			name:           "language flag with underscore format",
			args:           []string{"--lang", "fr_CA", "--help"},
			expectedLang:   language.MustParse("fr-CA"), // Ideal match if fr-CA is available
			acceptableLang: language.French,             // Fallback to base language
			expectHelp:     true,
		},
		{
			name:           "language flag with dash format",
			args:           []string{"--lang", "fr-CA", "--help"},
			expectedLang:   language.MustParse("fr-CA"), // Ideal match if fr-CA is available
			acceptableLang: language.French,             // Fallback to base language
			expectHelp:     true,
		},
		{
			name:    "LANG environment variable with system locale enabled",
			args:    []string{"--help"},
			envLang: "fr_FR.UTF-8",
			setupFunc: func(p *Parser) {
				p.SetCheckSystemLocale(true) // Enable system locale checking
			},
			expectedLang:   language.French,
			acceptableLang: language.Make("fr-FR"), // Also accept fr-FR variant
			expectHelp:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			oldLang := os.Getenv("LANG")
			oldGooptLang := os.Getenv("GOOPT_LANG")
			oldLcAll := os.Getenv("LC_ALL")
			oldLcMessages := os.Getenv("LC_MESSAGES")
			defer func() {
				os.Setenv("LANG", oldLang)
				os.Setenv("GOOPT_LANG", oldGooptLang)
				os.Setenv("LC_ALL", oldLcAll)
				os.Setenv("LC_MESSAGES", oldLcMessages)
			}()

			// Clear all locale environment variables first
			os.Unsetenv("LC_ALL")
			os.Unsetenv("LC_MESSAGES")
			os.Unsetenv("LANG")
			os.Unsetenv("GOOPT_LANG")

			// Set test environment
			if tt.envLang != "" {
				os.Setenv("LANG", tt.envLang)
			}
			if tt.envGooptLang != "" {
				os.Setenv("GOOPT_LANG", tt.envGooptLang)
			}

			// Create parser
			p := NewParser()

			// Override help end function to prevent os.Exit in tests
			p.SetEndHelpFunc(func() error {
				return nil
			})

			if tt.setupFunc != nil {
				tt.setupFunc(p)
			}

			// Add a simple flag for testing
			err := p.AddFlag("test", NewArg(WithDescription("Test flag")))
			require.NoError(t, err)

			// Parse arguments
			success := p.Parse(tt.args)

			// Check language
			actualLang := p.GetLanguage()

			// Handle the special -u-rg-xxzzzz format that the language matcher returns
			// for exact language matches with different regions
			actualLangStr := actualLang.String()
			if strings.Contains(actualLangStr, "-u-rg-") {
				// Extract the base language from tags like "fr-u-rg-frzzzz"
				if base, _ := actualLang.Base(); base.String() != "und" {
					// Create a new tag from the base language
					actualLang = language.Make(base.String())
				}
			}

			// Check if actual language matches either expected or acceptable
			if tt.acceptableLang != (language.Tag{}) {
				// If we have an acceptable alternative, check both
				if actualLang != tt.expectedLang && actualLang != tt.acceptableLang {
					t.Errorf("Language mismatch: got %v, want %v or %v", actualLang, tt.expectedLang, tt.acceptableLang)
				}
			} else {
				// Otherwise just check expected
				assert.Equal(t, tt.expectedLang, actualLang, "Language mismatch")
			}

			// Check help execution
			if tt.expectHelp {
				assert.True(t, p.WasHelpShown(), "Help should have been shown")
				assert.True(t, success, "Parse should succeed when help is shown")
			}
		})
	}
}

func TestFilterLanguageFlags(t *testing.T) {
	p := NewParser()
	p.SetAutoLanguage(true)

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "no language flags",
			args:     []string{"--help", "--verbose"},
			expected: []string{"--help", "--verbose"},
		},
		{
			name:     "language flag with value",
			args:     []string{"--help", "--lang", "fr"},
			expected: []string{"--help"},
		},
		{
			name:     "language flag with equals",
			args:     []string{"--help", "--lang=fr"},
			expected: []string{"--help"},
		},
		{
			name:     "short language flag",
			args:     []string{"--help", "-l", "fr"},
			expected: []string{"--help"},
		},
		{
			name:     "multiple language flags",
			args:     []string{"--lang", "fr", "--help", "--language", "de"},
			expected: []string{"--help"},
		},
		{
			name:     "language flag without value",
			args:     []string{"--help", "--lang"},
			expected: []string{"--help"},
		},
		{
			name:     "language flag with flag as value",
			args:     []string{"--help", "--lang", "--verbose"},
			expected: []string{"--help", "--verbose"},
		},
		{
			name:     "mixed flags and arguments",
			args:     []string{"cmd", "--lang", "fr", "arg", "--help"},
			expected: []string{"cmd", "arg", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := p.filterLanguageFlags(tt.args)
			assert.Equal(t, tt.expected, filtered)
		})
	}
}

func TestDetectLanguageInArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		languageFlags []string
		expectedLang  language.Tag
	}{
		{
			name:         "detect language with default flags",
			args:         []string{"--lang", "fr"},
			expectedLang: language.French,
		},
		{
			name:         "detect language with equals",
			args:         []string{"--language=es"},
			expectedLang: language.Spanish,
		},
		{
			name:         "detect short flag",
			args:         []string{"-l", "de"},
			expectedLang: language.German,
		},
		{
			name:         "no language flag",
			args:         []string{"--help", "--verbose"},
			expectedLang: language.Und,
		},
		{
			name:         "invalid language",
			args:         []string{"--lang", "xyz"},
			expectedLang: language.Und,
		},
		{
			name:          "custom language flags",
			args:          []string{"--idioma", "es"},
			languageFlags: []string{"idioma"},
			expectedLang:  language.Spanish,
		},
		{
			name:         "language flag without value",
			args:         []string{"--lang"},
			expectedLang: language.Und,
		},
		{
			name:         "language flag with flag as value",
			args:         []string{"--lang", "--help"},
			expectedLang: language.Und,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			p.SetAutoLanguage(true)

			if tt.languageFlags != nil {
				p.SetLanguageFlags(tt.languageFlags)
			}

			// Mock environment that returns empty strings
			mockGetenv := func(key string) string {
				return ""
			}

			lang := p.detectLanguageInArgsWithEnv(tt.args, mockGetenv)
			assert.Equal(t, tt.expectedLang, lang)
		})
	}
}

func TestAutoLanguageWithCommands(t *testing.T) {
	type Config struct {
		Global string `goopt:"name:global"`
		Server struct {
			Port int    `goopt:"name:port"`
			Host string `goopt:"name:host"`
		} `goopt:"kind:command;name:server"`
		Client struct {
			URL string `goopt:"name:url"`
		} `goopt:"kind:command;name:client"`
	}

	tests := []struct {
		name         string
		args         []string
		expectedLang language.Tag
	}{
		{
			name:         "language before command",
			args:         []string{"--lang", "fr", "server", "--help"},
			expectedLang: language.French,
		},
		{
			name:         "language after command",
			args:         []string{"server", "--lang", "fr", "--help"},
			expectedLang: language.French,
		},
		{
			name:         "language between commands",
			args:         []string{"server", "--lang", "fr", "--port", "8080"},
			expectedLang: language.French,
		},
		{
			name:         "help then language with command",
			args:         []string{"server", "--help", "--lang", "fr"},
			expectedLang: language.French,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			p, err := NewParserFromStruct(cfg)
			require.NoError(t, err)

			p.SetAutoLanguage(true)
			p.SetLanguage(language.English)

			// Override help end function to prevent os.Exit in tests
			p.SetEndHelpFunc(func() error {
				return nil
			})

			p.Parse(tt.args)

			actualLang := p.GetLanguage()
			assert.Equal(t, tt.expectedLang, actualLang)
		})
	}
}

func TestLanguageEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name              string
		envLang           string
		envGooptLang      string
		checkSystemLocale bool
		expectedLang      language.Tag
	}{
		{
			name:              "LANG not checked without system locale enabled",
			envLang:           "fr_FR.UTF-8",
			checkSystemLocale: false,
			expectedLang:      language.Und,
		},
		{
			name:              "LANG with full locale when system locale enabled",
			envLang:           "en_US.UTF-8",
			checkSystemLocale: true,
			expectedLang:      language.MustParse("en-US"),
		},
		{
			name:              "LANG with language only when system locale enabled",
			envLang:           "fr",
			checkSystemLocale: true,
			expectedLang:      language.French,
		},
		{
			name:              "GOOPT_LANG works without system locale",
			envGooptLang:      "de",
			checkSystemLocale: false,
			expectedLang:      language.German,
		},
		{
			name:              "GOOPT_LANG overrides LANG",
			envLang:           "fr_FR.UTF-8",
			envGooptLang:      "de",
			checkSystemLocale: true,
			expectedLang:      language.German,
		},
		{
			name:              "invalid LANG falls back",
			envLang:           "invalid",
			checkSystemLocale: true,
			expectedLang:      language.Und,
		},
		{
			name:              "LANG with underscore when system locale enabled",
			envLang:           "fr_CA",
			checkSystemLocale: true,
			expectedLang:      language.MustParse("fr-CA"),
		},
		{
			name:              "GOOPT_LANG with underscore normalization",
			envGooptLang:      "es_MX",
			checkSystemLocale: false,
			expectedLang:      language.MustParse("es-MX"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			oldLang := os.Getenv("LANG")
			oldGooptLang := os.Getenv("GOOPT_LANG")
			oldLcAll := os.Getenv("LC_ALL")
			oldLcMessages := os.Getenv("LC_MESSAGES")
			defer func() {
				os.Setenv("LANG", oldLang)
				os.Setenv("GOOPT_LANG", oldGooptLang)
				os.Setenv("LC_ALL", oldLcAll)
				os.Setenv("LC_MESSAGES", oldLcMessages)
			}()

			// Clear all locale environment variables first
			os.Unsetenv("LC_ALL")
			os.Unsetenv("LC_MESSAGES")
			os.Unsetenv("LANG")
			os.Unsetenv("GOOPT_LANG")

			// Set test environment
			if tt.envLang != "" {
				os.Setenv("LANG", tt.envLang)
			}
			if tt.envGooptLang != "" {
				os.Setenv("GOOPT_LANG", tt.envGooptLang)
			}

			p := NewParser()
			p.SetAutoLanguage(true)
			p.SetCheckSystemLocale(tt.checkSystemLocale)

			lang := p.detectLanguageInArgs([]string{})

			// Handle language canonicalization differences
			actualStr := lang.String()
			expectedStr := tt.expectedLang.String()

			// If actual contains region code like "en-US-u-rg-uszzzz", extract base
			if strings.Contains(actualStr, "-u-rg-") {
				if base, _ := lang.Base(); base.String() != "und" {
					lang = language.Make(base.String())
				}
			}

			assert.Equal(t, tt.expectedLang, lang, "Expected %s, got %s", expectedStr, actualStr)
		})
	}
}

func TestCustomLanguageEnvVar(t *testing.T) {
	// Save and restore environment
	oldGooptLang := os.Getenv("GOOPT_LANG")
	oldMyAppLang := os.Getenv("MYAPP_LANG")
	defer func() {
		os.Setenv("GOOPT_LANG", oldGooptLang)
		os.Setenv("MYAPP_LANG", oldMyAppLang)
	}()

	tests := []struct {
		name         string
		customEnvVar string
		gooptLang    string
		myAppLang    string
		expectedLang language.Tag
	}{
		{
			name:         "custom env var is used",
			customEnvVar: "MYAPP_LANG",
			gooptLang:    "fr",
			myAppLang:    "de",
			expectedLang: language.German, // MYAPP_LANG wins
		},
		{
			name:         "GOOPT_LANG ignored when custom env var set",
			customEnvVar: "MYAPP_LANG",
			gooptLang:    "fr",
			myAppLang:    "",
			expectedLang: language.Und, // GOOPT_LANG is ignored
		},
		{
			name:         "empty custom env var disables env checking",
			customEnvVar: "",
			gooptLang:    "fr",
			expectedLang: language.Und, // No env var is checked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("GOOPT_LANG")
			os.Unsetenv("MYAPP_LANG")

			// Set test environment
			if tt.gooptLang != "" {
				os.Setenv("GOOPT_LANG", tt.gooptLang)
			}
			if tt.myAppLang != "" {
				os.Setenv("MYAPP_LANG", tt.myAppLang)
			}

			p := NewParser()
			p.SetAutoLanguage(true)
			p.SetLanguageEnvVar(tt.customEnvVar)

			lang := p.detectLanguageInArgs([]string{})
			assert.Equal(t, tt.expectedLang, lang)
		})
	}
}
