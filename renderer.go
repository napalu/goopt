package goopt

import (
	"fmt"

	"github.com/napalu/goopt/internal/messages"
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
		return r.parser.i18n.T(f.NameKey)
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

	return r.parser.i18n.T(f.DescriptionKey)
}

func (r *DefaultRenderer) CommandName(c *Command) string {
	if c.NameKey != "" {
		return r.parser.i18n.T(c.NameKey)
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

	return r.parser.i18n.T(c.DescriptionKey)
}

// FlagUsage generates a usage string for a given command-line argument.
// The usage string includes the flag name, short name (if available), description,
// default value (if any), and whether the flag is required, optional, or conditional.
func (r *DefaultRenderer) FlagUsage(f *Argument) string {
	var usage string

	usage = "--" + r.FlagName(f)
	if f.Short != "" {
		usage += " " + r.parser.i18n.T(messages.MsgOrKey) + " -" + f.Short
	}

	description := r.FlagDescription(f)
	if description != "" {
		usage += " \"" + description + "\""
	}

	if f.DefaultValue != "" {
		usage += fmt.Sprintf(" (%s: %s)", r.parser.i18n.T(messages.MsgDefaultsToKey), f.DefaultValue)
	}

	requiredOrOptional := r.parser.i18n.T(messages.MsgOptionalKey)
	if f.Required {
		requiredOrOptional = r.parser.i18n.T(messages.MsgRequiredKey)
	} else if f.RequiredIf != nil {
		requiredOrOptional = r.parser.i18n.T(messages.MsgConditionalKey)
	}

	return usage + " (" + requiredOrOptional + ")"
}

// CommandUsage generates a usage string for a given command.
// The usage string includes the command name, description, and any subcommands.
func (r *DefaultRenderer) CommandUsage(c *Command) string {
	var usage string

	usage = r.CommandName(c)
	usage += " \"" + r.CommandDescription(c) + "\""

	return usage
}
