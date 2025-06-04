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
	"golang.org/x/text/language"
)

// createTestBundle creates a bundle with minimal required translations for extract tests
func createTestBundle() i18n.Translator {
	bundle := i18n.NewEmptyBundle()

	// Add minimal extract messages
	extractMessages := map[string]string{
		"app.extract.scanning_files":          "Scanning files...",
		"app.extract.found_strings":           "Found %d occurrences in %d files",
		"app.extract.unique_strings":          "%d unique strings",
		"app.extract.no_strings_found":        "No strings found",
		"app.extract.dry_run_mode":            "DRY RUN MODE - No files will be modified",
		"app.extract.updating_files":          "Updating locale files...",
		"app.extract.added":                   "Added",
		"app.extract.keys":                    "keys",
		"app.extract.key":                     "Key",
		"app.extract.occurrences":             "occurrences",
		"app.add.updated":                     "Updated",
		"app.extract.update_error":            "Failed to update %s: %s",
		"app.extract.auto_update_mode":        "Auto-update mode",
		"app.extract.no_replacements":         "No replacements needed",
		"app.extract.found_comments":          "Found %d i18n-todo comments",
		"app.extract.applying_replacements":   "Applying replacements...",
		"app.extract.update_complete":         "Update complete",
		"app.extract.backup_location":         "Backups saved to: %s",
		"app.extract.cleaning_comments":       "Cleaning i18n comments...",
		"app.extract.no_comments_found":       "No i18n comments found",
		"app.extract.found_comments_to_clean": "Found %d i18n comments to clean",
		"app.extract.clean_complete":          "Comment cleanup complete",
		"app.extract.glob_error":              "Failed to expand pattern %s: %s",
		"app.extract.file_error":              "Error processing %s: %s",
		"app.extract.invalid_regex":           "Invalid regex: %s",
		"app.ast.todo_prefix":                 "[TODO] %s",
	}

	bundle.AddLanguage(language.English, extractMessages)
	return bundle
}

