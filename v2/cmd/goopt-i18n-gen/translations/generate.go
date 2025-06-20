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
	"text/template"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/templates"
	"github.com/napalu/goopt/v2/types/orderedmap"
)

func Generate(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToGetConfig))
	}

	// Expand input files
	inputFiles, err := expandInputFiles(cfg.Input)
	if err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToExpandInput), err)
	}

	// Collect all unique keys from all locale files
	allKeys := make(map[string]bool)
	for _, inputFile := range inputFiles {
		// Ensure input file exists
		if err := ensureInputFile(inputFile); err != nil {
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
	// Track field names to detect duplicates
	fieldNameCount := make(map[string]int)

	for _, key := range keys {
		processedKey := key
		if cfg.Generate.Prefix != "" && strings.HasPrefix(key, cfg.Generate.Prefix) {
			processedKey = strings.TrimPrefix(key, cfg.Generate.Prefix)
			processedKey = strings.TrimPrefix(processedKey, ".")
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
			if cfg.Verbose {
				log.Printf("Skipping invalid key with empty part: %s", key)
			}
			continue
		}

		if len(parts) < 2 {
			// Top level key
			groupName := "Root"
			g, found := groups.Get(groupName)
			if !found {
				g = &templates.Group{Name: groupName, Fields: []templates.Field{}}
				groups.Set(groupName, g)
			}

			fieldName := toGoName(parts[0])
			// Check for duplicates within this group
			groupKey := groupName + "." + fieldName
			count := fieldNameCount[groupKey]
			fieldNameCount[groupKey] = count + 1
			if count > 0 {
				// Add numeric suffix for duplicates
				fieldName = fmt.Sprintf("%s%d", fieldName, count+1)
			}

			g.Fields = append(g.Fields, templates.Field{
				Name: fieldName,
				Key:  key,
			})
		} else {
			// Nested key
			groupPath := parts[:len(parts)-1]
			fieldName := parts[len(parts)-1]
			groupName := toGoName(strings.Join(groupPath, "_"))

			g, found := groups.Get(groupName)
			if !found {
				g = &templates.Group{Name: groupName, Fields: []templates.Field{}}
				groups.Set(groupName, g)
			}

			goFieldName := toGoName(fieldName)
			// Check for duplicates within this group
			groupKey := groupName + "." + goFieldName
			count := fieldNameCount[groupKey]
			fieldNameCount[groupKey] = count + 1
			if count > 0 {
				// Add numeric suffix for duplicates
				goFieldName = fmt.Sprintf("%s%d", goFieldName, count+1)
			}

			g.Fields = append(g.Fields, templates.Field{
				Name: goFieldName,
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
