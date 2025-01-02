package migration

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"go/parser"
	"go/printer"
	"go/token"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertLegacyTags(t *testing.T) {
	type TestStruct struct {
		Basic     string `long:"output" short:"o" description:"Output file"`
		TypeReq   string `long:"format" type:"single" required:"true"`
		SecPrompt string `long:"password" secure:"true" prompt:"Enter password:"`
		AccDep    string `long:"output" accepted:"{pattern:json|yaml,desc:Format type}" depends:"{flag:format,values:[json,yaml]}"`
		Default   string `long:"greeting" default:"Hello World!"`
		Path      string `long:"config" path:"config/path"`
		Empty     string
		NoLegacy  string `json:"test"`
		Invalid   string `invalid:":tag::format"`
		Complex   string `long:"complex" description:"Complex value" accepted:"{pattern:a|b|c,desc:Letters}" depends:"{flag:format,values:[json,yaml]},{flag:type,values:[single,multi]}"`
	}

	tests := []struct {
		name      string
		fieldName string
		expected  string
		wantErr   bool
		wantNil   bool
	}{
		{
			name:      "basic conversion",
			fieldName: "Basic",
			expected:  `goopt:"name:output;short:o;desc:Output file;type:single"`,
		},
		{
			name:      "with type and required",
			fieldName: "TypeReq",
			expected:  `goopt:"name:format;type:single;required:true"`,
		},
		{
			name:      "with secure and prompt",
			fieldName: "SecPrompt",
			expected:  `goopt:"name:password;type:single;secure:true;prompt:Enter password:"`,
		},
		{
			name:      "with pattern values and depends",
			fieldName: "AccDep",
			expected:  `goopt:"name:output;type:single;accepted:{pattern:json|yaml,desc:Format type};depends:{flag:format,values:[json,yaml]}"`,
		},
		{
			name:      "with default value",
			fieldName: "Default",
			expected:  `goopt:"name:greeting;type:single;default:Hello World!"`,
		},
		{
			name:      "with path",
			fieldName: "Path",
			expected:  `goopt:"name:config;type:single;path:config/path"`,
		},
		{
			name:      "no legacy tags",
			fieldName: "NoLegacy",
			wantNil:   true,
		},
		{
			name:      "empty field",
			fieldName: "Empty",
			wantNil:   true,
		},
		{
			name:      "complex with multiple patterns and depends",
			fieldName: "Complex",
			expected:  `goopt:"name:complex;desc:Complex value;type:single;accepted:{pattern:a|b|c,desc:Letters};depends:{flag:format,values:[json,yaml]},{flag:type,values:[single,multi]}"`,
		},
	}

	structType := reflect.TypeOf(TestStruct{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := structType.FieldByName(tt.fieldName)
			assert.True(t, ok, "Field %s not found", tt.fieldName)

			result, err := convertLegacyTags(field)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			if tt.wantNil {
				assert.Empty(t, result)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateTagValue(t *testing.T) {
	tests := []struct {
		name        string
		originalTag string
		newGooptTag string
		want        string
	}{
		{
			name:        "single legacy tag",
			originalTag: "`long:\"output\"`",
			newGooptTag: `goopt:"name:output"`,
			want:        "`goopt:\"name:output\"`",
		},
		{
			name:        "multiple legacy tags",
			originalTag: "`long:\"output\" short:\"o\" description:\"Output file\"`",
			newGooptTag: `goopt:"name:output;short:o;desc:Output file"`,
			want:        "`goopt:\"name:output;short:o;desc:Output file\"`",
		},
		{
			name:        "preserve non-goopt tags",
			originalTag: "`long:\"output\" json:\"output,omitempty\"`",
			newGooptTag: `goopt:"name:output"`,
			want:        "`json:\"output,omitempty\" goopt:\"name:output\"`",
		},
		{
			name:        "mixed legacy and existing goopt",
			originalTag: "`long:\"output\" goopt:\"name:old\" json:\"output\"`",
			newGooptTag: `goopt:"name:output"`,
			want:        "`json:\"output\" goopt:\"name:output\"`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updateTagValue(tt.originalTag, tt.newGooptTag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func readFile(t *testing.T, path string) string {
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func formatGoCode(t *testing.T, code string) string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", code, parser.ParseComments)
	require.NoError(t, err, "parsing code")

	var buf bytes.Buffer
	err = printer.Fprint(&buf, fset, node)
	require.NoError(t, err, "formatting code")

	return buf.String()
}

// Integration test with real files
func TestConvertFile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name: "simple struct",
			input: `package test
type Config struct {
    Output string ` + "`long:\"output\" short:\"o\"`" + `
    Format string ` + "`long:\"format\" description:\"Output format\"`" + `
}`,
			expected: `package test
type Config struct {
    Output string ` + "`goopt:\"name:output;short:o\"`" + `
    Format string ` + "`goopt:\"name:format;desc:Output format\"`" + `
}`,
		},
		{
			name: "preserve comments and formatting",
			input: `package test

// Config holds application settings
type Config struct {
    // Output file path
    Output string ` + "`long:\"output\" json:\"output\"`" + `
}`,
			expected: `package test

// Config holds application settings
type Config struct {
    // Output file path
    Output string ` + "`json:\"output\" goopt:\"name:output\"`" + `
}`,
		},
		{
			name: "complex tags",
			input: `package test
type Config struct {
    Format string ` + "`long:\"format\" accepted:\"{pattern:json|yaml,desc:Format type}\" depends:\"{flag:output}\"`" + `
}`,
			expected: `package test
type Config struct {
    Format string ` + "`goopt:\"name:format;accepted:{pattern:json|yaml,desc:Format type};depends:{flag:output}\"`" + `
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.go")

			err := os.WriteFile(tmpFile, []byte(tt.input), 0644)
			require.NoError(t, err)

			// Convert the file using the same directory as base
			err = ConvertSingleFile(tmpFile, tmpDir)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Read and verify the result
			got := readFile(t, tmpFile)
			assert.Equal(t, formatGoCode(t, tt.expected), got)

			// Verify backup was created in local .goopt-migration directory
			migrationDir := filepath.Join(tmpDir, migrationDirName)
			assert.DirExists(t, migrationDir, "migration directory should exist")
		})
	}
}

func TestConvertDir(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected map[string]string
		wantErr  bool
	}{
		{
			name: "multiple files",
			files: map[string]string{
				"config.go": `package test
type Config struct {
    Output string ` + "`long:\"output\" short:\"o\"`" + `
}`,
				"other.go": `package test
type Other struct {
    Format string ` + "`long:\"format\" description:\"Output format\"`" + `
}`,
			},
			expected: map[string]string{
				"config.go": `package test
type Config struct {
    Output string ` + "`goopt:\"name:output;short:o\"`" + `
}`,
				"other.go": `package test
type Other struct {
    Format string ` + "`goopt:\"name:format;desc:Output format\"`" + `
}`,
			},
		},
		{
			name: "single file",
			files: map[string]string{
				"config.go": `package test
type Config struct {
    Output string ` + "`long:\"output\" short:\"o\"`" + `
}`,
			},
			expected: map[string]string{
				"config.go": `package test
type Config struct {
    Output string ` + "`goopt:\"name:output;short:o\"`" + `
}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create test files
			for name, content := range tt.files {
				path := filepath.Join(tmpDir, name)
				err := os.WriteFile(path, []byte(content), 0644)
				require.NoError(t, err)
			}

			// Convert the directory using itself as base
			err := ConvertDir(tmpDir, tmpDir)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify each file
			for name, expectedContent := range tt.expected {
				path := filepath.Join(tmpDir, name)
				got := readFile(t, path)
				assert.Equal(t, formatGoCode(t, expectedContent), got)
			}

			// Verify migration directory exists
			migrationDir := filepath.Join(tmpDir, migrationDirName)
			assert.DirExists(t, migrationDir, "migration directory should exist")
		})
	}
}

func TestConvertSingleFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")

	input := `package test
type Config struct {
    Output string ` + "`long:\"output\" short:\"o\"`" + `
}`

	expected := `package test
type Config struct {
    Output string ` + "`goopt:\"name:output;short:o\"`" + `
}`

	err := os.WriteFile(tmpFile, []byte(input), 0644)
	require.NoError(t, err)

	// Convert the file
	err = ConvertSingleFile(tmpFile, tmpDir)
	require.NoError(t, err)

	// Verify the result
	got := readFile(t, tmpFile)
	assert.Equal(t, formatGoCode(t, expected), got)

	// Verify migration directory was cleaned up
	migrationDir, err := ensureMigrationDir(tmpDir)
	require.NoError(t, err)
	entries, err := os.ReadDir(migrationDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "migration directory should be empty after successful conversion")
}

func TestEnsureMigrationDir(t *testing.T) {
	t.Run("empty base dir", func(t *testing.T) {
		_, err := ensureMigrationDir("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base directory must be specified")
	})

	t.Run("creates migration dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		migrationDir, err := ensureMigrationDir(tmpDir)
		require.NoError(t, err)

		info, err := os.Stat(migrationDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		// Check permissions (on Unix systems)
		if runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
		}
	})
}
