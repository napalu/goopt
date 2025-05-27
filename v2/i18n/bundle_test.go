package i18n

import (
	"embed"
	"errors"
	"strings"
	"sync"
	"testing"

	"golang.org/x/text/language"
)

//go:embed testdata_bad
var badFS embed.FS

//go:embed testdata
var testFS embed.FS

func BenchmarkBundleInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewBundleWithFS(testFS, "testdata")
	}
}

func BenchmarkBundleAddLanguage(b *testing.B) {
	bundle, _ := NewBundleWithFS(testFS, "testdata")
	for i := 0; i < b.N; i++ {
		bundle.AddLanguage(language.English, map[string]string{"hello": "world"})
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
	b, _ := NewBundleWithFS(testFS, "testdata")
	esLang := language.Spanish

	t.Run("valid translation", func(t *testing.T) {
		err := b.AddLanguage(esLang, spanishTranslations)
		if err != nil {
			t.Fatalf("AddLanguage failed: %v", err)
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
	originalLangs := len(b.translations)

	// Create incomplete translations missing some keys
	badTranslations := map[string]string{"only.key": "value"}
	err := b.AddLanguage(language.French, badTranslations)

	if err == nil {
		t.Error("Expected validation error")
	}
	if len(b.translations) != originalLangs {
		t.Error("Should have rolled back translations")
	}
}

func TestValidateLanguageMissingDefault(t *testing.T) {
	b, _ := NewBundleWithFS(testFS, "testdata")
	delete(b.translations, b.defaultLang)

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
