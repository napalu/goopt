package goopt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/internal/messages"
	"github.com/napalu/goopt/v2/internal/util"
	"golang.org/x/text/language"
)

// HelpMode defines different help query modes
type HelpMode int

const (
	HelpModeDefault  HelpMode = iota
	HelpModeGlobals           // Show only global flags
	HelpModeCommands          // Show only commands
	HelpModeFlags             // Show only flags for a command
	HelpModeExamples          // Show usage examples
	HelpModeAll               // Show everything (no filtering)
	HelpModeSearch            // Search mode
	HelpModeHelp              // Show help about help options
)

// HelpOptions represents runtime help configuration
type HelpOptions struct {
	ShowDescriptions bool     `goopt:"short:d;desc:Show descriptions;default:true"`
	ShowDefaults     bool     `goopt:"short:D;desc:Show default values;default:false"`
	ShowTypes        bool     `goopt:"short:t;desc:Show types;default:false"`
	ShowValidators   bool     `goopt:"short:v;desc:Show validators;default:false"`
	ShowShortFlags   bool     `goopt:"short:s;desc:Show short flags;default:true"`
	Depth            int      `goopt:"short:dp;desc:Depth of subcommands;default:-1"`
	Filter           string   `goopt:"short:f;desc:Filter subcommands"`
	Search           string   `goopt:"short:q;desc:Search subcommands"`
	Command          []string `goopt:"pos:0;desc:Command path"`
	Style            string   `goopt:"desc:Help style;validators:isoneof(flat,grouped,compact,hierarchical,smart)"`

	// Negative flags for disabling features
	NoDescriptions bool `goopt:"name:no-desc;desc:Hide descriptions;default:false"`
	NoShort        bool `goopt:"name:no-short;desc:Hide short flags;default:false"`
}

// HelpContext determines where help output should go
type HelpContext int

const (
	HelpContextNormal HelpContext = iota // Normal help request (stdout)
	HelpContextError                     // Help due to error (stderr)
)

// HelpParser is an enhanced help parser
type HelpParser struct {
	mainParser *Parser
	config     HelpConfig
	options    *HelpOptions
	context    HelpContext
	hp         *Parser
}

// NewHelpParser creates a new improved help parser
func NewHelpParser(mainParser *Parser, config HelpConfig) *HelpParser {
	opts := &HelpOptions{}
	hp, err := NewParserFromStruct(opts,
		WithCommandNameConverter(ToKebabCase),
		WithFlagNameConverter(ToKebabCase))
	if err != nil {
		panic(err)
	}
	return &HelpParser{
		mainParser: mainParser,
		config:     config,
		options:    opts,
		context:    HelpContextNormal, // Default to normal context
		hp:         hp,
	}
}

// SetContext sets the help context (normal or error)
func (h *HelpParser) SetContext(ctx HelpContext) {
	h.context = ctx
}

// getWriter returns the appropriate writer based on context
func (h *HelpParser) getWriter() io.Writer {
	// Use the main parser's help writer which respects help behavior settings
	return h.mainParser.GetHelpWriter(h.context == HelpContextError)
}

// showVersionHeader shows the version header if configured
func (h *HelpParser) showVersionHeader(writer io.Writer) {
	if h.mainParser.showVersionInHelp && (h.mainParser.version != "" || h.mainParser.versionFunc != nil) {
		fmt.Fprintf(writer, "%s %s\n\n", filepath.Base(os.Args[0]), h.mainParser.GetVersion())
	}
}

// Parse parses help-specific arguments and renders appropriate help
func (h *HelpParser) Parse(args []string) error {
	// Check for help-for-help mode first (before parsing)
	mode := h.detectHelpMode(args)
	if mode == HelpModeHelp {
		writer := h.getWriter()
		return h.showHelpForHelp(writer)
	}

	// Parse the arguments
	newArgs, _ := h.normalize(args)
	h.hp.Parse(newArgs)

	// Handle negative flags
	if h.options.NoDescriptions {
		h.options.ShowDescriptions = false
	}
	if h.options.NoShort {
		h.options.ShowShortFlags = false
	}

	// Now detect the mode again after options are populated
	mode = h.detectHelpMode(args)

	// Detect command path and validate all parts
	commandPath, invalidPart := h.detectCommandPathWithValidation(h.options.Command)

	// Handle invalid command or subcommand
	if invalidPart != "" && mode != HelpModeHelp {
		h.context = HelpContextError // Switch to error context
		if commandPath == "" {
			// Invalid root command
			return h.handleInvalidCommand(invalidPart)
		} else {
			// Invalid subcommand
			return h.handleInvalidSubcommand(commandPath, invalidPart)
		}
	}

	writer := h.getWriter()

	// Only override the style if explicitly provided via --style
	if h.options.Style != "" {
		switch h.options.Style {
		case "flat":
			h.config.Style = HelpStyleFlat
		case "grouped":
			h.config.Style = HelpStyleGrouped
		case "compact":
			h.config.Style = HelpStyleCompact
		case "hierarchical":
			h.config.Style = HelpStyleHierarchical
		case "smart":
			h.config.Style = HelpStyleSmart
		}
	}
	// Otherwise, keep the style from the main parser's config

	// If we're in error context (set by main parser), show errors first
	if mode != HelpModeHelp && h.context == HelpContextError && h.mainParser.GetErrorCount() > 0 {
		h.showErrors(writer)
		fmt.Fprintln(writer) // Add blank line after error
	}

	// Execute based on mode
	switch mode {
	case HelpModeGlobals:
		return h.showGlobalsOnly(writer)
	case HelpModeCommands:
		return h.showCommandsOnly(writer)
	case HelpModeFlags:
		return h.showFlagsOnly(writer, commandPath)
	case HelpModeExamples:
		return h.showExamples(writer, commandPath)
	case HelpModeSearch:
		return h.showSearchResults(writer, h.options.Search)
	case HelpModeAll:
		return h.showAll(writer, commandPath)
	case HelpModeHelp:
		return h.showHelpForHelp(writer)
	default:
		if commandPath == "" {
			return h.showDefault(writer)
		} else {
			return h.renderCommandHelp(writer, commandPath)
		}
	}
}

