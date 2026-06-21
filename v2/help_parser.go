package goopt

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/internal/messages"
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
	Style            string   `goopt:"desc:Help style;validators:isoneof(flat,grouped,grouped-clean,compact,hierarchical,smart)"`

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
		case "grouped-clean":
			h.config.Style = HelpStyleGroupedClean
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
		return h.showExamples(writer)
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

	// Collect all non-flag, non-keyword strings from anywhere in the args.
	// These are command path components (e.g., "keyfile" "rekey").
	// We need to collect them regardless of position because pos:0 on a []string
	// only captures the first positional arg — multiple separate args get lost.
	commands := []string{}
	flags := []string{}
	for i, arg := range args {
		if h.hp.isFlag(arg) {
			if !h.isHelpArg(arg) {
				flags = append(flags, arg)
				// If this flag needs a value, also capture the next arg
				if i+1 < len(args) && !h.hp.isFlag(args[i+1]) {
					flags = append(flags, args[i+1])
				}
			}
			continue
		}
		// Skip values already consumed by flags above
		if i > 0 && h.hp.isFlag(args[i-1]) && !h.isHelpArg(args[i-1]) {
			continue
		}
		if !isHelpKeyword(arg) {
			commands = append(commands, arg)
		}
	}

	// Build new args
	newArgs := []string{}

	// Add commands as a comma-separated positional argument so pos:0 []string works
	if len(commands) > 0 {
		newArgs = append(newArgs, strings.Join(commands, ","))
	}

	// Add the non-help flags
	newArgs = append(newArgs, flags...)

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
		// Render each suggestion in the form closest to user input (shared with the
		// parse path so the two never diverge).
		displaySuggestions := h.mainParser.localizeSuggestions(invalidCmd, suggestions, func(key string) (string, bool) {
			p := h.mainParser
			if p.translationRegistry == nil {
				return "", false
			}
			if cmd, found := p.registeredCommands.Get(key); found && cmd.NameKey != "" {
				return p.translationRegistry.GetCommandTranslation(key, p.GetLanguage())
			}
			return "", false
		})

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

	// Find similar subcommands — shared with the parse path so suggestions are
	// i18n-aware (a typo of a localized subcommand name is matched and shown in the
	// user's language) rather than canonical-only.
	suggestions := h.mainParser.suggestSubcommands(cmd.Subcommands, invalidSub, commandPath)

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

// findSimilarCommandsWithContext finds commands anywhere in the command tree that
// resemble input (canonical or translated names), honoring the configured command
// suggestion threshold via the shared ranking core. Walks each registered command
// and its subcommands, keying suggestions by full path.
func (h *HelpParser) findSimilarCommandsWithContext(input string) ([]string, bool) {
	p := h.mainParser
	currentLang := p.GetLanguage()
	var items []suggestionItem
	var add func(path string, cmd *Command)
	add = func(path string, cmd *Command) {
		it := suggestionItem{key: path, names: []string{cmd.Name}}
		if p.translationRegistry != nil && cmd.NameKey != "" {
			if t, found := p.translationRegistry.GetCommandTranslation(path, currentLang); found {
				it.i18n = append(it.i18n, t)
			}
		}
		items = append(items, it)
		for i := range cmd.Subcommands {
			add(path+" "+cmd.Subcommands[i].Name, &cmd.Subcommands[i])
		}
	}
	for _, cmd := range p.registeredCommands.All() {
		add(cmd.Name, cmd)
	}
	return p.rankSuggestions(input, items, p.cmdSuggestionThreshold, 3)
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

	cfg := h.effectiveConfig()
	for _, flag := range filtered {
		fmt.Fprintf(writer, " %s\n", h.mainParser.renderer.FlagUsageWithConfig(flag, cfg))
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

	cfg := h.effectiveConfig()
	for _, flag := range filtered {
		fmt.Fprintf(writer, " %s\n", h.mainParser.renderer.FlagUsageWithConfig(flag, cfg))
	}

	return nil
}

// showExamples shows usage examples
func (h *HelpParser) showExamples(writer io.Writer) error {
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
		style = h.mainParser.detectBestStyle()
	}

	// Apply the selected style
	switch style {
	case HelpStyleFlat:
		return h.showFlatStyle(writer)
	case HelpStyleGrouped, HelpStyleGroupedClean:
		return h.showGroupedStyle(writer)
	case HelpStyleCompact:
		return h.showCompactStyle(writer)
	case HelpStyleHierarchical:
		return h.showHierarchicalStyle(writer)
	default:
		return h.showFlatStyle(writer)
	}
}

