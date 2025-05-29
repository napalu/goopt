package commands

import (
	"encoding/json"
	"fmt"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/ast"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/templates"
	"github.com/napalu/goopt/v2/types/orderedmap"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

func ExecuteGenerate(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToGetConfig))
	}

	// Expand input files
	inputFiles, err := ExpandInputFiles(cfg.Input)
	if err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToExpandInput), err)
	}

	// Collect all unique keys from all locale files
	allKeys := make(map[string]bool)
	for _, inputFile := range inputFiles {
		// Ensure input file exists
		if err := EnsureInputFile(inputFile); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToPrepareInput), inputFile, err)
		}

		// Read JSON file
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToReadInput), inputFile, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToParseJson), inputFile, err)
		}

		// Collect keys
		for key := range translations {
			allKeys[key] = true
		}

		if cfg.Verbose {
			fmt.Println(cfg.TR.T(messages.Keys.AppGenerate.FoundKeysInFile, len(translations), inputFile))
			fmt.Println()
		}
	}

	// Convert to sorted slice
	var keys []string
	for key := range allKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Group keys by their structure using OrderedMap to maintain order
	groups := orderedmap.NewOrderedMap[string, *templates.Group]()

	for _, key := range keys {
		processedKey := key
		if cfg.Generate.Prefix != "" && strings.HasPrefix(key, cfg.Generate.Prefix) {
			processedKey = strings.TrimPrefix(key, cfg.Generate.Prefix)
			processedKey = strings.TrimPrefix(processedKey, ".")
		}

		parts := strings.Split(processedKey, ".")
		if len(parts) < 2 {
			// Top level key
			groupName := "Root"
			g, found := groups.Get(groupName)
			if !found {
				g = &templates.Group{Name: groupName, Fields: []templates.Field{}}
				groups.Set(groupName, g)
			}
			g.Fields = append(g.Fields, templates.Field{
				Name: ToGoName(parts[0]),
				Key:  key,
			})
		} else {
			// Nested key
			groupPath := parts[:len(parts)-1]
			fieldName := parts[len(parts)-1]
			groupName := ToGoName(strings.Join(groupPath, "_"))

			g, found := groups.Get(groupName)
			if !found {
				g = &templates.Group{Name: groupName, Fields: []templates.Field{}}
				groups.Set(groupName, g)
			}
			g.Fields = append(g.Fields, templates.Field{
				Name: ToGoName(fieldName),
				Key:  key,
			})
		}
	}

	// Convert to slice for template
	var groupList []templates.Group
	iter := groups.Iterator()
	for idx, _, group := iter(); idx != nil; idx, _, group = iter() {
		groupList = append(groupList, *group)
	}

	// Generate Go code
	tmpl, err := template.New("generated").Parse(templates.GeneratedFileTemplate)
	if err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToParseTemplate), err)
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, templates.TemplateData{
		Package: cfg.Generate.Package,
		Groups:  groupList,
	})
	if err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToExecuteTemplate), err)
	}

	// Format the generated code
	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		log.Printf(cfg.TR.T(messages.Keys.AppWarning.FailedToFormat), err)
		formatted = []byte(buf.String())
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(cfg.Generate.Output)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToCreateOutputDir), err)
	}

	// Write output file
	if err := os.WriteFile(cfg.Generate.Output, formatted, 0644); err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToWriteOutput), err)
	}

	if cfg.Verbose {
		fmt.Println(cfg.TR.T(messages.Keys.AppGenerate.SuccessVerbose, cfg.Generate.Output, len(keys), len(inputFiles)))
	} else {
		fmt.Println(cfg.TR.T(messages.Keys.AppGenerate.Success, cfg.Generate.Output))
	}
	return nil
}

