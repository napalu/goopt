package ast

import (
	"testing"
)

func TestSlogKeyValueNotExtracted(t *testing.T) {
	code := `package main
import "log/slog"

func test() {
	slog.Info("Starting server", "port", 8080, "host", "localhost")
	logger.Error("Database error", "query", sql, "error", err)
}`
	
	// Test without skip pattern - shows the problem
	t.Run("without skip pattern", func(t *testing.T) {
		extractor, _ := NewStringExtractor(nil, "", "", 0)
		err := extractor.ExtractFromString("test.go", code)
		if err != nil {
			t.Fatalf("Failed to extract: %v", err)
		}
		
		extracted := extractor.GetExtractedStrings()
		
		// This will extract everything including keys
		if _, found := extracted["port"]; !found {
			t.Errorf("Without skip pattern, 'port' would be extracted")
		}
		
		t.Logf("Without skip pattern, extracted %d strings (including unwanted keys)", len(extracted))
	})
	
	// Test WITH skip pattern - shows the solution
	t.Run("with skip pattern", func(t *testing.T) {
		// Use the recommended pattern to skip strings without spaces
		extractor, _ := NewStringExtractor(nil, "", `^[^\s]+$`, 0)
		
		err := extractor.ExtractFromString("test.go", code)
		if err != nil {
			t.Fatalf("Failed to extract: %v", err)
		}
		
		extracted := extractor.GetExtractedStrings()
		
		// We should extract the message strings
		expectedMessages := []string{"Starting server", "Database error"}
		for _, msg := range expectedMessages {
			if _, found := extracted[msg]; !found {
				t.Errorf("Expected to extract message %q", msg)
			}
		}
		
		// We should NOT extract the keys when using skip pattern
		unexpectedKeys := []string{"port", "host", "query", "error", "localhost"}
		for _, key := range unexpectedKeys {
			if _, found := extracted[key]; found {
				t.Errorf("Should not extract slog key %q when using skip pattern", key)
			}
		}
		
		// Log what was actually extracted
		t.Logf("With skip pattern, extracted %d strings:", len(extracted))
		for str := range extracted {
			t.Logf("  - %q", str)
		}
	})
}