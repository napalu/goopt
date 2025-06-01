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
)

func TestGenerate(t *testing.T) {
	// Create temp directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		setup     func() (*goopt.Parser, *options.AppConfig)
		wantError bool
		validate  func(t *testing.T, outputPath string)
	}{
		{
			name: "successful generation",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				// Create test locale file
				localeFile := filepath.Join(tempDir, "en.json")
				localeData := map[string]string{
					"app.name":         "Test App",
					"app.description":  "Test Description",
					"error.not_found":  "Not Found",
					"error.forbidden":  "Forbidden",
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)

				// Create config
				outputFile := filepath.Join(tempDir, "messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: false,
			validate: func(t *testing.T, outputPath string) {
				content, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}
				
				// Check package declaration
				if !strings.Contains(string(content), "package messages") {
					t.Error("output should contain package declaration")
				}
				
				// Check generated keys structure
				expectedKeys := []string{
					"App struct",         // Group
					"Name string",        // Field in App
					"Description string", // Field in App
					"Error struct",       // Group
					"NotFound string",    // Field in Error
					"Forbidden string",   // Field in Error
				}
				
				for _, key := range expectedKeys {
					if !strings.Contains(string(content), key) {
						t.Errorf("output should contain key %q", key)
					}
				}
			},
		},
		{
			name: "with prefix stripping",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				localeFile := filepath.Join(tempDir, "prefix.json")
				localeData := map[string]string{
					"myapp.feature.enable":  "Enable Feature",
					"myapp.feature.disable": "Disable Feature",
					"other.key":             "Other",
				}
				data, _ := json.Marshal(localeData)
				os.WriteFile(localeFile, data, 0644)

				outputFile := filepath.Join(tempDir, "prefix_messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
						Prefix:  "myapp",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: false,
			validate: func(t *testing.T, outputPath string) {
				content, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}
				
				// Keys with prefix should have it stripped and be grouped
				if !strings.Contains(string(content), "Feature struct") {
					t.Error("output should contain Feature struct (prefix stripped)")
				}
				if !strings.Contains(string(content), "Enable string") {
					t.Error("output should contain Enable field")
				}
				
				// Keys without prefix should remain in root
				if !strings.Contains(string(content), "Other struct") {
					t.Error("output should contain Other struct")
				}
				if !strings.Contains(string(content), "Key string") {
					t.Error("output should contain Key field")
				}
			},
		},
		{
			name: "multiple input files",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				// Create multiple locale files
				enFile := filepath.Join(tempDir, "multi_en.json")
				enData := map[string]string{"common.yes": "Yes", "common.no": "No"}
				data, _ := json.Marshal(enData)
				os.WriteFile(enFile, data, 0644)

				frFile := filepath.Join(tempDir, "multi_fr.json") 
				frData := map[string]string{"common.yes": "Oui", "common.cancel": "Annuler"}
				data, _ = json.Marshal(frData)
				os.WriteFile(frFile, data, 0644)

				outputFile := filepath.Join(tempDir, "multi_messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{enFile, frFile},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: false,
			validate: func(t *testing.T, outputPath string) {
				content, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}
				
				// Should have all unique keys from both files in Common struct
				if !strings.Contains(string(content), "Common struct") {
					t.Error("output should contain Common struct")
				}
				expectedFields := []string{"Yes string", "No string", "Cancel string"}
				for _, field := range expectedFields {
					if !strings.Contains(string(content), field) {
						t.Errorf("output should contain field %q", field)
					}
				}
			},
		},
		{
			name: "empty json file",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				emptyFile := filepath.Join(tempDir, "empty.json")
				os.WriteFile(emptyFile, []byte("{}"), 0644)

				outputFile := filepath.Join(tempDir, "empty_messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{emptyFile},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: false,
			validate: func(t *testing.T, outputPath string) {
				content, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}
				
				// Should still have valid Go code structure
				if !strings.Contains(string(content), "package messages") {
					t.Error("output should contain package declaration")
				}
				if !strings.Contains(string(content), "var Keys") {
					t.Error("output should contain Keys variable")
				}
			},
		},
		{
			name: "invalid json file",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				invalidFile := filepath.Join(tempDir, "invalid.json")
				os.WriteFile(invalidFile, []byte("{invalid json}"), 0644)

				outputFile := filepath.Join(tempDir, "invalid_messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{invalidFile},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: true,
			validate:  nil,
		},
		{
			name: "non-existent input file",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				outputFile := filepath.Join(tempDir, "nofile_messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "really_nonexistent_file_that_wont_be_created.json")},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: false, // ensureInputFile will create it
			validate: func(t *testing.T, outputPath string) {
				// Should generate empty structure
				content, _ := os.ReadFile(outputPath)
				if !strings.Contains(string(content), "var Keys = struct") {
					t.Error("should generate empty Keys structure")
				}
			},
		},
		{
			name: "wildcard pattern",
			setup: func() (*goopt.Parser, *options.AppConfig) {
				// Create multiple files matching pattern
				for _, lang := range []string{"en", "fr", "de"} {
					file := filepath.Join(tempDir, "wildcard_"+lang+".json")
					data := map[string]string{
						"greeting": "Hello " + lang,
						"common":   "Common",
					}
					jsonData, _ := json.Marshal(data)
					os.WriteFile(file, jsonData, 0644)
				}

				outputFile := filepath.Join(tempDir, "wildcard_messages.go")
				bundle, _ := i18n.NewBundle()
				cfg := &options.AppConfig{
					Input: []string{filepath.Join(tempDir, "wildcard_*.json")},
					Generate: options.GenerateCmd{
						Output:  outputFile,
						Package: "messages",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg
			},
			wantError: false,
			validate: func(t *testing.T, outputPath string) {
				content, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}
				
				// Should have deduplicated keys in Root struct
				if !strings.Contains(string(content), "Root struct") {
					t.Error("output should contain Root struct for top-level keys")
				}
				if !strings.Contains(string(content), "Greeting string") {
					t.Error("output should contain Greeting field")
				}
				if !strings.Contains(string(content), "Common string") {
					t.Error("output should contain Common field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, cfg := tt.setup()
			
			err := Generate(parser, nil)
			
			if (err != nil) != tt.wantError {
				t.Errorf("Generate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, cfg.Generate.Output)
			}
		})
	}
}

func TestGenerateKeyGrouping(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create locale file with nested keys
	localeFile := filepath.Join(tempDir, "grouped.json")
	localeData := map[string]string{
		"user.profile.name":     "Name",
		"user.profile.email":    "Email",
		"user.settings.theme":   "Theme",
		"admin.users.list":      "User List",
		"admin.users.create":    "Create User",
		"standalone":            "Standalone Key",
	}
	data, _ := json.Marshal(localeData)
	os.WriteFile(localeFile, data, 0644)

	outputFile := filepath.Join(tempDir, "grouped_messages.go")
	bundle, _ := i18n.NewBundle()
	cfg := &options.AppConfig{
		Input: []string{localeFile},
		Generate: options.GenerateCmd{
			Output:  outputFile,
			Package: "messages",
		},
		TR: bundle,
	}

	parser, _ := goopt.NewParserFromStruct(cfg)
	
	err := Generate(parser, nil)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Check for proper nesting structure
	contentStr := string(content)
	
	// Should have nested structs
	if !strings.Contains(contentStr, "UserProfile struct") {
		t.Error("should have UserProfile nested struct")
	}
	if !strings.Contains(contentStr, "UserSettings struct") {
		t.Error("should have UserSettings nested struct")
	}
	if !strings.Contains(contentStr, "AdminUsers struct") {
		t.Error("should have AdminUsers nested struct")
	}
	
	// Check standalone key
	if !strings.Contains(contentStr, "Standalone") {
		t.Error("should have Standalone key")
	}
}