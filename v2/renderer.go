package goopt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/internal/messages"
	"github.com/napalu/goopt/v2/types"
)

type DefaultRenderer struct {
	parser       *Parser
	rtlLanguages map[string]bool
}

func NewRenderer(parser *Parser) *DefaultRenderer {
	return &DefaultRenderer{
		parser: parser,
		rtlLanguages: map[string]bool{
			"ar":  true, // Arabic
			"he":  true, // Hebrew
			"fa":  true, // Persian/Farsi
			"ur":  true, // Urdu
			"ps":  true, // Pashto
			"sd":  true, // Sindhi
			"dv":  true, // Dhivehi/Maldivian
			"yi":  true, // Yiddish
			"ku":  true, // Kurdish (Sorani)
			"arc": true, // Aramaic
		},
	}
}

// isRTLLanguage checks if the current language is RTL
func (r *DefaultRenderer) isRTLLanguage() bool {
	langTag := r.parser.GetLanguage()

	// First try the base language using the proper API
	base, _ := langTag.Base()
	if base.String() != "und" {
		if r.rtlLanguages[base.String()] {
			return true
		}
	}

	// Fall back to parsing the string representation
	lang := langTag.String()

	// Handle the -u-rg- format that language matcher might return
	if idx := strings.Index(lang, "-u-"); idx > 0 {
		lang = lang[:idx]
	}

	// Check base language code (e.g., "ar" from "ar-SA")
	if idx := strings.Index(lang, "-"); idx > 0 {
		lang = lang[:idx]
	}

	return r.rtlLanguages[lang]
}

// containsRTLRunes checks if a string contains RTL characters
func (r *DefaultRenderer) containsRTLRunes(s string) bool {
	for _, ch := range s {
		// Check for common RTL Unicode blocks
		if (ch >= 0x0590 && ch <= 0x05FF) || // Hebrew
			(ch >= 0x0600 && ch <= 0x06FF) || // Arabic
			(ch >= 0x0700 && ch <= 0x074F) || // Syriac
			(ch >= 0x0750 && ch <= 0x077F) || // Arabic Supplement
			(ch >= 0x08A0 && ch <= 0x08FF) || // Arabic Extended-A
			(ch >= 0xFB50 && ch <= 0xFDFF) || // Arabic Presentation Forms-A
			(ch >= 0xFE70 && ch <= 0xFEFF) { // Arabic Presentation Forms-B
			return true
		}
	}
	return false
}

// FlagName returns the name of the flag to display in help messages.
// It first checks if a translation key is defined for the flag and returns the translated string if it exists.
// Otherwise, it retrieves the long name of the flag and returns it.
// If the long name contains a comand-path, it only returns the flag part of the path.
func (r *DefaultRenderer) FlagName(f *Argument) string {
	if f.NameKey != "" {
		return r.parser.layeredProvider.GetMessage(f.NameKey)
	}

	longName := f.GetLongName(r.parser)
	if longName != "" {
		longName = splitPathFlag(longName)[0]
	}

	return longName
}

// FlagDescription returns the description of the given flag.
// If the flag has a DescriptionKey, it uses the parser's internationalization
// function to translate the key into the appropriate description.
// Otherwise, it returns the flag's Description field.
func (r *DefaultRenderer) FlagDescription(f *Argument) string {
	if f.DescriptionKey == "" {
		return f.Description
	}

	return r.parser.layeredProvider.GetMessage(f.DescriptionKey)
}

func (r *DefaultRenderer) CommandName(c *Command) string {
	if c.NameKey != "" {
		return r.parser.layeredProvider.GetMessage(c.NameKey)
	}

	return c.Name
}

// CommandDescription returns the description of the given command.
// If the command has a DescriptionKey, it uses the parser's internationalization
// function to translate the key into the appropriate description.
// Otherwise, it returns the command's Description field.
func (r *DefaultRenderer) CommandDescription(c *Command) string {
	if c.DescriptionKey == "" {
		return c.Description
	}

	return r.parser.layeredProvider.GetMessage(c.DescriptionKey)
}

