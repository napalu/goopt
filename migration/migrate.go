package migration

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/napalu/goopt/parse"
	"github.com/napalu/goopt/types"
)

// ConvertSingleFile converts a single file .go file to the new struct tag format
func ConvertSingleFile(filename string, baseDir string) error {
	sessionDir, err := createSessionDir(baseDir)
	if err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}

	if err := convertFile(filename, sessionDir); err != nil {
		return fmt.Errorf("conversion failed, backups in %s: %w", sessionDir, err)
	}

	// Success - clean up session
	if err := os.RemoveAll(sessionDir); err != nil {
		return &cleanupErr{op: "remove-session", err: err}
	}

	return nil
}

// ConvertDir converts all .go files in a directory (non-recursive) to the new struct tag format
func ConvertDir(dir string, baseDir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		goFiles = append(goFiles, filepath.Join(dir, entry.Name()))
	}

	if len(goFiles) == 0 {
		return nil
	}

	sessionDir, err := createSessionDir(baseDir)
	if err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}

	var conversionErrors []error
	for _, filename := range goFiles {
		fmt.Printf("Processing file: %s\n", filename)
		if err := convertFile(filename, sessionDir); err != nil {
			conversionErrors = append(conversionErrors,
				fmt.Errorf("converting %s: %w", filename, err))
		}
	}

	if len(conversionErrors) > 0 {
		return fmt.Errorf("failed to convert some files, backups in %s: %v",
			sessionDir, conversionErrors)
	}

	// Success - clean up session
	if err := os.RemoveAll(sessionDir); err != nil {
		return &cleanupErr{op: "remove-session", err: err}
	}

	return nil
}

// PreviewChanges shows what would be changed without making modifications
func PreviewChanges(filename string) (string, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing file: %w", err)
	}

	var preview strings.Builder

	var currentStruct string
	err = processFile(file, func(field *ast.Field, oldTag, newTag string, structName string) error {
		// Update current struct section if needed
		if structName != currentStruct {
			preview.WriteString(fmt.Sprintf("Struct: %s\n", structName))
			currentStruct = structName
		}

		preview.WriteString(fmt.Sprintf("  Field: %s\n", field.Names[0]))
		preview.WriteString(fmt.Sprintf("    Old: %s\n", oldTag))
		preview.WriteString(fmt.Sprintf("    New: `%s`\n", updateTagValue(oldTag, newTag)))

		return nil
	})

	return preview.String(), err
}

