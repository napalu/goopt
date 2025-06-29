package goopt

import (
	"fmt"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
	"sync"
	"testing"
)

func TestJITTranslationRegistry_RegisterAndRetrieve(t *testing.T) {
	// Create a parser with Spanish translations
	parser := NewParser()

	// Create user bundle with flag translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help":     "ayuda",
		"goopt.flag.name.version":  "versión",
		"goopt.flag.name.language": "idioma",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register some flag metadata
	// Note: The Spanish translations in es.SystemTranslations are:
	// "goopt.flag.name.help": "ayuda"
	// "goopt.flag.name.version": "versión"
	// "goopt.flag.name.language": "idioma"
	arg1 := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg1, "")

	arg2 := &Argument{NameKey: "goopt.flag.name.version"}
	registry.RegisterFlagMetadata("version", arg2, "")

	// Test canonical flag name retrieval
	tests := []struct {
		name     string
		flag     string
		lang     language.Tag
		expected string
		found    bool
	}{
		{"Spanish help translation", "ayuda", language.Spanish, "help", true},
		{"Spanish version translation", "versión", language.Spanish, "version", true},
		{"Non-existent translation", "nonexistent", language.Spanish, "", false},
		{"English fallback", "help", language.Spanish, "help", true}, // Direct match
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical, found := registry.GetCanonicalFlagName(tt.flag, tt.lang)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected, canonical)
			}
		})
	}
}

func TestJITTranslationRegistry_CommandContext(t *testing.T) {
	parser := NewParser()

	// Create user bundle with flag translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help": "ayuda",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register flags with command context
	arg := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg, "server")

	// Test retrieval with command context
	canonical, found := registry.GetCanonicalFlagName("ayuda", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "help", canonical) // GetCanonicalFlagName returns just the flag name
}

func TestJITTranslationRegistry_LanguageSwitch(t *testing.T) {
	parser := NewParser()

	// Start with English
	parser.SetLanguage(language.English)
	registry := parser.translationRegistry

	// Register flag metadata
	arg := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg, "")

	// First access in English
	canonical, found := registry.GetCanonicalFlagName("help", language.English)
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	// Add Spanish locale and switch to Spanish
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help": "ayuda",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	// Access in Spanish should rebuild cache
	canonical, found = registry.GetCanonicalFlagName("ayuda", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	// Verify English "help" is still a direct match
	canonical, found = registry.GetCanonicalFlagName("help", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "help", canonical) // Direct match, not translation
}

func TestJITTranslationRegistry_MultipleFlags(t *testing.T) {
	parser := NewParser()

	// Create user bundle with flag translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help":     "ayuda",
		"goopt.flag.name.version":  "versión",
		"goopt.flag.name.language": "idioma",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register multiple flags
	flags := []struct {
		name    string
		arg     *Argument
		command string
	}{
		{
			"help",
			&Argument{NameKey: "goopt.flag.name.help"},
			"",
		},
		{
			"version",
			&Argument{NameKey: "goopt.flag.name.version"},
			"",
		},
		{
			"language",
			&Argument{NameKey: "goopt.flag.name.language"},
			"",
		},
	}

	for _, f := range flags {
		registry.RegisterFlagMetadata(f.name, f.arg, f.command)
	}

	// Test all translations work
	translations := map[string]string{
		"ayuda":   "help",
		"versión": "version",
		"idioma":  "language",
	}

	for trans, expected := range translations {
		canonical, found := registry.GetCanonicalFlagName(trans, language.Spanish)
		assert.True(t, found, "Should find translation for %s", trans)
		assert.Equal(t, expected, canonical)
	}
}

func TestJITTranslationRegistry_NoTranslationKey(t *testing.T) {
	parser := NewParser()
	parser.SetLanguage(language.Spanish)
	registry := parser.translationRegistry

	// Register flag without translation keys
	arg := &Argument{} // No NameKey
	registry.RegisterFlagMetadata("help", arg, "")

	// Should not find any translations (since there's no NameKey)
	_, found := registry.GetCanonicalFlagName("ayuda", language.Spanish)
	assert.False(t, found)

	// Direct match should still work
	canonical, found := registry.GetCanonicalFlagName("help", language.Spanish)
	assert.True(t, found, "Should find direct match for 'help'")
	assert.Equal(t, "help", canonical)
}

func TestJITTranslationRegistry_EmptyRegistry(t *testing.T) {
	parser := NewParser()
	parser.SetLanguage(language.Spanish)
	registry := parser.translationRegistry

	// No flags registered
	_, found := registry.GetCanonicalFlagName("anything", language.Spanish)
	assert.False(t, found)
}

