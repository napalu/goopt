package goopt

import (
	"sort"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/internal/parse"
	"github.com/napalu/goopt/v2/types"
)

// positionalDeclaration represents a declared positional argument
type positionalDeclaration struct {
	key      string
	flag     *FlagInfo
	index    int
	required bool
}

type flagCache struct {
	flags        map[string]map[string]*FlagInfo
	needsValue   map[string]bool
	isStandalone map[string]bool
}

func (p *Parser) buildFlagCache() *flagCache {
	cache := &flagCache{
		flags:        make(map[string]map[string]*FlagInfo),
		needsValue:   make(map[string]bool),
		isStandalone: make(map[string]bool),
	}

	for flag := p.acceptedFlags.Front(); flag != nil; flag = flag.Next() {
		fv := flag.Value
		longName := *flag.Key

		// Extract the base flag name (before @)
		baseName := splitPathFlag(longName)[0]

		if _, exists := cache.flags[baseName]; !exists {
			cache.flags[baseName] = make(map[string]*FlagInfo)
		}

		if shortName := fv.Argument.Short; shortName != "" {
			if _, exists := cache.flags[shortName]; !exists {
				cache.flags[shortName] = make(map[string]*FlagInfo)
			}
		}

		cmdPath := ""
		if fv.CommandPath != "" {
			cmdPath = fv.CommandPath
		}

		cache.flags[baseName][cmdPath] = fv
		if shortName := fv.Argument.Short; shortName != "" {
			cache.flags[shortName][cmdPath] = fv
			cache.isStandalone[shortName] = fv.Argument.TypeOf == types.Standalone
			cache.needsValue[shortName] = fv.Argument.TypeOf != types.Standalone
		}

		// Store needsValue and isStandalone with the full key including command path
		cache.isStandalone[longName] = fv.Argument.TypeOf == types.Standalone
		cache.needsValue[longName] = fv.Argument.TypeOf != types.Standalone

		// Also store with base name for global flags
		if cmdPath == "" {
			cache.isStandalone[baseName] = fv.Argument.TypeOf == types.Standalone
			cache.needsValue[baseName] = fv.Argument.TypeOf != types.Standalone
		}

		// Don't treat positional arguments as flags that need values
		if fv.Argument.Position != nil {
			cache.needsValue[longName] = false
			cache.needsValue[baseName] = false
		}
	}

	return cache
}

// collectDeclaredPositionals extracts all declared positional arguments from accepted flags
func (p *Parser) collectDeclaredPositionals() []positionalDeclaration {
	declaredPos := make([]positionalDeclaration, 0, p.acceptedFlags.Len())

	for flag := p.acceptedFlags.Front(); flag != nil; flag = flag.Next() {
		fv := flag.Value
		if fv.Argument.Position != nil {
			declaredPos = append(declaredPos, positionalDeclaration{
				key:      *flag.Key,
				flag:     fv,
				index:    *fv.Argument.Position,
				required: fv.Argument.Required,
			})
		}
	}

	if len(declaredPos) > 0 {
		sort.SliceStable(declaredPos, func(i, j int) bool {
			return declaredPos[i].index < declaredPos[j].index
		})
	}

	return declaredPos
}

// checkIfFlagNeedsValue determines if a flag requires a value argument
func (p *Parser) checkIfFlagNeedsValue(canonicalName string, currentCmdPath []string, cache *flagCache) bool {
	if len(currentCmdPath) > 0 {
		if flagInfo, exists := cache.flags[canonicalName]; exists {
			cmdPath := strings.Join(currentCmdPath, " ")
			if cmdFlagInfo, cmdExists := flagInfo[cmdPath]; cmdExists {
				// Found flag in command context
				return cmdFlagInfo.Argument.TypeOf != types.Standalone && cmdFlagInfo.Argument.Position == nil
			} else if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
				// Not in command context, but exists as global flag
				return globalFlagInfo.Argument.TypeOf != types.Standalone && globalFlagInfo.Argument.Position == nil
			}
		}
	} else {
		// No command context, check global flags
		if flagInfo, exists := cache.flags[canonicalName]; exists {
			if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
				return globalFlagInfo.Argument.TypeOf != types.Standalone && globalFlagInfo.Argument.Position == nil
			}
		}
	}
	return false
}