func setupExtractTest(t *testing.T, bundle i18n.Translator, localeFile, goFile string, opts map[string]string) (*goopt.Parser, *options.AppConfig) {
	cfg := &options.AppConfig{
		TR: bundle,
	}
	cfg.Extract.Exec = Extract

	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithFlagNameConverter(goopt.ToKebabCase),
		goopt.WithCommandNameConverter(goopt.ToKebabCase))
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Build command line from options
	cmdLine := fmt.Sprintf("extract -i %s", localeFile)

	// Add source files
	if goFile != "" {
		cmdLine += fmt.Sprintf(" -s %s", goFile)
	}

	// Add optional parameters
	for flag, value := range opts {
		switch flag {
		case "keyPrefix":
			cmdLine += fmt.Sprintf(" -P %s", value)
		case "minLength":
			cmdLine += fmt.Sprintf(" -L %s", value)
		case "matchOnly":
			cmdLine += fmt.Sprintf(" -M %s", value)
		case "skipMatch":
			cmdLine += fmt.Sprintf(" -S %s", value)
		case "dryRun":
			if value == "true" {
				cmdLine += " -n"
			}
		case "autoUpdate":
			if value == "true" {
				cmdLine += " -u"
			}
		case "trPattern":
			cmdLine += fmt.Sprintf(" --tr-pattern %s", value)
		case "package":
			cmdLine += fmt.Sprintf(" -p %s", value)
		case "cleanComments":
			if value == "true" {
				cmdLine += " --clean-comments"
			}
		case "keepComments":
			if value == "true" {
				cmdLine += " --keep-comments"
			}
		case "transformMode":
			cmdLine += fmt.Sprintf(" --transform-mode %s", value)
			// Add more flags as needed
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

func TestExtract(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() (*goopt.Parser, *options.AppConfig, string)
		wantError bool
		validate  func(t *testing.T, tempDir string, cfg *options.AppConfig)
	}{
		{
			name: "basic string extraction",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	println("Hello, world!")
	println("Welcome to the application")
	
	name := "John"
	fmt.Printf("Hello, %s", name)
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()

				cfg := &options.AppConfig{
					TR: bundle,
				}
				cfg.Extract.Exec = Extract

				parser, _ := goopt.NewParserFromStruct(cfg)

				// Use ParseString to simulate command line args
				cmdLine := fmt.Sprintf("extract -i %s -s %s -P test -l 2",
					localeFile, goFile)
				if !parser.ParseString(cmdLine) {
					// Print errors for debugging
					for _, err := range parser.GetErrors() {
						fmt.Printf("Parser error: %v\n", err)
					}
					panic("Failed to parse command line: " + cmdLine)
				}

				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check locale file was updated
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should have extracted strings
				if len(data) == 0 {
					t.Error("no strings were extracted")
				}

				// Check specific strings
				hasHello := false
				hasWelcome := false
				for _, v := range data {
					if v == "Hello, world!" {
						hasHello = true
					}
					if v == "Welcome to the application" {
						hasWelcome = true
					}
				}

				if !hasHello {
					t.Error("'Hello, world!' should have been extracted")
				}
				if !hasWelcome {
					t.Error("'Welcome to the application' should have been extracted")
				}
			},
		},
		{
			name: "dry run mode",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	println("Test string")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				originalContent := []byte(`{"existing": "value"}`)
				os.WriteFile(localeFile, originalContent, 0644)

				bundle := createTestBundle()

				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"dryRun":    "true",
					"backupDir": filepath.Join(tempDir, ".backup"),
				})

				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Locale file should not be modified
				content, _ := os.ReadFile(cfg.Input[0])
				if string(content) != `{"existing": "value"}` {
					t.Error("locale file should not be modified in dry run mode")
				}
			},
		},
		{
			name: "minimum length filter",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	println("OK")
	println("This is a longer string")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()

				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"minLength": "10",
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should only have the longer string
				if len(data) != 1 {
					t.Errorf("expected 1 extracted string, got %d", len(data))
				}

				hasLong := false
				for _, v := range data {
					if v == "This is a longer string" {
						hasLong = true
					}
					if v == "OK" {
						t.Error("short string 'OK' should have been filtered out")
					}
				}

				if !hasLong {
					t.Error("longer string should have been extracted")
				}
			},
		},
		{
			name: "regex match filter",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	log.Debug("DEBUG: This is debug")
	fmt.Println("User logged in")
	log.Error("ERROR: Something failed")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"matchOnly": `^[^:]+$`,
				})

				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should only have "User logged in"
				if len(data) != 1 {
					t.Errorf("expected 1 extracted string, got %d", len(data))
				}

				hasUser := false
				for _, v := range data {
					if v == "User logged in" {
						hasUser = true
					}
					if strings.Contains(v, "DEBUG") || strings.Contains(v, "ERROR") {
						t.Error("debug/error strings should have been filtered out")
					}
				}

				if !hasUser {
					t.Error("'User logged in' should have been extracted")
				}
			},
		},
		{
			name: "skip match filter",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	fmt.Println("User message")
	log.Debug("DEBUG: Skip this")
	fmt.Println("Another user message")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"skipMatch": `^(DEBUG|INFO|WARN|ERROR):`, // Skip log prefixes
					"BackupDir": filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should have user messages but not debug
				hasUser := false
				hasAnother := false
				for _, v := range data {
					if v == "User message" {
						hasUser = true
					}
					if v == "Another user message" {
						hasAnother = true
					}
					if strings.Contains(v, "DEBUG") {
						t.Error("debug string should have been skipped")
					}
				}

				if !hasUser || !hasAnother {
					t.Error("user messages should have been extracted")
				}
			},
		},
		{
			name: "auto update with default tr pattern",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate": "true",
					"keyPrefix":  "test",
					"backupDir":  filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// With the new default, -u should do direct transformation
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Updated Go file content:\n%s", string(content))

				if !strings.Contains(string(content), "tr.T(") {
					t.Error("Go file should have strings replaced with tr.T calls")
				}

				// Check backup was created
				backupFiles, _ := os.ReadDir(cfg.Extract.BackupDir)
				if len(backupFiles) == 0 {
					t.Error("backup should have been created")
				}
			},
		},
		{
			name: "auto update with direct replacement",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Replace this string")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate": "true",
					"keyPrefix":  "app",
					"trPattern":  "tr.T",
					"package":    "messages",
					"backupDir":  filepath.Join(tempDir, ".backup"),
				})

				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check that Go file was updated
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Updated Go file content:\n%s", string(content))

				// Also check if a backup was created to confirm processing happened
				backupFiles, _ := os.ReadDir(filepath.Join(tempDir, ".backup"))
				t.Logf("Backup files created: %d", len(backupFiles))

				// Log the extract configuration
				t.Logf("AutoUpdate: %v, TrPattern: %s", cfg.Extract.AutoUpdate, cfg.Extract.TrPattern)

				if !strings.Contains(string(content), "tr.T(") {
					t.Error("Go file should have strings replaced with tr.T calls")
				}

				if !strings.Contains(string(content), "messages.Keys") {
					t.Error("Go file should reference messages.Keys")
				}
			},
		},
		{
			name: "clean comments mode",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	fmt.Println("Hello") // i18n-todo: tr.T(messages.Keys.Hello)
	fmt.Println("World") // i18n-skip
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"cleanComments": "true",
					"backupDir":     filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check that comments were removed
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))

				if strings.Contains(string(content), "i18n-todo") {
					t.Error("i18n-todo comments should have been removed")
				}

				if strings.Contains(string(content), "i18n-skip") {
					t.Error("i18n-skip comments should have been removed")
				}
			},
		},
		{
			name: "i18n-skip comment",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

func main() {
	fmt.Println("Extract this")
	fmt.Println("Skip this") // i18n-skip
	fmt.Println("Also extract this")
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"backupDir": filepath.Join(tempDir, ".backup"),
				})

				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should not have "Skip this"
				for _, v := range data {
					if v == "Skip this" {
						t.Error("string with i18n-skip comment should not be extracted")
					}
				}

				// Should have the other strings
				hasExtract := false
				hasAlso := false
				for _, v := range data {
					if v == "Extract this" {
						hasExtract = true
					}
					if v == "Also extract this" {
						hasAlso = true
					}
				}

				if !hasExtract || !hasAlso {
					t.Error("other strings should have been extracted")
				}
			},
		},
		{
			name: "keep comments with auto update",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Hello") // i18n-todo: tr.T(messages.Keys.Hello)
	msg := "World" // i18n-todo: tr.T(messages.Keys.World)
	fmt.Println(msg)
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate":   "true",
					"keepComments": "true",
					"keyPrefix":    "test",
					"backupDir":    filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {

				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Go file content with keep-comments:\n%s", string(content))

				// Should have tr.T replacements
				if !strings.Contains(string(content), "tr.T(") {
					t.Error("Go file should have strings replaced with tr.T calls")
				}

				if !strings.Contains(string(content), "i18n-todo") {
					t.Error("i18n-todo comments should be kept with --keep-comments flag")
				}
			},
		},
		{
			name: "remove comments with auto update",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Hello") // i18n-todo: tr.T(messages.Keys.Hello)
	msg := "World" // i18n-todo: tr.T(messages.Keys.World)
	fmt.Println(msg)
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate": "true",
					// Note: NOT setting keepComments, so it defaults to false
					"keyPrefix": "test",
					"backupDir": filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {

				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Go file content without keep-comments:\n%s", string(content))

				// Should have tr.T replacements
				if !strings.Contains(string(content), "tr.T(") {
					t.Error("Go file should have strings replaced with tr.T calls")
				}

				if strings.Contains(string(content), "i18n-todo") {
					t.Error("i18n-todo comments should be removed without --keep-comments flag")
				}
			},
		},
		{
			name: "format function handling",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	name := "Alice"
	fmt.Printf("Hello, %s!", name)
	fmt.Sprintf("Welcome, %s", name)
	fmt.Errorf("user %s not found", name)
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate": "true",
					"trPattern":  "tr.T",
					"package":    "messages",
					"backupDir":  filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				// Check locale file has format strings
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				hasHello := false
				hasWelcome := false
				hasError := false
				for _, v := range data {
					if v == "Hello, %s!" {
						hasHello = true
					}
					if v == "Welcome, %s" {
						hasWelcome = true
					}
					if v == "user %s not found" {
						hasError = true
					}
				}

				if !hasHello || !hasWelcome || !hasError {
					t.Error("format strings should have been extracted")
				}

				// Check Go file transformation
				goContent, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Updated Go file content:\n%s", string(goContent))

				// Should have Printf transformed (note: Printf becomes Print after transformation)
				if !strings.Contains(string(goContent), "fmt.Print(tr.T(") {
					t.Error("Printf should be transformed to Print with tr.T")
				}
			},
		},
		{
			name: "wildcard file patterns",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				// Create multiple Go files
				goFile1 := filepath.Join(tempDir, "file1.go")
				goContent1 := `package test
func f1() { println("String from file1") }
`
				os.WriteFile(goFile1, []byte(goContent1), 0644)

				goFile2 := filepath.Join(tempDir, "file2.go")
				goContent2 := `package test
func f2() { println("String from file2") }
`
				os.WriteFile(goFile2, []byte(goContent2), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, filepath.Join(tempDir, "*.go"), map[string]string{
					"backupDir": filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(cfg.Input[0])
				var data map[string]string
				json.Unmarshal(content, &data)

				// Should have strings from both files
				hasFile1 := false
				hasFile2 := false
				for _, v := range data {
					if v == "String from file1" {
						hasFile1 = true
					}
					if v == "String from file2" {
						hasFile2 = true
					}
				}

				if !hasFile1 || !hasFile2 {
					t.Error("strings from all files should have been extracted")
				}
			},
		},
		{
			name: "honor i18n-todo comments with transform-mode all-marked",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Hello") // i18n-todo: tr.T(messages.Keys.Hello)
	msg := "World" // i18n-todo: tr.T(messages.Keys.World)
	fmt.Println(msg)
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate":    "true",
					"transformMode": "all-marked",
					"keyPrefix":     "test",
					"backupDir":     filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Go file content with transform-mode=all-marked:\n%s", string(content))

				// Both strings should be transformed when userFacingOnly is false
				if !strings.Contains(string(content), `fmt.Println(tr.T(messages.Keys.Test.Hello))`) {
					t.Error("'Hello' should be transformed")
				}

				// Check that msg is assigned a tr.T call (may be formatted across lines)
				// The string "World" should be transformed to use tr.T
				if !strings.Contains(string(content), `msg := tr.T(`) || strings.Contains(string(content), `msg := "World"`) {
					t.Error("'World' should be transformed with all-marked mode")
				}

				// i18n-todo comments should be removed
				if strings.Contains(string(content), "i18n-todo") {
					t.Error("i18n-todo comments should be removed after transformation")
				}
			},
		},
		{
			name: "transform-mode with-comments only transforms i18n-todo strings",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Hello") // i18n-todo: tr.T(messages.Keys.Hello)
	msg := "World" // i18n-todo: tr.T(messages.Keys.World)
	fmt.Println(msg)
	fmt.Println("NoComment") // This should not be transformed
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate":    "true",
					"transformMode": "with-comments",
					"keyPrefix":     "test",
					"backupDir":     filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Go file content with transform-mode=with-comments:\n%s", string(content))

				// Only strings with i18n-todo comments should be transformed
				if !strings.Contains(string(content), `fmt.Println(tr.T(`) {
					t.Error("'Hello' with i18n-todo should be transformed")
				}

				if !strings.Contains(string(content), `msg := tr.T(`) {
					t.Error("'World' with i18n-todo should be transformed")
				}

				// String without comment should NOT be transformed
				if !strings.Contains(string(content), `fmt.Println("NoComment")`) {
					t.Error("'NoComment' without i18n-todo should NOT be transformed")
				}
			},
		},
		{
			name: "transform-mode all transforms all strings with keys",
			setup: func() (*goopt.Parser, *options.AppConfig, string) {
				tempDir := t.TempDir()

				goFile := filepath.Join(tempDir, "test.go")
				goContent := `package test

import "fmt"

func main() {
	fmt.Println("Hello")
	msg := "World"
	fmt.Println(msg)
}
`
				os.WriteFile(goFile, []byte(goContent), 0644)

				localeFile := filepath.Join(tempDir, "en.json")
				os.WriteFile(localeFile, []byte("{}"), 0644)

				bundle := createTestBundle()
				parser, cfg := setupExtractTest(t, bundle, localeFile, goFile, map[string]string{
					"autoUpdate":    "true",
					"transformMode": "all",
					"keyPrefix":     "test",
					"backupDir":     filepath.Join(tempDir, ".backup"),
				})
				return parser, cfg, tempDir
			},
			wantError: false,
			validate: func(t *testing.T, tempDir string, cfg *options.AppConfig) {
				content, _ := os.ReadFile(filepath.Join(tempDir, "test.go"))
				t.Logf("Go file content with transform-mode=all:\n%s", string(content))

				// All strings should be transformed
				if !strings.Contains(string(content), `fmt.Println(tr.T(messages.Keys.Test.Hello))`) {
					t.Error("'Hello' should be transformed in all mode")
				}

				if !strings.Contains(string(content), `msg := tr.T(`) {
					t.Error("'World' should be transformed in all mode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, cfg, tempDir := tt.setup()

			err := Extract(parser, nil)

			if (err != nil) != tt.wantError {
				t.Errorf("Extract() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, tempDir, cfg)
			}
		})
	}
}
