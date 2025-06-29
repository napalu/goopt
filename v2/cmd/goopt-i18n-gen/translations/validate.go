package translations

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/ast"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
)

func Validate(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return errors.ErrFailedToGetConfig
	}

	// Expand input files
	inputFiles, err := expandInputFiles(cfg.Input)
	if err != nil {
		return errors.ErrFailedToExpandInput.WithArgs(err)
	}

	// Read all translation files
	allTranslations := make(map[string]map[string]string) // filename -> translations
	for _, inputFile := range inputFiles {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return errors.ErrFailedToReadInput.WithArgs(inputFile, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return errors.ErrFailedToParseJson.WithArgs(inputFile, err)
		}

		allTranslations[inputFile] = translations
	}

	// Expand glob patterns for scan files
	var files []string
	for _, pattern := range cfg.Validate.Scan {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Fatalf(messages.Keys.AppError.FailedToExpandPattern, pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		fmt.Println(cfg.TR.T(messages.Keys.AppError.NoFiles))
		return errors.ErrNoFiles
	}

	// Scan for descKey references
	scanner := ast.NewScanner(cfg.TR)
	refs, err := scanner.ScanGoFiles(files)
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
				stubs := scanner.GenerateMissingKeys(missing)
				fmt.Printf("\n")
				fmt.Println(cfg.TR.T(messages.Keys.AppValidate.GeneratingStubs, inputFile) + ":")
				for key, value := range stubs {
					fmt.Printf("  \"%s\": \"%s\"\n", key, value)
					translations[key] = value
				}

				// Update the JSON file
				updatedData, err := json.MarshalIndent(translations, "", "  ")
				if err != nil {
					return errors.ErrFailedToMarshal.WithArgs(err)
				}

				if err := os.WriteFile(inputFile, updatedData, 0644); err != nil {
					return errors.ErrFailedToWriteJson.WithArgs(inputFile, err)
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
		return errors.ErrValidationFailed
	}

	return nil
}