func (h *HelpParser) normalize(args []string) ([]string, int) {
	// Find the first flag position
	firstFlagIndex := -1
	helpIndex := -1

	for i, arg := range args {
		if h.hp.isFlag(arg) {
			if firstFlagIndex == -1 {
				firstFlagIndex = i
			}
			if h.isHelpArg(arg) && helpIndex == -1 {
				helpIndex = i
			}
		}
	}

	if helpIndex == -1 {
		// No help flag found, shouldn't happen but handle gracefully
		return args, 0
	}

	// Collect all non-flag strings before the first flag
	commands := []string{}
	if firstFlagIndex > 0 {
		for i := 0; i < firstFlagIndex; i++ {
			commands = append(commands, args[i])
		}
	}

	// Build new args
	newArgs := []string{}

	// If we have commands, add them as a comma-separated positional argument
	if len(commands) > 0 {
		newArgs = append(newArgs, strings.Join(commands, ","))
	}

	// Add everything after the help flag (except the help flag itself)
	for i := helpIndex + 1; i < len(args); i++ {
		newArgs = append(newArgs, args[i])
	}

	return newArgs, len(commands)
}

// detectHelpMode determines the help mode from arguments
func (h *HelpParser) detectHelpMode(args []string) HelpMode {
	if h.options.Search != "" {
		return HelpModeSearch
	}

	// Check if user is asking for help about help
	// This could be: --help --help, --help help, help --help, help help
	// Or with custom help flags: --aide --aide, --aide help, etc.
	helpArgCount := 0
	for _, arg := range args {
		if h.isHelpArg(arg) {
			helpArgCount++
			if helpArgCount >= 2 {
				return HelpModeHelp
			}
		}
	}

	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "globals", "global":
			return HelpModeGlobals
		case "commands", "command", "cmds", "cmd":
			return HelpModeCommands
		case "flags", "flag":
			return HelpModeFlags
		case "examples", "example":
			return HelpModeExamples
		case "all", "full":
			return HelpModeAll
		}
	}

	return HelpModeDefault
}

// isHelpArg checks if an argument is a help flag or the word "help"
func (h *HelpParser) isHelpArg(arg string) bool {
	// Check if it's the word "help" (case insensitive)
	if strings.ToLower(arg) == "help" {
		return true
	}

	// Check if it's one of the configured help flags
	if h.mainParser.isFlag(arg) {
		stripped := strings.TrimLeftFunc(arg, h.mainParser.prefixFunc)
		for _, helpFlag := range h.mainParser.helpFlags {
			if stripped == helpFlag {
				return true
			}
		}
	}

	return false
}

// showErrors displays all errors from the main parser
func (h *HelpParser) showErrors(writer io.Writer) {
	errors := h.mainParser.GetErrors()
	for _, err := range errors {
		fmt.Fprintf(writer, "%s: %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgErrorPrefixKey), err.Error())
	}
}

// handleInvalidCommand handles invalid command errors
func (h *HelpParser) handleInvalidCommand(invalidCmd string) error {
	writer := h.getWriter()

	// Find similar commands
	suggestions, _ := h.findSimilarCommandsWithContext(invalidCmd)

	fmt.Fprintf(writer, "%s: %s\n\n",
		h.mainParser.layeredProvider.GetMessage(messages.MsgErrorPrefixKey),
		h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgUnknownCommandKey, invalidCmd))

	if len(suggestions) > 0 {
		// Display each suggestion in the form that was closest to user input
		displaySuggestions := make([]string, len(suggestions))
		for i, suggestion := range suggestions {
			// By default show canonical
			displaySuggestions[i] = suggestion

			// Check if we should show translated form
			if h.mainParser.translationRegistry != nil {
				// Get the command to check translation
				var cmd *Command
				if c, found := h.mainParser.registeredCommands.Get(suggestion); found {
					cmd = c
				}

				if cmd != nil && cmd.NameKey != "" {
					if translated, found := h.mainParser.translationRegistry.GetCommandTranslation(suggestion, h.mainParser.GetLanguage()); found {
						// Compare distances to determine which form to show
						canonicalDist := util.LevenshteinDistance(invalidCmd, suggestion)
						translatedDist := util.LevenshteinDistance(invalidCmd, translated)

						// Debug logging
						// fmt.Printf("DEBUG handleInvalidCommand: invalidCmd=%s, suggestion=%s, translated=%s, canonicalDist=%d, translatedDist=%d\n",
						//     invalidCmd, suggestion, translated, canonicalDist, translatedDist)

						// Show the form that's closer to what user typed
						if translatedDist < canonicalDist {
							displaySuggestions[i] = translated
						} else if translatedDist == canonicalDist && translated != suggestion {
							// If equal distance and different words, show both forms
							displaySuggestions[i] = fmt.Sprintf("%s / %s", suggestion, translated)
						}
					}
				}
			}
		}

		// Use the parser's suggestions formatter if available
		var formatted string
		if h.mainParser.suggestionsFormatter != nil {
			formatted = h.mainParser.suggestionsFormatter(displaySuggestions)
		} else {
			// Default format for help system
			formatted = "\n  " + strings.Join(displaySuggestions, "\n  ")
		}
		fmt.Fprintf(writer, "%s %s\n\n",
			h.mainParser.layeredProvider.GetMessage(messages.MsgDidYouMeanKey),
			formatted)
	}

	// Show available commands
	fmt.Fprintf(writer, "%s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgAvailableCommandsKey))
	err := h.showCommandsOnly(writer)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "\n%s\n",
		h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgUseHelpForInfoKey, os.Args[0]))

	return errs.ErrCommandNotFound.WithArgs(invalidCmd)
}