// FlagUsage generates a usage string for a given command-line argument.
// The usage string includes the flag name, short name (if available), description,
// default value (if any), and whether the flag is required, optional, or conditional.
// This method respects the HelpConfig settings and automatically handles RTL languages.
func (r *DefaultRenderer) FlagUsage(f *Argument) string {
	config := r.parser.GetHelpConfig()
	isRTL := r.isRTLLanguage()

	// Get the flag name (potentially translated)
	flagName := r.FlagName(f)

	// Build the flag representation
	var flagPart string
	if f.Short != "" && config.ShowShortFlags {
		if isRTL || r.containsRTLRunes(flagName) {
			// In RTL languages, use slash separator
			flagPart = fmt.Sprintf("--%s / -%s", flagName, f.Short)
		} else {
			// In LTR languages, use "or" separator for backward compatibility
			orMsg := r.parser.layeredProvider.GetMessage(messages.MsgOrKey)
			flagPart = fmt.Sprintf("--%s %s -%s", flagName, orMsg, f.Short)
		}
	} else {
		flagPart = "--" + flagName
	}

	// Build the description part
	var parts []string

	if config.ShowDescription {
		description := r.FlagDescription(f)
		if description != "" {
			// Keep quotes for backward compatibility in LTR
			if isRTL {
				parts = append(parts, description)
			} else {
				parts = append(parts, "\""+description+"\"")
			}
		}
	}

	if f.DefaultValue != "" && config.ShowDefaults {
		// Format numeric default values according to locale
		formattedDefault := r.formatDefaultValue(f)
		defaultMsg := fmt.Sprintf("(%s: %s)",
			r.parser.layeredProvider.GetMessage(messages.MsgDefaultsToKey),
			formattedDefault)
		parts = append(parts, defaultMsg)
	}

	if config.ShowRequired {
		requiredOrOptional := r.parser.layeredProvider.GetMessage(messages.MsgOptionalKey)
		if f.Required {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgRequiredKey)
		} else if f.RequiredIf != nil {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgConditionalKey)
		}
		parts = append(parts, "("+requiredOrOptional+")")
	}

	// Format based on RTL/LTR
	if isRTL {
		// In RTL, description comes first, then the flag
		if len(parts) > 0 {
			return strings.Join(parts, " ") + " " + flagPart
		}
		return flagPart
	} else {
		// In LTR, flag comes first, then description
		if len(parts) > 0 {
			return flagPart + " " + strings.Join(parts, " ")
		}
		return flagPart
	}
}

// CommandUsage generates a usage string for a given command.
// The usage string includes the command name, description, and any subcommands.
// This method respects the HelpConfig settings and automatically handles RTL languages.
func (r *DefaultRenderer) CommandUsage(c *Command) string {
	config := r.parser.GetHelpConfig()
	isRTL := r.isRTLLanguage()

	cmdName := r.CommandName(c)

	if config.ShowDescription {
		description := r.CommandDescription(c)
		if description != "" {
			if isRTL || r.containsRTLRunes(cmdName) || r.containsRTLRunes(description) {
				// In RTL, description comes first
				return description + " :" + cmdName
			} else {
				// In LTR, command comes first (keep original format with quotes)
				return cmdName + " \"" + description + "\""
			}
		}
	}

	return cmdName
}

// formatDefaultValue formats a default value according to locale and type
func (r *DefaultRenderer) formatDefaultValue(f *Argument) string {
	// Try to detect numeric types and format accordingly
	switch f.TypeOf {
	case types.Single:
		// Try to parse as int
		if intVal, err := strconv.Atoi(f.DefaultValue); err == nil {
			return r.parser.layeredProvider.FormatInt(intVal)
		}
		// Try to parse as float
		if floatVal, err := strconv.ParseFloat(f.DefaultValue, 64); err == nil {
			// Determine precision from original string
			precision := 2
			if idx := strings.Index(f.DefaultValue, "."); idx >= 0 {
				precision = len(f.DefaultValue) - idx - 1
			}
			return r.parser.layeredProvider.FormatFloat(floatVal, precision)
		}
	}
	// Return as-is for non-numeric or other types
	return f.DefaultValue
}
