package translations

import (
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/templates"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/util"
)

func Generate(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return errors.ErrFailedToGetConfig
	}

	// Expand input files
	inputFiles, err := expandInputFiles(cfg.Input)
	if err != nil {
		return errors.ErrFailedToExpandInput.WithArgs(err)
	}

	// Collect all unique keys from all locale files
	allKeys := make(map[string]bool)
	for _, inputFile := range inputFiles {
		// Ensure input file exists
		if err := ensureInputFile(inputFile); err != nil {
			return errors.ErrFailedToPrepareInput.WithArgs(inputFile, err)
		}

		// Read JSON file
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return errors.ErrFailedToReadInput.WithArgs(inputFile, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return errors.ErrFailedToParseJson.WithArgs(inputFile, err)
		}

		// Collect keys
		for key := range translations {
			allKeys[key] = true
		}

		if cfg.Verbose {
			fmt.Println(cfg.TR.T(messages.Keys.App.Generate.FoundKeysInFile, len(translations), inputFile))
			fmt.Println()
		}
	}

	// Convert to sorted slice for stable output
	var keys []string
	for key := range allKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build nested structure
	root := buildNestedStructure(keys, cfg.Generate.Prefix)

	// Generate Go code directly without complex template
	var buf strings.Builder
	buf.WriteString("// Code generated by goopt-i18n-gen. DO NOT EDIT.\n\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", cfg.Generate.Package))

	// Generate the struct definition with initialization
	generateStruct(&buf, root, "")

	// Format the generated code
	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		log.Print(cfg.TR.T(messages.Keys.App.Warning.FailedToFormat, err))
		// Try to provide more info about the error
		log.Printf("Generated code:\n%s", buf.String())
		formatted = []byte(buf.String())
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(cfg.Generate.Output)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return errors.ErrFailedToCreateOutputDir.WithArgs(err)
	}

	// Write output file
	if err := os.WriteFile(cfg.Generate.Output, formatted, 0644); err != nil {
		return errors.ErrFailedToWriteOutput.WithArgs(err)
	}

	if cfg.Verbose {
		fmt.Println(cfg.TR.T(messages.Keys.App.Generate.SuccessVerbose, cfg.Generate.Output, len(keys), len(inputFiles)))
	} else {
		fmt.Println(cfg.TR.T(messages.Keys.App.Generate.Success, cfg.Generate.Output))
	}
	return nil
}

// buildNestedStructure creates a hierarchical structure from flat keys
func buildNestedStructure(keys []string, prefix string) *templates.NestedGroup {
	root := &templates.NestedGroup{
		Name:      "Keys",
		Fields:    []templates.Field{},
		SubGroups: make(map[string]*templates.NestedGroup),
	}

	// Track field names to detect duplicates
	fieldNameCount := make(map[string]int)

	// Default prefix if not specified
	if prefix == "" {
		prefix = "app"
	}

	for _, key := range keys {
		// If the key doesn't start with the prefix, prepend it
		processedKey := key
		if !strings.HasPrefix(key, prefix+".") {
			processedKey = prefix + "." + key
		}

		parts := strings.Split(processedKey, ".")

		// Skip keys with empty parts
		hasEmptyPart := false
		for _, part := range parts {
			if part == "" {
				hasEmptyPart = true
				break
			}
		}
		if hasEmptyPart {
			continue
		}

		// Navigate/create the structure
		current := root
		for i, part := range parts {
			if i == len(parts)-1 {
				// This is a field
				fieldName := util.KeyToGoName(part)

				// Check for duplicates
				fullPath := getGroupPath(current) + "." + fieldName
				count := fieldNameCount[fullPath]
				fieldNameCount[fullPath] = count + 1
				if count > 0 {
					fieldName = fmt.Sprintf("%s%d", fieldName, count+1)
				}

				current.Fields = append(current.Fields, templates.Field{
					Name: fieldName,
					Key:  key,
				})
			} else {
				// This is a subgroup
				groupName := util.KeyToGoName(part)
				if _, exists := current.SubGroups[groupName]; !exists {
					current.SubGroups[groupName] = &templates.NestedGroup{
						Name:      groupName,
						Fields:    []templates.Field{},
						SubGroups: make(map[string]*templates.NestedGroup),
					}
				}
				current = current.SubGroups[groupName]
			}
		}
	}

	return root
}

// getGroupPath returns the full path of a group for duplicate detection
func getGroupPath(group *templates.NestedGroup) string {
	// This is a simplified version - in a real implementation,
	// you'd track the full path as you traverse
	return group.Name
}

