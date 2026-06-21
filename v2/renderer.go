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

// FlagUsage generates a usage string for a given command-line argument using the
// parser's current HelpConfig. The usage string includes the flag name, short name
// (if available), description, default value (if any), and whether the flag is
// required, optional, or conditional. This method respects the HelpConfig settings
// and automatically handles RTL languages.
func (r *DefaultRenderer) FlagUsage(f *Argument) string {
	return r.FlagUsageWithConfig(f, r.parser.GetHelpConfig())
}

// FlagUsageWithConfig renders a flag line under an explicit HelpConfig. It is the
// single flag-line renderer: PrintHelp/PrintUsage pass the parser config, and the
// runtime help system (--help) passes a config derived from its runtime options.
// Keeping both entry points here is what stops the flat --help path from drifting
// back into a hand-rolled renderer that ignored ShowRequired and the bidi handling.
func (r *DefaultRenderer) FlagUsageWithConfig(f *Argument, config HelpConfig) string {
	isRTLLocale := i18n.IsRTL(r.parser.GetLanguage())

	// Get the flag name (potentially translated)
	flagName := r.FlagName(f)
	description := ""
	if config.ShowDescription {
		description = r.FlagDescription(f)
	}

	// One direction decision drives the whole line (separator, quoting, assembly).
	rtl := r.rtlInvolved(isRTLLocale, flagName, description, f.DefaultValue)

	// Build the flag representation. When RTL is involved use a neutral "/"
	// separator rather than the translated "or" word (which would itself need
	// isolating); plain LTR keeps "or" for backward compatibility.
	var flagPart string
	if f.Short != "" && config.ShowShortFlags {
		if rtl {
			flagPart = fmt.Sprintf("--%s / -%s", flagName, f.Short)
		} else {
			orMsg := r.parser.layeredProvider.GetMessage(messages.MsgOrKey)
			flagPart = fmt.Sprintf("--%s %s -%s", flagName, orMsg, f.Short)
		}
	} else {
		flagPart = "--" + flagName
	}

	// Build fields in LOGICAL order — assembly handles direction.
	fields := []string{flagPart}

	if description != "" {
		// Quotes only on the plain LTR path; in bidi mode FSI isolation, not
		// quoting, keeps the description from reordering its neighbours.
		if rtl {
			fields = append(fields, description)
		} else {
			fields = append(fields, "\""+description+"\"")
		}
	}

	if config.ShowTypes {
		fields = append(fields, "("+strings.ToLower(f.TypeOf.String())+")")
	}

	if f.DefaultValue != "" && config.ShowDefaults {
		// Show the literal default the user would type; locale-format only on opt-in
		// (a port "8080" must not become "8,080").
		formattedDefault := f.DefaultValue
		if config.LocaleAwareDefaults {
			formattedDefault = r.formatDefaultValue(f)
		}
		fields = append(fields, fmt.Sprintf("(%s: %s)",
			r.parser.layeredProvider.GetMessage(messages.MsgDefaultsToKey),
			formattedDefault))
	}

	if config.ShowValidators && len(f.Validators) > 0 {
		fields = append(fields, fmt.Sprintf("[%s: %d]",
			r.parser.layeredProvider.GetMessage(messages.MsgValidatorsKey), len(f.Validators)))
	}

	if config.ShowRequired {
		requiredOrOptional := r.parser.layeredProvider.GetMessage(messages.MsgOptionalKey)
		if f.Required {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgRequiredKey)
		} else if f.RequiredIf != nil {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgConditionalKey)
		}
		fields = append(fields, "("+requiredOrOptional+")")
	}

	return r.bidiAssemble(fields, rtl, isRTLLocale)
}

