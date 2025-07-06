package translations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
					"app.name":            "Test App",
					"app.description":     "Test Description",
					"app.error.not_found": "Not Found",
					"app.error.forbidden": "Forbidden",
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

				// Check generated keys structure using regex to ignore formatting
				expectedPatterns := []struct {
					pattern string
					desc    string
				}{
					{`type\s+App\s+struct`, "App struct declaration"},
					{`type\s+Error\s+struct`, "Error struct declaration"},
					{`Name\s+string`, "Name field in App"},
					{`Description\s+string`, "Description field in App"},
					{`NotFound\s+string`, "NotFound field in Error"},
					{`Forbidden\s+string`, "Forbidden field in Error"},
				}

				for _, exp := range expectedPatterns {
					matched, _ := regexp.MatchString(exp.pattern, string(content))
					if !matched {
						t.Errorf("output should contain %s (pattern: %s)", exp.desc, exp.pattern)
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

				// Check using regex patterns
				patterns := []struct {
					pattern string
					desc    string
				}{
					{`type\s+Feature\s+struct`, "Feature struct"},
					{`type\s+Other\s+struct`, "Other struct"},
					{`type\s+Myapp\s+struct`, "Myapp struct (from prefix)"},
					{`Enable\s+string`, "Enable field"},
					{`Disable\s+string`, "Disable field"},
					{`Key\s+string`, "Key field"},
				}

				for _, p := range patterns {
					matched, _ := regexp.MatchString(p.pattern, string(content))
					if !matched {
						t.Errorf("output should contain %s", p.desc)
					}
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
						Prefix:  "common",
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
				patterns := []struct {
					pattern string
					desc    string
				}{
					{`type\s+Common\s+struct`, "Common struct (prefix is 'common')"},
					{`Yes\s+string`, "Yes field"},
					{`No\s+string`, "No field"},
					{`Cancel\s+string`, "Cancel field"},
				}

				for _, p := range patterns {
					matched, _ := regexp.MatchString(p.pattern, string(content))
					if !matched {
						t.Errorf("output should contain %s", p.desc)
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
				if !strings.Contains(string(content), "var Keys struct") {
					t.Error("should generate Keys structure")
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
						Prefix:  "root",
					},
					TR: bundle,
				}

				parser, _ := goopt.NewParserFromStruct(cfg)
				// Parse with generate command and prefix argument
				parser.Parse([]string{"generate", "-i", filepath.Join(tempDir, "wildcard_*.json"), "-o", outputFile, "--prefix", "root"})
				return parser, cfg
			},
			wantError: false,
			validate: func(t *testing.T, outputPath string) {
				content, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}

				// Debug: print first 500 chars of content
				if testing.Verbose() {
					t.Logf("Generated content preview: %.500s", string(content))
				}

				// Should have deduplicated keys in Root struct (since prefix is "root")
				patterns := []struct {
					pattern string
					desc    string
				}{
					{`type\s+Root\s+struct`, "Root struct (prefix is 'root')"},
					{`Greeting\s+string`, "Greeting field"},
					{`Common\s+string`, "Common field"},
				}

				for _, p := range patterns {
					matched, _ := regexp.MatchString(p.pattern, string(content))
					if !matched {
						t.Errorf("output should contain %s", p.desc)
					}
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
		"user.profile.name":   "Name",
		"user.profile.email":  "Email",
		"user.settings.theme": "Theme",
		"admin.users.list":    "User List",
		"admin.users.create":  "Create User",
		"standalone":          "Standalone Key",
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
	if !strings.Contains(contentStr, "Profile struct") {
		t.Error("should have Profile nested struct")
	}
	if !strings.Contains(contentStr, "Settings struct") {
		t.Error("should have Settings nested struct")
	}
	if !strings.Contains(contentStr, "Users struct") {
		t.Error("should have Users nested struct")
	}
	if !strings.Contains(contentStr, "User struct") {
		t.Error("should have User struct")
	}
	if !strings.Contains(contentStr, "Admin struct") {
		t.Error("should have Admin struct")
	}

	// Should have App struct with default prefix
	if !strings.Contains(contentStr, "App struct") {
		t.Error("should have App struct")
	}

	// Check standalone key
	if !strings.Contains(contentStr, "Standalone") {
		t.Error("should have Standalone key")
	}
}
