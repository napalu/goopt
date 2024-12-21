// completion/data.go
package completion

// CompletionValue is used to store the flag values for a given flag
type CompletionValue struct {
	Pattern     string // The regex pattern
	Description string // Human-readable description
}

// CompletionData is used to store the completion data for all configured flags and commands
type CompletionData struct {
	Commands            []string
	Flags               []string
	CommandFlags        map[string][]string
	Descriptions        map[string]string
	CommandDescriptions map[string]string
	FlagValues          map[string][]CompletionValue
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
