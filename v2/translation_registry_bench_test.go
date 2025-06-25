package goopt

import (
	"fmt"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

// Benchmark flag matching performance with JIT translation registry
func BenchmarkTranslationRegistry_FlagMatching(b *testing.B) {
	scenarios := []struct {
		name        string
		setupFunc   func(*Parser)
		lookupName  string
		lookupLang  language.Tag
		shouldMatch bool
	}{
		{
			name: "DirectMatch_NoTranslation",
			setupFunc: func(p *Parser) {
				// Register flag without NameKey (like auto-help)
				arg := &Argument{}
				p.translationRegistry.RegisterFlagMetadata("help", arg, "")
			},
			lookupName:  "help",
			lookupLang:  language.English,
			shouldMatch: true,
		},
		{
			name: "DirectMatch_WithTranslation",
			setupFunc: func(p *Parser) {
				// Register flag with NameKey
				arg := &Argument{NameKey: "goopt.flag.name.version"}
				p.translationRegistry.RegisterFlagMetadata("version", arg, "")
			},
			lookupName:  "version",
			lookupLang:  language.English,
			shouldMatch: true,
		},
		{
			name: "TranslatedMatch_Spanish",
			setupFunc: func(p *Parser) {
				// Add Spanish translations
				userBundle := i18n.NewEmptyBundle()
				userBundle.AddLanguage(language.Spanish, map[string]string{
					"goopt.flag.name.help": "ayuda",
				})
				p.SetUserBundle(userBundle)

				// Register flag with translation
				arg := &Argument{NameKey: "goopt.flag.name.help"}
				p.translationRegistry.RegisterFlagMetadata("help", arg, "")
			},
			lookupName:  "ayuda",
			lookupLang:  language.Spanish,
			shouldMatch: true,
		},
		{
			name: "NoMatch",
			setupFunc: func(p *Parser) {
				// Register some flag
				arg := &Argument{NameKey: "goopt.flag.name.help"}
				p.translationRegistry.RegisterFlagMetadata("help", arg, "")
			},
			lookupName:  "nonexistent",
			lookupLang:  language.English,
			shouldMatch: false,
		},
		{
			name: "CommandContext_DirectMatch",
			setupFunc: func(p *Parser) {
				// Register flag with command context
				arg := &Argument{NameKey: "goopt.flag.name.verbose"}
				p.translationRegistry.RegisterFlagMetadata("verbose", arg, "server")
			},
			lookupName:  "verbose",
			lookupLang:  language.English,
			shouldMatch: true,
		},
		{
			name: "ManyFlags_DirectMatch",
			setupFunc: func(p *Parser) {
				// Register many flags to test performance with larger registry
				for i := 0; i < 100; i++ {
					arg := &Argument{NameKey: fmt.Sprintf("goopt.flag.name.flag%d", i)}
					p.translationRegistry.RegisterFlagMetadata(fmt.Sprintf("flag%d", i), arg, "")
				}
				// Add the one we'll look up
				arg := &Argument{NameKey: "goopt.flag.name.target"}
				p.translationRegistry.RegisterFlagMetadata("target", arg, "")
			},
			lookupName:  "target",
			lookupLang:  language.English,
			shouldMatch: true,
		},
		{
			name: "ManyFlags_TranslatedMatch",
			setupFunc: func(p *Parser) {
				// Add Spanish translations for many flags
				translations := make(map[string]string)
				for i := 0; i < 100; i++ {
					translations[fmt.Sprintf("goopt.flag.name.flag%d", i)] = fmt.Sprintf("bandera%d", i)
				}
				translations["goopt.flag.name.target"] = "objetivo"

				userBundle := i18n.NewEmptyBundle()
				userBundle.AddLanguage(language.Spanish, translations)
				p.SetUserBundle(userBundle)

				// Register the flags
				for i := 0; i < 100; i++ {
					arg := &Argument{NameKey: fmt.Sprintf("goopt.flag.name.flag%d", i)}
					p.translationRegistry.RegisterFlagMetadata(fmt.Sprintf("flag%d", i), arg, "")
				}
				arg := &Argument{NameKey: "goopt.flag.name.target"}
				p.translationRegistry.RegisterFlagMetadata("target", arg, "")
			},
			lookupName:  "objetivo",
			lookupLang:  language.Spanish,
			shouldMatch: true,
		},
		{
			name: "CacheWarm_RepeatedLookup",
			setupFunc: func(p *Parser) {
				// Add translations
				userBundle := i18n.NewEmptyBundle()
				userBundle.AddLanguage(language.Spanish, map[string]string{
					"goopt.flag.name.help": "ayuda",
				})
				p.SetUserBundle(userBundle)

				arg := &Argument{NameKey: "goopt.flag.name.help"}
				p.translationRegistry.RegisterFlagMetadata("help", arg, "")

				// Warm the cache
				p.translationRegistry.GetCanonicalFlagName("ayuda", language.Spanish)
			},
			lookupName:  "ayuda",
			lookupLang:  language.Spanish,
			shouldMatch: true,
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			parser := NewParser()
			parser.SetLanguage(scenario.lookupLang)
			scenario.setupFunc(parser)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				canonical, found := parser.translationRegistry.GetCanonicalFlagName(
					scenario.lookupName,
					scenario.lookupLang,
				)
				if found != scenario.shouldMatch {
					b.Fatalf("Expected match=%v but got %v for %s",
						scenario.shouldMatch, found, scenario.lookupName)
				}
				_ = canonical
			}
		})
	}
}

