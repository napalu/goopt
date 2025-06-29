package i18n_test

import (
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
	"testing"
)

func BenchmarkLanguageMatching(b *testing.B) {
	// Create a bundle with various language variants
	bundle := i18n.NewEmptyBundle()

	// Add translations for various languages
	languages := []language.Tag{
		language.English,
		language.AmericanEnglish,
		language.BritishEnglish,
		language.German,
		language.French,
		language.Spanish,
		language.Italian,
		language.Portuguese,
		language.BrazilianPortuguese,
		language.Japanese,
		language.SimplifiedChinese,
		language.TraditionalChinese,
		language.Korean,
		language.Arabic,
		language.Russian,
	}

	// Add basic translations for each language
	for _, lang := range languages {
		translations := map[string]string{
			"hello":   "Hello",
			"goodbye": "Goodbye",
			"welcome": "Welcome",
		}
		bundle.AddLanguage(lang, translations)
	}

	// Test cases for matching
	testCases := []struct {
		name      string
		requested language.Tag
	}{
		{"ExactMatch_English", language.English},
		{"ExactMatch_German", language.German},
		{"RegionalFallback_CanadianEnglish", language.MustParse("en-CA")},
		{"RegionalFallback_AustrianGerman", language.MustParse("de-AT")},
		{"BaseExpansion_GenericEnglish", language.MustParse("en")},
		{"NoMatch_Swahili", language.Swahili},
		{"ComplexTag_EnglishUS_Latn", language.MustParse("en-US-u-ca-gregory")},
	}

	b.Run("MatchLanguage", func(b *testing.B) {
		for _, tc := range testCases {
			b.Run(tc.name, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = bundle.MatchLanguage(tc.requested)
				}
			})
		}
	})

	b.Run("DirectLookup_Baseline", func(b *testing.B) {
		// Baseline: direct map lookup without matching
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = bundle.HasLanguage(language.English)
		}
	})
}

func BenchmarkLayeredProviderSetLanguage(b *testing.B) {
	// Create bundles
	defaultBundle := i18n.NewEmptyBundle()
	systemBundle := i18n.NewEmptyBundle()
	userBundle := i18n.NewEmptyBundle()

	// Add translations
	languages := []language.Tag{
		language.English,
		language.AmericanEnglish,
		language.BritishEnglish,
		language.German,
		language.French,
	}

	for _, lang := range languages {
		translations := map[string]string{
			"hello": "Hello",
			"bye":   "Bye",
		}
		defaultBundle.AddLanguage(lang, translations)
		systemBundle.AddLanguage(lang, translations)
		userBundle.AddLanguage(lang, translations)
	}

	provider := i18n.NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle)

	testCases := []struct {
		name string
		lang language.Tag
	}{
		{"ExactMatch", language.English},
		{"RegionalMatch", language.MustParse("en-CA")},
		{"BaseMatch", language.MustParse("en")},
		{"NoMatch", language.Swahili},
	}

	b.Run("SetDefaultLanguage", func(b *testing.B) {
		for _, tc := range testCases {
			b.Run(tc.name, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					provider.SetDefaultLanguage(tc.lang)
				}
			})
		}
	})
}

func BenchmarkMessageRetrieval(b *testing.B) {
	// Create a realistic scenario with layered provider
	defaultBundle := i18n.NewEmptyBundle()
	systemBundle := i18n.NewEmptyBundle()
	userBundle := i18n.NewEmptyBundle()

	// Add translations
	defaultBundle.AddLanguage(language.English, map[string]string{
		"app.name":        "My App",
		"app.description": "A great application",
		"error.generic":   "An error occurred: %s",
	})

	systemBundle.AddLanguage(language.English, map[string]string{
		"error.generic": "System error occurred: %s",
	})

	userBundle.AddLanguage(language.English, map[string]string{
		"app.name": "Custom App Name",
	})
	userBundle.AddLanguage(language.German, map[string]string{
		"app.name":        "Meine App",
		"app.description": "Eine großartige Anwendung",
		"error.generic":   "Ein Fehler ist aufgetreten: %s",
	})

	provider := i18n.NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle)

	b.Run("GetMessage_DefaultLang", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.GetMessage("app.name")
		}
	})

	b.Run("GetMessage_AfterLanguageSwitch", func(b *testing.B) {
		provider.SetDefaultLanguage(language.German)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.GetMessage("app.name")
		}
	})

	b.Run("T_NoArgs", func(b *testing.B) {
		// Test T() without arguments
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.T("app.name")
		}
	})

	b.Run("TL_German", func(b *testing.B) {
		// Test TL() with specific language
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.TL(language.German, "app.name")
		}
	})
}

func BenchmarkMatcher(b *testing.B) {
	// Benchmark the raw language.Matcher performance
	tags := []language.Tag{
		language.English,
		language.AmericanEnglish,
		language.BritishEnglish,
		language.German,
		language.French,
		language.Spanish,
		language.Italian,
		language.Portuguese,
		language.BrazilianPortuguese,
		language.Japanese,
		language.SimplifiedChinese,
		language.TraditionalChinese,
	}

	matcher := language.NewMatcher(tags)

	testRequests := []language.Tag{
		language.MustParse("en-CA"),
		language.MustParse("en"),
		language.MustParse("de-AT"),
		language.Swahili,
		language.MustParse("pt-PT"),
		language.MustParse("zh"),
	}

	b.Run("RawMatcher", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, req := range testRequests {
				_, _, _ = matcher.Match(req)
			}
		}
	})
}
