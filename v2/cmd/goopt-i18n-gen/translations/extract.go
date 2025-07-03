package translations

import (
	"fmt"
	"sort"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/ast"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/constants"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/errors"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/util"
)

// Extract handles string extraction from go files and supports 2 modes: Comment-based extraction or code transformation
func Extract(parser *goopt.Parser, _ *goopt.Command) error {
	config, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return errors.ErrFailedToGetConfig
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
		return errors.ErrInvalidRegex.WithArgs(err.Error())
	}

	// Find and process files
	fmt.Println(tr.T(messages.Keys.App.Extract.ScanningFiles))

	// Handle glob patterns using our utility that supports **
	patterns := strings.Split(extractCmd.Files, ",")
	filesToProcess, err := util.ExpandGlobPatterns(patterns)
	if err != nil {
		return errors.ErrGlobError.WithArgs(extractCmd.Files, err.Error())
	}

	// Extract strings from all files
	fileCount := 0
	for _, file := range filesToProcess {
		if err := extractor.ExtractFromFiles(file); err != nil {
			if config.Verbose {
				fmt.Printf(tr.T(messages.Keys.App.Extract.FileError, file, err.Error()))
			}
			continue
		}
		fileCount++
	}

	// Get extracted strings
	extractedStrings := extractor.GetExtractedStrings()

	if len(extractedStrings) == 0 {
		fmt.Println(tr.T(messages.Keys.App.Extract.NoStringsFound))
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

	fmt.Printf(tr.T(messages.Keys.App.Extract.FoundStrings, totalOccurrences, fileCount))
	fmt.Printf(tr.T(messages.Keys.App.Extract.UniqueStrings, len(extractedStrings)))

	if extractCmd.DryRun {
		fmt.Println("\n" + tr.T(messages.Keys.App.Extract.DryRunMode))
	}

	// Prepare translations map
	translations := make(map[string]string)
	for _, str := range sortedStrings {
		key := generateKey(extractCmd.KeyPrefix, str)
		translations[key] = str

		if config.Verbose || extractCmd.DryRun {
			data := extractedStrings[str]
			fmt.Printf("\n%s (%d %s)\n", str, len(data.Locations), tr.T(messages.Keys.App.Extract.Occurrences))
			fmt.Printf("  → %s: %s\n", tr.T(messages.Keys.App.Extract.Key), key)

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
		fmt.Println("\n" + tr.T(messages.Keys.App.Extract.UpdatingFiles))

		// Expand input files, creating them if necessary
		inputFiles, err := expandInputFiles(config.Input)
		if err != nil {
			return errors.ErrFailedToExpandInput.WithArgs(err)
		}

		updatedCount := 0
		for _, file := range inputFiles {
			// Ensure the file exists
			if err := ensureInputFile(file); err != nil {
				return errors.ErrFailedToPrepareInput.WithArgs(file, err)
			}

			opts := TranslationUpdateOptions{
				Mode:       UpdateModeSkip,
				DryRun:     false,
				TodoPrefix: constants.DefaultTODOPrefix,
			}
			result, err := UpdateTranslationFile(file, translations, opts)
			if err != nil {
				return errors.ErrUpdateError.WithArgs(file, err.Error())
			}
			if result.Modified {
				fmt.Printf("✓ %s %s\n", tr.T(messages.Keys.App.Add.Updated), file)
				updatedCount++
			}
		}

		if updatedCount > 0 {
			fmt.Printf("\n✓ %s %d %s\n", tr.T(messages.Keys.App.Extract.Added), len(translations), tr.T(messages.Keys.App.Extract.Keys))
		}
	}

	// Handle auto-update mode or comment addition
	if extractCmd.AutoUpdate {
		return handleAutoUpdate(config, translations, filesToProcess, extractCmd.DryRun)
	} else if !extractCmd.DryRun {
		// When not in auto-update mode and not dry-run, add comments to source files
		return addCommentsToFiles(config, translations, filesToProcess)
	}

	return nil
}

func addCommentsToFiles(config *options.AppConfig, translations map[string]string, files []string) error {
	tr := config.TR
	extractCmd := config.Extract

	fmt.Println("adding comments to files")

	// Resolve package path based on module context
	packagePath, err := resolvePackagePath(extractCmd.Package, ".")
	if err != nil {
		// If we can't resolve, use the package name as-is
		packagePath = extractCmd.Package
	}
	trConfig := util.ToTransformConfig(extractCmd)
	trConfig.IsUpdateMode = false
	trConfig.PackagePath = packagePath
	trConfig.Translator = config.TR

	replacer := ast.NewTransformationReplacer(trConfig)

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

	// Get replacements for reporting
	replacements := replacer.GetReplacements()
	if len(replacements) == 0 {
		fmt.Println(tr.T(messages.Keys.App.Extract.NoReplacements))
		return nil
	}

	fmt.Printf(tr.T(messages.Keys.App.Extract.FoundComments, len(replacements)))

	if config.Verbose {
		for _, r := range replacements {
			fmt.Printf("  %s:%d: %s\n", r.File, r.Pos, r.Replacement)
		}
	}

	// Apply replacements (add comments)
	fmt.Println("\n" + tr.T(messages.Keys.App.Extract.ApplyingReplacements))
	if err := replacer.ApplyReplacements(); err != nil {
		return err
	}

	fmt.Printf("✓ %s\n", tr.T(messages.Keys.App.Extract.UpdateComplete))
	if extractCmd.BackupDir != "" {
		fmt.Printf(tr.T(messages.Keys.App.Extract.BackupLocation, extractCmd.BackupDir))
	}

	return nil
}

// generateKey generates a translation key from a string value
func generateKey(prefix, value string) string {
	return util.GenerateKeyFromString(prefix, value)
}

// handleAutoUpdate handles the auto-update functionality
func handleAutoUpdate(config *options.AppConfig, translations map[string]string, files []string, dryRun bool) error {
	tr := config.TR
	extractCmd := config.Extract

	fmt.Println("\n" + tr.T(messages.Keys.App.Extract.AutoUpdateMode))

	// Create a string replacer
	// Use TransformationReplacer for both comment mode and direct replacement mode
	var replacer interface {
		SetKeyMap(map[string]string)
		ProcessFiles([]string) error
		GetReplacements() []ast.Replacement
		ApplyReplacements() error
	}

	// Resolve package path based on module context
	packagePath, err := resolvePackagePath(extractCmd.Package, ".")
	if err != nil {
		// If we can't resolve, use the package name as-is
		packagePath = extractCmd.Package
	}

	trConfig := util.ToTransformConfig(extractCmd)
	trConfig.PackagePath = packagePath
	trConfig.Translator = config.TR
	replacer = ast.NewTransformationReplacer(trConfig)

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

	// Handle different modes
	if extractCmd.AutoUpdate {
		// For direct transformation mode
		// Check if we have strings to transform
		if len(translations) == 0 {
			fmt.Println(tr.T(messages.Keys.App.Extract.NoReplacements))
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
			fmt.Println(tr.T(messages.Keys.App.Extract.NoReplacements))
			return nil
		}

		fmt.Printf(tr.T(messages.Keys.App.Extract.FoundComments, len(replacements)))

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
	fmt.Println("\n" + tr.T(messages.Keys.App.Extract.ApplyingReplacements))
	if err := replacer.ApplyReplacements(); err != nil {
		return err
	}

	// If keepComments is false and we're in update mode, clean up i18n comments after transformation
	if extractCmd.AutoUpdate && !extractCmd.KeepComments {
		// Create a new replacer just for cleaning comments
		// func NewTransformationReplacer(tr i18n.Translator, trPattern string, keepComments, cleanComments, isUpdateMode bool, backupDir, packagePath string) *TransformationReplacer {
		trConfig := util.ToTransformConfig(extractCmd)
		trConfig.KeepComments = false
		trConfig.CleanComments = true
		trConfig.IsUpdateMode = false
		trConfig.PackagePath = packagePath
		trConfig.Translator = config.TR
		cleanReplacer := ast.NewTransformationReplacer(trConfig)
		if err := cleanReplacer.ProcessFiles(files); err != nil {
			return err
		}
		if err := cleanReplacer.ApplyReplacements(); err != nil {
			return err
		}
	}

	fmt.Printf("✓ %s\n", tr.T(messages.Keys.App.Extract.UpdateComplete))
	if extractCmd.BackupDir != "" {
		fmt.Printf(tr.T(messages.Keys.App.Extract.BackupLocation, extractCmd.BackupDir))
	}

	return nil
}

func cleanI18nComments(config *options.AppConfig) error {
	tr := config.TR
	extractCmd := config.Extract

	fmt.Println(tr.T(messages.Keys.App.Extract.CleaningComments))

	// Find files to process using our utility that supports **
	patterns := strings.Split(extractCmd.Files, ",")
	filesToProcess, err := util.ExpandGlobPatterns(patterns)
	if err != nil {
		return errors.ErrGlobError.WithArgs(extractCmd.Files, err.Error())
	}

	// Resolve package path based on module context
	packagePath, err := resolvePackagePath(extractCmd.Package, ".")
	if err != nil {
		// If we can't resolve, use the package name as-is
		packagePath = extractCmd.Package
	}

	trConfig := util.ToTransformConfig(extractCmd)
	trConfig.PackagePath = packagePath
	trConfig.Translator = config.TR
	replacer := ast.NewTransformationReplacer(trConfig)

	if err := replacer.ProcessFiles(filesToProcess); err != nil {
		return err
	}

	replacements := replacer.GetReplacements()
	if len(replacements) == 0 {
		fmt.Println(tr.T(messages.Keys.App.Extract.NoCommentsFound))
		return nil
	}

	fmt.Printf(tr.T(messages.Keys.App.Extract.FoundCommentsToClean, len(replacements)))

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

	fmt.Printf("✓ %s\n", tr.T(messages.Keys.App.Extract.CleanComplete))
	return nil
}
