package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/internal/parse"
)

// Scanner handles AST-based source file scanning with i18n support
type Scanner struct {
	tr i18n.Translator
}

// NewScanner creates a new AST scanner with the given translator
func NewScanner(tr i18n.Translator) *Scanner {
	return &Scanner{tr: tr}
}

// DescKeyReference represents a descKey found in source code
type DescKeyReference struct {
	Key       string
	File      string
	Line      int
	FieldName string
}

// FieldWithoutDescKey represents a goopt field that needs a descKey
type FieldWithoutDescKey struct {
	File       string
	Line       int
	FieldName  string
	StructName string
	FieldPath  string // e.g., "Config.User.Create"
	Kind       string // "flag" or "command"
	Name       string // The name from goopt tag
	Desc       string // The desc from goopt tag (if any)
}

// ScanGoFiles scans Go source files for descKey references in struct tags
func (s *Scanner) ScanGoFiles(files []string) ([]DescKeyReference, error) {
	var refs []DescKeyReference

	for _, file := range files {
		fileRefs, err := scanFile(file)
		if err != nil {
			return nil, fmt.Errorf(s.tr.T(messages.Keys.AppAst.FailedScanFile), file, err)
		}
		refs = append(refs, fileRefs...)
	}

	return refs, nil
}

func scanFile(filename string) ([]DescKeyReference, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var refs []DescKeyReference

	// Walk the AST looking for struct types
	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		// Check each field in the struct
		for _, field := range structType.Fields.List {
			if field.Tag == nil {
				continue
			}

			// Parse the tag
			tag := strings.Trim(field.Tag.Value, "`")
			gooptTag := getTagValue(tag, "goopt")
			if gooptTag == "" {
				continue
			}

			// Create a dummy reflect.StructField for UnmarshalTagFormat
			fieldName := ""
			if len(field.Names) > 0 {
				fieldName = field.Names[0].Name
			}
			dummyField := reflect.StructField{
				Name: fieldName,
				Type: reflect.TypeOf(""), // dummy type
			}

			// Use goopt's tag parser
			tagConfig, err := parse.UnmarshalTagFormat(gooptTag, dummyField)
			if err != nil {
				continue // Skip malformed tags
			}

			if tagConfig.DescriptionKey != "" {
				pos := fset.Position(field.Pos())

				refs = append(refs, DescKeyReference{
					Key:       tagConfig.DescriptionKey,
					File:      filename,
					Line:      pos.Line,
					FieldName: fieldName,
				})
			}
		}

		return true
	})

	return refs, nil
}

// getTagValue extracts the value for a specific tag key from a struct tag
func getTagValue(tag, key string) string {
	// Handle struct tags like: goopt:"..." json:"..."
	// We need to properly parse quoted values

	// Look for key:"value" pattern
	prefix := key + `:`
	idx := strings.Index(tag, prefix)
	if idx == -1 {
		return ""
	}

	// Start after the key:
	start := idx + len(prefix)

	// Check if value is quoted
	if start < len(tag) && tag[start] == '"' {
		// Find the closing quote
		end := start + 1
		for end < len(tag) {
			if tag[end] == '"' && (end == start+1 || tag[end-1] != '\\') {
				return tag[start+1 : end]
			}
			end++
		}
	} else {
		// Unquoted value - find the end (space or end of string)
		end := start
		for end < len(tag) && tag[end] != ' ' && tag[end] != '\t' {
			end++
		}
		return tag[start:end]
	}

	return ""
}

// ValidateDescKeys checks that all descKey references exist in translations
func ValidateDescKeys(refs []DescKeyReference, translations map[string]string) []DescKeyReference {
	var missing []DescKeyReference

	for _, ref := range refs {
		if _, exists := translations[ref.Key]; !exists {
			missing = append(missing, ref)
		}
	}

	return missing
}

// GenerateMissingKeys creates stub entries for missing translation keys
func (s *Scanner) GenerateMissingKeys(missing []DescKeyReference) map[string]string {
	stubs := make(map[string]string)
	caser := cases.Title(language.Und)
	for _, ref := range missing {
		// Generate a reasonable default based on the key
		parts := strings.Split(ref.Key, ".")
		lastPart := parts[len(parts)-1]

		// Convert to human-readable format using strcase
		defaultText := strcase.ToDelimited(lastPart, ' ')
		defaultText = caser.String(defaultText)

		stubs[ref.Key] = s.tr.T(messages.Keys.AppAst.TodoPrefix, defaultText)
	}

	return stubs
}

// ScanForMissingDescKeys finds goopt fields without descKey tags
func (s *Scanner) ScanForMissingDescKeys(files []string) ([]FieldWithoutDescKey, error) {
	var fields []FieldWithoutDescKey

	for _, file := range files {
		fileFields, err := scanFileForMissingDescKeys(file)
		if err != nil {
			return nil, fmt.Errorf(s.tr.T(messages.Keys.AppAst.FailedScanFile), file, err)
		}
		fields = append(fields, fileFields...)
	}

	return fields, nil
}