// Benchmark the cost of language switching
func BenchmarkTranslationRegistry_LanguageSwitch(b *testing.B) {
	parser := NewParser()

	// Set up multi-language environment
	userBundle := i18n.NewEmptyBundle()
	userBundle.AddLanguage(language.Spanish, map[string]string{
		"goopt.flag.name.help":    "ayuda",
		"goopt.flag.name.version": "versión",
	})
	userBundle.AddLanguage(language.French, map[string]string{
		"goopt.flag.name.help":    "aide",
		"goopt.flag.name.version": "version",
	})
	userBundle.AddLanguage(language.German, map[string]string{
		"goopt.flag.name.help":    "hilfe",
		"goopt.flag.name.version": "version",
	})
	parser.SetUserBundle(userBundle)

	// Register flags
	registry := parser.translationRegistry
	arg1 := &Argument{NameKey: "goopt.flag.name.help"}
	registry.RegisterFlagMetadata("help", arg1, "")
	arg2 := &Argument{NameKey: "goopt.flag.name.version"}
	registry.RegisterFlagMetadata("version", arg2, "")

	languages := []language.Tag{
		language.English,
		language.Spanish,
		language.French,
		language.German,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Cycle through languages
		lang := languages[i%len(languages)]

		// This will trigger cache rebuild if language changed
		_, _ = registry.GetCanonicalFlagName("help", lang)
	}
}

// Benchmark registration performance
func BenchmarkTranslationRegistry_Registration(b *testing.B) {
	b.Run("SingleFlag", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			parser := NewParser()
			arg := &Argument{NameKey: "goopt.flag.name.help"}
			parser.translationRegistry.RegisterFlagMetadata("help", arg, "")
		}
	})

	b.Run("FlagWithoutNameKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			parser := NewParser()
			arg := &Argument{} // No NameKey
			parser.translationRegistry.RegisterFlagMetadata("help", arg, "")
		}
	})

	b.Run("FlagWithCommandContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			parser := NewParser()
			arg := &Argument{NameKey: "goopt.flag.name.verbose"}
			parser.translationRegistry.RegisterFlagMetadata("verbose", arg, "server start")
		}
	})

	b.Run("ManyFlags", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			parser := NewParser()
			for j := 0; j < 50; j++ {
				arg := &Argument{NameKey: fmt.Sprintf("goopt.flag.name.flag%d", j)}
				parser.translationRegistry.RegisterFlagMetadata(fmt.Sprintf("flag%d", j), arg, "")
			}
		}
	})
}

// Benchmark memory allocations
func BenchmarkTranslationRegistry_Allocations(b *testing.B) {
	b.Run("GetCanonicalFlagName_Cached", func(b *testing.B) {
		parser := NewParser()
		arg := &Argument{NameKey: "goopt.flag.name.help"}
		parser.translationRegistry.RegisterFlagMetadata("help", arg, "")

		// Warm cache
		parser.translationRegistry.GetCanonicalFlagName("help", language.English)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = parser.translationRegistry.GetCanonicalFlagName("help", language.English)
		}
	})

	b.Run("GetCanonicalFlagName_CacheMiss", func(b *testing.B) {
		parser := NewParser()
		userBundle := i18n.NewEmptyBundle()
		userBundle.AddLanguage(language.Spanish, map[string]string{
			"goopt.flag.name.help": "ayuda",
		})
		parser.SetUserBundle(userBundle)

		arg := &Argument{NameKey: "goopt.flag.name.help"}
		parser.translationRegistry.RegisterFlagMetadata("help", arg, "")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Force cache miss by alternating languages
			if i%2 == 0 {
				_, _ = parser.translationRegistry.GetCanonicalFlagName("help", language.English)
			} else {
				_, _ = parser.translationRegistry.GetCanonicalFlagName("ayuda", language.Spanish)
			}
		}
	})
}
