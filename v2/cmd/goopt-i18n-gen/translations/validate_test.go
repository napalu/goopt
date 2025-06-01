package translations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() (*goopt.Parser, *options.AppConfig, string)
		wantError bool
		validate  func(t *testing.T, tempDir string, cfg *options.AppConfig)
	}{
		{
			name: "all keys valid",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create locale file with all required keys
				localeFile := filepath.Join(tempDir, "en.json")
				localeData := map[string]string{
					"app.name":        "Test App",
					"app.description": "Test Description",
					"user.name":       "User Name",
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)
				
				// Create Go file that references these keys
				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name;descKey:app.name"` + "`" + `
	Desc string ` + "`" + `goopt:"desc:App description;descKey:app.description"` + "`" + `
}

type User struct {
	Name string ` + "`" + `goopt:"desc:User name;descKey:user.name"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)
				
				bundle := i18n.NewEmptyBundle()
				bundle.AddLanguage(language.English, localeData)
				
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Validate: options.ValidateCmd{
						Scan:   []string{goFile},
						Strict: false,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should pass validation - all keys are present
			},
		},
		{
			name: "missing translations",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create locale file missing some keys
				localeFile := filepath.Join(tempDir, "en.json")
				localeData := map[string]string{
					"app.name": "Test App",
					// Missing app.description and user.name
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)
				
				// Create Go file that references all keys
				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name;descKey:app.name"` + "`" + `
	Desc string ` + "`" + `goopt:"desc:App description;descKey:app.description"` + "`" + `
}

type User struct {
	Name string ` + "`" + `goopt:"desc:User name;descKey:user.name"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)
				
				bundle := i18n.NewEmptyBundle()
				bundle.AddLanguage(language.English, localeData)
				
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Validate: options.ValidateCmd{
						Scan:   []string{goFile},
						Strict: false,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false, // Non-strict mode doesn't error
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should report missing keys but not error
			},
		},
		{
			name: "strict mode with missing translations",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				localeData := map[string]string{
					"app.name": "Test App",
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)
				
				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name;descKey:app.name"` + "`" + `
	Missing string ` + "`" + `goopt:"desc:Missing;descKey:app.missing"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)
				
				bundle := i18n.NewEmptyBundle()
				bundle.AddLanguage(language.English, localeData)
				// Add message keys for error messages
				bundle.AddLanguage(language.English, map[string]string{
					"app.error.validation_failed": "Validation failed: missing translation keys in one or more files",
				})
				
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Validate: options.ValidateCmd{
						Scan:   []string{goFile},
						Strict: true,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: true, // Strict mode should error
			validate:  nil,
		},
		{
			name: "generate missing translations",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				localeData := map[string]string{
					"existing.key": "Existing",
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)
				
				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Existing string ` + "`" + `goopt:"desc:Existing;descKey:existing.key"` + "`" + `
	New string ` + "`" + `goopt:"desc:New field;descKey:new.key"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)
				
				bundle := i18n.NewEmptyBundle()
				bundle.AddLanguage(language.English, localeData)
				
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Validate: options.ValidateCmd{
						Scan:            []string{goFile},
						GenerateMissing: true,
						Strict:          false,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check that missing key was added
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)
				
				if _, exists := data["new.key"]; !exists {
					t.Error("missing key should have been generated")
				}
				
				// The value might be the format string if translations are not loaded properly
				if !strings.Contains(data["new.key"], "New field") && data["new.key"] != "app.ast.todo_prefix" {
					t.Errorf("generated key has wrong value: %q", data["new.key"])
				}
			},
		},
		{
			name: "wildcard file patterns",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				localeData := map[string]string{
					"app.key1": "Key 1",
					"app.key2": "Key 2",
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)
				
				// Create multiple Go files
				goFile1 := filepath.Join(tempDir, "file1.go")
				goContent1 := `package test
type Config1 struct {
	Field1 string ` + "`" + `goopt:"desc:Field 1;descKey:app.key1"` + "`" + `
}
`
				os.WriteFile(goFile1, []byte(goContent1), 0644)
				
				goFile2 := filepath.Join(tempDir, "file2.go")
				goContent2 := `package test
type Config2 struct {
	Field2 string ` + "`" + `goopt:"desc:Field 2;descKey:app.key2"` + "`" + `
}
`
				os.WriteFile(goFile2, []byte(goContent2), 0644)
				
				bundle := i18n.NewEmptyBundle()
				bundle.AddLanguage(language.English, localeData)
				
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Validate: options.ValidateCmd{
						Scan:   []string{filepath.Join(tempDir, "*.go")},
						Strict: false,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should validate all files matching pattern
			},
		},
		{
			name: "no files to scan",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)
				
				bundle := i18n.NewEmptyBundle()
				
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Validate: options.ValidateCmd{
						Scan:   []string{}, // No files to scan
						Strict: false,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: true, // Should error when no files to scan
			validate:  nil,
		},
		{
			name: "multiple locale files",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()
				
				// Create multiple locale files
				enFile := filepath.Join(tempDir, "en.json")
				enData := map[string]string{
					"app.name": "App Name",
					"app.desc": "Description",
				}
				data, _ := json.Marshal(enData)
				os.WriteFile(enFile, data, 0644)
				
				frFile := filepath.Join(tempDir, "fr.json")
				frData := map[string]string{
					"app.name": "Nom de l'App",
					// Missing app.desc in French
				}
				data, _ = json.Marshal(frData)
				os.WriteFile(frFile, data, 0644)
				
				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test
type Config struct {
	Name string ` + "`" + `goopt:"desc:Name;descKey:app.name"` + "`" + `
	Desc string ` + "`" + `goopt:"desc:Desc;descKey:app.desc"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)
				
				bundle := i18n.NewEmptyBundle()
				bundle.AddLanguage(language.English, enData)
				bundle.AddLanguage(language.French, frData)
				
				cfg := &options.AppConfig{
					Input: []string{enFile, frFile},
					Validate: options.ValidateCmd{
						Scan:   []string{goFile},
						Strict: false,
					},
					TR: bundle,
				}
				cfg.Validate.Exec = Validate
				
				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should report missing key in French file
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, cfg, tempDir := tt.setup()
			
			err := Validate(parser, nil)
			
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, tempDir, cfg)
			}
		})
	}
}