package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/util"
)

// Sync is the command handler for the sync command
func Sync(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return errors.ErrFailedToGetConfig
	}
	return ExecuteSyncCommand(cfg, &cfg.Sync)
}

// ExecuteSyncCommand synchronizes translation keys across locale files
func ExecuteSyncCommand(cfg *options.AppConfig, cmd *options.SyncCmd) error {
	// If no target files specified, sync all input files with each other
	if len(cmd.Target) == 0 {
		return syncWithinFiles(cfg, cmd)
	}

	// Otherwise, sync target files against reference files
	return syncTargetFiles(cfg, cmd)
}

// syncWithinFiles syncs all input files to have the same keys (original behavior)
func syncWithinFiles(cfg *options.AppConfig, cmd *options.SyncCmd) error {
	// Expand wildcards in input files
	files, err := util.ExpandGlobPatterns(cfg.Input)
	if err != nil {
		return errors.ErrFailedToExpandFilePatterns.WithArgs(err)
	}

	if len(files) < 2 {
		return errors.ErrSyncRequiresAtLeastTwoFiles.WithArgs(len(files))
	}

	// Use the first file as base
	baseFile := files[0]

	// Load all locale files
	locales := make(map[string]map[string]interface{})
	for _, file := range files {
		data, err := loadJSONFile(file)
		if err != nil {
			return errors.ErrFailedToLoadFile.WithArgs(file, err)
		}
		locales[file] = data
	}

	// Get base locale data
	baseData, exists := locales[baseFile]
	if !exists {
		return errors.ErrBaseFileNotFound.WithArgs(baseFile)
	}

	// Get all keys from base file
	baseKeys := getAllKeys(baseData)

	// Process sync, skipping the base file
	return processSyncWithinFiles(cfg, cmd, baseFile, baseKeys, baseData, locales)
}

// syncTargetFiles syncs target files against reference files
func syncTargetFiles(cfg *options.AppConfig, cmd *options.SyncCmd) error {
	// Expand wildcards in reference files
	refFiles, err := util.ExpandGlobPatterns(cfg.Input)
	if err != nil {
		return errors.ErrFailedToExpandReferencePatterns.WithArgs(err)
	}

	if len(refFiles) == 0 {
		return errors.ErrNoReferenceFiles
	}

	// Expand wildcards in target files
	targetFiles, err := util.ExpandGlobPatterns(cmd.Target)
	if err != nil {
		return errors.ErrFailedToExpandTargetPatterns.WithArgs(err)
	}

	if len(targetFiles) == 0 {
		return errors.ErrNoTargetFiles
	}

	// Load all reference files and merge their keys
	allRefKeys := make(map[string]interface{})
	refData := make(map[string]map[string]interface{})

	for _, file := range refFiles {
		data, err := loadJSONFile(file)
		if err != nil {
			return errors.ErrFailedToLoadReferenceFile.WithArgs(file, err)
		}
		refData[file] = data

		// Merge keys from this file
		mergeKeys(allRefKeys, data)
	}

	// Get all unique keys from reference files
	refKeys := getAllKeys(allRefKeys)

	// Load target files
	targetData := make(map[string]map[string]interface{})
	for _, file := range targetFiles {
		data, err := loadJSONFile(file)
		if err != nil {
			return errors.ErrFailedToLoadTargetFile.WithArgs(file, err)
		}
		targetData[file] = data
	}

	// For each target file, find a matching reference file by language
	baseDataMap := make(map[string]map[string]interface{})
	for targetFile := range targetData {
		targetLang := extractLanguageFromPath(targetFile)

		// Find matching reference file
		var matchingRef string
		var englishRef string
		for refFile := range refData {
			refLang := extractLanguageFromPath(refFile)
			if refLang == targetLang {
				matchingRef = refFile
				baseDataMap[targetFile] = refData[refFile]
				break
			}
			if refLang == "en" {
				englishRef = refFile
			}
		}

		// If no matching language found, use English if available, otherwise first file
		if matchingRef == "" {
			if englishRef != "" {
				baseDataMap[targetFile] = refData[englishRef]
			} else if len(refFiles) > 0 {
				baseDataMap[targetFile] = refData[refFiles[0]]
			}
		}
	}

	return processSyncTargetFiles(cfg, cmd, refKeys, allRefKeys, targetData, baseDataMap)
}