// generateStruct generates a struct type with values initialized inline
func generateStruct(buf *strings.Builder, group *templates.NestedGroup, indent string) {
	// Check if we have exactly one top-level group (e.g., "App" when using default prefix)
	// This is the common case when using a prefix like "app"
	if len(group.Fields) == 0 && len(group.SubGroups) == 1 {
		// Get the single top-level group
		var topLevelName string
		var topLevelGroup *templates.NestedGroup
		for name, subGroup := range group.SubGroups {
			topLevelName = name
			topLevelGroup = subGroup
			break
		}

		// Generate all nested type definitions first
		generateNestedTypes(buf, topLevelGroup)

		// Generate the top-level type (e.g., App)
		buf.WriteString(fmt.Sprintf("type %s struct {\n", topLevelName))

		// Add fields
		for _, field := range topLevelGroup.Fields {
			buf.WriteString(fmt.Sprintf("\t%s string\n", field.Name))
		}

		// Add sub-structs (sorted for deterministic output)
		var subGroupNames []string
		for name := range topLevelGroup.SubGroups {
			if len(topLevelGroup.SubGroups[name].Fields) > 0 || len(topLevelGroup.SubGroups[name].SubGroups) > 0 {
				subGroupNames = append(subGroupNames, name)
			}
		}
		sort.Strings(subGroupNames)

		for _, name := range subGroupNames {
			buf.WriteString(fmt.Sprintf("\t%s %s\n", name, name))
		}

		buf.WriteString("}\n\n")

		// Generate the Keys variable with the single top-level struct
		buf.WriteString("// Keys provides compile-time safe access to translation keys\n")
		buf.WriteString(fmt.Sprintf("var Keys struct {\n\t%s %s\n} = struct {\n\t%s %s\n}{\n",
			topLevelName, topLevelName, topLevelName, topLevelName))

		// Initialize the top-level struct
		buf.WriteString(fmt.Sprintf("\t%s: %s{\n", topLevelName, topLevelName))
		generateInitializer(buf, topLevelGroup, "\t\t")
		buf.WriteString("\t},\n")
		buf.WriteString("}\n")

		return
	}

	// Handle the case where we have multiple top-level groups or fields
	// This happens when there's no common prefix or mixed top-level elements

	// First generate all the type definitions for nested structures
	var typeNames []string
	for name, subGroup := range group.SubGroups {
		if len(subGroup.Fields) == 0 && len(subGroup.SubGroups) == 0 {
			continue
		}
		typeNames = append(typeNames, name)

		// Generate nested types first (depth-first)
		generateNestedTypes(buf, subGroup)

		// Generate type definition for this group
		buf.WriteString(fmt.Sprintf("type %s struct {\n", name))

		// Add fields
		for _, field := range subGroup.Fields {
			buf.WriteString(fmt.Sprintf("\t%s string\n", field.Name))
		}

		// Add sub-structs
		for subName, subSubGroup := range subGroup.SubGroups {
			if len(subSubGroup.Fields) == 0 && len(subSubGroup.SubGroups) == 0 {
				continue
			}
			buf.WriteString(fmt.Sprintf("\t%s %s\n", subName, subName))
		}

		buf.WriteString("}\n\n")
	}

	// Sort type names for deterministic output
	sort.Strings(typeNames)

	// Generate the Keys variable
	buf.WriteString("// Keys provides compile-time safe access to translation keys\n")
	buf.WriteString("var Keys struct {\n")

	// Add top-level fields
	for _, field := range group.Fields {
		buf.WriteString(fmt.Sprintf("\t%s string\n", field.Name))
	}

	// Add top-level groups
	for _, name := range typeNames {
		buf.WriteString(fmt.Sprintf("\t%s %s\n", name, name))
	}

	buf.WriteString("} = struct {\n")

	// Repeat structure for initialization
	for _, field := range group.Fields {
		buf.WriteString(fmt.Sprintf("\t%s string\n", field.Name))
	}
	for _, name := range typeNames {
		buf.WriteString(fmt.Sprintf("\t%s %s\n", name, name))
	}

	buf.WriteString("}{\n")

	// Initialize top-level fields
	for _, field := range group.Fields {
		buf.WriteString(fmt.Sprintf("\t%s: \"%s\",\n", field.Name, field.Key))
	}

	// Initialize top-level groups
	for _, name := range typeNames {
		subGroup := group.SubGroups[name]
		buf.WriteString(fmt.Sprintf("\t%s: %s{\n", name, name))
		generateInitializer(buf, subGroup, "\t\t")
		buf.WriteString("\t},\n")
	}

	buf.WriteString("}\n")
}

// generateNestedTypes generates type definitions for nested structures
func generateNestedTypes(buf *strings.Builder, group *templates.NestedGroup) {
	for name, subGroup := range group.SubGroups {
		if len(subGroup.Fields) == 0 && len(subGroup.SubGroups) == 0 {
			continue
		}

		// Recursively generate nested types first
		generateNestedTypes(buf, subGroup)

		// Generate type definition
		buf.WriteString(fmt.Sprintf("type %s struct {\n", name))

		// Add fields
		for _, field := range subGroup.Fields {
			buf.WriteString(fmt.Sprintf("\t%s string\n", field.Name))
		}

		// Add sub-structs
		for subName, subSubGroup := range subGroup.SubGroups {
			if len(subSubGroup.Fields) == 0 && len(subSubGroup.SubGroups) == 0 {
				continue
			}
			buf.WriteString(fmt.Sprintf("\t%s %s\n", subName, subName))
		}

		buf.WriteString("}\n\n")
	}
}

// generateInitializer generates the initialization code for a group
func generateInitializer(buf *strings.Builder, group *templates.NestedGroup, indent string) {
	// Initialize fields
	for _, field := range group.Fields {
		buf.WriteString(fmt.Sprintf("%s%s: \"%s\",\n", indent, field.Name, field.Key))
	}

	// Initialize sub-groups
	for name, subGroup := range group.SubGroups {
		if len(subGroup.Fields) == 0 && len(subGroup.SubGroups) == 0 {
			continue
		}

		buf.WriteString(fmt.Sprintf("%s%s: %s{\n", indent, name, name))
		generateInitializer(buf, subGroup, indent+"\t")
		buf.WriteString(fmt.Sprintf("%s},\n", indent))
	}
}