// handleInvalidSubcommand handles invalid subcommand errors
func (h *HelpParser) handleInvalidSubcommand(commandPath, invalidSub string) error {
	writer := h.getWriter()

	// Add error to match main parser behavior
	fullPath := commandPath + " " + invalidSub
	err := errs.ErrCommandNotFound.WithArgs(fullPath)
	h.mainParser.addError(err)

	// Get the parent command
	cmd, _ := h.mainParser.getCommand(commandPath)

	// Find similar subcommands
	suggestions := h.findSimilarSubcommands(cmd.Subcommands, invalidSub)

	// Show the main parser error
	h.showErrors(writer)

	// Show suggestions if any
	if len(suggestions) > 0 {
		// Use the parser's suggestions formatter if available
		var formatted string
		if h.mainParser.suggestionsFormatter != nil {
			formatted = h.mainParser.suggestionsFormatter(suggestions)
		} else {
			// Default format for help system
			formatted = "\n  " + strings.Join(suggestions, "\n  ")
		}
		fmt.Fprintf(writer, "%s %s\n",
			h.mainParser.layeredProvider.GetMessage(messages.MsgDidYouMeanKey),
			formatted)
		h.mainParser.addError(fmt.Errorf("%s %s",
			h.mainParser.layeredProvider.GetMessage(messages.MsgDidYouMeanKey),
			formatted))
	}

	fmt.Fprintln(writer)

	// Show help for the parent command
	return h.renderCommandHelp(writer, commandPath)
}

// suggestion represents a command suggestion with its distance and translation status
type suggestion struct {
	commandPath  string
	distance     int
	isTranslated bool
}

// findSimilarCommandsWithContext finds commands similar to the input and detects if input is likely translated
func (h *HelpParser) findSimilarCommandsWithContext(input string) ([]string, bool) {

	var allSuggestions []suggestion
	currentLang := h.mainParser.GetLanguage()

	// Check all commands - both canonical and translated names
	for kv := h.mainParser.registeredCommands.Front(); kv != nil; kv = kv.Next() {
		cmd := kv.Value

		// Check canonical name
		distance := util.LevenshteinDistance(input, cmd.Name)
		if distance > 0 && distance <= 2 {
			allSuggestions = append(allSuggestions, suggestion{
				commandPath:  cmd.Name,
				distance:     distance,
				isTranslated: false,
			})
		}

		// Check translated name if available
		if h.mainParser.translationRegistry != nil && cmd.NameKey != "" {
			if translated, found := h.mainParser.translationRegistry.GetCommandTranslation(cmd.Name, currentLang); found {
				translatedDistance := util.LevenshteinDistance(input, translated)
				if translatedDistance > 0 && translatedDistance <= 2 {
					// Check if we already have this command in suggestions
					found := false
					for i, s := range allSuggestions {
						if s.commandPath == cmd.Name {
							// Update if translated is closer
							if translatedDistance < s.distance {
								allSuggestions[i].distance = translatedDistance
								allSuggestions[i].isTranslated = true
							}
							found = true
							break
						}
					}
					if !found {
						allSuggestions = append(allSuggestions, suggestion{
							commandPath:  cmd.Name,
							distance:     translatedDistance,
							isTranslated: true,
						})
					}
				}
			}
		}

		// Check subcommands
		if len(cmd.Subcommands) > 0 {
			h.findSimilarCommandsInSliceWithContext(cmd.Name, cmd.Subcommands, input, &allSuggestions, currentLang)
		}
	}

	// Find minimum distance
	minDistance := 3
	for _, s := range allSuggestions {
		if s.distance < minDistance {
			minDistance = s.distance
		}
	}

	// If we have distance 1 matches, only show those
	// Otherwise show all matches up to distance 2
	threshold := minDistance
	if minDistance > 1 {
		threshold = 2
	}

	// Filter and determine if we should show translated names
	var finalSuggestions []string
	hasTranslated := false

	for _, s := range allSuggestions {
		if s.distance <= threshold {
			finalSuggestions = append(finalSuggestions, s.commandPath)
			if s.isTranslated {
				hasTranslated = true
			}
		}
	}

	// Remove duplicates
	uniqueSuggestions := make(map[string]bool)
	var filtered []string
	for _, s := range finalSuggestions {
		if !uniqueSuggestions[s] {
			uniqueSuggestions[s] = true
			filtered = append(filtered, s)
		}
	}
	finalSuggestions = filtered

	// Sort by distance
	sort.Slice(finalSuggestions, func(i, j int) bool {
		dist1 := 3
		dist2 := 3
		for _, s := range allSuggestions {
			if s.commandPath == finalSuggestions[i] {
				dist1 = s.distance
			}
			if s.commandPath == finalSuggestions[j] {
				dist2 = s.distance
			}
		}
		return dist1 < dist2
	})

	// Limit to top 3
	if len(finalSuggestions) > 3 {
		finalSuggestions = finalSuggestions[:3]
	}

	return finalSuggestions, hasTranslated
}

// findSimilarCommands finds commands similar to the input
func (h *HelpParser) findSimilarCommands(input string) []string {
	suggestions, _ := h.findSimilarCommandsWithContext(input)
	return suggestions
}

// findSimilarSubcommands finds subcommands similar to the input from a specific command's subcommands
func (h *HelpParser) findSimilarSubcommands(subcommands []Command, input string) []string {
	var suggestions []string
	threshold := 2 // Levenshtein distance threshold

	for _, cmd := range subcommands {
		distance := util.LevenshteinDistance(input, cmd.Name)
		// Skip exact matches (distance 0) and only suggest similar commands
		if distance > 0 && distance <= threshold {
			suggestions = append(suggestions, cmd.Name)
		}
	}

	// Sort by similarity
	sort.Slice(suggestions, func(i, j int) bool {
		return util.LevenshteinDistance(input, suggestions[i]) < util.LevenshteinDistance(input, suggestions[j])
	})

	// Limit to top 3
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}

	return suggestions
}