func TestJITTranslationRegistry_ConcurrentAccess(t *testing.T) {
	parser := NewParser()

	// Create user bundle with flag translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help": "ayuda",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register flag
	arg := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg, "")

	// Concurrent access
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			canonical, found := registry.GetCanonicalFlagName("ayuda", language.Spanish)
			if !found || canonical != "help" {
				errors <- fmt.Errorf("unexpected result: found=%v, canonical=%s", found, canonical)
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

func TestJITTranslationRegistry_ComplexCommandStructure(t *testing.T) {
	parser := NewParser()

	// Create user bundle with flag translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help": "ayuda",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register flags at different command levels
	arg := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg, "")
	registry.RegisterFlagMetadata("help", arg, "server")
	registry.RegisterFlagMetadata("help", arg, "server start")

	// Should return one of the registered versions
	canonical, found := registry.GetCanonicalFlagName("ayuda", language.Spanish)
	assert.True(t, found)
	// Without context, we get one of the registered ones
	assert.True(t, canonical == "help" || canonical == "help@server" || canonical == "help@server start",
		"Got: %s", canonical)
}

func TestJITTranslationRegistry_DirectMatches(t *testing.T) {
	parser := NewParser()
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register flags
	arg1 := &Argument{
		NameKey: "goopt.flag.name.help",
		Short:   "h",
	}
	registry.RegisterFlagMetadata("help", arg1, "")

	// Test that the canonical name itself is recognized
	canonical, found := registry.GetCanonicalFlagName("help", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	// Test with command context
	arg2 := &Argument{NameKey: "goopt.flag.name.version"}
	registry.RegisterFlagMetadata("version", arg2, "server")

	// Direct match should work for canonical with context
	canonical, found = registry.GetCanonicalFlagName("version", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "version", canonical)
}

func TestJITTranslationRegistry_Commands(t *testing.T) {
	// Create a parser with Spanish translations
	parser := NewParser()

	// Create user bundle with command translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.command.name.server": "servidor",
		"goopt.command.name.start":  "iniciar",
		"goopt.command.name.stop":   "detener",
		"goopt.command.name.config": "configuración",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register some command metadata
	cmd1 := &Command{NameKey: "goopt.command.name.server"}
	registry.RegisterCommandMetadata("server", cmd1)

	cmd2 := &Command{NameKey: "goopt.command.name.start"}
	registry.RegisterCommandMetadata("start", cmd2)

	cmd3 := &Command{NameKey: "goopt.command.name.stop"}
	registry.RegisterCommandMetadata("stop", cmd3)

	// Test canonical command path retrieval
	tests := []struct {
		name     string
		cmd      string
		lang     language.Tag
		expected string
		found    bool
	}{
		{"Spanish server translation", "servidor", language.Spanish, "server", true},
		{"Spanish start translation", "iniciar", language.Spanish, "start", true},
		{"Spanish stop translation", "detener", language.Spanish, "stop", true},
		{"Non-existent translation", "nonexistent", language.Spanish, "", false},
		{"English fallback", "server", language.Spanish, "server", true}, // Direct match
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical, found := registry.GetCanonicalCommandPath(tt.cmd, tt.lang)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected, canonical)
			}
		})
	}
}

func TestJITTranslationRegistry_CommandsNoTranslationKey(t *testing.T) {
	parser := NewParser()
	parser.SetLanguage(language.Spanish)
	registry := parser.translationRegistry

	// Register command without translation key
	cmd := &Command{} // No NameKey
	registry.RegisterCommandMetadata("help", cmd)

	// Should not be registered since there's no NameKey
	_, found := registry.GetCanonicalCommandPath("ayuda", language.Spanish)
	assert.False(t, found)

	// Direct match should not work either (wasn't registered)
	_, found = registry.GetCanonicalCommandPath("help", language.Spanish)
	assert.False(t, found)
}