// convertFile converts a single file, using the provided session directory for backups
func convertFile(filename string, sessionDir string) error {
	// First check if we can access/write the file
	if err := checkFileAccess(filename); err != nil {
		return fmt.Errorf("checking file access: %w", err)
	}

	// Create backup in the session directory
	backupName := filepath.Base(filename) + ".bak"
	backupPath := filepath.Join(sessionDir, backupName)

	if err := copyFile(filename, backupPath); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	// Process the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	modified := false
	if err := processFile(file, func(field *ast.Field, oldTag, newTag string, _ string) error {
		field.Tag.Value = updateTagValue(oldTag, newTag)
		modified = true
		return nil
	}); err != nil {
		return fmt.Errorf("processing file: %w", err)
	}

	if !modified {
		return nil
	}

	// Create temporary file for atomic write
	tempFile, err := os.CreateTemp(filepath.Dir(filename), ".goopt-convert-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tempName := tempFile.Name()
	defer func() {
		tempFile.Close()
		if err := cleanup(tempName); err != nil {
			log.Printf("warning: failed to cleanup temporary file: %v", err)
		}
	}()

	if err := printer.Fprint(tempFile, fset, file); err != nil {
		return fmt.Errorf("writing modified AST: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("syncing temporary file: %w", err)
	}
	tempFile.Close()

	if err := os.Rename(tempName, filename); err != nil {
		if err := copyFile(tempName, filename); err != nil {
			return fmt.Errorf("failed to update file: %w", err)
		}
		err = cleanup(tempName)
		if err != nil {
			return fmt.Errorf("failed to cleanup temporary file: %w", err)
		}
	}

	fmt.Printf("Successfully converted file: %s\n", filename)

	return nil
}

// processFile handles the common AST processing logic
func processFile(file *ast.File, handler func(field *ast.Field, oldTag, newTag string, structName string) error) error {
	var processStructType func(st *ast.StructType, structName string) error

	processStructType = func(st *ast.StructType, structName string) error {
		for _, field := range st.Fields.List {
			// Process current field's tags if present
			if field.Tag != nil {
				structField, err := astToStructField(field)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: processing field %s: %v\n", field.Names[0], err)
					continue
				}

				config, err := parse.LegacyUnmarshalTagFormat(structField)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: parsing tags for field %s: %v\n", field.Names[0], err)
					continue
				}
				if config == nil {
					// No legacy tags found
					continue
				}

				newTag, err := convertLegacyTags(structField)
				if err != nil || newTag == "" {
					continue
				}

				if err := handler(field, field.Tag.Value, newTag, structName); err != nil {
					fmt.Fprintf(os.Stderr, "warning: handling field %s: %v\n", field.Names[0], err)
				}
			}

			// If field is a struct type, process it recursively
			if fieldType, ok := field.Type.(*ast.StructType); ok {
				fieldName := ""
				if len(field.Names) > 0 {
					fieldName = field.Names[0].Name
				}
				nestedStructName := structName
				if fieldName != "" {
					nestedStructName = structName + "." + fieldName
				}
				if err := processStructType(fieldType, nestedStructName); err != nil {
					return err
				}
			}
		}
		return nil
	}

	ast.Inspect(file, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		if err := processStructType(structType, typeSpec.Name.Name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: processing struct %s: %v\n", typeSpec.Name.Name, err)
		}
		return true
	})

	return nil
}

// updateTagValue removes all legacy tags and adds the new goopt tag
func updateTagValue(originalTag string, newGooptTag string) string {
	converter := NewTagCollector(originalTag)
	_ = converter.Parse(IsLegacyTag) // We can ignore the error as we just want to filter tags

	// Build new tag string with non-legacy, non-goopt tags
	var newTags []string
	for f := converter.otherTags.Front(); f != nil; f = f.Next() {
		if *f.Key != "goopt" {
			newTags = append(newTags, fmt.Sprintf(`%s:"%s"`, *f.Key, f.Value))
		}
	}

	// Add the new goopt tag
	newTags = append(newTags, newGooptTag)

	// Reconstruct the tag string
	return "`" + strings.Join(newTags, " ") + "`"
}