// processSyncWithinFiles handles synchronization between input files
func processSyncWithinFiles(cfg *options.AppConfig, cmd *options.SyncCmd, baseFile string,
	baseKeys []string, baseData map[string]interface{}, locales map[string]map[string]interface{}) error {

	// Track changes
	var changes []string
	totalAdded := 0
	totalRemoved := 0

	// Process each locale file
	for file, data := range locales {
		// Skip the base file to avoid bidirectional sync
		if file == baseFile {
			continue
		}

		currentKeys := getAllKeys(data)

		// Find missing keys
		missingKeys := findMissingKeys(baseKeys, currentKeys)

		// Find extra keys
		extraKeys := findMissingKeys(currentKeys, baseKeys)

		if len(missingKeys) == 0 && len(extraKeys) == 0 {
			if cfg.Verbose {
				changes = append(changes, fmt.Sprintf("%s: already in sync", filepath.Base(file)))
			}
			continue
		}

		if cmd.DryRun {
			// Report what would be changed
			if len(missingKeys) > 0 {
				changes = append(changes, fmt.Sprintf("\n%s: would add %d missing keys:", filepath.Base(file), len(missingKeys)))
				for _, key := range missingKeys {
					changes = append(changes, fmt.Sprintf("  + %s", key))
				}
			}
			if len(extraKeys) > 0 && cmd.RemoveExtra {
				changes = append(changes, fmt.Sprintf("\n%s: would remove %d extra keys:", filepath.Base(file), len(extraKeys)))
				for _, key := range extraKeys {
					changes = append(changes, fmt.Sprintf("  - %s", key))
				}
			}
		} else {
			// Make actual changes
			modified := false

			// Add missing keys
			for _, key := range missingKeys {
				value := getValueFromPath(baseData, key)
				// Add prefix for non-matching languages
				if cmd.TodoPrefix != "" {
					lang := extractLanguageFromPath(file)
					baseLang := extractLanguageFromPath(baseFile)
					if lang != baseLang && lang != "en" {
						value = fmt.Sprintf("%s %v", cmd.TodoPrefix, value)
					}
				}
				setValueAtPath(data, key, value)
				modified = true
				totalAdded++
			}

			// Remove extra keys if requested
			if cmd.RemoveExtra {
				for _, key := range extraKeys {
					removeKeyFromPath(data, key)
					modified = true
					totalRemoved++
				}
			}

			// Save file if modified
			if modified {
				if err := saveJSONFile(file, data); err != nil {
					return errors.ErrFailedToSaveFile.WithArgs(file, err)
				}

				msg := fmt.Sprintf("%s: ", filepath.Base(file))
				if len(missingKeys) > 0 {
					msg += fmt.Sprintf("added %d keys", len(missingKeys))
				}
				if len(extraKeys) > 0 && cmd.RemoveExtra {
					if len(missingKeys) > 0 {
						msg += ", "
					}
					msg += fmt.Sprintf("removed %d keys", len(extraKeys))
				}
				changes = append(changes, msg)
			}
		}
	}

	// Print summary
	if len(changes) > 0 {
		fmt.Println("Synchronization summary:")
		for _, change := range changes {
			fmt.Println(change)
		}

		if !cmd.DryRun && (totalAdded > 0 || totalRemoved > 0) {
			fmt.Printf("\nTotal: %d keys added, %d keys removed\n", totalAdded, totalRemoved)
		}
	} else {
		fmt.Println("✓ All locale files are in sync")
	}

	return nil
}

