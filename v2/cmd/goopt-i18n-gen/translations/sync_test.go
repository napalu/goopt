package translations

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	goopterrors "github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/types/orderedmap"
)

// Helper function to create OrderedMap for test files
func createTestFiles(files ...struct {
	name    string
	content map[string]interface{}
}) *orderedmap.OrderedMap[string, map[string]interface{}] {
	om := orderedmap.NewOrderedMap[string, map[string]interface{}]()
	for _, file := range files {
		om.Set(file.name, file.content)
	}
	return om
}

func TestSyncCommand(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  *orderedmap.OrderedMap[string, map[string]interface{}]
		targetFiles map[string]map[string]interface{}
		syncCmd     options.SyncCmd
		expectError bool
		expectFiles map[string]map[string]interface{}
	}{
		{
			name: "sync within files - add missing keys",
			setupFiles: createTestFiles(
				struct {
					name    string
					content map[string]interface{}
				}{
					"en.json",
					map[string]interface{}{
						"app.title":   "My App",
						"app.welcome": "Welcome",
					},
				},
				struct {
					name    string
					content map[string]interface{}
				}{
					"de.json",
					map[string]interface{}{
						"app.title": "Meine App",
						// missing app.welcome
					},
				},
			),
			syncCmd: options.SyncCmd{
				TodoPrefix: "[TODO]",
			},
			expectFiles: map[string]map[string]interface{}{
				"en.json": {
					"app.title":   "My App",
					"app.welcome": "Welcome",
				},
				"de.json": {
					"app.title":   "Meine App",
					"app.welcome": "[TODO] Welcome",
				},
			},
		},
		{
			name: "sync target files against reference",
			setupFiles: createTestFiles(
				struct {
					name    string
					content map[string]interface{}
				}{
					"locales/en.json",
					map[string]interface{}{
						"app.title":   "My App",
						"app.welcome": "Welcome",
						"app.help":    "Help",
					},
				},
				struct {
					name    string
					content map[string]interface{}
				}{
					"locales/de.json",
					map[string]interface{}{
						"app.title":   "Meine App",
						"app.welcome": "Willkommen",
						"app.help":    "Hilfe",
					},
				},
			),
			targetFiles: map[string]map[string]interface{}{
				"system/es.json": {
					"app.title": "Mi App",
					// missing app.welcome and app.help
				},
			},
			syncCmd: options.SyncCmd{
				Target:     []string{"system/*.json"},
				TodoPrefix: "[TODO]",
			},
			expectFiles: map[string]map[string]interface{}{
				"system/es.json": {
					"app.title":   "Mi App",
					"app.welcome": "[TODO] Welcome",
					"app.help":    "[TODO] Help",
				},
			},
		},
		{
			name: "remove extra keys",
			setupFiles: createTestFiles(
				struct {
					name    string
					content map[string]interface{}
				}{
					"en.json",
					map[string]interface{}{
						"app.title": "My App",
					},
				},
				struct {
					name    string
					content map[string]interface{}
				}{
					"de.json",
					map[string]interface{}{
						"app.title":  "Meine App",
						"app.extra1": "Extra 1",
						"app.extra2": "Extra 2",
					},
				},
			),
			syncCmd: options.SyncCmd{
				RemoveExtra: true,
			},
			expectFiles: map[string]map[string]interface{}{
				"en.json": {
					"app.title": "My App",
				},
				"de.json": {
					"app.title": "Meine App",
				},
			},
		},
		{
			name: "dry run mode",
			setupFiles: createTestFiles(
				struct {
					name    string
					content map[string]interface{}
				}{
					"en.json",
					map[string]interface{}{
						"app.title": "My App",
						"app.new":   "New",
					},
				},
				struct {
					name    string
					content map[string]interface{}
				}{
					"de.json",
					map[string]interface{}{
						"app.title": "Meine App",
					},
				},
			),
			syncCmd: options.SyncCmd{
				DryRun: true,
			},
			expectFiles: map[string]map[string]interface{}{
				"de.json": {
					"app.title": "Meine App",
					// app.new should NOT be added in dry run
				},
			},
		},
		{
			name: "flat keys with dots",
			setupFiles: createTestFiles(
				struct {
					name    string
					content map[string]interface{}
				}{
					"en.json",
					map[string]interface{}{
						"app.ui.button.ok":     "OK",
						"app.ui.button.cancel": "Cancel",
					},
				},
				struct {
					name    string
					content map[string]interface{}
				}{
					"de.json",
					map[string]interface{}{
						"app.ui.button.ok": "OK",
						// missing app.ui.button.cancel
					},
				},
			),
			syncCmd: options.SyncCmd{
				TodoPrefix: "[TODO]",
			},
			expectFiles: map[string]map[string]interface{}{
				"de.json": {
					"app.ui.button.ok":     "OK",
					"app.ui.button.cancel": "[TODO] Cancel",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tempDir := t.TempDir()

			// Setup reference files
			var inputPaths []string
			// Use the iterator to maintain insertion order
			iter := tt.setupFiles.Iterator()
			for idx, filename, content := iter(); idx != nil; idx, filename, content = iter() {
				fullPath := filepath.Join(tempDir, *filename)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				data, _ := json.MarshalIndent(content, "", "  ")
				if err := os.WriteFile(fullPath, data, 0644); err != nil {
					t.Fatal(err)
				}
				inputPaths = append(inputPaths, fullPath)
			}

			// Setup target files if any
			for filename, content := range tt.targetFiles {
				fullPath := filepath.Join(tempDir, filename)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				data, _ := json.MarshalIndent(content, "", "  ")
				if err := os.WriteFile(fullPath, data, 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Update target paths to be absolute
			if len(tt.syncCmd.Target) > 0 {
				for i, target := range tt.syncCmd.Target {
					tt.syncCmd.Target[i] = filepath.Join(tempDir, target)
				}
			}

			// Create config
			cfg := &options.AppConfig{
				Input:   inputPaths,
				Sync:    tt.syncCmd,
				TR:      i18n.NewEmptyBundle(),
				Verbose: false,
			}

			// Execute sync
			err := ExecuteSyncCommand(cfg, &cfg.Sync)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check expected files
			for filename, expectedContent := range tt.expectFiles {
				fullPath := filepath.Join(tempDir, filename)
				data, err := os.ReadFile(fullPath)
				if err != nil {
					t.Errorf("failed to read %s: %v", filename, err)
					continue
				}

				var actual map[string]interface{}
				if err := json.Unmarshal(data, &actual); err != nil {
					t.Errorf("failed to unmarshal %s: %v", filename, err)
					continue
				}

				if !deepEqual(actual, expectedContent) {
					t.Errorf("file %s content mismatch\nexpected: %+v\nactual: %+v",
						filename, expectedContent, actual)
				}
			}
		})
	}
}

func TestSyncCommandErrors(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *options.AppConfig
		expectedError error
	}{
		{
			name: "less than 2 files",
			cfg: &options.AppConfig{
				Input: []string{"en.json"},
				TR:    i18n.NewEmptyBundle(),
			},
			expectedError: goopterrors.ErrSyncRequiresAtLeastTwoFiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecuteSyncCommand(tt.cfg, &tt.cfg.Sync)
			if err == nil {
				t.Errorf("expected error %v, got nil", tt.expectedError)
			} else if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// deepEqual compares two map[string]interface{} structures
func deepEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v1 := range a {
		v2, ok := b[k]
		if !ok {
			return false
		}

		// Convert both values to strings for comparison
		// This handles the case where one might be a string and the other an interface{}
		str1 := fmt.Sprintf("%v", v1)
		str2 := fmt.Sprintf("%v", v2)

		if str1 != str2 {
			return false
		}
	}
	return true
}
