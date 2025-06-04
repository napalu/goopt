package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/i18n"
)

// setupAuditTest creates a parser and config for audit tests using command line parsing
func setupAuditTest(t *testing.T, bundle i18n.Translator, localeFile string, opts map[string]string) (*goopt.Parser, *options.AppConfig) {
	cfg := &options.AppConfig{
		TR: bundle,
	}
	cfg.Audit.Exec = Audit

	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithFlagNameConverter(goopt.ToKebabCase),
		goopt.WithCommandNameConverter(goopt.ToKebabCase))
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Build command line from options
	cmdLine := fmt.Sprintf("audit -i %s", localeFile)

	// Add optional parameters
	for flag, value := range opts {
		switch flag {
		case "files":
			// Don't use default, specify files explicitly
			cmdLine += fmt.Sprintf(" --files %s", value)
		case "generateDescKeys":
			if value == "true" {
				cmdLine += " -d"
			}
		case "generateMissing":
			if value == "true" {
				cmdLine += " -g"
			}
		case "keyPrefix":
			cmdLine += fmt.Sprintf(" --key-prefix %s", value)
		case "autoUpdate":
			if value == "true" {
				cmdLine += " -u"
			}
		case "backupDir":
			cmdLine += fmt.Sprintf(" --backup-dir %s", value)
		}
	}

	if !parser.ParseString(cmdLine) {
		// Print errors for debugging
		for _, err := range parser.GetErrors() {
			t.Logf("Parser error: %v", err)
		}
		t.Fatalf("Failed to parse command line: %s", cmdLine)
	}

	return parser, cfg
}

func TestAudit(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() (*goopt.Parser, *options.AppConfig, string)
		wantError bool
		validate  func(t *testing.T, tempDir string, cfg *options.AppConfig)
	}{
		{
			name: "all fields have descKey",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name;descKey:app.name"` + "`" + `
	Port int    ` + "`" + `goopt:"desc:Server port;descKey:app.port"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Audit: options.AuditCmd{
						Files: []string{goFile},
					},
					TR: bundle,
				}
				cfg.Audit.Exec = Audit

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should report all fields have descKey
			},
		},
		{
			name: "fields without descKey",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name;descKey:app.name"` + "`" + `
	Port int    ` + "`" + `goopt:"desc:Server port"` + "`" + `  // Missing descKey
	Host string ` + "`" + `goopt:"desc:Server host"` + "`" + `  // Missing descKey
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Audit: options.AuditCmd{
						Files: []string{goFile},
					},
					TR: bundle,
				}
				cfg.Audit.Exec = Audit

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should report fields without descKey
			},
		},
		{
			name: "generate descKeys",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type AppConfig struct {
	ServerPort int    ` + "`" + `goopt:"desc:Server port"` + "`" + `
	DebugMode  bool   ` + "`" + `goopt:"desc:Enable debug mode"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Audit: options.AuditCmd{
						Files:            []string{goFile},
						GenerateDescKeys: true,
						KeyPrefix:        "test",
					},
					TR: bundle,
				}
				cfg.Audit.Exec = Audit

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should suggest generated keys with prefix
			},
		},
		{
			name: "auto update source files",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name"` + "`" + `
	Port int    ` + "`" + `goopt:"desc:Server port"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				opts := map[string]string{
					"files":            goFile,
					"generateDescKeys": "true",
					"autoUpdate":       "true",
					"keyPrefix":        "app",
					"backupDir":        filepath.Join(tempDir, ".backup"),
				}

				parser, cfg := setupAuditTest(t, bundle, localeFile, opts)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check that source file was updated
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))

				// Should have added descKey tags
				if !strings.Contains(string(content), "descKey:") {
					t.Error("source file should have been updated with descKey tags")
				}

				// Check backup was created
				backupFiles, _ := os.ReadDir(cfg.Audit.BackupDir)
				if len(backupFiles) == 0 {
					t.Error("backup file should have been created")
				}
			},
		},
		{
			name: "generate missing translations",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

type Config struct {
	Name string ` + "`" + `goopt:"desc:App name"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				opts := map[string]string{
					"files":            goFile,
					"generateDescKeys": "true",
					"generateMissing":  "true",
					"keyPrefix":        "app",
				}

				parser, cfg := setupAuditTest(t, bundle, localeFile, opts)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check that translations were added to locale file
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should have generated translation
				if len(data) == 0 {
					t.Error("translations should have been generated")
				}
			},
		},
		{
			name: "wildcard file patterns",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				// Create multiple Go files
				for i := 1; i <= 3; i++ {
					goFile := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
					goContent := fmt.Sprintf(`package test

type Config%d struct {
	Field%d string `+"`"+`goopt:"desc:Field %d"`+"`"+`
}
`, i, i, i)
					os.WriteFile(goFile, []byte(goContent), 0644)
				}

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Audit: options.AuditCmd{
						Files: []string{filepath.Join(tempDir, "*.go")},
					},
					TR: bundle,
				}
				cfg.Audit.Exec = Audit

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should process all matching files
			},
		},
		{
			name: "nested structs",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "nested.go")
				goContent := `package test

type Config struct {
	Server struct {
		Host string ` + "`" + `goopt:"desc:Server host"` + "`" + `
		Port int    ` + "`" + `goopt:"desc:Server port;descKey:server.port"` + "`" + `
	}
	Database struct {
		URL string ` + "`" + `goopt:"desc:Database URL"` + "`" + `
	}
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Audit: options.AuditCmd{
						Files: []string{goFile},
					},
					TR: bundle,
				}
				cfg.Audit.Exec = Audit

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should handle nested struct fields
			},
		},
		{
			name: "command structs",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "commands.go")
				goContent := `package test

type App struct {
	Init InitCmd ` + "`" + `goopt:"kind:command;name:init;desc:Initialize"` + "`" + `
}

type InitCmd struct {
	Force bool ` + "`" + `goopt:"desc:Force initialization"` + "`" + `
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := i18n.NewEmptyBundle()

				cfg := &options.AppConfig{
					Input: []string{localeFile},
					Audit: options.AuditCmd{
						Files: []string{goFile},
					},
					TR: bundle,
				}
				cfg.Audit.Exec = Audit

				parser, _ := goopt.NewParserFromStruct(cfg)
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Should find fields in command structs
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, cfg, tempDir := tt.setup()

			err := Audit(parser, nil)

			if (err != nil) != tt.wantError {
				t.Errorf("Audit() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, tempDir, cfg)
			}
		})
	}
}
