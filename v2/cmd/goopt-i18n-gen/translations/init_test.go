package translations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/i18n"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() (*goopt.Parser, *options.AppConfig, string)
		wantError bool
		validate  func(t *testing.T, tempDir string, cfg *options.AppConfig)
	}{
		{
			name: "create new locale file",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "locales", "en.json")},
					Init: options.InitCmd{
						Force: false,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check file was created
				filePath := filepath.Join(tempDir, "locales", "en.json")
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("expected file %q to be created", filePath)
					return
				}

				// Check content
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("failed to read created file: %v", err)
				}

				var data map[string]interface{}
				if err := json.Unmarshal(content, &data); err != nil {
					t.Fatalf("created file is not valid JSON: %v", err)
				}

				// Should have example keys
				expectedKeys := []string{"app.name", "app.description", "app.version"}
				for _, key := range expectedKeys {
					if _, ok := data[key]; !ok {
						t.Errorf("expected key %q in initialized file", key)
					}
				}
			},
		},
		{
			name: "file already exists without force",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				existingFile := filepath.Join(tempDir, "existing.json")
				
				// Create existing file
				os.MkdirAll(filepath.Dir(existingFile), 0755)
				os.WriteFile(existingFile, []byte(`{"existing": "content"}`), 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{existingFile},
					Init: options.InitCmd{
						Force: false,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false, // Init doesn't error, just skips existing files
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Original file should remain unchanged
				content, _ := os.ReadFile(cfg.Input[0])
				if string(content) != `{"existing": "content"}` {
					t.Error("existing file should not be modified without force")
				}
			},
		},
		{
			name: "force overwrite existing file",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				existingFile := filepath.Join(tempDir, "force.json")
				
				// Create existing file
				os.MkdirAll(filepath.Dir(existingFile), 0755)
				os.WriteFile(existingFile, []byte(`{"old": "data"}`), 0644)
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{existingFile},
					Init: options.InitCmd{
						Force: true,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, err := os.ReadFile(cfg.Input[0])
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				var data map[string]interface{}
				if err := json.Unmarshal(content, &data); err != nil {
					t.Fatalf("file is not valid JSON: %v", err)
				}

				// Should have new content, not old
				if _, ok := data["old"]; ok {
					t.Error("old content should be overwritten")
				}
				if _, ok := data["app.name"]; !ok {
					t.Error("should have new initialized content")
				}
			},
		},
		{
			name: "default to locales/en.json when no input",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				// Change to temp dir to test default behavior
				oldWd, _ := os.Getwd()
				os.Chdir(tempDir)
				t.Cleanup(func() { os.Chdir(oldWd) })
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{}, // Empty input
					Init: options.InitCmd{
						Force: false,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should create locales/en.json in current directory
				defaultPath := filepath.Join(tempDir, "locales", "en.json")
				if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
					t.Errorf("expected default file %q to be created", defaultPath)
				}
			},
		},
		{
			name: "multiple files initialization",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				files := []string{
					filepath.Join(tempDir, "en.json"),
					filepath.Join(tempDir, "fr.json"),
					filepath.Join(tempDir, "de.json"),
				}
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: files,
					Init: options.InitCmd{
						Force: false,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// All files should be created
				for _, file := range cfg.Input {
					if _, err := os.Stat(file); os.IsNotExist(err) {
						t.Errorf("expected file %q to be created", file)
						continue
					}

					// Each should have valid JSON
					content, _ := os.ReadFile(file)
					var data map[string]interface{}
					if err := json.Unmarshal(content, &data); err != nil {
						t.Errorf("file %q is not valid JSON: %v", file, err)
					}
				}
			},
		},
		{
			name: "create deeply nested directory structure",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				deepPath := filepath.Join(tempDir, "deep", "nested", "path", "locale.json")
				
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{deepPath},
					Init: options.InitCmd{
						Force: false,
					},
					TR: bundle,
				}
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				if _, err := os.Stat(cfg.Input[0]); os.IsNotExist(err) {
					t.Errorf("expected file %q to be created in deep path", cfg.Input[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, cfg, tempDir := tt.setup()
			
			err := Init(parser, nil)
			
			if (err != nil) != tt.wantError {
				t.Errorf("Init() error = %v, wantError %v", err, tt.wantError)
				return
			}
			
			if tt.validate != nil {
				tt.validate(t, tempDir, cfg)
			}
		})
	}
}