package types

import (
	"regexp"
)

// OptionType used to define Flag types (such as Standalone, Single, Chained)
type OptionType int

// String returns the string representation of an OptionType
func (o OptionType) String() string {
	switch o {
	case Standalone:
		return "standalone"
	case Single:
		return "single"
	case Chained:
		return "chained"
	case File:
		return "file"
	case Empty:
		fallthrough
	default:
		return "empty"
	}
}

const (
	Empty      OptionType = iota // Empty denotes a Flag which is not set
	Single     OptionType = 1    // Single denotes a Flag accepting a string value
	Chained    OptionType = 2    // Chained denotes a Flag accepting a string value which should be evaluated as a list (split on ' ', '|' and ',')
	Standalone OptionType = 3    // Standalone denotes a boolean Flag (does not accept a value)
	File       OptionType = 4    // File denotes a Flag which is evaluated as a path (the content of the file is treated as the value)
)

// PatternValue is used to define an acceptable value for a Flag. The 'pattern' argument is compiled to a regular expression
// and the description argument is used to provide a human-readable description of the pattern.
type PatternValue struct {
	Pattern     string
	Description string
	Compiled    *regexp.Regexp
}

// KeyValue denotes Key Value pairs
type KeyValue[K, V any] struct {
	Key   K
	Value V
}

// Secure set to Secure to true to solicit non-echoed user input from stdin.
// If Prompt is empty a "password :" prompt will be displayed. Set to the desired value to override.
type Secure struct {
	IsSecure bool
	Prompt   string
}

// Kind is used to define the kind of entity a struct tag represents
type Kind string

const (
	KindFlag    Kind = "flag"
	KindCommand Kind = "command"
	KindEmpty   Kind = ""
)

// TagConfig is used to store struct tag information about a flag or command
type TagConfig struct {
	Kind           Kind
	Name           string
	NameKey        string // Translation key for the flag/command name
	Short          string
	TypeOf         OptionType
	Description    string
	DescriptionKey string
	Default        string
	Required       bool
	Secure         Secure
	Path           string
	AcceptedValues []PatternValue
	DependsOn      map[string][]string
	Capacity       int
	Position       *int
	Validators     []string // List of validator specifications
}

// Describe a PatternValue (regular expression with a human-readable explanation of the pattern)
func (r *PatternValue) Describe() string {
	if len(r.Description) > 0 {
		return r.Description
	}

	return r.Pattern
}

// ListDelimiterFunc signature to match when supplying a user-defined function to check for the runes which form list delimiters.
// Defaults to ',' || r == '|' || r == ' '.
type ListDelimiterFunc func(matchOn rune) bool

// EnvGetter is a function type for getting environment variables (mockable for testing)
type EnvGetter func(string) string