// CommandUsage generates a usage string for a given command.
// The usage string includes the command name, description, and any subcommands.
// This method respects the HelpConfig settings and automatically handles RTL languages.
func (r *DefaultRenderer) CommandUsage(c *Command) string {
	config := r.parser.GetHelpConfig()

	// Use the full command path for proper hierarchy display, or fall back to name
	cmdName := c.path
	if cmdName == "" {
		cmdName = r.CommandName(c)
	}

	// Build command usage with positionals
	usageLine := cmdName
	for _, pos := range r.parser.getPositionalsForCommand(c.path) {
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

	description := ""
	if config.ShowDescription {
		description = r.CommandDescription(c)
	}
	return r.CommandListItem(usageLine, description)
}

// CommandListItem renders a "<name> <description>" line with the same quoting and
// bidi handling as CommandUsage, for an explicit display name. It is the single
// command-line formatter shared by the command tree, command-scoped help and search
// results — the hand-rolled "name - description" variants bypassed it (and its RTL
// isolation), so command help scrambled in RTL and drifted in format.
func (r *DefaultRenderer) CommandListItem(name, description string) string {
	isRTL := i18n.IsRTL(r.parser.GetLanguage())
	rtl := r.rtlInvolved(isRTL, name, description)
	fields := []string{name}
	if description != "" {
		// Quotes on plain LTR; bidi mode relies on FSI isolation instead.
		if rtl {
			fields = append(fields, description)
		} else {
			fields = append(fields, "\""+description+"\"")
		}
	}
	return r.bidiAssemble(fields, rtl, isRTL)
}

// PositionalUsage generates a usage string for a positional argument.
// The format is similar to FlagUsage but without the -- prefix:
// name "description" (required/optional)
func (r *DefaultRenderer) PositionalUsage(f *Argument, position int) string {
	config := r.parser.GetHelpConfig()
	isRTLLocale := i18n.IsRTL(r.parser.GetLanguage())

	// Get the flag name (positionals use flag storage internally)
	flagName := r.FlagName(f)

	// Wrap positional name in brackets to distinguish from flags:
	// <name> for required, [name] for optional
	if f.Required {
		flagName = "<" + flagName + ">"
	} else {
		flagName = "[" + flagName + "]"
	}

	description := ""
	if config.ShowDescription {
		description = r.FlagDescription(f)
	}
	rtl := r.rtlInvolved(isRTLLocale, flagName, description)

	// Build fields in LOGICAL order — assembly handles direction.
	fields := []string{flagName}
	if description != "" {
		if rtl {
			fields = append(fields, description)
		} else {
			fields = append(fields, "\""+description+"\"")
		}
	}

	if config.ShowRequired {
		requiredOrOptional := r.parser.layeredProvider.GetMessage(messages.MsgOptionalKey)
		if f.Required {
			requiredOrOptional = r.parser.layeredProvider.GetMessage(messages.MsgRequiredKey)
		}
		fields = append(fields, "("+requiredOrOptional+")")
	}

	return r.bidiAssemble(fields, rtl, isRTLLocale)
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

// rtlInvolved reports whether a help line must use the bidi-aware assembly path:
// either the UI locale is RTL, or some piece of content carries RTL runes (e.g.
// an Arabic description in an otherwise-English help screen). When it returns
// false the line stays on the plain LTR path and is byte-identical to historical
// output, so ordinary ASCII help never gains zero-width bidi controls.
func (r *DefaultRenderer) rtlInvolved(isRTLLocale bool, texts ...string) bool {
	if isRTLLocale {
		return true
	}
	for _, t := range texts {
		if r.containsRTLRunes(t) {
			return true
		}
	}
	return false
}

// bidiAssemble joins help-line fields given in LOGICAL (reading) order. With no
// RTL involved it is a plain space-join (the historical LTR output). Otherwise
// every non-empty field is FSI-isolated — so an LTR run (a --flag, a path, a
// number) cannot reorder an adjacent RTL run, or vice versa — and an RTL UI
// locale wraps the whole line to assert a right-to-left base direction (help
// lines start with a neutral "--", so first-strong detection would otherwise
// frame the line LTR). This single assembly, shared by flags, commands and
// positionals, replaces the per-element manual reordering that used to drift.
func (r *DefaultRenderer) bidiAssemble(fields []string, needsBidi, baseRTL bool) string {
	var nonEmpty []string
	for _, f := range fields {
		if f != "" {
			nonEmpty = append(nonEmpty, f)
		}
	}
	if !needsBidi {
		return strings.Join(nonEmpty, " ")
	}
	for i, f := range nonEmpty {
		nonEmpty[i] = i18n.Isolate(f)
	}
	line := strings.Join(nonEmpty, " ")
	if baseRTL {
		line = i18n.IsolateRTL(line)
	}
	return line
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
