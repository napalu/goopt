package translations

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/ast"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/util"
)

func Audit(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return errors.ErrFailedToGetConfig
	}
	// Determine which files to audit
	scanPatterns := cfg.Audit.Files
	if len(scanPatterns) == 0 {
		// Default to all Go files in current directory
		scanPatterns = []string{"*.go"}
	}

	// Expand glob patterns using our utility that supports **
	files, err := util.ExpandGlobPatterns(scanPatterns)
	if err != nil {
		log.Fatalf("Failed to expand patterns: %v", err)
	}

	if len(files) == 0 {
		return errors.ErrNoFiles
	}

	// Scan for fields without descKey tags
	scanner := ast.NewScanner(cfg.TR)
	fieldsWithoutKeys, err := scanner.ScanForMissingDescKeys(files)
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

	// Update JSON files when generating descKeys or when explicitly requested
	if (cfg.Audit.GenerateDescKeys || cfg.Audit.GenerateMissing) && len(cfg.Input) > 0 && len(generatedTranslations) > 0 {
		// Expand input files
		inputFiles, err := expandInputFiles(cfg.Input)
		if err != nil {
			return errors.ErrFailedToExpandInput.WithArgs(err)
		}

		// Update each locale file
		for _, inputFile := range inputFiles {
			// Ensure input file exists
			if err := ensureInputFile(inputFile); err != nil {
				return errors.ErrFailedToPrepareInput.WithArgs(inputFile, err)
			}

			// Read existing translations
			data, err := os.ReadFile(inputFile)
			if err != nil {
				return errors.ErrFailedToReadInput.WithArgs(inputFile, err)
			}

			var translations map[string]string
			if err := json.Unmarshal(data, &translations); err != nil {
				return errors.ErrFailedToParseJson.WithArgs(inputFile, err)
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
					return errors.ErrFailedToMarshal.WithArgs(err)
				}

				if err := os.WriteFile(inputFile, updatedData, 0644); err != nil {
					return errors.ErrFailedToWriteJson.WithArgs(inputFile, err)
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
		updater := ast.NewUpdater(cfg.TR)
		if err := updater.UpdateSourceFiles(fieldsWithoutKeys, generatedKeys, cfg.Audit.BackupDir); err != nil {
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