func ExecuteValidate(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	// Expand input files
	inputFiles, err := ExpandInputFiles(cfg.Input)
	if err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToExpandInput), err)
	}

	// Read all translation files
	allTranslations := make(map[string]map[string]string) // filename -> translations
	for _, inputFile := range inputFiles {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToReadInput), inputFile, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToParseJson), inputFile, err)
		}

		allTranslations[inputFile] = translations
	}

	// Expand glob patterns for scan files
	var files []string
	for _, pattern := range cfg.Validate.Scan {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Fatalf("Failed to expand pattern %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		fmt.Println(cfg.TR.T(messages.Keys.AppValidate.NoFiles))
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.NoFiles))
	}

	// Scan for descKey references
	refs, err := ast.ScanGoFiles(files)
	if err != nil {
		log.Fatalf("Failed to scan Go files: %v", err)
	}

	if cfg.Verbose {
		fmt.Println(cfg.TR.T(messages.Keys.AppValidate.FoundReferences, len(refs), len(files)))
	}

	// Validate references against each locale file
	hasErrors := false
	for _, inputFile := range inputFiles {
		translations := allTranslations[inputFile]
		missing := ast.ValidateDescKeys(refs, translations)

		if len(missing) > 0 {
			fmt.Printf("\n%s: ", inputFile)
			fmt.Println(cfg.TR.T(messages.Keys.AppValidate.MissingTranslations, len(missing)) + ":")
			for _, ref := range missing {
				fmt.Printf("  %s ", ref.Key)
				fmt.Println(cfg.TR.T(messages.Keys.AppValidate.UsedInFile, ref.File, ref.Line, ref.FieldName))
			}

			// Generate missing keys if requested
			if cfg.Validate.GenerateMissing {
				stubs := ast.GenerateMissingKeys(missing)
				fmt.Printf("\n")
				fmt.Println(cfg.TR.T(messages.Keys.AppValidate.GeneratingStubs, inputFile) + ":")
				for key, value := range stubs {
					fmt.Printf("  \"%s\": \"%s\"\n", key, value)
					translations[key] = value
				}

				// Update the JSON file
				updatedData, err := json.MarshalIndent(translations, "", "  ")
				if err != nil {
					return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToMarshal), err)
				}

				if err := os.WriteFile(inputFile, updatedData, 0644); err != nil {
					return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToWriteJson), inputFile, err)
				}
				fmt.Println(cfg.TR.T(messages.Keys.AppValidate.UpdatedFile, inputFile))
			}

			hasErrors = true
		} else {
			fmt.Printf("\n%s: ", inputFile)
			fmt.Println(cfg.TR.T(messages.Keys.AppValidate.AllKeysValid))
		}
	}

	if hasErrors && cfg.Validate.Strict && !cfg.Validate.GenerateMissing {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.ValidationFailed))
	}

	return nil
}

func ExecuteAudit(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}
	// Determine which files to audit
	scanPatterns := cfg.Audit.Files
	if len(scanPatterns) == 0 {
		// Default to all Go files in current directory
		scanPatterns = []string{"*.go"}
	}

	// Expand glob patterns
	var files []string
	for _, pattern := range scanPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Fatalf("Failed to expand pattern %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.NoFiles))
	}

	// Scan for fields without descKey tags
	fieldsWithoutKeys, err := ast.ScanForMissingDescKeys(files)
	if err != nil {
		log.Fatalf("Failed to scan for missing descKeys: %v", err)
	}

	if len(fieldsWithoutKeys) == 0 {
		fmt.Println(cfg.TR.T(messages.Keys.AppAudit.AllFieldsHaveKeys))
		return nil
	}

	fmt.Println(cfg.TR.T(messages.Keys.AppAudit.FoundFieldsWithoutKeys, len(fieldsWithoutKeys)) + ":")
	for _, field := range fieldsWithoutKeys {
		fmt.Printf("  %s.%s (%s:%d) - %s %s",
			field.StructName, field.FieldName, field.File, field.Line, field.Kind, field.Name)
		if field.Desc != "" {
			fmt.Printf(" [%s]", cfg.TR.T(messages.Keys.AppAudit.DescLabel, field.Desc))
		}
		fmt.Println()
	}

	if !cfg.Audit.GenerateDescKeys {
		fmt.Println()
		fmt.Println(cfg.TR.T(messages.Keys.AppAudit.TipGenerateKeys))
		return nil
	}

	// Generate descKeys and translations
	generatedKeys, generatedTranslations := ast.GenerateDescKeysAndTranslations(fieldsWithoutKeys, cfg.Audit.KeyPrefix)

	fmt.Println()
	fmt.Println(cfg.TR.T(messages.Keys.AppAudit.GeneratedKeysHeader))
	for fieldPath, descKey := range generatedKeys {
		fmt.Printf("  %s -> descKey:%s\n", fieldPath, descKey)
		translation := generatedTranslations[descKey]
		fmt.Printf("    %s\n", cfg.TR.T(messages.Keys.AppAudit.TranslationLabel, translation))
	}

	// Update JSON files if requested
	if cfg.Audit.GenerateMissing && len(cfg.Input) > 0 {
		// Expand input files
		inputFiles, err := ExpandInputFiles(cfg.Input)
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToExpandInput), err)
		}

		// Update each locale file
		for _, inputFile := range inputFiles {
			// Ensure input file exists
			if err := EnsureInputFile(inputFile); err != nil {
				return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToPrepareInput), inputFile, err)
			}

			// Read existing translations
			data, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToReadInput), inputFile, err)
			}

			var translations map[string]string
			if err := json.Unmarshal(data, &translations); err != nil {
				return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToParseJson), inputFile, err)
			}

			// Add generated translations
			updated := false
			for descKey, translation := range generatedTranslations {
				if _, exists := translations[descKey]; !exists {
					translations[descKey] = translation
					updated = true
				}
			}

			if updated {
				// Write updated JSON
				updatedData, err := json.MarshalIndent(translations, "", "  ")
				if err != nil {
					return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToMarshal), err)
				}

				if err := os.WriteFile(inputFile, updatedData, 0644); err != nil {
					return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToWriteJson), inputFile, err)
				}
				fmt.Println()
				fmt.Println(cfg.TR.T(messages.Keys.AppAudit.UpdatedJsonFile, inputFile))
			}
		}
	}

	// Update source files if requested
	if cfg.Audit.AutoUpdate {
		fmt.Println()
		fmt.Println(cfg.TR.T(messages.Keys.AppAudit.AutoUpdating))
		if err := ast.UpdateSourceFiles(fieldsWithoutKeys, generatedKeys, cfg.Audit.BackupDir); err != nil {
			fmt.Println(cfg.TR.T(messages.Keys.AppWarning.UpdateFailed, err))
		}
	} else {
		fmt.Println()
		fmt.Println(cfg.TR.T(messages.Keys.AppAudit.ManualInstructions))
		for _, field := range fieldsWithoutKeys {
			descKey := generatedKeys[field.FieldPath]
			fmt.Println()
			fmt.Printf("  %s:\n", cfg.TR.T(messages.Keys.AppAudit.InFileUpdateTag, field.File, field.Line))
			fmt.Printf("    descKey:%s\n", descKey)
		}
		fmt.Println()
		fmt.Println(cfg.TR.T(messages.Keys.AppAudit.TipAutoUpdate))
	}

	return nil
}

