package ast

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

// Updater handles AST-based source file updates with i18n support
type Updater struct {
	tr i18n.Translator
}

// NewUpdater creates a new AST updater with the given translator
func NewUpdater(tr i18n.Translator) *Updater {
	return &Updater{tr: tr}
}

// UpdateSourceFiles automatically adds descKey tags to source files
func (u *Updater) UpdateSourceFiles(fieldsToUpdate []FieldWithoutDescKey, generatedKeys map[string]string, backupDir string) error {
	// Create backup directory with timestamp
	sessionDir := filepath.Join(backupDir, fmt.Sprintf("session_%s", time.Now().Format("20060102_150405")))
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedCreateBackupDir), err)
	}

	// Group fields by file
	fileUpdates := make(map[string][]FieldWithoutDescKey)
	for _, field := range fieldsToUpdate {
		fileUpdates[field.File] = append(fileUpdates[field.File], field)
	}

	// Process each file
	var updateErrors []error
	for filename, fields := range fileUpdates {
		if err := u.updateFile(filename, fields, generatedKeys, sessionDir); err != nil {
			updateErrors = append(updateErrors, fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedUpdateFile), filename, err))
		}
	}

	// If all updates succeeded, we can optionally remove the backup directory
	if len(updateErrors) == 0 {
		fmt.Println()
		fmt.Println(u.tr.T(messages.Keys.AppAst.AllFilesUpdated, sessionDir))
		fmt.Println(u.tr.T(messages.Keys.AppAst.RestoreHint, sessionDir))
	} else {
		fmt.Println()
		fmt.Println(u.tr.T(messages.Keys.AppAst.SomeFilesFailed, sessionDir))
		for _, err := range updateErrors {
			fmt.Println(u.tr.T("  - %v", err))
		}
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedUpdateCount), len(updateErrors))
	}

	return nil
}

func (u *Updater) updateFile(filename string, fields []FieldWithoutDescKey, generatedKeys map[string]string, backupDir string) error {
	// Create backup
	backupPath := filepath.Join(backupDir, filepath.Base(filename))
	if err := copyFile(filename, backupPath); err != nil {
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedCreateBackup), err)
	}

	// Parse the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedParseFile), err)
	}

	// Create a map of line numbers to fields for quick lookup
	lineToField := make(map[int]FieldWithoutDescKey)
	for _, field := range fields {
		lineToField[field.Line] = field
	}

	// Update the AST
	var modified bool
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if st, ok := x.Type.(*ast.StructType); ok {
				modified = updateStructFields(fset, st, lineToField, generatedKeys) || modified
			}
		}
		return true
	})

	if !modified {
		return nil // No changes needed
	}

	// Write the modified AST to a temporary file
	tempFile, err := os.CreateTemp(filepath.Dir(filename), ".goopt-i18n-*.tmp")
	if err != nil {
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedCreateTemp), err)
	}
	tempPath := tempFile.Name()
	defer tempFile.Close()
	defer os.Remove(tempPath) // Clean up temp file

	// Use go/format to ensure proper formatting
	if err := format.Node(tempFile, fset, file); err != nil {
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedWriteFormatted), err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedSyncTemp), err)
	}

	// Atomic rename (or copy if cross-device)
	if err := os.Rename(tempPath, filename); err != nil {
		// Fallback to copy if rename fails (e.g., cross-device)
		if err := copyFile(tempPath, filename); err != nil {
			return fmt.Errorf(u.tr.T(messages.Keys.AppAst.FailedUpdate), err)
		}
	}

	fmt.Println(u.tr.T(messages.Keys.AppAst.FileUpdated, filename))
	return nil
}

func updateStructFields(fset *token.FileSet, st *ast.StructType, lineToField map[int]FieldWithoutDescKey, generatedKeys map[string]string) bool {
	var modified bool

	for _, field := range st.Fields.List {
		// First check if this field itself needs updating
		pos := fset.Position(field.Pos())
		if fieldUpdate, exists := lineToField[pos.Line]; exists {
			// This field needs a descKey
			if descKey, hasKey := generatedKeys[fieldUpdate.FieldPath]; hasKey {
				if updateFieldTag(field, descKey) {
					modified = true
				}
			}
		}

		// If this is a struct (could be a command or just a container), check if:
		// 1. The struct tag itself needs updating (for commands)
		// 2. Its fields need updating (recurse)
		if innerSt, ok := field.Type.(*ast.StructType); ok {
			// Check if the struct tag line matches any of our updates
			// For struct tags, the tag is after the closing brace, so we need to check the tag position
			if field.Tag != nil {
				tagPos := fset.Position(field.Tag.Pos())
				if fieldUpdate, exists := lineToField[tagPos.Line]; exists && fieldUpdate.Kind == "command" {
					// This is a command struct that needs a descKey
					if descKey, hasKey := generatedKeys[fieldUpdate.FieldPath]; hasKey {
						if updateFieldTag(field, descKey) {
							modified = true
						}
					}
				}
			}

			// Recurse into the struct's fields
			if updateStructFields(fset, innerSt, lineToField, generatedKeys) {
				modified = true
			}
		}
	}

	return modified
}

func updateFieldTag(field *ast.Field, descKey string) bool {
	if field.Tag == nil {
		// Create new tag
		field.Tag = &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("`goopt:\"descKey:%s\"`", descKey),
		}
		return true
	} else {
		// Update existing tag
		oldTag := field.Tag.Value
		newTag := addDescKeyToTag(oldTag, descKey)
		if newTag != oldTag {
			field.Tag.Value = newTag
			return true
		}
	}
	return false
}

func addDescKeyToTag(tagValue string, descKey string) string {
	// Remove backticks
	tag := strings.Trim(tagValue, "`")

	// Find goopt tag
	gooptStart := strings.Index(tag, `goopt:"`)
	if gooptStart == -1 {
		// No goopt tag, add one
		if tag == "" {
			return fmt.Sprintf("`goopt:\"descKey:%s\"`", descKey)
		}
		return fmt.Sprintf("`goopt:\"descKey:%s\" %s`", descKey, tag)
	}

	// Find the end of goopt tag value
	gooptValueStart := gooptStart + 7 // len(`goopt:"`)
	gooptEnd := gooptValueStart
	for gooptEnd < len(tag) && tag[gooptEnd] != '"' {
		if tag[gooptEnd] == '\\' && gooptEnd+1 < len(tag) {
			gooptEnd += 2 // Skip escaped character
		} else {
			gooptEnd++
		}
	}

	if gooptEnd >= len(tag) {
		// Malformed tag, don't modify
		return tagValue
	}

	// Extract goopt value
	gooptValue := tag[gooptValueStart:gooptEnd]

	// Check if descKey already exists
	if strings.Contains(gooptValue, "descKey:") {
		// Already has descKey, don't modify
		return tagValue
	}

	// Add descKey to goopt value
	if gooptValue == "" {
		gooptValue = fmt.Sprintf("descKey:%s", descKey)
	} else {
		gooptValue = fmt.Sprintf("%s;descKey:%s", gooptValue, descKey)
	}

	// Reconstruct the tag
	before := tag[:gooptValueStart]
	after := tag[gooptEnd:]
	return fmt.Sprintf("`%s%s%s`", before, gooptValue, after)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}