// detectBestStyle automatically selects the best help style based on CLI complexity
func (h *HelpParser) detectBestStyle() HelpStyle {
	flagCount := h.mainParser.acceptedFlags.Count()
	cmdCount := h.mainParser.registeredCommands.Len()

	// Large CLI with many flags and commands
	if float64(flagCount) > float64(h.config.CompactThreshold)*1.4 && cmdCount > 5 {
		return HelpStyleHierarchical
	}

	// Medium CLI with moderate complexity
	if flagCount > h.config.CompactThreshold {
		return HelpStyleCompact
	}

	// Small CLI with commands
	if cmdCount > 3 {
		return HelpStyleGrouped
	}

	// Simple CLI
	return HelpStyleFlat
}

// findSimilarCommandsInSliceWithContext recursively searches for similar commands considering both canonical and translated names
func (h *HelpParser) findSimilarCommandsInSliceWithContext(prefix string, commands []Command, input string, suggestions *[]suggestion, currentLang language.Tag) {
	for i := range commands {
		cmd := &commands[i]
		commandPath := cmd.Name
		if prefix != "" {
			commandPath = prefix + " " + cmd.Name
		}

		// Check canonical name
		distance := util.LevenshteinDistance(input, cmd.Name)
		if distance > 0 && distance <= 2 {
			*suggestions = append(*suggestions, suggestion{
				commandPath:  commandPath,
				distance:     distance,
				isTranslated: false,
			})
		}

		// Check translated name if available
		if h.mainParser.translationRegistry != nil && cmd.NameKey != "" {
			if translated, found := h.mainParser.translationRegistry.GetCommandTranslation(commandPath, currentLang); found {
				translatedDistance := util.LevenshteinDistance(input, translated)
				if translatedDistance > 0 && translatedDistance <= 2 {
					// Check if we already have this command in suggestions
					found := false
					for j, s := range *suggestions {
						if s.commandPath == commandPath {
							// Update if translated is closer
							if translatedDistance < s.distance {
								(*suggestions)[j].distance = translatedDistance
								(*suggestions)[j].isTranslated = true
							}
							found = true
							break
						}
					}
					if !found {
						*suggestions = append(*suggestions, suggestion{
							commandPath:  commandPath,
							distance:     translatedDistance,
							isTranslated: true,
						})
					}
				}
			}
		}

		// Check subcommands
		if len(cmd.Subcommands) > 0 {
			h.findSimilarCommandsInSliceWithContext(commandPath, cmd.Subcommands, input, suggestions, currentLang)
		}
	}
}

// findSimilarCommandsInSlice recursively searches for similar commands in a slice
func (h *HelpParser) findSimilarCommandsInSlice(prefix string, commands []Command, input string, threshold int, suggestions *[]string) {
	for i := range commands {
		cmd := &commands[i]
		distance := util.LevenshteinDistance(input, cmd.Name)
		// Skip exact matches (distance 0) and only suggest similar commands
		if distance > 0 && distance <= threshold {
			fullPath := cmd.Name
			if prefix != "" {
				fullPath = prefix + " " + cmd.Name
			}
			*suggestions = append(*suggestions, fullPath)
		}

		// Check subcommands
		if len(cmd.Subcommands) > 0 {
			newPrefix := cmd.Name
			if prefix != "" {
				newPrefix = prefix + " " + cmd.Name
			}
			h.findSimilarCommandsInSlice(newPrefix, cmd.Subcommands, input, threshold, suggestions)
		}
	}
}

// showGlobalsOnly shows only global flags
func (h *HelpParser) showGlobalsOnly(writer io.Writer) error {
	h.showVersionHeader(writer)
	fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgGlobalFlagsHeaderKey))

	globalFlags := h.mainParser.getGlobalFlags()
	filtered := h.filterFlags(globalFlags)

	if len(filtered) == 0 {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgNoGlobalFlagsKey))
		return nil
	}

	for _, flag := range filtered {
		h.printDetailedFlag(writer, flag)
	}

	return nil
}

// showCommandsOnly shows only commands
func (h *HelpParser) showCommandsOnly(writer io.Writer) error {
	h.showVersionHeader(writer)
	if h.mainParser.registeredCommands.Len() == 0 {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgNoCommandsDefinedKey))
		return nil
	}

	// Show command tree
	h.mainParser.printCommandTree(writer)

	return nil
}

// showFlagsOnly shows only flags for a specific command or globally
func (h *HelpParser) showFlagsOnly(writer io.Writer, commandPath string) error {
	h.showVersionHeader(writer)
	if commandPath != "" {
		fmt.Fprintf(writer, "%s\n\n",
			h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgFlagsForCommandKey, commandPath))
	} else {
		fmt.Fprintf(writer, "%s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgAllFlagsKey))
	}

	flags := h.collectFlags(commandPath)
	filtered := h.filterFlags(flags)

	if len(filtered) == 0 {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgNoFlagsFoundKey))
		return nil
	}

	for _, flag := range filtered {
		h.printDetailedFlag(writer, flag)
	}

	return nil
}

