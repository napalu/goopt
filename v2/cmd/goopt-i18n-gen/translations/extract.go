package translations

import (
	"encoding/json"
	"fmt"
	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/ast"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/i18n"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Extract handles string extraction from go files and supports 2 modes: Comment-based extraction or code transformation
func Extract(parser *goopt.Parser, _ *goopt.Command) error {
	config, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}
	tr := config.TR
	extractCmd := config.Extract

	// Handle clean comments mode
	if extractCmd.CleanComments {
		return cleanI18nComments(config)
	}

	// Create string extractor
	extractor, err := ast.NewStringExtractor(tr, extractCmd.MatchOnly, extractCmd.SkipMatch, extractCmd.MinLength)
	if err != nil {
		return fmt.Errorf(tr.T(messages.Keys.AppExtract.InvalidRegex, err.Error()))
	}

	// Find and process files
	fmt.Println(tr.T(messages.Keys.AppExtract.ScanningFiles))

	// Handle glob patterns
	var filesToProcess []string
	patterns := strings.Split(extractCmd.Files, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf(tr.T(messages.Keys.AppExtract.GlobError, pattern, err.Error()))
		}
		filesToProcess = append(filesToProcess, matches...)
	}

	// Extract strings from all files
	fileCount := 0
	for _, file := range filesToProcess {
		if err := extractor.ExtractFromFiles(file); err != nil {
			if config.Verbose {
				fmt.Printf(tr.T(messages.Keys.AppExtract.FileError, file, err.Error()))
			}
			continue
		}
		fileCount++
	}

	// Get extracted strings
	extractedStrings := extractor.GetExtractedStrings()

	if len(extractedStrings) == 0 {
		fmt.Println(tr.T(messages.Keys.AppExtract.NoStringsFound))
		return nil
	}

	// Sort strings for consistent output
	var sortedStrings []string
	for str := range extractedStrings {
		sortedStrings = append(sortedStrings, str)
	}
	sort.Strings(sortedStrings)

	// Show summary
	totalOccurrences := 0
	for _, data := range extractedStrings {
		totalOccurrences += len(data.Locations)
	}

	fmt.Printf(tr.T(messages.Keys.AppExtract.FoundStrings, totalOccurrences, fileCount))
	fmt.Printf(tr.T(messages.Keys.AppExtract.UniqueStrings, len(extractedStrings)))

	if extractCmd.DryRun {
		fmt.Println("\n" + tr.T(messages.Keys.AppExtract.DryRunMode))
	}

	// Prepare translations map
	translations := make(map[string]string)
	for _, str := range sortedStrings {
		key := generateKey(extractCmd.KeyPrefix, str)
		translations[key] = str

		if config.Verbose || extractCmd.DryRun {
			data := extractedStrings[str]
			fmt.Printf("\n%s (%d %s)\n", str, len(data.Locations), tr.T(messages.Keys.AppExtract.Occurrences))
			fmt.Printf("  → %s: %s\n", tr.T(messages.Keys.AppExtract.Key), key)

			if config.Verbose {
				for _, loc := range data.Locations {
					fmt.Printf("    - %s:%d (in function %s)\n", loc.File, loc.Line, loc.Function)
				}
			}
		}
	}

	// If dry run and not auto-update, stop here
	if extractCmd.DryRun && !extractCmd.AutoUpdate {
		return nil
	}

	// If not dry run, update locale files
	if !extractCmd.DryRun {
		fmt.Println("\n" + tr.T(messages.Keys.AppExtract.UpdatingFiles))

		for _, inputFile := range config.Input {
			files, err := filepath.Glob(inputFile)
			if err != nil {
				return fmt.Errorf(tr.T(messages.Keys.AppAdd.GlobError, inputFile, err.Error()))
			}

			for _, file := range files {
				if err := addTranslationsToFile(file, translations, tr); err != nil {
					return fmt.Errorf(tr.T(messages.Keys.AppExtract.UpdateError, file, err.Error()))
				}
				fmt.Printf("✓ %s %s\n", tr.T(messages.Keys.AppAdd.Updated), file)
			}
		}

		fmt.Printf("\n✓ %s %d %s\n", tr.T(messages.Keys.AppExtract.Added), len(translations), tr.T(messages.Keys.AppExtract.Keys))
	}

	// Handle auto-update mode
	if extractCmd.AutoUpdate {
		return handleAutoUpdate(config, translations, filesToProcess, extractCmd.DryRun)
	}

	return nil
}

// generateKey generates a translation key from a string value
func generateKey(prefix, value string) string {
	key := regexp.MustCompile(`[^\w\s]+`).ReplaceAllString(value, " ")
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)

	// Convert to snake_case
	key = strcase.ToSnake(key)

	// Limit length
	if len(key) > 50 {
		key = key[:50]
	}

	// Remove trailing underscores
	key = strings.TrimRight(key, "_")

	return fmt.Sprintf("%s.%s", prefix, key)
}

