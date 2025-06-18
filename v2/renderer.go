package goopt

import (
	"fmt"

	"github.com/napalu/goopt/v2/internal/messages"
)

type DefaultRenderer struct {
	parser *Parser
}

func NewRenderer(parser *Parser) *DefaultRenderer {
	return &DefaultRenderer{parser: parser}
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
// This method respects the HelpConfig settings.
func (r *DefaultRenderer) FlagUsage(f *Argument) string {
	var usage string
	config := r.parser.GetHelpConfig()

	usage = "--" + r.FlagName(f)
	if f.Short != "" && config.ShowShortFlags {
		usage += " " + r.parser.layeredProvider.GetMessage(messages.MsgOrKey) + " -" + f.Short
	}

	if config.ShowDescription {
		description := r.FlagDescription(f)
		if description != "" {
			usage += " \"" + description + "\""
		}
	}

	if f.DefaultValue != "" && config.ShowDefaults {
		usage += fmt.Sprintf(" (%s: %s)", r.parser.layeredProvider.GetMessage(messages.MsgDefaultsToKey), f.DefaultValue)
	}

	if config.ShowRequired {
		requiredOrOptional := r.parser.layeredProvider.GetMessage(messages.MsgOptionalKey)
		if f.Required {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgRequiredKey)
		} else if f.RequiredIf != nil {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgConditionalKey)
		}
		usage += " (" + requiredOrOptional + ")"
	}

	return usage
}

// CommandUsage generates a usage string for a given command.
// The usage string includes the command name, description, and any subcommands.
// This method respects the HelpConfig settings.
func (r *DefaultRenderer) CommandUsage(c *Command) string {
	var usage string
	config := r.parser.GetHelpConfig()

	usage = r.CommandName(c)

	if config.ShowDescription {
		description := r.CommandDescription(c)
		if description != "" {
			usage += " \"" + description + "\""
		}
	}

	return usage
}
