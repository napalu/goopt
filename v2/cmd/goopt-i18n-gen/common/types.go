package common

import "github.com/napalu/goopt/v2/i18n"

// TransformationConfig holds configuration for the TransformationReplacer
type TransformationConfig struct {
	Translator          i18n.Translator
	TrPattern           string
	KeepComments        bool
	CleanComments       bool
	IsUpdateMode        bool
	TransformMode       string // "user-facing", "with-comments", "all-marked", "all"
	BackupDir           string
	PackagePath         string
	VarName             string   // Variable name for generated constants (default: "Keys")
	UserFacingRegex     []string // regex patterns to identify user-facing functions
	FormatFunctionRegex []string // regex patterns with format arg index (pattern:index)
}