// addTranslationsToFile adds translations to a locale file (reused from add command)
func addTranslationsToFile(filename string, translations map[string]string, tr i18n.Translator) error {
	// Read existing content
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(filename); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return err
		}
	}

	// Detect language from filename
	isEnglish := strings.Contains(strings.ToLower(filename), "en.json") ||
		strings.Contains(strings.ToLower(filename), "english")

	// Add new translations
	added := 0
	for key, value := range translations {
		if _, exists := existing[key]; !exists {
			if !isEnglish {
				value = fmt.Sprintf("[TODO] %s", value)
			}
			existing[key] = value
			added++
		}
	}

	if added == 0 {
		return nil
	}

	// Write back
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// handleAutoUpdate handles the auto-update functionality
func handleAutoUpdate(config *options.AppConfig, translations map[string]string, files []string, dryRun bool) error {
	tr := config.TR
	extractCmd := config.Extract

	fmt.Println("\n" + tr.T(messages.Keys.AppExtract.AutoUpdateMode))

	// Create a string replacer
	// Use TransformationReplacer (AST-based) replacer for direct replacement mode, CommentReplacer for comments
	var replacer interface {
		SetKeyMap(map[string]string)
		ProcessFiles([]string) error
		GetReplacements() []ast.Replacement
		ApplyReplacements() error
	}

	if extractCmd.TrPattern != "" {
		// Use AST-based replacer for format function handling
		replacer = ast.NewTransformationReplacer(tr, extractCmd.TrPattern, extractCmd.KeepComments, false, extractCmd.BackupDir)
	} else {
		// Use simple replacer for comment mode
		replacer = ast.NewCommentReplacer(tr, extractCmd.TrPattern, extractCmd.KeepComments, false, extractCmd.BackupDir)
	}

	// Build key map (reverse of translations map)
	keyMap := make(map[string]string)
	for key, value := range translations {
		keyMap[value] = key
	}
	replacer.SetKeyMap(keyMap)

	// Process files to find replacements
	if err := replacer.ProcessFiles(files); err != nil {
		return err
	}

	// For AST-based transformation mode, we handle differently
	if extractCmd.TrPattern != "" {
		// Check if we have strings to transform
		if len(translations) == 0 {
			fmt.Println(tr.T(messages.Keys.AppExtract.NoReplacements))
			return nil
		}

		fmt.Printf("Found %d strings to replace\n", len(translations))

		// If dry run, stop here for AST mode
		if dryRun {
			return nil
		}
	} else {
		// For comment mode, use the regular flow
		replacements := replacer.GetReplacements()
		if len(replacements) == 0 {
			fmt.Println(tr.T(messages.Keys.AppExtract.NoReplacements))
			return nil
		}

		fmt.Printf(tr.T(messages.Keys.AppExtract.FoundComments, len(replacements)))

		if config.Verbose || dryRun {
			for _, r := range replacements {
				fmt.Printf("  %s:%d: %s\n", r.File, r.Pos, r.Replacement)
			}
		}

		// If dry run, stop here
		if dryRun {
			return nil
		}
	}

	// Apply replacements
	fmt.Println("\n" + tr.T(messages.Keys.AppExtract.ApplyingReplacements))
	if err := replacer.ApplyReplacements(); err != nil {
		return err
	}

	fmt.Printf("✓ %s\n", tr.T(messages.Keys.AppExtract.UpdateComplete))
	if extractCmd.BackupDir != "" {
		fmt.Printf(tr.T(messages.Keys.AppExtract.BackupLocation, extractCmd.BackupDir))
	}

	return nil
}

// cleanI18nComments removes all i18n-* comments from files
func cleanI18nComments(config *options.AppConfig) error {
	tr := config.TR
	extractCmd := config.Extract

	fmt.Println(tr.T(messages.Keys.AppExtract.CleaningComments))

	// Find files to process
	var filesToProcess []string
	patterns := strings.Split(extractCmd.Files, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf(tr.T(messages.Keys.AppExtract.GlobError, pattern, err.Error()))
		}
		filesToProcess = append(filesToProcess, matches...)
	}

	// Create replacer in clean mode
	replacer := ast.NewCommentReplacer(tr, "", false, true, extractCmd.BackupDir)

	if err := replacer.ProcessFiles(filesToProcess); err != nil {
		return err
	}

	replacements := replacer.GetReplacements()
	if len(replacements) == 0 {
		fmt.Println(tr.T(messages.Keys.AppExtract.NoCommentsFound))
		return nil
	}

	fmt.Printf(tr.T(messages.Keys.AppExtract.FoundCommentsToClean, len(replacements)))

	if extractCmd.DryRun {
		for _, r := range replacements {
			fmt.Printf("  %s:%d: %s\n", r.File, r.Pos, r.Original)
		}
		return nil
	}

	// Apply cleaning
	if err := replacer.ApplyReplacements(); err != nil {
		return err
	}

	fmt.Printf("✓ %s\n", tr.T(messages.Keys.AppExtract.CleanComplete))
	return nil
}