// showExamples shows usage examples
func (h *HelpParser) showExamples(writer io.Writer, commandPath string) error {
	h.showVersionHeader(writer)
	fmt.Fprintf(writer, "%s:\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExamplesKey))

	prog := os.Args[0]

	// Basic examples
	fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgShowThisHelpKey))
	fmt.Fprintf(writer, "%s --help\n\n", prog)

	fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgShowOnlyGlobalFlagsKey))
	fmt.Fprintf(writer, "%s --help globals\n\n", prog)

	fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgShowAllCommandsKey))
	fmt.Fprintf(writer, "%s --help commands\n\n", prog)

	if h.mainParser.registeredCommands.Count() > 0 {
		// Command-specific examples
		if first := h.mainParser.registeredCommands.Front(); first != nil {
			fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgShowHelpForCommandKey))
			fmt.Fprintf(writer, "%s %s --help\n\n", prog, first.Value.Name)

			if len(first.Value.Subcommands) > 0 {
				fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgShowHelpForSubcommandKey))
				fmt.Fprintf(writer, "%s %s %s --help\n\n", prog, first.Value.Name, first.Value.Subcommands[0].Name)
			}
		}
	}

	// Advanced examples
	fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgSearchHelpContentKey))
	fmt.Fprintf(writer, "%s --help --search \"database\"\n\n", prog)

	fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgShowHelpWithDetailsKey))
	fmt.Fprintf(writer, "%s --help --show-defaults --show-types\n\n", prog)

	fmt.Fprintf(writer, "# %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgFilterFlagsByPatternKey))
	fmt.Fprintf(writer, "%s --help --filter \"core.*\"\n", prog)

	return nil
}

// showSearchResults shows search results
func (h *HelpParser) showSearchResults(writer io.Writer, query string) error {
	h.showVersionHeader(writer)
	if query == "" {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgSearchQueryEmptyKey))
		return nil
	}

	fmt.Fprintf(writer, "%s\n\n", h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgSearchResultsKey, query))

	results := h.searchHelp(query)

	if len(results) == 0 {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgNoResultsFoundKey))
		return nil
	}

	// Group results by type
	var flagResults, cmdResults []searchResult
	for _, r := range results {
		if r.Type == "flag" {
			flagResults = append(flagResults, r)
		} else {
			cmdResults = append(cmdResults, r)
		}
	}

	// Show command results
	if len(cmdResults) > 0 {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsHeaderKey))
		for _, r := range cmdResults {
			fmt.Fprintf(writer, "  %s - %s\n", r.Name, r.Description)
			if r.Context != "" {
				fmt.Fprintf(writer, "    %s: %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgContextKey), r.Context)
			}
		}
		fmt.Fprintln(writer)
	}

	// Show flag results
	if len(flagResults) > 0 {
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetMessage(messages.MsgFlagsHeaderKey))
		for _, r := range flagResults {
			fmt.Fprintf(writer, "  --%s", r.Name)
			if r.Short != "" {
				fmt.Fprintf(writer, ", -%s", r.Short)
			}
			fmt.Fprintf(writer, " - %s\n", r.Description)
			if r.Context != "" {
				fmt.Fprintf(writer, "    %s: %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey), r.Context)
			}
		}
	}

	return nil
}

// showDefault shows default help with runtime options applied
func (h *HelpParser) showDefault(writer io.Writer) error {
	// Get the configured help style (it was already set in Parse from either --style or main parser config)
	style := h.config.Style

	// Auto-detect style if set to Smart
	if style == HelpStyleSmart {
		style = h.detectBestStyle()
	}

	// Apply the selected style
	switch style {
	case HelpStyleFlat:
		return h.showFlatStyle(writer)
	case HelpStyleGrouped:
		return h.showGroupedStyle(writer)
	case HelpStyleCompact:
		return h.showCompactStyle(writer)
	case HelpStyleHierarchical:
		return h.showHierarchicalStyle(writer)
	default:
		return h.showFlatStyle(writer)
	}
}

// showFlatStyle shows traditional flat help
func (h *HelpParser) showFlatStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Show usage line
	fmt.Fprintln(writer, h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgUsageKey, os.Args[0]))

	// Show positional args if any
	h.mainParser.PrintPositionalArgs(writer)

	// Show flags with options applied
	flags := h.collectFlags("")
	filtered := h.filterFlags(flags)

	if len(filtered) > 0 {
		for _, flag := range filtered {
			// Format flag similar to renderer but with runtime options
			h.printFlatStyleFlag(writer, flag)
		}
	}

	// Show commands
	if h.mainParser.registeredCommands.Len() > 0 {
		fmt.Fprintf(writer, "\n%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey))
		h.mainParser.PrintCommands(writer)
	}

	return nil
}

// showGroupedStyle shows help with flags grouped by command
func (h *HelpParser) showGroupedStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Use PrintUsageWithGroups which shows "Global Flags:" header
	h.mainParser.PrintUsageWithGroups(writer)
	return nil
}

// showCompactStyle shows deduplicated, compact help
func (h *HelpParser) showCompactStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Just delegate to the main parser's printCompactHelp which handles everything
	h.mainParser.printCompactHelp(writer)
	return nil
}

