// completion/data.go
package completion

// CompletionValue represents a possible value for a flag with its description
type CompletionValue struct {
	Pattern     string // The regex pattern
	Description string // Human-readable description
}

// FlagType mirrors goopt.OptionType but keeps packages decoupled
type FlagType int

const (
	// Single denotes a flag accepting a string value
	FlagTypeSingle FlagType = 0
	// Chained denotes a flag accepting a string value which should be evaluated as a list
	FlagTypeChained FlagType = 1
	// Standalone denotes a boolean flag (does not accept a value)
	FlagTypeStandalone FlagType = 2
	// File denotes a flag which is evaluated as a path
	FlagTypeFile FlagType = 3
)

// FlagPair represents a short and long version of the same flag
type FlagPair struct {
	Short       string
	Long        string
	Description string
	Type        FlagType
}

// CompletionData holds all the data needed for shell completion
type CompletionData struct {
	Commands            []string
	Flags               []FlagPair
	CommandFlags        map[string][]FlagPair
	FlagValues          map[string][]CompletionValue
	CommandDescriptions map[string]string

	// I18n support - maps canonical names to translated names
	TranslatedCommands map[string]string // canonical -> translated
	TranslatedFlags    map[string]string // canonical -> translated

	// I18n support for descriptions (for testing)
	TranslatedCommandDescriptions map[string]string // canonical -> translated description
	TranslatedFlagDescriptions    map[string]string // canonical -> translated description
}

// CompletionPaths holds information about completion script locations
type CompletionPaths struct {
	Primary   string // Main completion path
	Fallback  string // Alternative path if primary isn't available
	Extension string // File extension for completion script (if any)
	Comment   string // Documentation about the path choice
}

// CompletionFileInfo holds shell-specific naming conventions
type CompletionFileInfo struct {
	Prefix    string // Some shells require specific prefixes
	Extension string // File extension if required
	Comment   string // Documentation about the naming convention
}
