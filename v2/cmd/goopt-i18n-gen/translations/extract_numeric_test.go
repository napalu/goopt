package translations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/util"
)

// TestExtractNumericStringsConsistency tests that numeric strings generate consistent
// keys and comments between extract and generate commands
func TestExtractNumericStringsConsistency(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create test Go file with numeric strings
	testFile := filepath.Join(tmpDir, "numeric_test.go")
	testCode := `package main

import "fmt"

func main() {
	// Various numeric strings that caused issues
	fmt.Println("Error 404: Not found")
	fmt.Println("2023-12-25 00:00:00")
	fmt.Println("0000_00_00_00_00_00")
	fmt.Println("123 items found")
	fmt.Println("12345")
	fmt.Println("Order #12345 processed")
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create locale directory
	localeDir := filepath.Join(tmpDir, "locales")
	if err := os.MkdirAll(localeDir, 0755); err != nil {
		t.Fatalf("Failed to create locale dir: %v", err)
	}

	// Create empty en.json file
	enFile := filepath.Join(localeDir, "en.json")
	if err := os.WriteFile(enFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write en.json: %v", err)
	}

	// This test verifies that the numeric strings will generate the expected keys
	// The actual testing would need to be done through the command line interface
	// or by refactoring the extract command to be more testable

	// For now, we test the key generation directly
	testCases := []struct {
		input          string
		expectedKey    string
		expectedGoName string
	}{
		{
			input:          "Error 404: Not found",
			expectedKey:    "app.extracted.error_404__not_found",
			expectedGoName: "ErrorN404NotFound",
		},
		{
			input:          "2023-12-25 00:00:00",
			expectedKey:    "app.extracted.n2023_12_25_00_00_00",
			expectedGoName: "N20231225000000",
		},
		{
			input:          "0000_00_00_00_00_00",
			expectedKey:    "app.extracted.n0000_00_00_00_00_00",
			expectedGoName: "N00000000000000",
		},
		{
			input:          "123 items found",
			expectedKey:    "app.extracted.n123_items_found",
			expectedGoName: "N123ItemsFound",
		},
		{
			input:          "12345",
			expectedKey:    "app.extracted.n12345",
			expectedGoName: "N12345",
		},
		{
			input:          "Order #12345 processed",
			expectedKey:    "app.extracted.order__12345_processed",
			expectedGoName: "OrderN12345Processed",
		},
	}

	// Test key generation
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			// Use the actual GenerateKeyFromString from util package
			key := util.GenerateKeyFromString("app.extracted", tc.input)
			if key != tc.expectedKey {
				t.Errorf("GenerateKeyFromString(%q) = %q, want %q", tc.input, key, tc.expectedKey)
			}

			// Use the actual KeyToGoName from util package
			lastPart := strings.Split(tc.expectedKey, ".")
			goName := util.KeyToGoName(lastPart[len(lastPart)-1])
			if goName != tc.expectedGoName {
				t.Errorf("KeyToGoName for %q = %q, want %q", tc.input, goName, tc.expectedGoName)
			}
		})
	}
}
