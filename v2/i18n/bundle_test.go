package i18n

import (
	"embed"
	"errors"
	"github.com/stretchr/testify/require"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"golang.org/x/text/language"
)

//go:embed testdata_bad
var badFS embed.FS

//go:embed testdata
var testFS embed.FS

// Test translations embedded as strings
const testEnglishTranslations = `{
	"test": "test value",
	"error.required_flag": "Required flag missing: %s",
	"error.invalid_number": "Invalid number: %s",
	"help.usage": "Usage: %s",
	"help.commands": "Commands: %s",
	"help.flags": "Flags: %s"
}`

const testSpanishTranslations = `{
	"test": "valor de prueba",
	"error.required_flag": "Falta el flag requerido: %s",
	"error.invalid_number": "Número inválido: %s",
	"help.usage": "Uso: %s",
	"help.commands": "Comandos: %s",
	"help.flags": "Flags: %s"
}`

// createTestBundle creates a bundle with test translations loaded from strings
func createTestBundle(t *testing.T) *Bundle {
	bundle := NewEmptyBundle()

	// Load English translations
	err := bundle.LoadFromString(language.English, testEnglishTranslations)
	if err != nil {
		t.Fatalf("Failed to load English translations: %v", err)
	}

	// Set English as default
	bundle.SetDefaultLanguage(language.English)

	return bundle
}

func BenchmarkBundleInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewBundleWithFS(testFS, "testdata")
	}
}

func BenchmarkBundleAddLanguage(b *testing.B) {
	bundle, _ := NewBundleWithFS(testFS, "testdata")
	for i := 0; i < b.N; i++ {
		_ = bundle.AddLanguage(language.English, map[string]string{"hello": "world"})
	}
}

func TestNewBundle(t *testing.T) {
	b, err := NewBundleWithFS(testFS, "testdata")
	if err != nil {
		t.Fatalf("NewBundleWithFS failed: %v", err)
	}

	t.Run("default language exists", func(t *testing.T) {
		if !b.HasLanguage(language.English) {
			t.Error("default language not found")
		}
	})

	t.Run("contains embedded translations", func(t *testing.T) {
		expectedKeys := []string{"error.required_flag", "help.usage"}
		for _, key := range expectedKeys {
			if !b.HasKey(language.English, key) {
				t.Errorf("missing key %q in default language", key)
			}
		}
	})
}

var spanishTranslations = map[string]string{
	"test":                 "valor de prueba",
	"error.required_flag":  "Falta el flag requerido: %s",
	"error.invalid_number": "Número inválido: %s",
	"help.usage":           "Uso: %s",
	"help.commands":        "Comandos: %s",
	"help.flags":           "Flags: %s",
}

var frenchTranslations = map[string]string{
	"error.required_flag":  "Erreur: le flag requis est manquant: %s",
	"error.invalid_number": "Erreur: le nombre est invalide: %s",
	"help.usage":           "Utilisation: %s",
}

var germanTranslations = map[string]string{
	"error.required_flag":  "Erforderliches Flag fehlt: %s",
	"error.invalid_number": "Ungültige Nummer: %s",
	"help.usage":           "Verwendung: %s",
	"help.commands":        "Befehle: %s",
	"help.flags":           "Flags: %s",
	"invalid.key":          "ungültig",
}

