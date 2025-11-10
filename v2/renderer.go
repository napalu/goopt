package goopt

import (
	"fmt"
	"github.com/napalu/goopt/v2/i18n"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/internal/messages"
	"github.com/napalu/goopt/v2/types"
)

type DefaultRenderer struct {
	parser *Parser
}

func NewRenderer(parser *Parser) *DefaultRenderer {
	return &DefaultRenderer{
		parser: parser,
	}
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
	isRTL := i18n.IsRTL(r.parser.GetLanguage())

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
	isRTL := i18n.IsRTL(r.parser.GetLanguage())

	// Use the full command path for proper hierarchy display, or fall back to name
	cmdName := c.path
	if cmdName == "" {
		cmdName = r.CommandName(c)
	}

	// Get positional arguments for this command
	positionals := r.parser.getPositionalsForCommand(c.path)

	// Build command usage with positionals
	usageLine := cmdName
	if len(positionals) > 0 {
		for _, pos := range positionals {
			// Extract just the flag name without the command path
			flagName := pos.Value
			if idx := strings.LastIndex(flagName, "@"); idx >= 0 {
				flagName = flagName[:idx]
			}

			// Format as <name> for required or [name] for optional
			if pos.Argument.Required {
				usageLine += " <" + flagName + ">"
			} else {
				usageLine += " [" + flagName + "]"
			}
		}
	}

	if config.ShowDescription {
		description := r.CommandDescription(c)
		if description != "" {
			if isRTL || r.containsRTLRunes(cmdName) || r.containsRTLRunes(description) {
				// In RTL, description comes first
				return description + " :" + usageLine
			} else {
				// In LTR, command comes first (keep original format with quotes)
				return usageLine + " \"" + description + "\""
			}
		}
	}

	return usageLine
}

// PositionalUsage generates a usage string for a positional argument.
// The format is similar to FlagUsage but without the -- prefix:
// name "description" (required/optional)
func (r *DefaultRenderer) PositionalUsage(f *Argument, position int) string {
	config := r.parser.GetHelpConfig()
	isRTL := i18n.IsRTL(r.parser.GetLanguage())

	// Get the flag name (positionals use flag storage internally)
	flagName := r.FlagName(f)

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

	if config.ShowRequired {
		requiredOrOptional := r.parser.layeredProvider.GetMessage(messages.MsgOptionalKey)
		if f.Required {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgRequiredKey)
		}
		parts = append(parts, "("+requiredOrOptional+")")
	}

	// Format based on RTL/LTR
	if isRTL {
		// In RTL, description comes first, then the name
		if len(parts) > 0 {
			return strings.Join(parts, " ") + " " + flagName
		}
		return flagName
	} else {
		// In LTR, name comes first, then description
		if len(parts) > 0 {
			return flagName + " " + strings.Join(parts, " ")
		}
		return flagName
	}
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
