package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDamerauLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		// Basic cases (same as plain Levenshtein)
		{"identical strings", "hello", "hello", 0},
		{"one character different", "hello", "hallo", 1},
		{"completely different", "abc", "xyz", 3},
		{"empty strings", "", "", 0},
		{"one empty string", "hello", "", 5},
		{"case sensitive", "Hello", "hello", 1},
		{"longer example", "kitten", "sitting", 3},
		{"reversed strings", "abc", "cba", 2},
		// Transposition cases (improved over plain Levenshtein)
		{"adjacent transposition", "ab", "ba", 1},
		{"transposition rekey", "rekye", "rekey", 1},
		{"transposition list", "lits", "list", 1},
		{"transposition convert", "convret", "convert", 1},
		{"transposition keyfile", "keyifle", "keyfile", 1},
		{"transposition in compound", "lsit-users", "list-users", 1},
		{"transposition + missing char", "lits-usrs", "list-users", 2},
		{"transposition middle", "acb", "abc", 1},
		// Unicode tests
		{"japanese identical", "こんにちは", "こんにちは", 0},
		{"japanese one char diff", "こんにちは", "こんにちわ", 1},
		{"chinese characters", "你好", "您好", 1},
		{"arabic rtl", "مرحبا", "مرحبا", 0},
		{"arabic with diff", "مرحبا", "مرحب", 1},
		{"emoji", "😀😃", "😀😄", 1},
		{"mixed scripts", "hello世界", "hello世间", 1},
		{"combining chars", "café", "cafe", 1}, // é vs e
		{"hebrew rtl", "שלום", "שלם", 1},
		{"devanagari", "नमस्ते", "नमस्कार", 3},
		// Unicode transposition
		{"unicode transposition", "こん", "んこ", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DamerauLevenshteinDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLevenshteinDistanceAlias(t *testing.T) {
	// Verify the deprecated alias returns the same results
	assert.Equal(t, DamerauLevenshteinDistance("hello", "hallo"), LevenshteinDistance("hello", "hallo"))
	assert.Equal(t, DamerauLevenshteinDistance("rekye", "rekey"), LevenshteinDistance("rekye", "rekey"))
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxLen   int
		expected string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max length", "hello", 5, "hello"},
		{"longer than max", "hello world", 8, "hello..."},
		{"empty string", "", 5, ""},
		{"max length 0", "hello", 0, "..."},
		{"max length 3", "hello", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.s, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

