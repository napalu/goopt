package orderedmap

import (
	"testing"
)

// Benchmark comparing OrderedMap vs built-in map performance

func BenchmarkOrderedMapSet(b *testing.B) {
	om := NewOrderedMap[string, int]()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		om.Set("key", i)
	}
}

func BenchmarkBuiltinMapSet(b *testing.B) {
	m := make(map[string]int)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m["key"] = i
	}
}

func BenchmarkOrderedMapGet(b *testing.B) {
	om := NewOrderedMap[string, int]()
	om.Set("key", 42)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = om.Get("key")
	}
}

func BenchmarkBuiltinMapGet(b *testing.B) {
	m := make(map[string]int)
	m["key"] = 42
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m["key"]
	}
}

func BenchmarkOrderedMapSetMultiple(b *testing.B) {
	keys := []string{"en", "fr", "de", "es", "it", "pt", "ja", "zh", "ko", "ar"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		om := NewOrderedMap[string, int]()
		for j, key := range keys {
			om.Set(key, j)
		}
	}
}

func BenchmarkBuiltinMapSetMultiple(b *testing.B) {
	keys := []string{"en", "fr", "de", "es", "it", "pt", "ja", "zh", "ko", "ar"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m := make(map[string]int)
		for j, key := range keys {
			m[key] = j
		}
	}
}

func BenchmarkOrderedMapIteration(b *testing.B) {
	om := NewOrderedMap[string, int]()
	for i := 0; i < 10; i++ {
		om.Set(string(rune('a'+i)), i)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		count := 0
		for iter := om.Front(); iter != nil; iter = iter.Next() {
			count += iter.Value
		}
	}
}

func BenchmarkBuiltinMapIteration(b *testing.B) {
	m := make(map[string]int)
	for i := 0; i < 10; i++ {
		m[string(rune('a'+i))] = i
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		count := 0
		for _, v := range m {
			count += v
		}
	}
}

// Memory allocation benchmarks
func BenchmarkOrderedMapMemory(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		om := NewOrderedMap[string, map[string]string]()
		// Simulate language bundle scenario
		om.Set("en", map[string]string{"hello": "Hello", "world": "World"})
		om.Set("fr", map[string]string{"hello": "Bonjour", "world": "Monde"})
		om.Set("de", map[string]string{"hello": "Hallo", "world": "Welt"})
	}
}

func BenchmarkBuiltinMapMemory(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := make(map[string]map[string]string)
		// Simulate language bundle scenario
		m["en"] = map[string]string{"hello": "Hello", "world": "World"}
		m["fr"] = map[string]string{"hello": "Bonjour", "world": "Monde"}
		m["de"] = map[string]string{"hello": "Hallo", "world": "Welt"}
	}
}