// convertLegacyTags converts old-style struct tags to the new format
func convertLegacyTags(field reflect.StructField) (string, error) {
	// Skip if already converted
	if field.Tag.Get("goopt") != "" {
		return "", nil
	}

	// Parse using existing logic
	config, err := parse.LegacyUnmarshalTagFormat(field)
	if err != nil {
		return "", err
	}
	if config == nil {
		return "", nil // No legacy tags found
	}

	var parts []string

	// Add fields in order
	if config.Name != "" {
		parts = append(parts, fmt.Sprintf("name:%s", config.Name))
	}
	if config.Short != "" {
		parts = append(parts, fmt.Sprintf("short:%s", config.Short))
	}
	if config.Description != "" {
		desc := escapeSequences(config.Description)
		parts = append(parts, fmt.Sprintf("desc:%s", desc))
	}
	if config.TypeOf != types.Empty {
		parts = append(parts, fmt.Sprintf("type:%s", config.TypeOf))
	}
	if config.Required {
		parts = append(parts, "required:true")
	}
	if config.Kind != "" && config.Kind != "flag" {
		parts = append(parts, fmt.Sprintf("kind:%s", config.Kind))
	}
	if config.Default != "" {
		parts = append(parts, fmt.Sprintf("default:%s", config.Default))
	}
	if config.Secure.IsSecure {
		parts = append(parts, "secure:true")
	}
	if config.Secure.Prompt != "" {
		prompt := escapeSequences(config.Secure.Prompt)
		parts = append(parts, fmt.Sprintf("prompt:%s", prompt))
	}
	if config.Path != "" {
		parts = append(parts, fmt.Sprintf("path:%s", config.Path))
	}

	if len(config.AcceptedValues) > 0 {
		var patternParts []string
		for _, pv := range config.AcceptedValues {
			var patternValue string
			if pv.Pattern != "" {
				patternValue = fmt.Sprintf("pattern:%s", pv.Pattern)
			}
			if pv.Description != "" {
				if patternValue != "" {
					patternValue = fmt.Sprintf("%s,desc:%s", patternValue, pv.Description)
				} else {
					patternValue = fmt.Sprintf("desc:%s", pv.Description)
				}
			}
			patternParts = append(patternParts, fmt.Sprintf("{%s}", patternValue))
		}
		sort.Strings(patternParts)
		parts = append(parts, fmt.Sprintf("accepted:%s", strings.Join(patternParts, ",")))
	}

	if len(config.DependsOn) > 0 {
		var dependsParts []string
		for flag, values := range config.DependsOn {
			var dependsValue string
			if len(values) > 0 {
				dependsValue = fmt.Sprintf("flag:%s,values:[%s]", flag, strings.Join(values, ","))
			} else {
				dependsValue = fmt.Sprintf("flag:%s", flag)
			}
			dependsParts = append(dependsParts, fmt.Sprintf("{%s}", dependsValue))
		}
		sort.Strings(dependsParts)
		parts = append(parts, fmt.Sprintf("depends:%s", strings.Join(dependsParts, ",")))
	}

	if len(parts) == 0 {
		return "", nil
	}

	return fmt.Sprintf(`goopt:"%s"`, strings.Join(parts, ";")), nil
}

// astToStructField converts an AST field to reflect.StructField
func astToStructField(field *ast.Field) (reflect.StructField, error) {
	if len(field.Names) == 0 {
		return reflect.StructField{}, fmt.Errorf("anonymous fields not supported")
	}

	structField := reflect.StructField{
		Name: field.Names[0].Name,
	}

	if field.Tag != nil {
		tag := strings.Trim(field.Tag.Value, "`")
		structField.Tag = reflect.StructTag(tag)
	}

	return structField, nil
}

const migrationDirName = ".goopt-migration"

// cleanupErr wraps cleanup errors to distinguish them from conversion errors
type cleanupErr struct {
	op  string
	err error
}

func (e *cleanupErr) Error() string {
	return fmt.Sprintf("cleanup failed (%s): %v", e.op, e.err)
}

// ensureMigrationDir creates the migration directory if it doesn't exist
func ensureMigrationDir(baseDir string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("base directory must be specified")
	}

	migrationDir := filepath.Join(baseDir, migrationDirName)
	err := os.MkdirAll(migrationDir, 0700)
	if err != nil {
		return "", fmt.Errorf("creating migration directory: %w", err)
	}

	return migrationDir, nil
}

// cleanup attempts to remove a file and wraps any error
func cleanup(file string) error {
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: failed to remove file %s: %v", file, err)
		return &cleanupErr{op: "remove", err: err}
	}
	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

func createSessionDir(baseDir string) (string, error) {
	migrationDir, err := ensureMigrationDir(baseDir)
	if err != nil {
		return "", err
	}

	// Create a unique session directory
	sessionName := fmt.Sprintf("session_%d", time.Now().UnixNano())
	sessionDir := filepath.Join(migrationDir, sessionName)

	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return "", fmt.Errorf("creating session directory: %w", err)
	}

	return sessionDir, nil
}

func checkFileAccess(filename string) error {
	f, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return f.Close()
}

func escapeSequences(s string) string {
	// Replace newlines with \n
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	// Replace any remaining carriage returns
	s = strings.ReplaceAll(s, "\r", "")
	// Escape quotes
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
