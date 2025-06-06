package translations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToGoName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "snake_case",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "kebab-case",
			input:    "hello-world",
			expected: "HelloWorld",
		},
		{
			name:     "mixed separators",
			input:    "hello_world-foo",
			expected: "HelloWorldFoo",
		},
		{
			name:     "with dots",
			input:    "app.error.failed",
			expected: "App.Error.Failed",
		},
		{
			name:     "with spaces",
			input:    "hello world",
			expected: "HelloWorld",
		},
		{
			name:     "already pascal case",
			input:    "HelloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "single word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "with numbers",
			input:    "error_404_not_found",
			expected: "Error404NotFound",
		},
		{
			name:     "all caps",
			input:    "HTTP_ERROR",
			expected: "HTTPERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toGoName(tt.input)
			if result != tt.expected {
				t.Errorf("toGoName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEnsureInputFile(t *testing.T) {
	// Create temp directory for tests
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		path      string
		setup     func()
		wantError bool
	}{
		{
			name: "file already exists",
			path: filepath.Join(tempDir, "existing.json"),
			setup: func() {
				os.WriteFile(filepath.Join(tempDir, "existing.json"), []byte("{}"), 0644)
			},
			wantError: false,
		},
		{
			name:      "create new file",
			path:      filepath.Join(tempDir, "new.json"),
			setup:     func() {},
			wantError: false,
		},
		{
			name:      "create file in new directory",
			path:      filepath.Join(tempDir, "subdir", "new.json"),
			setup:     func() {},
			wantError: false,
		},
		{
			name:      "invalid path",
			path:      "",
			setup:     func() {},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			err := ensureInputFile(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("ensureInputFile(%q) error = %v, wantError %v", tt.path, err, tt.wantError)
			}

			// Check if file was created
			if !tt.wantError && tt.path != "" {
				if _, err := os.Stat(tt.path); os.IsNotExist(err) {
					t.Errorf("expected file %q to exist", tt.path)
				}

				// Check content for newly created files
				if tt.name == "create new file" || tt.name == "create file in new directory" {
					content, err := os.ReadFile(tt.path)
					if err != nil {
						t.Fatalf("failed to read created file: %v", err)
					}
					if string(content) != "{}" {
						t.Errorf("expected file content to be '{}', got %q", string(content))
					}
				}
			}
		})
	}
}

func TestExpandInputFiles(t *testing.T) {
	// Create temp directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"en.json",
		"fr.json",
		"de.json",
		"config.yaml", // Different extension
	}

	for _, file := range testFiles {
		path := filepath.Join(tempDir, file)
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}

	// Create subdirectory with more files
	subDir := filepath.Join(tempDir, "sub")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "es.json"), []byte("{}"), 0644)

	tests := []struct {
		name    string
		inputs  []string
		wantLen int
		wantErr bool
	}{
		{
			name:    "single file",
			inputs:  []string{filepath.Join(tempDir, "en.json")},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "wildcard pattern",
			inputs:  []string{filepath.Join(tempDir, "*.json")},
			wantLen: 3, // en.json, fr.json, de.json
			wantErr: false,
		},
		{
			name: "multiple patterns",
			inputs: []string{
				filepath.Join(tempDir, "en.json"),
				filepath.Join(tempDir, "fr.json"),
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "non-existent file (literal)",
			inputs:  []string{filepath.Join(tempDir, "nonexistent.json")},
			wantLen: 1, // Should return the literal path
			wantErr: false,
		},
		{
			name:    "subdirectory pattern",
			inputs:  []string{filepath.Join(tempDir, "sub", "*.json")},
			wantLen: 1, // Just es.json in subdirectory
			wantErr: false,
		},
		{
			name:    "empty input",
			inputs:  []string{},
			wantLen: 0,
			wantErr: true, // expandInputFiles returns error for no files
		},
		{
			name: "mixed wildcards and literals",
			inputs: []string{
				filepath.Join(tempDir, "*.json"),
				filepath.Join(tempDir, "config.yaml"),
			},
			wantLen: 4, // 3 json files + 1 yaml file
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandInputFiles(tt.inputs)

			if (err != nil) != tt.wantErr {
				t.Errorf("expandInputFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(result) != tt.wantLen {
				t.Errorf("expandInputFiles() returned %d files, want %d", len(result), tt.wantLen)
				t.Errorf("Files returned: %v", result)
			}
		})
	}
}