// processSyncTargetFiles handles synchronization of target files against reference files
func processSyncTargetFiles(cfg *options.AppConfig, cmd *options.SyncCmd, refKeys []string,
	allRefKeys map[string]interface{}, targetFiles map[string]map[string]interface{},
	baseDataMap map[string]map[string]interface{}) error {

	// Track changes
	var changes []string
	totalAdded := 0
	totalRemoved := 0

	// Process each target file
	for file, data := range targetFiles {
		currentKeys := getAllKeys(data)

		// Find missing keys
		missingKeys := findMissingKeys(refKeys, currentKeys)

		// Find extra keys
		extraKeys := findMissingKeys(currentKeys, refKeys)

		if len(missingKeys) == 0 && len(extraKeys) == 0 {
			if cfg.Verbose {
				changes = append(changes, fmt.Sprintf("%s: already in sync", filepath.Base(file)))
			}
			continue
		}

		if cmd.DryRun {
			// Report what would be changed
			if len(missingKeys) > 0 {
				changes = append(changes, fmt.Sprintf("\n%s: would add %d missing keys:", filepath.Base(file), len(missingKeys)))
				for _, key := range missingKeys {
					changes = append(changes, fmt.Sprintf("  + %s", key))
				}
			}
			if len(extraKeys) > 0 && cmd.RemoveExtra {
				changes = append(changes, fmt.Sprintf("\n%s: would remove %d extra keys:", filepath.Base(file), len(extraKeys)))
				for _, key := range extraKeys {
					changes = append(changes, fmt.Sprintf("  - %s", key))
				}
			}
		} else {
			// Make actual changes
			modified := false

			// Use the matched base data for this target file, or fallback to allRefKeys
			sourceData := allRefKeys
			if baseData, exists := baseDataMap[file]; exists && baseData != nil {
				sourceData = baseData
			}

			// Add missing keys
			for _, key := range missingKeys {
				value := getValueFromPath(sourceData, key)
				// Add prefix for non-matching languages
				if cmd.TodoPrefix != "" {
					lang := extractLanguageFromPath(file)
					if lang != "en" {
						value = fmt.Sprintf("%s %v", cmd.TodoPrefix, value)
					}
				}
				setValueAtPath(data, key, value)
				modified = true
				totalAdded++
			}

			// Remove extra keys if requested
			if cmd.RemoveExtra {
				for _, key := range extraKeys {
					removeKeyFromPath(data, key)
					modified = true
					totalRemoved++
				}
			}

			// Save file if modified
			if modified {
				if err := saveJSONFile(file, data); err != nil {
					return errors.ErrFailedToSaveFile.WithArgs(file, err)
				}

				msg := fmt.Sprintf("%s: ", filepath.Base(file))
				if len(missingKeys) > 0 {
					msg += fmt.Sprintf("added %d keys", len(missingKeys))
				}
				if len(extraKeys) > 0 && cmd.RemoveExtra {
					if len(missingKeys) > 0 {
						msg += ", "
					}
					msg += fmt.Sprintf("removed %d keys", len(extraKeys))
				}
				changes = append(changes, msg)
			}
		}
	}

	// Print summary
	if len(changes) > 0 {
		fmt.Println("Synchronization summary:")
		for _, change := range changes {
			fmt.Println(change)
		}

		if !cmd.DryRun && (totalAdded > 0 || totalRemoved > 0) {
			fmt.Printf("\nTotal: %d keys added, %d keys removed\n", totalAdded, totalRemoved)
		}
	} else {
		fmt.Println("✓ All locale files are in sync")
	}

	return nil
}

// mergeKeys merges keys from source into dest
func mergeKeys(dest, source map[string]interface{}) {
	for k, v := range source {
		if destMap, isMap := dest[k].(map[string]interface{}); isMap {
			if srcMap, isSrcMap := v.(map[string]interface{}); isSrcMap {
				mergeKeys(destMap, srcMap)
			} else {
				dest[k] = v
			}
		} else {
			dest[k] = v
		}
	}
}

// getAllKeys gets all keys from a map (handles both flat and nested structures)
func getAllKeys(data map[string]interface{}) []string {
	var keys []string

	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

// findMissingKeys returns keys that are in source but not in target
func findMissingKeys(source, target []string) []string {
	targetMap := make(map[string]bool)
	for _, k := range target {
		targetMap[k] = true
	}

	var missing []string
	for _, k := range source {
		if !targetMap[k] {
			missing = append(missing, k)
		}
	}
	return missing
}

// getValueFromPath gets a value from map using dot notation as key
func getValueFromPath(data map[string]interface{}, path string) interface{} {
	// For JSON translation files, keys are stored flat with dots, not nested
	return data[path]
}

// setValueAtPath sets a value in map using dot notation as key
func setValueAtPath(data map[string]interface{}, path string, value interface{}) {
	// For JSON translation files, keys are stored flat with dots, not nested
	data[path] = value
}

// removeKeyFromPath removes a key from map using dot notation as key
func removeKeyFromPath(data map[string]interface{}, path string) {
	// For JSON translation files, keys are stored flat with dots, not nested
	delete(data, path)
}

// extractLanguageFromPath extracts language code from file path
func extractLanguageFromPath(path string) string {
	base := filepath.Base(path)
	lang := strings.TrimSuffix(base, ".json")
	return lang
}

// loadJSONFile loads a JSON file into a map
func loadJSONFile(filename string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// saveJSONFile saves a map to a JSON file
func saveJSONFile(filename string, data map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, jsonData, 0644)
}