func scanFileForMissingDescKeys(filename string) ([]FieldWithoutDescKey, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var fields []FieldWithoutDescKey

	// Track current struct path for nested structs
	var scanStruct func(ast.Node, []string) bool
	scanStruct = func(n ast.Node, structPath []string) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if st, ok := x.Type.(*ast.StructType); ok {
				structName := x.Name.Name
				currentPath := append(structPath, structName)
				scanStructFields(fset, filename, st, currentPath, &fields)
			}
		}
		return true
	}

	ast.Inspect(node, func(n ast.Node) bool {
		return scanStruct(n, []string{})
	})

	return fields, nil
}

func scanStructFields(fset *token.FileSet, filename string, st *ast.StructType, structPath []string, fields *[]FieldWithoutDescKey) {
	for _, field := range st.Fields.List {
		// Get field name for debugging
		fieldName := ""
		if len(field.Names) > 0 {
			fieldName = field.Names[0].Name
		}

		if field.Tag == nil {
			// Check if this is a struct without tags that might contain flags
			if _, ok := field.Type.(*ast.StructType); ok {
				// Recurse into tagless structs too
				newPath := append(structPath, fieldName)
				scanStructFields(fset, filename, field.Type.(*ast.StructType), newPath, fields)
			}
			continue
		}

		tag := strings.Trim(field.Tag.Value, "`")
		gooptTag := getTagValue(tag, "goopt")
		if gooptTag == "" {
			continue
		}

		// Create a dummy reflect.StructField for UnmarshalTagFormat
		dummyField := reflect.StructField{
			Name: fieldName,
			Type: reflect.TypeOf(""), // dummy type
		}

		// Use goopt's tag parser
		tagConfig, err := parse.UnmarshalTagFormat(gooptTag, dummyField)
		if err != nil {
			continue // Skip malformed tags
		}

		// Check if it already has descKey
		if tagConfig.DescriptionKey != "" {
			continue
		}

		// Get values from parsed config
		kind := string(tagConfig.Kind)
		if kind == "" {
			kind = "flag" // default
		}

		name := tagConfig.Name
		if name == "" {
			name = fieldName
		}

		desc := tagConfig.Description

		pos := fset.Position(field.Pos())

		// Check if this is a nested struct that might have sub-fields
		if structType, ok := field.Type.(*ast.StructType); ok {
			// This is a struct - could be a command or just a container for flags

			// If it's a command without descKey, add it to the list
			if kind == "command" && tagConfig.DescriptionKey == "" {
				*fields = append(*fields, FieldWithoutDescKey{
					File:       filename,
					Line:       pos.Line,
					FieldName:  fieldName,
					StructName: strings.Join(structPath, "."),
					FieldPath:  strings.Join(append(structPath, fieldName), "."),
					Kind:       kind,
					Name:       name,
					Desc:       desc,
				})
			}

			// Always scan its sub-fields recursively (for both commands and flag containers)
			newPath := append(structPath, fieldName)
			scanStructFields(fset, filename, structType, newPath, fields)
		} else {
			// Regular field - add it (we have a goopt tag but no descKey)
			*fields = append(*fields, FieldWithoutDescKey{
				File:       filename,
				Line:       pos.Line,
				FieldName:  fieldName,
				StructName: strings.Join(structPath, "."),
				FieldPath:  strings.Join(append(structPath, fieldName), "."),
				Kind:       kind,
				Name:       name,
				Desc:       desc,
			})
		}
	}
}

// GenerateDescKeysAndTranslations generates descKey values and translation stubs
func GenerateDescKeysAndTranslations(fields []FieldWithoutDescKey, keyPrefix string) (map[string]string, map[string]string) {
	descKeys := make(map[string]string)     // field path -> descKey
	translations := make(map[string]string) // descKey -> translation

	for _, field := range fields {
		// Generate descKey based on the field path
		keyParts := []string{keyPrefix}

		// Add struct path parts (converting to snake_case)
		pathParts := strings.Split(field.FieldPath, ".")
		for _, part := range pathParts {
			keyParts = append(keyParts, strcase.ToSnake(part))
		}

		// Add suffix based on kind
		keyParts[len(keyParts)-1] = keyParts[len(keyParts)-1] + "_desc"

		descKey := strings.Join(keyParts, ".")
		descKeys[field.FieldPath] = descKey

		// Generate translation based on desc tag or field name
		var translation string
		if field.Desc != "" {
			translation = field.Desc
		} else {
			// Convert field name to human-readable text
			translation = strcase.ToDelimited(field.FieldName, ' ')
			caser := cases.Title(language.Und)
			translation = caser.String(translation)

			// Add context based on kind
			if field.Kind == "command" {
				translation = translation + " command"
			}
		}

		translations[descKey] = translation
	}

	return descKeys, translations
}
