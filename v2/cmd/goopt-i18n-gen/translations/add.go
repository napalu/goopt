package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
)

// Add adds new translation keys to locale files
func Add(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToGetConfig))
	}

	// Validate inputs
	keysToAdd := make(map[string]string)

	// Check if we have keys to add
	hasFromFile := cfg.Add.FromFile != ""
	hasSingleKey := cfg.Add.Key != ""

	if !hasFromFile && !hasSingleKey {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.NoKeys))
	}

	if hasFromFile && hasSingleKey {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.BothSingleAndFile))
	}

	if hasSingleKey && cfg.Add.Value == "" {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.MissingValue))
	}

	// Validate mode
	validModes := map[string]bool{"skip": true, "replace": true, "error": true}
	if !validModes[cfg.Add.Mode] {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.InvalidMode), cfg.Add.Mode)
	}

	// Load keys
	if hasFromFile {
		if cfg.Verbose {
			fmt.Println(cfg.TR.T(messages.Keys.AppAdd.ReadingKeysFile, cfg.Add.FromFile))
		}

		data, err := os.ReadFile(cfg.Add.FromFile)
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.FailedReadKeysFile), cfg.Add.FromFile, err)
		}

		if err := json.Unmarshal(data, &keysToAdd); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.FailedParseKeysFile), cfg.Add.FromFile, err)
		}

		if cfg.Verbose {
			fmt.Println(cfg.TR.T(messages.Keys.AppAdd.FoundKeysInFile, len(keysToAdd), cfg.Add.FromFile))
			fmt.Println()
		}
	} else {
		keysToAdd[cfg.Add.Key] = cfg.Add.Value
	}

	// Expand input files
	inputFiles, err := expandInputFiles(cfg.Input)
	if err != nil {
		return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToExpandInput), err)
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

		fmt.Println(cfg.TR.T(messages.Keys.AppAdd.ProcessingLocale, inputFile))

		// Read existing translations
		var translations map[string]string
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToReadInput), inputFile, err)
		}

		if err := json.Unmarshal(data, &translations); err != nil {
			translations = make(map[string]string)
		}

		// Process keys
		fileAdded := 0
		fileSkipped := 0
		fileReplaced := 0

		if cfg.Add.DryRun {
			fmt.Println(cfg.TR.T(messages.Keys.AppAdd.DryRunWouldAdd, inputFile))
		}

		for key, value := range keysToAdd {
			existingValue, exists := translations[key]

			if exists {
				switch cfg.Add.Mode {
				case "error":
					return fmt.Errorf(cfg.TR.T(messages.Keys.AppAdd.KeyExistsError), key, inputFile)
				case "skip":
					if cfg.Add.DryRun {
						fmt.Println(cfg.TR.T(messages.Keys.AppAdd.DryRunWouldSkip, key))
						fileSkipped++
					} else if cfg.Verbose {
						fmt.Println(cfg.TR.T(messages.Keys.AppAdd.KeyExistsSkip, key))
						fileSkipped++
					} else {
						fileSkipped++
					}
				case "replace":
					if cfg.Add.DryRun {
						fmt.Printf(cfg.TR.T(messages.Keys.AppAdd.DryRunWouldReplace, key))
						displayValue := value
						if !isDefaultLang {
							displayValue = cfg.TR.T(messages.Keys.AppAst.TodoPrefix, value)
						}
						fmt.Printf(" %s -> %s\n", existingValue, displayValue)
						fileReplaced++
					} else {
						if cfg.Verbose {
							fmt.Println(cfg.TR.T(messages.Keys.AppAdd.KeyExistsReplace, key))
						}
						if isDefaultLang {
							translations[key] = value
						} else {
							translations[key] = cfg.TR.T(messages.Keys.AppAst.TodoPrefix, value)
						}
						fileReplaced++
					}
				}
			} else {
				displayValue := value
				actualValue := value
				if !isDefaultLang {
					actualValue = cfg.TR.T(messages.Keys.AppAst.TodoPrefix, value)
					displayValue = actualValue
				}

				if cfg.Add.DryRun {
					fmt.Printf("  %s = %s\n", key, displayValue)
					fileAdded++
				} else {
					if isDefaultLang {
						if cfg.Verbose {
							fmt.Println(cfg.TR.T(messages.Keys.AppAdd.AddingKey, key, displayValue))
						}
					} else {
						if cfg.Verbose {
							fmt.Println(cfg.TR.T(messages.Keys.AppAdd.AddingKeyTodo, key, value))
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
				return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToMarshal), err)
			}

			if err := os.WriteFile(inputFile, sortedData, 0644); err != nil {
				return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToWriteJson), inputFile, err)
			}

			fmt.Println(cfg.TR.T(messages.Keys.AppAdd.UpdatedFile, inputFile, fileAdded, fileSkipped, fileReplaced))
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
		fmt.Println(cfg.TR.T(messages.Keys.AppAdd.DryRunSummary, totalAdded, totalSkipped, totalReplaced, filesProcessed))
	} else {
		fmt.Println(cfg.TR.T(messages.Keys.AppAdd.Summary, totalAdded, totalSkipped, totalReplaced, filesProcessed))
	}

	return nil
}
