package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() (*goopt.Parser, *options.AppConfig, string)
		wantError bool
		validate  func(t *testing.T, tempDir string, cfg *options.AppConfig)
	}{
		{
			name: "add single key",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create existing locale file
				localeFile := filepath.Join(tempDir, "en.json")
				existingData := map[string]string{
					"existing.key": "Existing Value",
				}
				data, _ := json.Marshal(existingData)
				os.WriteFile(localeFile, data, 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Add: options.AddCmd{
						Key:   "new.key",
						Value: "New Value",
						Mode:  "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)
				
				if data["new.key"] != "New Value" {
					t.Error("new key should be added")
				}
				if data["existing.key"] != "Existing Value" {
					t.Error("existing key should remain")
				}
			},
		},
		{
			name: "add from file",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create locale file
				localeFile := filepath.Join(tempDir, "en.json")
				existingData := map[string]string{
					"existing": "value",
				}
				data, _ := json.Marshal(existingData)
				os.WriteFile(localeFile, data, 0644)
				
				// Create keys file
				keysFile := filepath.Join(tempDir, "keys.json")
				newKeys := map[string]string{
					"key1": "Value 1",
					"key2": "Value 2",
				}
				keysData, _ := json.Marshal(newKeys)
				os.WriteFile(keysFile, keysData, 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Add: options.AddCmd{
						FromFile: keysFile,
						Mode:     "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)
				
				if data["key1"] != "Value 1" {
					t.Error("key1 should be added")
				}
				if data["key2"] != "Value 2" {
					t.Error("key2 should be added")
				}
				if data["existing"] != "value" {
					t.Error("existing key should remain")
				}
			},
		},
		{
			name: "both single key and file error",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "en.json")},
					Add: options.AddCmd{
						Key:      "key",
						Value:    "value",
						FromFile: "keys.json",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: true,
			validate:  nil,
		},
		{
			name: "missing value for single key",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "en.json")},
					Add: options.AddCmd{
						Key:  "key",
						Mode: "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: true,
			validate:  nil,
		},
		{
			name: "skip mode with existing key",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				existingData := map[string]string{
					"existing": "Original Value",
				}
				data, _ := json.Marshal(existingData)
				os.WriteFile(localeFile, data, 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Add: options.AddCmd{
						Key:   "existing",
						Value: "New Value",
						Mode:  "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)
				
				if data["existing"] != "Original Value" {
					t.Error("existing key should not be changed in skip mode")
				}
			},
		},
		{
			name: "replace mode with existing key",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				existingData := map[string]string{
					"existing": "Original Value",
				}
				data, _ := json.Marshal(existingData)
				os.WriteFile(localeFile, data, 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					TR: bundle,
				}
				cfg.Add.Exec = Add
				
				parser, err := goopt.NewParserFromStruct(cfg, 
					goopt.WithFlagNameConverter(goopt.ToKebabCase),
					goopt.WithCommandNameConverter(goopt.ToKebabCase))
				if err != nil {
					t.Fatalf("Failed to create parser: %v", err)
				}
				
				// Parse command line with the add command
				cmdLine := fmt.Sprintf("add -i %s -k existing -V \"Replaced Value\" -m replace", localeFile)
				if !parser.ParseString(cmdLine) {
					// Print parser errors for debugging
					for _, err := range parser.GetErrors() {
						t.Logf("Parser error: %v", err)
					}
					t.Fatalf("Failed to parse command line: %s", cmdLine)
				}
				
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)
				
				if data["existing"] != "Replaced Value" {
					t.Errorf("existing key should be replaced in replace mode, got %q", data["existing"])
				}
			},
		},
		{
			name: "error mode with existing key",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				existingData := map[string]string{
					"existing": "Original Value",
				}
				data, _ := json.Marshal(existingData)
				os.WriteFile(localeFile, data, 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					TR: bundle,
				}
				cfg.Add.Exec = Add
				
				parser, err := goopt.NewParserFromStruct(cfg,
					goopt.WithFlagNameConverter(goopt.ToKebabCase),
					goopt.WithCommandNameConverter(goopt.ToKebabCase))
				if err != nil {
					t.Fatalf("Failed to create parser: %v", err)
				}
				
				cmdLine := fmt.Sprintf("add -i %s -k existing -V \"New Value\" -m error", localeFile)
				if !parser.ParseString(cmdLine) {
					t.Fatalf("Failed to parse command line: %s", cmdLine)
				}
				
				return parser, cfg, tempDir
			},
			wantError: true,
			validate:  nil,
		},
		{
			name: "invalid mode",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "en.json")},
					Add: options.AddCmd{
						Key:   "key",
						Value: "value",
						Mode:  "invalid",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: true,
			validate:  nil,
		},
		{
			name: "dry run mode",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				existingData := map[string]string{
					"existing": "value",
				}
				data, _ := json.Marshal(existingData)
				os.WriteFile(localeFile, data, 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Add: options.AddCmd{
						Key:    "new",
						Value:  "value",
						Mode:   "skip",
						DryRun: true,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)
				
				if _, exists := data["new"]; exists {
					t.Error("new key should not be added in dry run mode")
				}
			},
		},
		{
			name: "multiple locale files",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create multiple locale files
				enFile := filepath.Join(tempDir, "en.json")
				frFile := filepath.Join(tempDir, "fr.json")
				
				os.WriteFile(enFile, []byte("{}"), 0644)
				os.WriteFile(frFile, []byte("{}"), 0644)
				
				bundle := i18n.NewEmptyBundle()
				// Load test translations for TODO prefix
				bundle.AddLanguage(language.English, map[string]string{
					"app.ast.todo_prefix": "[TODO] %s",
				})
				
				cfg := &options.AppConfig{
					Input: []string{enFile, frFile},
					Add: options.AddCmd{
						Key:   "greeting",
						Value: "Hello",
						Mode:  "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check English file
				enContent, _ := os.ReadFile(cfg.Input[0])
				var enData map[string]string
				json.Unmarshal(enContent, &enData)
				
				if enData["greeting"] != "Hello" {
					t.Error("key should be added to English file")
				}
				
				// Check French file
				frContent, _ := os.ReadFile(cfg.Input[1])
				var frData map[string]string
				json.Unmarshal(frContent, &frData)
				
				if frData["greeting"] != "[TODO] Hello" {
					t.Errorf("key should be added to French file with TODO prefix, got %q", frData["greeting"])
				}
			},
		},
		{
			name: "no keys provided",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "en.json")},
					Add: options.AddCmd{
						Mode: "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: true,
			validate:  nil,
		},
		{
			name: "wildcard input pattern",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create multiple locale files
				os.WriteFile(filepath.Join(tempDir, "en.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(tempDir, "fr.json"), []byte("{}"), 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "*.json")},
					Add: options.AddCmd{
						Key:   "test",
						Value: "Test Value",
						Mode:  "skip",
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should have added to both files
				files, _ := filepath.Glob(filepath.Join(tempDir, "*.json"))
				if len(files) != 2 {
					t.Fatalf("expected 2 files, got %d", len(files))
				}
				
				for _, file := range files {
					content, _ := os.ReadFile(file)
					var data map[string]string
					json.Unmarshal(content, &data)
					
					if _, exists := data["test"]; !exists {
						t.Errorf("key should be added to %s", file)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, cfg, tempDir := tt.setup()
			
			err := Add(parser, nil)
			
			if (err != nil) != tt.wantError {
				t.Errorf("Add() error = %v, wantError %v", err, tt.wantError)
				return
			}
			
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, tempDir, cfg)
			}
		})
	}
}