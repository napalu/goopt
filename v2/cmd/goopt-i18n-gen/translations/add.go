package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
)

// Add adds new translation keys to locale files
func Add(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return errors.ErrFailedToGetConfig
	}

	// Validate inputs
	keysToAdd := make(map[string]string)

	// Check if we have keys to add
	hasFromFile := cfg.Add.FromFile != ""
	hasSingleKey := cfg.Add.Key != ""

	if !hasFromFile && !hasSingleKey {
		return errors.ErrNoKeys
	}

	if hasFromFile && hasSingleKey {
		return errors.ErrBothSingleAndFile
	}

	if hasSingleKey && cfg.Add.Value == "" {
		return errors.ErrMissingValue
	}

	// Validate mode
	validModes := map[string]bool{"skip": true, "replace": true, "error": true}
	if !validModes[cfg.Add.Mode] {
		return errors.ErrInvalidMode.WithArgs(cfg.Add.Mode)
	}

	// Load keys
	if hasFromFile {
		if cfg.Verbose {
			fmt.Println(cfg.TR.T(messages.Keys.App.Add.ReadingKeysFile, cfg.Add.FromFile))
		}

		data, err := os.ReadFile(cfg.Add.FromFile)
		if err != nil {
			return errors.ErrFailedReadKeysFile.WithArgs(cfg.Add.FromFile, err)
		}

		if err := json.Unmarshal(data, &keysToAdd); err != nil {
			return errors.ErrFailedParseKeysFile.WithArgs(cfg.Add.FromFile, err)
		}

		if cfg.Verbose {
			fmt.Println(cfg.TR.T(messages.Keys.App.Add.FoundKeysInFile, len(keysToAdd), cfg.Add.FromFile))
			fmt.Println()
		}
	} else {
		keysToAdd[cfg.Add.Key] = cfg.Add.Value
	}

	// Expand input files
	inputFiles, err := expandInputFiles(cfg.Input)
	if err != nil {
		return errors.ErrFailedToExpandInput.WithArgs(err)
	}

	// Determine default language (English if available, otherwise first file)
	defaultLang := "en"
	hasEnglish := false
	for _, file := range inputFiles {
		if strings.Contains(file, "en.json") {
			hasEnglish = true
			break
		}
	}
	if !hasEnglish && len(inputFiles) > 0 {
		defaultLang = strings.TrimSuffix(filepath.Base(inputFiles[0]), ".json")
	}

	// Process each locale file
	totalAdded := 0
	totalSkipped := 0
	totalReplaced := 0
	filesProcessed := 0

	for _, inputFile := range inputFiles {
		lang := strings.TrimSuffix(filepath.Base(inputFile), ".json")
		isDefaultLang := lang == defaultLang

		fmt.Println(cfg.TR.T(messages.Keys.App.Add.ProcessingLocale, inputFile))

		// Read existing translations
		var translations map[string]string
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return errors.ErrFailedToReadInput.WithArgs(inputFile, err)
		}

		if err := json.Unmarshal(data, &translations); err != nil {
			translations = make(map[string]string)
		}

		// Process keys
		fileAdded := 0
		fileSkipped := 0
		fileReplaced := 0

		if cfg.Add.DryRun {
			fmt.Println(cfg.TR.T(messages.Keys.App.Add.DryRunWouldAdd, inputFile))
		}

		for key, value := range keysToAdd {
			existingValue, exists := translations[key]

			if exists {
				switch cfg.Add.Mode {
				case "error":
					return errors.ErrKeyExistsError.WithArgs(key, inputFile)
				case "skip":
					if cfg.Add.DryRun {
						fmt.Println(cfg.TR.T(messages.Keys.App.Add.DryRunWouldSkip, key))
						fileSkipped++
					} else if cfg.Verbose {
						fmt.Println(cfg.TR.T(messages.Keys.App.Add.KeyExistsSkip, key))
						fileSkipped++
					} else {
						fileSkipped++
					}
				case "replace":
					if cfg.Add.DryRun {
						fmt.Printf(cfg.TR.T(messages.Keys.App.Add.DryRunWouldReplace, key))
						displayValue := value
						if !isDefaultLang {
							displayValue = cfg.TR.T(messages.Keys.App.Ast.TodoPrefix, value)
						}
						fmt.Printf(" %s -> %s\n", existingValue, displayValue)
						fileReplaced++
					} else {
						if cfg.Verbose {
							fmt.Println(cfg.TR.T(messages.Keys.App.Add.KeyExistsReplace, key))
						}
						if isDefaultLang {
							translations[key] = value
						} else {
							translations[key] = cfg.TR.T(messages.Keys.App.Ast.TodoPrefix, value)
						}
						fileReplaced++
					}
				}
			} else {
				displayValue := value
				actualValue := value
				if !isDefaultLang {
					actualValue = cfg.TR.T(messages.Keys.App.Ast.TodoPrefix, value)
					displayValue = actualValue
				}

				if cfg.Add.DryRun {
					fmt.Printf("  %s = %s\n", key, displayValue)
					fileAdded++
				} else {
					if isDefaultLang {
						if cfg.Verbose {
							fmt.Println(cfg.TR.T(messages.Keys.App.Add.AddingKey, key, displayValue))
						}
					} else {
						if cfg.Verbose {
							fmt.Println(cfg.TR.T(messages.Keys.App.Add.AddingKeyTodo, key, value))
						}
					}
					translations[key] = actualValue
					fileAdded++
				}
			}
		}

		// Write updated translations
		if !cfg.Add.DryRun && (fileAdded > 0 || fileReplaced > 0) {
			// Marshal with sorted keys
			sortedData, err := json.MarshalIndent(translations, "", "  ")
			if err != nil {
				return errors.ErrFailedToMarshal.WithArgs(err)
			}

			if err := os.WriteFile(inputFile, sortedData, 0644); err != nil {
				return errors.ErrFailedToWriteJson.WithArgs(inputFile, err)
			}

			fmt.Println(cfg.TR.T(messages.Keys.App.Add.UpdatedFile, inputFile, fileAdded, fileSkipped, fileReplaced))
		}

		totalAdded += fileAdded
		totalSkipped += fileSkipped
		totalReplaced += fileReplaced
		filesProcessed++

		if cfg.Verbose || cfg.Add.DryRun {
			fmt.Println()
		}
	}

	// Print summary
	fmt.Println()
	if cfg.Add.DryRun {
		fmt.Println(cfg.TR.T(messages.Keys.App.Add.DryRunSummary, totalAdded, totalSkipped, totalReplaced, filesProcessed))
	} else {
		fmt.Println(cfg.TR.T(messages.Keys.App.Add.Summary, totalAdded, totalSkipped, totalReplaced, filesProcessed))
	}

	return nil
}