// showHierarchicalStyle shows command-focused, drill-down help
func (h *HelpParser) showHierarchicalStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Match the format from help_styles.go printHierarchicalHelp
	fmt.Fprintf(writer, "%s\n\n", h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgUsageHierarchicalKey, os.Args[0]))

	// Only essential global flags
	globalFlags := h.mainParser.getGlobalFlags()
	filtered := h.filterFlags(globalFlags)
	if len(filtered) > 0 {
		fmt.Fprintf(writer, "%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgGlobalFlagsKey))
		shown := 0
		for _, flag := range filtered {
			// Only show help and essential flags
			if flag.Short == "h" || flag.Short == "help" || flag.Required {
				h.mainParser.printCompactFlag(writer, flag)
				shown++
			}
			if shown >= 5 {
				break
			}
		}
		if len(filtered) > shown {
			fmt.Fprintf(writer, "  ... %s %d %s\n",
				h.mainParser.layeredProvider.GetMessage(messages.MsgAndKey),
				len(filtered)-shown,
				h.mainParser.layeredProvider.GetMessage(messages.MsgMoreKey))
		}
	}

	// Shared flag groups summary
	sharedGroups := h.mainParser.detectSharedFlagGroups()
	if len(sharedGroups) > 0 {
		fmt.Fprintf(writer, "\n%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgSharedFlagGroupsKey))

		// Sort and display
		type groupInfo struct {
			name     string
			cmdCount int
		}
		var groups []groupInfo
		for prefix, info := range sharedGroups {
			if len(info.commands) > 1 {
				groups = append(groups, groupInfo{prefix, len(info.commands)})
			}
		}
		sort.Slice(groups, func(i, j int) bool {
			return groups[i].cmdCount > groups[j].cmdCount
		})

		for _, g := range groups {
			fmt.Fprintf(writer, "  %-20s %s %d %s\n",
				g.name+".*",
				h.mainParser.layeredProvider.GetMessage(messages.MsgUsedByKey),
				g.cmdCount,
				h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey))
		}
	}

	// Command structure
	if h.mainParser.registeredCommands.Count() > 0 {
		fmt.Fprintf(writer, "\n%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgCommandStructureKey))
		h.mainParser.printCommandTree(writer)
	}

	// Examples
	fmt.Fprintf(writer, "\n%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExamplesKey))
	fmt.Fprintf(writer, "  %s --help                    # %s\n",
		os.Args[0], h.mainParser.layeredProvider.GetMessage(messages.MsgThisHelpKey))
	if h.mainParser.registeredCommands.Count() > 0 {
		// Show first command as example
		if first := h.mainParser.registeredCommands.Front(); first != nil {
			fmt.Fprintf(writer, "  %s %s --help              # %s\n",
				os.Args[0], first.Value.Name, h.mainParser.layeredProvider.GetMessage(messages.MsgCommandHelpKey))
			if len(first.Value.Subcommands) > 0 {
				fmt.Fprintf(writer, "  %s %s %s --help       # %s\n",
					os.Args[0], first.Value.Name, first.Value.Subcommands[0].Name,
					h.mainParser.layeredProvider.GetMessage(messages.MsgSubcommandHelpKey))
			}
		}
	}

	return nil
}

// showAll shows all help information
func (h *HelpParser) showAll(writer io.Writer, commandPath string) error {
	h.showVersionHeader(writer)
	var err error
	if commandPath == "" {
		// Show usage
		fmt.Fprintln(writer, h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgUsageKey, os.Args[0]))
		fmt.Fprintln(writer)

		// Show positional arguments
		h.mainParser.PrintPositionalArgs(writer)

		// Show global flags
		globalFlags := h.collectFlags("")
		if len(globalFlags) > 0 {
			fmt.Fprintf(writer, "%s:\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgGlobalFlagsKey))
			for _, flag := range globalFlags {
				h.printDetailedFlag(writer, flag)
			}
			fmt.Fprintln(writer)
		}

		// Show all commands with their flags
		fmt.Fprintf(writer, "%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey))
		h.mainParser.printCommandTree(writer)
		fmt.Fprintln(writer)

		// Show examples
		err = h.showExamples(writer, "")
	} else {
		// Show detailed command help
		err = h.renderCommandHelp(writer, commandPath)
		if err != nil {
			return err
		}
		fmt.Fprintln(writer)

		// Show command-specific examples
		err = h.showExamples(writer, commandPath)
	}

	return err
}

// Helper methods

// filterFlags filters flags based on current options
func (h *HelpParser) filterFlags(flags []*Argument) []*Argument {
	if h.options.Filter == "" {
		return flags
	}

	filtered := []*Argument{}
	for _, flag := range flags {
		name := h.mainParser.renderer.FlagName(flag)
		if matchesPattern(name, h.options.Filter) {
			filtered = append(filtered, flag)
		}
	}

	return filtered
}

// collectFlags collects flags for a command path
func (h *HelpParser) collectFlags(commandPath string) []*Argument {
	var flags []*Argument

	for f := h.mainParser.acceptedFlags.Front(); f != nil; f = f.Next() {
		if commandPath == "" || f.Value.CommandPath == commandPath {
			flags = append(flags, f.Value.Argument)
		}
	}

	return flags
}

// printDetailedFlag prints a flag with options-controlled detail
func (h *HelpParser) printDetailedFlag(writer io.Writer, arg *Argument) {
	name := h.mainParser.renderer.FlagName(arg)

	// Base format
	fmt.Fprintf(writer, "  --%s", name)

	if h.options.ShowShortFlags && arg.Short != "" {
		fmt.Fprintf(writer, ", -%s", arg.Short)
	}

	if h.options.ShowTypes {
		fmt.Fprintf(writer, " (%s)", arg.TypeOf)
	}

	if h.options.ShowDescriptions && arg.Description != "" {
		fmt.Fprintf(writer, " - %s", h.mainParser.renderer.FlagDescription(arg))
	}

	if h.options.ShowDefaults && arg.DefaultValue != "" {
		fmt.Fprintf(writer, " (%s: %s)", h.mainParser.layeredProvider.GetMessage(messages.MsgDefaultsToKey), arg.DefaultValue)
	}

	if h.options.ShowValidators && len(arg.Validators) > 0 {
		fmt.Fprintf(writer, " [%s: %d]", h.mainParser.layeredProvider.GetMessage(messages.MsgValidatorsKey), len(arg.Validators))
	}

	if arg.Required {
		fmt.Fprintf(writer, " (%s)", h.mainParser.layeredProvider.GetMessage(messages.MsgRequiredKey))
	}

	fmt.Fprintln(writer)
}

// searchResult represents a search result
type searchResult struct {
	Type        string // "flag" or "command"
	Name        string
	Short       string // For flags
	Description string
	Context     string // Command path for flags
}

// searchHelp searches through help content
func (h *HelpParser) searchHelp(query string) []searchResult {
	var results []searchResult

	// Check if query contains wildcards
	hasWildcards := strings.Contains(query, "*") || strings.Contains(query, "?")

	// Search flags
	for f := h.mainParser.acceptedFlags.Front(); f != nil; f = f.Next() {
		flagName := *f.Key
		arg := f.Value.Argument
		desc := h.mainParser.renderer.FlagDescription(arg)

		var matches bool
		if hasWildcards {
			// Use wildcard matching
			matches = matchesPattern(strings.ToLower(flagName), strings.ToLower(query)) ||
				matchesPattern(strings.ToLower(desc), strings.ToLower(query)) ||
				(arg.Short != "" && matchesPattern(strings.ToLower(arg.Short), strings.ToLower(query)))
		} else {
			// Use substring matching for backward compatibility
			queryLower := strings.ToLower(query)
			matches = strings.Contains(strings.ToLower(flagName), queryLower) ||
				strings.Contains(strings.ToLower(desc), queryLower) ||
				(arg.Short != "" && strings.Contains(strings.ToLower(arg.Short), queryLower))
		}

		if matches {
			results = append(results, searchResult{
				Type:        "flag",
				Name:        h.mainParser.renderer.FlagName(arg),
				Short:       arg.Short,
				Description: desc,
				Context:     f.Value.CommandPath,
			})
		}
	}

	// Search commands
	for cmd := h.mainParser.registeredCommands.Front(); cmd != nil; cmd = cmd.Next() {
		cmdName := cmd.Value.Name
		desc := h.mainParser.renderer.CommandDescription(cmd.Value)

		var matches bool
		if hasWildcards {
			// Use wildcard matching
			matches = matchesPattern(strings.ToLower(cmdName), strings.ToLower(query)) ||
				matchesPattern(strings.ToLower(desc), strings.ToLower(query))
		} else {
			// Use substring matching for backward compatibility
			queryLower := strings.ToLower(query)
			matches = strings.Contains(strings.ToLower(cmdName), queryLower) ||
				strings.Contains(strings.ToLower(desc), queryLower)
		}

		if matches {
			results = append(results, searchResult{
				Type:        "command",
				Name:        cmdName,
				Description: desc,
				Context:     cmd.Value.path,
			})
		}
	}

	return results
}

// isRegisteredCommand checks if a command path is registered
func (h *HelpParser) isRegisteredCommand(path string) bool {
	_, found := h.mainParser.registeredCommands.Get(path)
	return found
}

// detectCommandPathWithValidation detects the command path and returns any invalid part
func (h *HelpParser) detectCommandPathWithValidation(args []string) (commandPath string, invalidPart string) {
	var cmdPath []string
	p := h.mainParser

	// Build command path and detect invalid parts
	for i := 0; i < len(args); i++ {
		if p.isFlag(args[i]) {
			break
		}

		// Skip help mode keywords
		if isHelpKeyword(args[i]) {
			continue
		}

		// Check if this extends the valid command path
		testPath := strings.Join(append(cmdPath, args[i]), " ")
		if h.isRegisteredCommand(testPath) {
			cmdPath = append(cmdPath, args[i])
		} else {
			// This is not a valid command extension
			// If we haven't found any valid command yet, this is an invalid root command
			// Otherwise, it's an invalid subcommand
			return strings.Join(cmdPath, " "), args[i]
		}
	}

	return strings.Join(cmdPath, " "), ""
}

// renderCommandHelp renders help for a specific command
func (h *HelpParser) renderCommandHelp(writer io.Writer, commandPath string) error {
	h.showVersionHeader(writer)
	cmd, found := h.mainParser.getCommand(commandPath)
	if !found {
		return errs.ErrCommandNotFound.WithArgs(commandPath)
	}

	pp := h.mainParser.DefaultPrettyPrintConfig()
	parts := strings.Split(commandPath, " ")

	// Show command hierarchy
	for i := range parts {
		path := strings.Join(parts[:i+1], " ")
		if c, ok := h.mainParser.getCommand(path); ok {
			fmt.Fprintf(writer, "%s%s: %s\n",
				strings.Repeat(pp.OuterLevelBindPrefix, i),
				path,
				h.mainParser.renderer.CommandDescription(c))
		}
	}

	// Show inherited flags message
	if len(parts) > 1 {
		fmt.Fprintf(writer, "%s + %s\n",
			strings.Repeat(pp.OuterLevelBindPrefix, len(parts)-1),
			h.mainParser.layeredProvider.GetMessage(messages.MsgAllParentFlagsKey))
	}

	// Show command-specific flags
	h.mainParser.PrintCommandSpecificFlags(writer, commandPath, len(parts)-1, pp)

	// Show subcommands if present
	if len(cmd.Subcommands) > 0 && (h.options.Depth == -1 || h.options.Depth > 0) {
		fmt.Fprintf(writer, "\n%s:\n",
			h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey))

		mainPrefix := strings.TrimSpace(pp.DefaultPrefix)
		termPrefix := strings.TrimSpace(pp.TerminalPrefix)

		for i := range cmd.Subcommands {
			prefix := mainPrefix
			if i == len(cmd.Subcommands)-1 {
				prefix = termPrefix
			}

			subCmd := &cmd.Subcommands[i]
			fmt.Fprintf(writer, " %s %s - %s\n",
				prefix,
				subCmd.Name,
				h.mainParser.renderer.CommandDescription(subCmd))
		}
	}

	return nil
}

// Utility functions

// matchesPattern checks if a string matches a glob-like pattern
func matchesPattern(s, pattern string) bool {
	// Simple glob matching (supports * and ?)
	// Convert pattern to regex
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = strings.ReplaceAll(pattern, "?", ".")
	pattern = "^" + pattern + "$"

	return regexp.MustCompile(pattern).MatchString(s)
}

// isHelpKeyword checks if a string is a help mode keyword
func isHelpKeyword(s string) bool {
	keywords := []string{"globals", "global", "commands", "command", "cmds", "cmd",
		"flags", "flag", "examples", "example", "all", "full"}
	s = strings.ToLower(s)
	for _, k := range keywords {
		if s == k {
			return true
		}
	}
	return false
}

// showHelpForHelp shows help about the help system itself
func (h *HelpParser) showHelpForHelp(writer io.Writer) error {
	h.showVersionHeader(writer)
	// Title
	fmt.Fprintf(writer, "%s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpSystemKey))

	// Introduction
	fmt.Fprintf(writer, "%s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpSystemDescKey))

	// Help modes section
	fmt.Fprintf(writer, "%s:\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModesKey))

	// Default mode
	fmt.Fprintf(writer, "  %s --help\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeDefaultDescKey))

	// Globals mode
	fmt.Fprintf(writer, "  %s --help globals\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeGlobalsDescKey))

	// Commands mode
	fmt.Fprintf(writer, "  %s --help commands\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeCommandsDescKey))

	// Flags mode
	fmt.Fprintf(writer, "  %s --help flags\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeFlagsDescKey))

	// Examples mode
	fmt.Fprintf(writer, "  %s --help examples\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeExamplesDescKey))

	// All mode
	fmt.Fprintf(writer, "  %s --help all\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeAllDescKey))

	// Command-specific help
	fmt.Fprintf(writer, "  %s <command> --help\n", os.Args[0])
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpModeCommandSpecificDescKey))

	// Help options section
	fmt.Fprintf(writer, "%s:\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionsKey))

	// Show descriptions option
	fmt.Fprintf(writer, "  --show-descriptions, -d\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionShowDescriptionsKey))

	fmt.Fprintf(writer, "  --no-desc\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionNoDescriptionsKey))

	// Show defaults option
	fmt.Fprintf(writer, "  --show-defaults\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionShowDefaultsKey))

	// Show types option
	fmt.Fprintf(writer, "  --show-types\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionShowTypesKey))

	// Show validators option
	fmt.Fprintf(writer, "  --show-validators\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionShowValidatorsKey))

	// No short flags option
	fmt.Fprintf(writer, "  --no-short\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionNoShortKey))

	// Filter option
	fmt.Fprintf(writer, "  --filter <pattern>\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionFilterKey))

	// Depth option
	fmt.Fprintf(writer, "  --depth <number>\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionDepthKey))

	// Search option
	fmt.Fprintf(writer, "  --search <query>\n")
	fmt.Fprintf(writer, "    %s\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionSearchKey))

	// Style option
	fmt.Fprintf(writer, "  --style <style>\n")
	fmt.Fprintf(writer, "    %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgHelpOptionStyleKey))
	fmt.Fprintf(writer, "    %s: flat, grouped, compact, hierarchical, smart\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgAvailableStylesKey))

	// Examples section
	fmt.Fprintf(writer, "%s:\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExamplesKey))

	// Example 1: Show help with all details
	fmt.Fprintf(writer, "  # %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExampleShowAllDetailsKey))
	fmt.Fprintf(writer, "  %s --help --show-defaults --show-types --show-validators\n\n", os.Args[0])

	// Example 2: Search for specific flags
	fmt.Fprintf(writer, "  # %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExampleSearchFlagsKey))
	fmt.Fprintf(writer, "  %s --help --search \"database\"\n\n", os.Args[0])

	// Example 3: Filter flags by pattern
	fmt.Fprintf(writer, "  # %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExampleFilterFlagsKey))
	fmt.Fprintf(writer, "  %s --help --filter \"*.port\"\n\n", os.Args[0])

	// Example 4: Show command help with custom style
	fmt.Fprintf(writer, "  # %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgExampleCustomStyleKey))
	fmt.Fprintf(writer, "  %s --help --style compact\n\n", os.Args[0])

	// Tips section
	fmt.Fprintf(writer, "%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgTipsKey))
	fmt.Fprintf(writer, "- %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgTipHelpHelpKey))
	fmt.Fprintf(writer, "- %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgTipSearchPatternKey))
	fmt.Fprintf(writer, "- %s\n", h.mainParser.layeredProvider.GetMessage(messages.MsgTipStyleAutoKey))

	return nil
}

// printFlatStyleFlag prints a flag in flat style format with runtime options
func (h *HelpParser) printFlatStyleFlag(writer io.Writer, arg *Argument) {
	var usage string

	// Use renderer for name to ensure proper translation
	usage = "--" + h.mainParser.renderer.FlagName(arg)

	// Add short flag if enabled
	if h.options.ShowShortFlags && arg.Short != "" {
		usage += " " + h.mainParser.layeredProvider.GetMessage(messages.MsgOrKey) + " -" + arg.Short
	}

	// Show description if enabled
	if h.options.ShowDescriptions {
		description := h.mainParser.renderer.FlagDescription(arg)
		if description != "" {
			usage += " \"" + description + "\""
		}
	}

	// Show type if enabled (not in default renderer, but requested by runtime options)
	if h.options.ShowTypes {
		// Convert TypeOf to lowercase string representation
		typeStr := strings.ToLower(arg.TypeOf.String())
		usage += fmt.Sprintf(" (%s)", typeStr)
	}

	// Show default value if enabled
	if h.options.ShowDefaults && arg.DefaultValue != "" {
		// Always use round brackets for consistency
		usage += fmt.Sprintf(" (%s: %s)", h.mainParser.layeredProvider.GetMessage(messages.MsgDefaultsToKey), arg.DefaultValue)
	} else if arg.DefaultValue != "" && h.options.ShowDescriptions {
		// Show default value in standard format when descriptions are on (matching renderer behavior)
		usage += fmt.Sprintf(" (%s: %s)", h.mainParser.layeredProvider.GetMessage(messages.MsgDefaultsToKey), arg.DefaultValue)
	}

	// Show required/optional status
	requiredOrOptional := h.mainParser.layeredProvider.GetMessage(messages.MsgOptionalKey)
	if arg.Required {
		requiredOrOptional = h.mainParser.layeredProvider.GetMessage(messages.MsgRequiredKey)
	} else if arg.RequiredIf != nil {
		requiredOrOptional = h.mainParser.layeredProvider.GetMessage(messages.MsgConditionalKey)
	}
	usage += " (" + requiredOrOptional + ")"

	fmt.Fprintf(writer, " %s\n", usage)
}