func ExecuteInit(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}
	if len(cfg.Input) == 0 {
		// Default to locales/en.json
		cfg.Input = []string{"locales/en.json"}
	}

	// Initialize each specified file
	for _, inputFile := range cfg.Input {
		// Check if file exists
		if _, err := os.Stat(inputFile); err == nil && !cfg.Init.Force {
			fmt.Println(cfg.TR.T(messages.Keys.AppInit.FileExists, inputFile))
			continue
		}

		// Create directory if needed
		dir := filepath.Dir(inputFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToCreateDir), dir, err)
		}

		// Create initial JSON with some example keys
		initialData := map[string]string{
			"app.name":        "My Application",
			"app.description": "Application description",
			"app.version":     "Version",
		}

		data, err := json.MarshalIndent(initialData, "", "  ")
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToMarshal), err)
		}

		if err := os.WriteFile(inputFile, data, 0644); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToCreateFile), inputFile, err)
		}

		fmt.Println(cfg.TR.T(messages.Keys.AppInit.CreatedFile, inputFile))
	}

	if len(cfg.Input) > 0 {
		fmt.Println()
		fmt.Println(cfg.TR.T(messages.Keys.AppInit.NextSteps))
		fmt.Printf("1. %s\n", cfg.TR.T(messages.Keys.AppInit.Step1, strings.Join(cfg.Input, ", ")))
		fmt.Printf("2. %s\n", cfg.TR.T(messages.Keys.AppInit.Step2, strings.Join(cfg.Input, ",")))
		fmt.Println("3. " + cfg.TR.T(messages.Keys.AppInit.Step3))
	}

	return nil
}

// toGoName converts a string to a valid Go identifier
func ToGoName(s string) string {
	// Replace common separators with underscores
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")

	// Split on underscores and capitalize each part
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, "")
}

// ensureInputFile creates the directory and file if they don't exist
func EnsureInputFile(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, nothing to do
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create empty JSON file
	emptyJSON := []byte("{}")
	if err := os.WriteFile(path, emptyJSON, 0644); err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}

	fmt.Printf("✓ Created %s\n", path)
	return nil
}

// expandInputFiles expands wildcards in input paths and returns all matching files
func ExpandInputFiles(inputs []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, pattern := range inputs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to expand pattern %s: %w", pattern, err)
		}

		// If no matches, treat as literal file
		if len(matches) == 0 {
			matches = []string{pattern}
		}

		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				files = append(files, match)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no input files found")
	}

	return files, nil
}