// shouldSkipBooleanAfterStandalone checks if current arg should be skipped because
// it's a boolean value consumed by a previous standalone flag
func (p *Parser) shouldSkipBooleanAfterStandalone(args []string, i int, currentCmdPath []string, cache *flagCache) bool {
	if i == 0 || !p.isFlag(args[i-1]) {
		return false
	}

	prevName := strings.TrimFunc(args[i-1], p.prefixFunc)

	// Try to get canonical name from translation registry
	canonicalPrevName := prevName
	if canonical, ok := p.translationRegistry.GetCanonicalFlagName(prevName, p.GetLanguage()); ok {
		canonicalPrevName = canonical
	}

	// Check if previous flag was standalone
	isStandalone := false
	if len(currentCmdPath) > 0 {
		if flagInfo, exists := cache.flags[canonicalPrevName]; exists {
			cmdPath := strings.Join(currentCmdPath, " ")
			if cmdFlagInfo, cmdExists := flagInfo[cmdPath]; cmdExists {
				isStandalone = cmdFlagInfo.Argument.TypeOf == types.Standalone
			} else if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
				isStandalone = globalFlagInfo.Argument.TypeOf == types.Standalone
			}
		}
	} else {
		if flagInfo, exists := cache.flags[canonicalPrevName]; exists {
			if globalFlagInfo, globalExists := flagInfo[""]; globalExists {
				isStandalone = globalFlagInfo.Argument.TypeOf == types.Standalone
			}
		}
	}

	// If previous was standalone and current is a valid boolean, skip it
	if isStandalone {
		if _, err := strconv.ParseBool(args[i]); err == nil {
			return true
		}
	}

	return false
}

// updateCommandPath updates the current command path based on the argument
// Returns true if the argument was a command
func (p *Parser) updateCommandPath(arg string, currentCmdPath *[]string, argPos *int, executedCommands map[string]bool) bool {
	isCmd := false
	canonicalArg := arg

	// Get canonical command name if it's a translated command
	if canonical, ok := p.translationRegistry.GetCanonicalCommandPath(arg, p.GetLanguage()); ok {
		canonicalArg = canonical
	}

	if len(*currentCmdPath) == 0 {
		if p.isCommand(arg) {
			*currentCmdPath = append(*currentCmdPath, canonicalArg)
			isCmd = true
			*argPos = 0 // Reset position counter for new command
		}
	} else {
		switch {
		case p.isCommand(strings.Join(append(*currentCmdPath, arg), " ")):
			*currentCmdPath = append(*currentCmdPath, canonicalArg)
			isCmd = true
		case p.isCommand(arg):
			*currentCmdPath = []string{canonicalArg}
			isCmd = true
			*argPos = 0
		}
	}

	if isCmd {
		// Mark this command path as executed
		executedCommands[strings.Join(*currentCmdPath, " ")] = true
	}

	return isCmd
}

// matchPositionalArgument matches an argument to its declared positional and processes it
// Returns whether this positional should be skipped
func (p *Parser) matchPositionalArgument(pa *PositionalArgument, cmdPath string, argPos int,
	declaredPos []positionalDeclaration, arg string) bool {

	skipThisPositional := false

	for _, decl := range declaredPos {
		// Check if this declaration belongs to the current command path
		if decl.flag.CommandPath == cmdPath && *decl.flag.Argument.Position == argPos {
			lookup := buildPathFlag(decl.key, decl.flag.CommandPath)

			// Check if this flag was already explicitly set via flag syntax
			if _, alreadySet := p.options[lookup]; alreadySet {
				// This positional slot was filled via explicit flag syntax
				// Don't treat this value as a positional argument
				skipThisPositional = true
				continue
			}

			pa.Argument = decl.flag.Argument

			// Run validators on positional argument
			if len(decl.flag.Argument.Validators) > 0 {
				for _, validator := range decl.flag.Argument.Validators {
					if err := validator(arg); err != nil {
						p.addError(errs.WrapOnce(err, errs.ErrProcessingFlag, lookup))
					}
				}
			}

			p.registerFlagValue(lookup, arg, arg)
			p.options[lookup] = arg
			if err := p.setBoundVariable(arg, lookup); err != nil {
				p.addError(errs.ErrSettingBoundValue.WithArgs(lookup).Wrap(err))
			}
			break
		}
	}

	return skipThisPositional
}