func TestAddLanguage(t *testing.T) {
	b := createTestBundle(t)
	esLang := language.Spanish

	t.Run("valid translation", func(t *testing.T) {
		// Use LoadFromString instead of AddLanguage with map
		err := b.LoadFromString(esLang, testSpanishTranslations)
		if err != nil {
			t.Fatalf("LoadFromString failed: %v", err)
		}

		if !b.HasLanguage(esLang) {
			t.Error("spanish language not added")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		err := b.AddLanguage(language.French, frenchTranslations)
		if err == nil {
			t.Fatal("expected error for missing key")
		}

		if !errors.Is(err, ErrInvalidTranslations) && !strings.Contains(err.Error(), "missing key") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("extra key", func(t *testing.T) {
		err := b.AddLanguage(language.German, germanTranslations)
		if err == nil {
			t.Fatal("expected error for extra key")
		}
		if !errors.Is(err, ErrInvalidTranslations) && !strings.Contains(err.Error(), "extra key") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestFormatter(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	// 1. Verify default language exists
	if !b.HasLanguage(language.English) {
		t.Fatal("default language missing")
	}

	// 2. Add complete Spanish translations
	esLang := language.Spanish
	if err := b.AddLanguage(esLang, spanishTranslations); err != nil {
		t.Fatalf("AddLanguage failed: %v", err)
	}

	// 3. Test Spanish translations
	t.Run("existing language", func(t *testing.T) {
		p := b.Formatter(esLang)

		// Directly test translation
		expected := "Falta el flag requerido: --output"
		actual := p.Sprintf("error.required_flag", "--output")
		if actual != expected {
			t.Errorf("expected %q, got %q", expected, actual)
		}
	})

	// 4. Test fallback mechanism
	t.Run("fallback to default", func(t *testing.T) {
		p := b.Formatter(language.French) // Unadded language

		expected := "Required flag missing: --config"
		actual := p.Sprintf("error.required_flag", "--config")
		if actual != expected {
			t.Errorf("expected fallback %q, got %q", expected, actual)
		}
	})
}

func TestHasKey(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	t.Run("existing key", func(t *testing.T) {
		if !b.HasKey(language.English, "error.required_flag") {
			t.Error("key not found")
		}
	})

	t.Run("non-existent key", func(t *testing.T) {
		if b.HasKey(language.English, "invalid.key") {
			t.Error("unexpected key found")
		}
	})
}

func TestLanguages(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	t.Run("default language", func(t *testing.T) {
		langs := b.Languages()
		if len(langs) != 1 || langs[0] != language.English {
			t.Errorf("expected default language %q, got %v", language.English, langs)
		}
	})

	esLang := language.Spanish
	if err := b.AddLanguage(esLang, spanishTranslations); err != nil {
		t.Fatalf("AddLanguage failed: %v", err)
	}

	t.Run("added language", func(t *testing.T) {
		langs := b.Languages()
		if len(langs) != 2 || langs[1] != esLang {
			t.Errorf("expected added language %q, got %v", esLang, langs)
		}
	})
}

func TestHasLanguage(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	t.Run("existing language", func(t *testing.T) {
		if !b.HasLanguage(language.English) {
			t.Error("default language not found")
		}
	})

	t.Run("non-existent language", func(t *testing.T) {
		if b.HasLanguage(language.French) {
			t.Error("unexpected language found")
		}
	})
}

func TestValidateLanguage(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	t.Run("valid language", func(t *testing.T) {
		err := b.AddLanguage(language.Spanish, spanishTranslations)
		if err != nil {
			t.Fatalf("AddLanguage failed: %v", err)
		}
		if errs := b.validateLanguage(language.Spanish); len(errs) > 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		err := b.AddLanguage(language.French, map[string]string{
			"help.usage": "Usage: %s",
		})
		if err == nil {
			t.Fatal("expected error for missing key")
		}
		if !errors.Is(err, ErrInvalidTranslations) && !strings.Contains(err.Error(), "missing key") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidateAndGetPrinter(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	t.Run("valid language", func(t *testing.T) {
		err := b.AddLanguage(language.Spanish, spanishTranslations)
		if err != nil {
			t.Fatalf("AddLanguage failed: %v", err)
		}
		p := b.Formatter(language.Spanish)
		expected := "Falta el flag requerido: --output"
		if actual := p.Sprintf("error.required_flag", "--output"); actual != expected {
			t.Errorf("unexpected translation: %q", actual)
		}
	})
}

func TestFormatterConcurrency(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")

	// Add the language first to ensure printer creation
	err := b.AddLanguage(language.Spanish, spanishTranslations)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Verify we get a valid printer
			p := b.Formatter(language.Spanish)
			if p == nil {
				t.Error("Got nil printer")
			}
		}()
	}
	wg.Wait()
}

func TestNewBundleError(t *testing.T) {

	// Contains invalid directory structure
	_, err := NewBundleWithFS(badFS, "testdata_bad")
	if !errors.Is(err, ErrInvalidLanguage) {
		t.Error("Expected error for invalid directory structure")
	}
}

func TestLoadEmbeddedErrors(t *testing.T) {
	b, bundleErr := NewBundleWithFS(badFS, "testdata_bad")
	if bundleErr == nil {
		t.Fatalf("newBundleWithFS should have failed")
	}

	// Testdata contains:
	// - empty.json (empty file)
	// - invalid.json (malformed JSON)
	// - dir/ (directory)
	err := b.LoadFromFS(badFS, "testdata_bad")
	if err.Error() != bundleErr.Error() {
		t.Error("Expected errors loading test data")
	}
}

func TestAddLanguageValidationRollback(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")
	originalLangs := b.translations.Len()

	// Create incomplete translations missing some keys
	badTranslations := map[string]string{"only.key": "value"}
	err := b.AddLanguage(language.French, badTranslations)

	if err == nil {
		t.Error("Expected validation error")
	}
	if b.translations.Len() != originalLangs {
		t.Error("Should have rolled back translations")
	}
}

func TestValidateLanguageMissingDefault(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")
	b.translations.Delete(b.defaultLang.String())

	errs := b.validateLanguage(language.German)
	if len(errs) == 0 {
		t.Error("Should report missing default language")
	}
}

func TestAddLanguage_UpdatesExistingTranslations(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")
	key := "test.update.key"
	initial := "initial value"
	updated := "updated value"

	// First addition
	err := b.AddLanguage(language.English, map[string]string{
		key: initial,
	})
	if err != nil {
		t.Fatalf("Initial AddLanguage failed: %v", err)
	}

	// Verify initial value
	p := b.Formatter(language.English)
	if v := p.Sprintf(key); v != initial {
		t.Errorf("Expected %q, got %q", initial, v)
	}

	// Update translation
	err = b.AddLanguage(language.English, map[string]string{
		key: updated,
	})
	if err != nil {
		t.Fatalf("Update should succeed: %v", err)
	}

	// Verify updated value
	if v := p.Sprintf(key); v != updated {
		t.Errorf("Expected %q, got %q", updated, v)
	}
}

func TestBundleDefaults(t *testing.T) {
	// Store the current default to restore it after this test
	originalDefault := Default()
	defer SetDefault(originalDefault)

	// Test Default() singleton
	b1 := Default()
	b2 := Default()
	if b1 != b2 {
		t.Error("Default() should return same instance")
	}

	// Test SetDefault()
	custom := NewEmptyBundle()
	SetDefault(custom)
	if Default() != custom {
		t.Error("SetDefault() didn't update default bundle")
	}
}

func TestTranslations(t *testing.T) {
	b := NewEmptyBundle()
	b.AddLanguage(language.English, map[string]string{
		"test.key":    "Hello %s",
		"test.simple": "Simple",
	})

	// Test T()
	if b.T("test.simple") != "Simple" {
		t.Error("T() failed")
	}

	// Test TL()
	if b.TL(language.English, "test.key", "World") != "Hello World" {
		t.Error("TL() with args failed")
	}
}

func TestNewBundle_LoadsDefaultTranslations(t *testing.T) {
	bundle, err := NewBundle()
	assert.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.Equal(t, language.English, bundle.defaultLang)
	assert.NotNil(t, bundle.translations)
	assert.NotNil(t, bundle.catalog)
	assert.NotNil(t, bundle.printers)

	// Should have some default translations loaded
	assert.NotEmpty(t, bundle.translations)
}

func TestBundle_SetDefaultLanguage(t *testing.T) {
	bundle := NewEmptyBundle()

	// Test setting default language on empty bundle - should be allowed
	err := bundle.SetDefaultLanguage(language.Spanish)
	assert.NoError(t, err)
	// On empty bundle, any language can be set as default
	assert.Equal(t, language.Spanish, bundle.GetDefaultLanguage())

	// Add English first (to ensure it's the fallback)
	bundle.AddLanguage(language.English, map[string]string{"test": "test"})

	// Now add Spanish
	bundle.AddLanguage(language.Spanish, map[string]string{"test": "prueba"})

	// Test language matching - request French, should match to English (first in OrderedMap)
	err = bundle.SetDefaultLanguage(language.French)
	assert.NoError(t, err)
	assert.Equal(t, language.English, bundle.GetDefaultLanguage())

	// Test with immutable bundle
	bundle.isImmutable = true
	err = bundle.SetDefaultLanguage(language.German)
	assert.Error(t, err)
	assert.Equal(t, ErrBundleImmutable, err)
	assert.Equal(t, language.English, bundle.GetDefaultLanguage()) // Should not change
}

func TestBundle_Languages(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add some test translations
	bundle.translations.Set(language.English.String(), map[string]string{"test": "test"})
	bundle.translations.Set(language.French.String(), map[string]string{"test": "test"})
	bundle.translations.Set(language.German.String(), map[string]string{"test": "test"})

	languages := bundle.Languages()
	assert.Len(t, languages, 3)

	// Check that all languages are present (order not guaranteed)
	langMap := make(map[language.Tag]bool)
	for _, lang := range languages {
		langMap[lang] = true
	}
	assert.True(t, langMap[language.English])
	assert.True(t, langMap[language.French])
	assert.True(t, langMap[language.German])
}

func TestBundle_GetTranslations(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add test translations for English
	testTranslations := map[string]string{
		"hello":   "Hello",
		"goodbye": "Goodbye",
		"welcome": "Welcome",
	}
	bundle.translations.Set(language.English.String(), testTranslations)

	// Get translations
	translations := bundle.GetTranslations(language.English)
	assert.Len(t, translations, 3)
	assert.Equal(t, "Hello", translations["hello"])
	assert.Equal(t, "Goodbye", translations["goodbye"])
	assert.Equal(t, "Welcome", translations["welcome"])

	// Verify it's a copy, not the original map
	translations["new"] = "New message"
	trans, _ := bundle.translations.Get(language.English.String())
	assert.Len(t, trans, 3) // Original should still have 3

	// Test getting translations for non-existent language
	translations = bundle.GetTranslations(language.Japanese)
	assert.Nil(t, translations)
}

func TestBundleLanguagesWithKey(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add languages with matching keys (required for validation)
	err := bundle.AddLanguage(language.English, map[string]string{
		"common.key":   "English value",
		"specific.key": "English specific",
	})
	require.NoError(t, err)

	err = bundle.AddLanguage(language.Spanish, map[string]string{
		"common.key":   "Valor español",
		"specific.key": "Español específico",
	})
	require.NoError(t, err)

	err = bundle.AddLanguage(language.French, map[string]string{
		"common.key":   "Valeur française",
		"specific.key": "Français spécifique",
	})
	require.NoError(t, err)

	// Test common key present in all languages
	langs := bundle.LanguagesWithKey("common.key")
	assert.Len(t, langs, 3)

	// Check that all expected languages are present
	langSet := make(map[language.Tag]bool)
	for _, lang := range langs {
		langSet[lang] = true
	}
	assert.True(t, langSet[language.English])
	assert.True(t, langSet[language.Spanish])
	assert.True(t, langSet[language.French])

	// Test specific key present in all languages
	langs = bundle.LanguagesWithKey("specific.key")
	assert.Len(t, langs, 3)

	// Test non-existent key
	langs = bundle.LanguagesWithKey("nonexistent")
	assert.Len(t, langs, 0)
}

// TestBundleGetPrinter tests the GetPrinter method
func TestBundleGetPrinter(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add some languages
	err := bundle.AddLanguage(language.English, map[string]string{
		"test": "test",
	})
	require.NoError(t, err)

	err = bundle.AddLanguage(language.Spanish, map[string]string{
		"test": "prueba",
	})
	require.NoError(t, err)

	// Set default language
	bundle.SetDefaultLanguage(language.English)

	// Test getting printer (uses default language)
	printer := bundle.GetPrinter()
	assert.NotNil(t, printer)

	// Change default language and test again
	bundle.SetDefaultLanguage(language.Spanish)
	printer = bundle.GetPrinter()
	assert.NotNil(t, printer)
}

// TestBundleTLWithFormattingAndPlurals tests TL method with various cases
func TestBundleTLWithFormattingAndPlurals(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add English with formatting placeholders
	err := bundle.AddLanguage(language.English, map[string]string{
		"greeting":       "Hello, %s!",
		"count.items":    "%d items",
		"multiple.args":  "%s has %d %s",
		"no.args":        "Simple message",
		"percent.escape": "100%% complete",
	})
	require.NoError(t, err)

	// Test simple formatting
	result := bundle.TL(language.English, "greeting", "World")
	assert.Equal(t, "Hello, World!", result)

	// Test with number
	result = bundle.TL(language.English, "count.items", 42)
	assert.Equal(t, "42 items", result)

	// Test with multiple args
	result = bundle.TL(language.English, "multiple.args", "John", 3, "apples")
	assert.Equal(t, "John has 3 apples", result)

	// Test with no args
	result = bundle.TL(language.English, "no.args")
	assert.Equal(t, "Simple message", result)

	// Test with escaped percent
	result = bundle.TL(language.English, "percent.escape")
	assert.Equal(t, "100% complete", result)

	// Test missing key returns key
	result = bundle.TL(language.English, "missing.key")
	assert.Equal(t, "missing.key", result)

	// Test with args for missing key
	result = bundle.TL(language.English, "missing.key", "arg1", "arg2")
	assert.Equal(t, "missing.key", result)
}

// TestLoadFromStringErrors tests error cases in LoadFromString
func TestLoadFromStringErrors(t *testing.T) {
	bundle := NewEmptyBundle()

	// Test with empty JSON
	err := bundle.LoadFromString(language.English, "{}")
	assert.NoError(t, err) // Empty JSON is valid

	// Test with invalid JSON
	err = bundle.LoadFromString(language.English, "not json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")

	// Test with non-string values
	err = bundle.LoadFromString(language.English, `{"key": 123}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")

	// Test with nested objects
	err = bundle.LoadFromString(language.English, `{"key": {"nested": "value"}}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

//go:embed testdata
var testFS2 embed.FS

// TestLoadFromFSErrors tests error cases in LoadFromFS
func TestLoadFromFSErrors(t *testing.T) {
	bundle := NewEmptyBundle()

	// Create a test filesystem structure
	// We'll use the actual testdata directory if it exists

	// Test with non-existent path
	err := bundle.LoadFromFS(testFS2, "nonexistent")
	assert.Error(t, err)

	// Test with valid path
	err = bundle.LoadFromFS(testFS2, "testdata")
	assert.NoError(t, err)
}

// TestBundleFormatterCoverage tests the Formatter method thoroughly
func TestBundleFormatterCoverage(t *testing.T) {
	bundle := NewEmptyBundle()

	// Add test data
	err := bundle.AddLanguage(language.English, map[string]string{
		"simple":    "Simple message",
		"with.arg":  "Hello %s",
		"two.args":  "%s and %s",
		"number":    "Count: %d",
		"mixed":     "%s has %d items",
		"many.args": "%s %s %s %s %s",
	})
	require.NoError(t, err)

	// Test getting formatter for a language
	printer := bundle.Formatter(language.English)
	assert.NotNil(t, printer)

	// Test with non-existent language (should return fallback printer)
	printer = bundle.Formatter(language.Japanese)
	assert.NotNil(t, printer)

	// Test actual formatting through TL method instead
	tests := []struct {
		name     string
		key      string
		args     []interface{}
		expected string
	}{
		{
			name:     "Simple message",
			key:      "simple",
			args:     nil,
			expected: "Simple message",
		},
		{
			name:     "One string arg",
			key:      "with.arg",
			args:     []interface{}{"World"},
			expected: "Hello World",
		},
		{
			name:     "Two string args",
			key:      "two.args",
			args:     []interface{}{"Alice", "Bob"},
			expected: "Alice and Bob",
		},
		{
			name:     "Number arg",
			key:      "number",
			args:     []interface{}{42},
			expected: "Count: 42",
		},
		{
			name:     "Mixed args",
			key:      "mixed",
			args:     []interface{}{"John", 5},
			expected: "John has 5 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bundle.TL(language.English, tt.key, tt.args...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateLanguageEdgeCases tests edge cases in validateLanguage
func TestValidateLanguageEdgeCases(t *testing.T) {
	bundle := NewEmptyBundle()

	// First add a valid language to establish the key set
	err := bundle.AddLanguage(language.English, map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	})
	require.NoError(t, err)

	// Test adding language with exact same keys (should succeed)
	err = bundle.AddLanguage(language.Spanish, map[string]string{
		"key1": "valor1",
		"key2": "valor2",
		"key3": "valor3",
	})
	assert.NoError(t, err)

	// Test adding language with missing key
	err = bundle.AddLanguage(language.French, map[string]string{
		"key1": "valeur1",
		"key2": "valeur2",
		// missing key3
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing key")

	// Test adding language with extra key
	err = bundle.AddLanguage(language.German, map[string]string{
		"key1": "wert1",
		"key2": "wert2",
		"key3": "wert3",
		"key4": "wert4", // extra key
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extra key")
}

// TestDefaultSystemBundle tests the DefaultSystemBundle function
func TestDefaultSystemBundle(t *testing.T) {
	// Test creating default system bundle
	bundle, err := DefaultSystemBundle()
	require.NoError(t, err)
	assert.NotNil(t, bundle)

	// Test bundle has expected languages
	assert.True(t, bundle.HasLanguage(language.English))
	assert.True(t, bundle.HasLanguage(language.German))
	assert.True(t, bundle.HasLanguage(language.French))

	// Test it has some expected keys
	result := bundle.T("goopt.error.bind_nil")
	assert.NotEqual(t, "goopt.error.bind_nil", result, "Should have actual translation")
	assert.Equal(t, "can't bind flag to nil", result)
}

// TestLayeredProviderEdgeCases tests edge cases in layered provider
func TestLayeredProviderEdgeCases(t *testing.T) {
	// Create bundles
	userBundle := NewEmptyBundle()
	systemBundle := NewEmptyBundle()
	defaultBundle := NewEmptyBundle()

	// Add test data
	userBundle.AddLanguage(language.English, map[string]string{
		"user.only":     "user value",
		"user.override": "user override",
	})

	systemBundle.AddLanguage(language.English, map[string]string{
		"system.only":   "system value",
		"user.override": "system value (overridden)",
		"sys.override":  "system override",
	})

	defaultBundle.AddLanguage(language.English, map[string]string{
		"default.only":  "default value",
		"sys.override":  "default value (overridden)",
		"user.override": "default value (overridden)",
	})

	// Create provider
	provider := &LayeredMessageProvider{
		userBundle:    userBundle,
		systemBundle:  systemBundle,
		defaultBundle: defaultBundle,
	}

	// Test layering - user overrides system and default
	result, found := provider.tryGetMessage(provider.userBundle, language.English, "user.override")
	assert.True(t, found)
	assert.Equal(t, "user override", result)

	// Test system doesn't have user.override
	result, found = provider.tryGetMessage(provider.systemBundle, language.English, "user.override")
	assert.True(t, found)
	assert.Equal(t, "system value (overridden)", result)

	// Test system overrides default
	result, found = provider.tryGetMessage(provider.systemBundle, language.English, "sys.override")
	assert.True(t, found)
	assert.Equal(t, "system override", result)

	// Test default only
	result, found = provider.tryGetMessage(provider.defaultBundle, language.English, "default.only")
	assert.True(t, found)
	assert.Equal(t, "default value", result)

	// Test language not in any bundle
	result, found = provider.tryGetMessage(provider.userBundle, language.Japanese, "any.key")
	assert.False(t, found)
	assert.Equal(t, "", result)

	// Test key not in any bundle
	result, found = provider.tryGetMessage(provider.userBundle, language.English, "nonexistent")
	assert.False(t, found)
	assert.Equal(t, "", result)

	// Test nil bundles
	provider2 := &LayeredMessageProvider{
		userBundle:    nil,
		systemBundle:  nil,
		defaultBundle: defaultBundle,
	}
	result, found = provider2.tryGetMessage(provider2.defaultBundle, language.English, "default.only")
	assert.True(t, found)
	assert.Equal(t, "default value", result)

	// Test all nil bundles
	provider3 := &LayeredMessageProvider{
		userBundle:    nil,
		systemBundle:  nil,
		defaultBundle: nil,
	}
	result, found = provider3.tryGetMessage(provider3.defaultBundle, language.English, "any.key")
	assert.False(t, found)
	assert.Equal(t, "", result)
}
