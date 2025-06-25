package main

import (
	"fmt"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

func main() {
	// Create a bundle with various language variants
	bundle := i18n.NewEmptyBundle()

	// Add translations for various languages
	languages := []language.Tag{
		language.English,
		language.MustParse("en-US"),
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

	fmt.Println("Language Matching Performance Test")
	fmt.Printf("Bundle contains %d languages\n\n", len(languages))

	// Test cases
	testCases := []struct {
		name      string
		requested language.Tag
	}{
		{"Exact Match (English)", language.English},
		{"Exact Match (German)", language.German},
		{"Regional Fallback (en-CA → en-US)", language.MustParse("en-CA")},
		{"Regional Fallback (de-AT → de)", language.MustParse("de-AT")},
		{"Base Expansion (en → en-US)", language.MustParse("en")},
		{"No Match (Swahili → default)", language.Swahili},
		{"Complex Tag", language.MustParse("en-US-u-ca-gregory")},
	}

	// Warm up
	for i := 0; i < 1000; i++ {
		for _, tc := range testCases {
			_ = bundle.MatchLanguage(tc.requested)
		}
	}

	// Benchmark each case
	const iterations = 100000
	fmt.Printf("Running %d iterations per test case:\n\n", iterations)

	for _, tc := range testCases {
		start := time.Now()
		var result language.Tag

		for i := 0; i < iterations; i++ {
			result = bundle.MatchLanguage(tc.requested)
		}

		elapsed := time.Since(start)
		nsPerOp := elapsed.Nanoseconds() / int64(iterations)

		fmt.Printf("%-40s: %8.2f ns/op (%.2f µs/op) → %s\n",
			tc.name,
			float64(nsPerOp),
			float64(nsPerOp)/1000,
			result)
	}

	// Compare with direct map lookup
	fmt.Println("\nBaseline comparison (direct map lookup):")
	start := time.Now()
	for i := 0; i < iterations; i++ {
		bundle.HasLanguage(language.English)
	}
	elapsed := time.Since(start)
	nsPerOp := elapsed.Nanoseconds() / int64(iterations)
	fmt.Printf("%-40s: %8.2f ns/op (%.2f µs/op)\n",
		"Direct map lookup",
		float64(nsPerOp),
		float64(nsPerOp)/1000)

	// Test LayeredProvider performance
	fmt.Println("\nLayeredProvider SetDefaultLanguage performance:")
	defaultBundle := i18n.NewEmptyBundle()
	systemBundle := i18n.NewEmptyBundle()
	userBundle := i18n.NewEmptyBundle()

	for _, lang := range []language.Tag{language.English, language.German, language.French} {
		translations := map[string]string{"hello": "Hello"}
		defaultBundle.AddLanguage(lang, translations)
		systemBundle.AddLanguage(lang, translations)
		userBundle.AddLanguage(lang, translations)
	}

	provider := i18n.NewLayeredMessageProvider(defaultBundle, systemBundle, userBundle)

	testLangs := []struct {
		name string
		lang language.Tag
	}{
		{"Exact match", language.English},
		{"No match", language.Spanish},
		{"Regional match", language.MustParse("en-CA")},
	}

	for _, tl := range testLangs {
		start := time.Now()
		for i := 0; i < iterations/10; i++ { // Fewer iterations as this is more expensive
			provider.SetDefaultLanguage(tl.lang)
		}
		elapsed := time.Since(start)
		nsPerOp := elapsed.Nanoseconds() / int64(iterations/10)
		fmt.Printf("%-40s: %8.2f ns/op (%.2f µs/op)\n",
			tl.name,
			float64(nsPerOp),
			float64(nsPerOp)/1000)
	}
}