// validateMissingPositionals checks for missing required positionals and applies defaults
func (p *Parser) validateMissingPositionals(positional *[]PositionalArgument,
	declaredPos []positionalDeclaration, executedCommands map[string]bool) {

	for _, decl := range declaredPos {
		// Only check positionals for commands that were actually executed
		if !executedCommands[decl.flag.CommandPath] {
			continue
		}

		// Check if this positional was provided
		found := false
		for _, pos := range *positional {
			if pos.Argument == decl.flag.Argument {
				found = true
				break
			}
		}

		if !found {
			// Check if a flag was provided that takes precedence
			flagFromArg := p.flagOrShortFlag(decl.key, decl.flag.CommandPath)
			if p.HasRawFlag(flagFromArg) {
				continue
			}

			if decl.flag.Argument.DefaultValue != "" {
				// Apply default value
				lookup := buildPathFlag(decl.key, decl.flag.CommandPath)
				p.registerFlagValue(lookup, decl.flag.Argument.DefaultValue, decl.flag.Argument.DefaultValue)
				p.options[lookup] = decl.flag.Argument.DefaultValue
				if err := p.setBoundVariable(decl.flag.Argument.DefaultValue, lookup); err != nil {
					p.addError(errs.ErrSettingBoundValue.WithArgs(lookup).Wrap(err))
				}

				// Add a positional argument entry for the default value
				*positional = append(*positional, PositionalArgument{
					Position: decl.index, // Use the declared position
					Value:    decl.flag.Argument.DefaultValue,
					Argument: decl.flag.Argument,
					ArgPos:   decl.index,
				})
			} else if decl.required {
				// Missing required positional
				p.addError(errs.ErrRequiredPositionalFlag.WithArgs(decl.key, decl.index))
			}
		}
	}
}

// filterAndSortPositionals filters out empty positionals and sorts by position
func filterAndSortPositionals(positional []PositionalArgument) []PositionalArgument {
	newResult := make([]PositionalArgument, 0, len(positional))

	for i := range positional {
		// Keep positions that either:
		// 1. Have a non-empty value
		// 2. Are unbound (Argument == nil) and have any value
		if positional[i].Value != "" {
			newResult = append(newResult, positional[i])
		}
	}

	// Sort by position to maintain order
	sort.Slice(newResult, func(i, j int) bool {
		return newResult[i].Position < newResult[j].Position
	})

	return newResult
}

// setPositionalArgumentsRefactored is the modularized version of setPositionalArguments
func (p *Parser) setPositionalArguments(state parse.State) {
	args := state.Args()
	positional := make([]PositionalArgument, 0, len(args))
	cache := p.buildFlagCache()

	// Step 1: Collect all declared positionals
	declaredPos := p.collectDeclaredPositionals()

	// Step 2: Process command line arguments
	skipNext := false
	currentCmdPath := make([]string, 0, 3) // Pre-allocate for typical command depth
	argPos := 0
	executedCommands := make(map[string]bool) // Track which commands were encountered
	executedCommands[""] = true               // Always check global positionals

	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		// Handle flags
		if p.isFlag(arg) {
			name := strings.TrimLeft(arg, "-")
			// Try to get canonical name from translation registry
			canonicalName := name
			if canonical, ok := p.translationRegistry.GetCanonicalFlagName(name, p.GetLanguage()); ok {
				canonicalName = canonical
			}

			// Check if flag is known (exists in cache)
			isKnownFlag := false
			if flagInfo, exists := cache.flags[canonicalName]; exists {
				cmdPath := strings.Join(currentCmdPath, " ")
				_, cmdExists := flagInfo[cmdPath]
				_, globalExists := flagInfo[""]
				isKnownFlag = cmdExists || globalExists
			}

			// If unknown flag and treatUnknownAsPositionals is enabled, don't skip it
			if !isKnownFlag && p.treatUnknownAsPositionals {
				// Treat as positional - fall through to positional handling
			} else {
				// Check if this flag needs a value
				if p.checkIfFlagNeedsValue(canonicalName, currentCmdPath, cache) {
					skipNext = true
				}
				continue
			}
		}

		// Handle standalone flags with boolean values
		if p.shouldSkipBooleanAfterStandalone(args, i, currentCmdPath, cache) {
			continue
		}

		// Check if argument is a command
		if p.updateCommandPath(arg, &currentCmdPath, &argPos, executedCommands) {
			continue
		}

		// Process as positional argument
		pa := PositionalArgument{
			Position: i,
			Value:    arg,
			Argument: nil,
			ArgPos:   argPos,
		}

		cmdPath := strings.Join(currentCmdPath, " ")
		skipThisPositional := p.matchPositionalArgument(&pa, cmdPath, argPos, declaredPos, arg)

		if !skipThisPositional {
			positional = append(positional, pa)
		}
		argPos++
	}

	// Step 3: Validate missing positionals and apply defaults
	p.validateMissingPositionals(&positional, declaredPos, executedCommands)

	// Step 4: Filter and sort results
	p.positionalArgs = filterAndSortPositionals(positional)
}
