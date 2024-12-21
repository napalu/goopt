// completion/data.go
package completion

type CompletionValue struct {
	Pattern     string // The regex pattern
	Description string // Human-readable description
}

type CompletionData struct {
	Commands            []string
	Flags               []string
	CommandFlags        map[string][]string
	Descriptions        map[string]string
	CommandDescriptions map[string]string
	FlagValues          map[string][]CompletionValue
}