// showFlatStyle shows traditional flat help. It renders each flag through the SAME
// shared renderer as p.PrintHelp (DefaultRenderer.FlagUsageWithConfig) so the two
// entry points cannot drift — the runtime --help options are folded into a HelpConfig
// (effectiveConfig) and passed in, rather than re-implementing the flag line.
func (h *HelpParser) showFlatStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Show usage line
	fmt.Fprintln(writer, h.mainParser.layeredProvider.GetFormattedMessage(messages.MsgUsageKey, os.Args[0]))

	// Show positional args if any
	h.mainParser.PrintPositionalArgs(writer)

	// Show flags via the shared renderer, honoring runtime --help options.
	flags := h.filterFlags(h.collectFlags(""))
	if len(flags) > 0 {
		cfg := h.effectiveConfig()
		for _, flag := range flags {
			fmt.Fprintf(writer, " %s\n", h.mainParser.renderer.FlagUsageWithConfig(flag, cfg))
		}
	}

	// Show commands
	if h.mainParser.registeredCommands.Len() > 0 {
		fmt.Fprintf(writer, "\n%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey))
		h.mainParser.PrintCommands(writer)
	}

	return nil
}

// effectiveConfig folds the runtime --help options onto the parser's HelpConfig so the
// shared renderer reflects them. Runtime flags are applied as DELTAS on top of the
// configured base (which carries ShowRequired etc.), keeping --help aligned with
// p.PrintHelp by construction while still honoring --no-desc / --show-types / etc. It
// drives flag rendering for every --help mode (flat, --globals, --flags, --all).
func (h *HelpParser) effectiveConfig() HelpConfig {
	cfg := h.config
	if h.options.NoDescriptions {
		cfg.ShowDescription = false
	}
	if h.options.NoShort {
		cfg.ShowShortFlags = false
	}
	if h.options.ShowDefaults {
		cfg.ShowDefaults = true
	}
	if h.options.ShowTypes {
		cfg.ShowTypes = true
	}
	if h.options.ShowValidators {
		cfg.ShowValidators = true
	}
	return cfg
}

// showGroupedStyle shows help with flags grouped by command
func (h *HelpParser) showGroupedStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Use PrintUsageWithGroups which shows "Global Flags:" header
	h.mainParser.PrintUsageWithGroups(writer)
	return nil
}

// showGroupedCleanStyle shows grouped help with clean, compact formatting (no ** markers, tighter spacing)
func (h *HelpParser) showGroupedCleanStyle(writer io.Writer) error {
	h.showVersionHeader(writer)

	// Use PrintUsageWithGroups with clean pretty print config
	cleanConfig := &PrettyPrintConfig{
		NewCommandPrefix:     " +  ",
		DefaultPrefix:        " ├─ ",
		TerminalPrefix:       " └─ ",
		InnerLevelBindPrefix: "  ", // 2 spaces - clean and compact
		OuterLevelBindPrefix: " │  ",
	}
	h.mainParser.PrintUsageWithGroups(writer, cleanConfig)
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
		slices.SortFunc(groups, func(a, b groupInfo) int {
			return cmp.Compare(b.cmdCount, a.cmdCount)
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
			cfg := h.effectiveConfig()
			for _, flag := range globalFlags {
				fmt.Fprintf(writer, " %s\n", h.mainParser.renderer.FlagUsageWithConfig(flag, cfg))
			}
			fmt.Fprintln(writer)
		}

		// Show all commands with their flags
		fmt.Fprintf(writer, "%s:\n", h.mainParser.layeredProvider.GetMessage(messages.MsgCommandsKey))
		h.mainParser.printCommandTree(writer)
		fmt.Fprintln(writer)

		// Show examples
		err = h.showExamples(writer)
	} else {
		// Show detailed command help
		err = h.renderCommandHelp(writer, commandPath)
		if err != nil {
			return err
		}
		fmt.Fprintln(writer)

		// Show command-specific examples
		err = h.showExamples(writer)
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

	for _, flagInfo := range h.mainParser.acceptedFlags.All() {
		// Skip positional arguments - they are shown inline with commands
		if flagInfo.Argument.isPositional() {
			continue
		}

		if commandPath == "" || flagInfo.CommandPath == commandPath {
			flags = append(flags, flagInfo.Argument)
		}
	}

	return flags
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
	for flagName, flagInfo := range h.mainParser.acceptedFlags.All() {
		arg := flagInfo.Argument
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
				Context:     flagInfo.CommandPath,
			})
		}
	}

	// Search commands
	for _, cmd := range h.mainParser.registeredCommands.All() {
		cmdName := cmd.Name
		desc := h.mainParser.renderer.CommandDescription(cmd)

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
				Context:     cmd.path,
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
	for i := range len(args) {
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
	fmt.Fprintf(writer, "    %s: flat, grouped, grouped-clean, compact, hierarchical, smart\n\n", h.mainParser.layeredProvider.GetMessage(messages.MsgAvailableStylesKey))

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