func TestJITTranslationRegistry_NestedCommands(t *testing.T) {
	parser := NewParser()

	// Create user bundle with command translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.command.name.server":           "servidor",
		"goopt.command.name.server.start":     "servidor iniciar",
		"goopt.command.name.database":         "base-de-datos",
		"goopt.command.name.database.migrate": "base-de-datos migrar",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register nested commands
	cmd1 := &Command{NameKey: "goopt.command.name.server"}
	registry.RegisterCommandMetadata("server", cmd1)

	cmd2 := &Command{NameKey: "goopt.command.name.server.start"}
	registry.RegisterCommandMetadata("server start", cmd2)

	cmd3 := &Command{NameKey: "goopt.command.name.database"}
	registry.RegisterCommandMetadata("database", cmd3)

	cmd4 := &Command{NameKey: "goopt.command.name.database.migrate"}
	registry.RegisterCommandMetadata("database migrate", cmd4)

	// Test nested command translations
	tests := []struct {
		name     string
		cmd      string
		lang     language.Tag
		expected string
		found    bool
	}{
		{"Top-level server", "servidor", language.Spanish, "server", true},
		{"Nested server start", "servidor iniciar", language.Spanish, "server start", true},
		{"Top-level database", "base-de-datos", language.Spanish, "database", true},
		{"Nested database migrate", "base-de-datos migrar", language.Spanish, "database migrate", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical, found := registry.GetCanonicalCommandPath(tt.cmd, tt.lang)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected, canonical)
			}
		})
	}
}

// Benchmark tests
func BenchmarkJITTranslationRegistry_FirstAccess(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser()
		userBundle := i18n.NewEmptyBundle()
		userBundle.AddLanguage(language.Spanish, map[string]string{
			"goopt.flag.name.help": "ayuda",
		})
		parser.SetUserBundle(userBundle)
		parser.SetLanguage(language.Spanish)

		registry := parser.translationRegistry

		// Register 50 flags (typical CLI app)
		for j := 0; j < 50; j++ {
			arg := &Argument{NameKey: fmt.Sprintf("key%d", j)}
			registry.RegisterFlagMetadata(fmt.Sprintf("flag%d", j), arg, "")
		}

		// First access triggers cache build
		registry.GetCanonicalFlagName("flag0", language.Spanish)
	}
}

func BenchmarkJITTranslationRegistry_CachedAccess(b *testing.B) {
	parser := NewParser()
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help": "ayuda",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register and warm up cache
	arg := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg, "")
	registry.GetCanonicalFlagName("ayuda", language.Spanish) // Build cache

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GetCanonicalFlagName("ayuda", language.Spanish)
	}
}

func TestJITTranslationRegistry_CommandsConcurrentAccess(t *testing.T) {
	parser := NewParser()

	// Create user bundle with command translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.command.name.server": "servidor",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register command
	cmd := &Command{NameKey: "goopt.command.name.server"}
	registry.RegisterCommandMetadata("server", cmd)

	// Concurrent access
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			canonical, found := registry.GetCanonicalCommandPath("servidor", language.Spanish)
			if !found || canonical != "server" {
				errors <- fmt.Errorf("unexpected result: found=%v, canonical=%s", found, canonical)
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

func TestJITTranslationRegistry_MixedFlagsAndCommands(t *testing.T) {
	parser := NewParser()

	// Create user bundle with both flag and command translations
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help":      "ayuda",
		"goopt.flag.name.version":   "versión",
		"goopt.command.name.server": "servidor",
		"goopt.command.name.client": "cliente",
	})
	parser.SetUserBundle(userBundle)
	parser.SetLanguage(language.Spanish)

	registry := parser.translationRegistry

	// Register flags and commands
	arg1 := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg1, "")

	arg2 := &Argument{NameKey: "goopt.flag.name.version"}
	registry.RegisterFlagMetadata("version", arg2, "")

	cmd1 := &Command{NameKey: "goopt.command.name.server"}
	registry.RegisterCommandMetadata("server", cmd1)

	cmd2 := &Command{NameKey: "goopt.command.name.client"}
	registry.RegisterCommandMetadata("client", cmd2)

	// Test that flags and commands don't interfere with each other
	// Flags
	canonical, found := registry.GetCanonicalFlagName("ayuda", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "help", canonical)

	canonical, found = registry.GetCanonicalFlagName("versión", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "version", canonical)

	// Commands
	canonical, found = registry.GetCanonicalCommandPath("servidor", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "server", canonical)

	canonical, found = registry.GetCanonicalCommandPath("cliente", language.Spanish)
	assert.True(t, found)
	assert.Equal(t, "client", canonical)

	// Ensure no cross-contamination
	_, found = registry.GetCanonicalFlagName("servidor", language.Spanish)
	assert.False(t, found, "Command translation should not work as flag")

	_, found = registry.GetCanonicalCommandPath("ayuda", language.Spanish)
	assert.False(t, found, "Flag translation should not work as command")
}
