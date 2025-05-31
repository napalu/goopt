package ast

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
)

func TestI18nSkipCommentsWithFileExtraction(t *testing.T) {
	tests := []struct {
		name              string
		code              string
		expectedExtracted []string
		expectedSkipped   []string
	}{
		{
			name: "multiline string with skip comment via file extraction",
			code: `package test

func test() {
	// i18n-skip
	query := ` + "`" + `CREATE TABLE IF NOT EXISTS "sync_groups"
				(
					group_name TEXT	primary key,
					owner      TEXT not null
				);` + "`" + `
	
	msg := "This should be extracted"
}`,
			expectedExtracted: []string{"This should be extracted"},
			expectedSkipped:   []string{"CREATE TABLE IF NOT EXISTS \"sync_groups\""},
		},
		{
			name: "inline skip comment via file extraction",
			code: `package test

func test() {
	query := "SELECT * FROM users" // i18n-skip
	msg := "This should be extracted"
}`,
			expectedExtracted: []string{"This should be extracted"},
			expectedSkipped:   []string{"SELECT * FROM users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.go")
			if err := os.WriteFile(tmpFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Create extractor
			bundle := i18n.NewEmptyBundle()
			extractor, err := NewStringExtractor(bundle, "", "", 0)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Extract from file (this uses extractFromFile internally)
			err = extractor.ExtractFromFiles(tmpFile)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check expected extracted strings
			for _, expected := range tt.expectedExtracted {
				if _, found := extracted[expected]; !found {
					t.Errorf("Expected to extract %q but it was not found", expected)
				}
			}

			// Check that skipped strings were not extracted
			for _, skipped := range tt.expectedSkipped {
				found := false
				for str := range extracted {
					if str == skipped || contains(str, skipped) {
						found = true
						break
					}
				}
				if found {
					t.Errorf("Expected to skip %q but it was extracted", skipped)
				}
			}

			// Log what was extracted for debugging
			if t.Failed() {
				t.Logf("Extracted %d strings:", len(extracted))
				for str := range extracted {
					t.Logf("  - %q", str)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}