// completion/data.go
package completion

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
