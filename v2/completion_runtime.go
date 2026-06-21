package goopt

import (
	"strings"

	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/types/orderedmap"
	"github.com/napalu/goopt/v2/validation"
)

// Suggestion is a single completion candidate. Description is surfaced by shells that
// support it (zsh, fish) and ignored by others (bash).
type Suggestion struct {
	Value       string
	Description string
}

// CompleterContext is passed to a CompleterFunc. Command is the resolved command path
// at the cursor; Prefix is the partial value typed so far; Parser gives access to the
// already-resolved context for dependent completion (read-only in spirit).
type CompleterContext struct {
	Command string
	Prefix  string
	Parser  *Parser
}

// CompleterFunc computes dynamic value suggestions for a flag at completion time. It is
// the runtime successor to the deprecated AcceptedValues — values a static script can't
// know (git branches, files in context, rows from a DB).
type CompleterFunc func(CompleterContext) []Suggestion

// CompletionKind classifies what the cursor token in a partial command line is
// completing.
type CompletionKind int

const (
	// CompCommand: the cursor is at a command position — subcommands (and flag names)
	// valid here are candidates.
	CompCommand CompletionKind = iota
	// CompFlagName: the cursor is a partial flag (starts with a prefix char).
	CompFlagName
	// CompFlagValue: the cursor is the value for ValueFlag (the preceding token).
	CompFlagValue
)

// CompletionContext describes where the cursor is in a partial command line. It is
// produced by driving the REAL parser loop in completion mode (resolution only), so
// the command context honours every parsing rule the parser itself applies — there is
// no parallel parser to drift.
type CompletionContext struct {
	Command   string         // resolved command path at the cursor ("" = top level)
	Kind      CompletionKind // what the cursor token is completing
	Prefix    string         // the partial token being completed
	ValueFlag string         // canonical flag name when Kind == CompFlagValue
}

// resolveCompletionContext determines the cursor context for the words of a partial
// command line (words[0] is the program name; the final word is the cursor token,
// possibly empty). It runs the parser's own loop over the confirmed prefix to resolve
// the command path, then classifies the cursor token via the parser's own primitives.
func (p *Parser) resolveCompletionContext(words []string) CompletionContext {
	if len(words) == 0 {
		return CompletionContext{}
	}
	cursor := words[len(words)-1]
	prefix := words[:len(words)-1] // still includes the program name at [0]

	cmdPath := p.resolveCommandPathForCompletion(prefix)

	ctx := CompletionContext{Command: cmdPath, Prefix: cursor}
	switch {
	case p.isFlag(cursor):
		ctx.Kind = CompFlagName
	default:
		if vf, ok := p.pendingValueFlag(prefix, cmdPath); ok {
			ctx.Kind = CompFlagValue
			ctx.ValueFlag = vf
		} else {
			ctx.Kind = CompCommand
		}
	}
	return ctx
}

// resolveCommandPathForCompletion runs the real parse loop in completion mode over the
// confirmed prefix and returns the deepest command path it resolved (the cursor's
// command context). Transient parse state is swapped out and restored, so completion
// never mutates the parser.
func (p *Parser) resolveCommandPathForCompletion(prefix []string) string {
	saved := p.beginCompletionParse()
	p.Parse(prefix)
	cmdPath := p.completionPath
	p.endCompletionParse(saved)
	return cmdPath
}

// pendingValueFlag reports whether the last token of the confirmed prefix is a flag
// that expects a value but hasn't been given one inline — i.e. the cursor is its
// value. Every structural decision delegates to the parser's own primitives.
func (p *Parser) pendingValueFlag(prefix []string, cmdPath string) (string, bool) {
	if len(prefix) < 2 { // [0] is the program name
		return "", false
	}
	last := prefix[len(prefix)-1]
	if !p.isFlag(last) {
		return "", false
	}
	name := strings.TrimLeftFunc(last, p.prefixFunc)
	if strings.ContainsRune(name, '=') {
		return "", false // value already supplied inline (--flag=val)
	}
	fi, ok := p.getFlagInCommandPath(name, cmdPath)
	if !ok {
		return "", false
	}
	if fi.Argument.TypeOf == types.Standalone {
		return "", false // boolean flag — takes no value
	}
	return name, true
}

// Suggest returns the completion candidates for a resolved cursor context, prefix-
// filtered. It reuses the parser's own structure: subcommands from the registered
// command tree, flag names via the SAME inheritance rule the parser resolves with, and
// values via the value-source ladder. No knowledge is duplicated.
func (p *Parser) Suggest(ctx CompletionContext) []Suggestion {
	var out []Suggestion
	switch ctx.Kind {
	case CompFlagValue:
		out = p.valueSuggestions(ctx)
	case CompFlagName:
		out = p.flagNameSuggestions(ctx.Command)
	default: // CompCommand: both subcommands and flag names are valid here
		out = append(out, p.subcommandSuggestions(ctx.Command)...)
		out = append(out, p.flagNameSuggestions(ctx.Command)...)
	}
	return filterByPrefix(out, ctx.Prefix)
}

// subcommandSuggestions lists the DIRECT subcommands of cmdPath (one token deeper).
func (p *Parser) subcommandSuggestions(cmdPath string) []Suggestion {
	var out []Suggestion
	for _, cmd := range p.registeredCommands.All() {
		if cmd == nil {
			continue
		}
		child, ok := directChild(cmdPath, cmd.path)
		if !ok {
			continue
		}
		desc := p.renderer.CommandDescription(cmd)
		// Offer the localized name too (the parser accepts both): GetCommandTranslation
		// returns the translated last token for the command's full path.
		if tr, ok := p.commandTranslation(cmd.path); ok && tr != child {
			out = append(out, Suggestion{Value: tr, Description: desc})
		}
		out = append(out, Suggestion{Value: child, Description: desc})
	}
	return out
}

// commandTranslation returns the current-language translation of a command's own name
// (the last token of its path), if any.
func (p *Parser) commandTranslation(cmdPath string) (string, bool) {
	if p.translationRegistry == nil {
		return "", false
	}
	return p.translationRegistry.GetCommandTranslation(cmdPath, p.GetLanguage())
}

// flagTranslation returns the current-language translation of a flag's canonical name.
func (p *Parser) flagTranslation(canonical string) (string, bool) {
	if p.translationRegistry == nil {
		return "", false
	}
	return p.translationRegistry.GetFlagTranslation(canonical, p.GetLanguage())
}

// directChild returns the next path token of full if full is a direct child of parent.
func directChild(parent, full string) (string, bool) {
	if parent == "" {
		if !strings.Contains(full, " ") {
			return full, true
		}
		return "", false
	}
	if !strings.HasPrefix(full, parent+" ") {
		return "", false
	}
	rest := full[len(parent)+1:]
	if rest == "" || strings.Contains(rest, " ") {
		return "", false
	}
	return rest, true
}

// flagNameSuggestions lists the flags valid at cmdPath — own plus inherited from any
// ancestor plus globals — exactly the set the parser would resolve there. Positionals
// are excluded (they aren't --flags). Nearest owner wins on a name collision.
func (p *Parser) flagNameSuggestions(cmdPath string) []Suggestion {
	seen := map[string]bool{}
	var out []Suggestion
	// Walk own path then ancestors so the nearest definition wins, then globals.
	owners := append(ancestorPaths(cmdPath), "")
	for _, owner := range owners {
		for flagKey, fi := range p.acceptedFlags.All() {
			if fi == nil || fi.Argument == nil || fi.Argument.Position != nil {
				continue // skip positionals
			}
			parts := splitPathFlag(flagKey)
			name := parts[0]
			fOwner := ""
			if len(parts) > 1 {
				fOwner = parts[1]
			}
			if fOwner != owner || seen[name] {
				continue
			}
			seen[name] = true
			desc := p.renderer.FlagDescription(fi.Argument)
			// Offer the localized flag name too (the parser accepts both).
			if tr, ok := p.flagTranslation(name); ok && tr != name {
				out = append(out, Suggestion{Value: "--" + tr, Description: desc})
			}
			out = append(out, Suggestion{Value: "--" + name, Description: desc})
		}
	}
	return out
}

// ancestorPaths returns [cmdPath, parent, ..., root] (excluding the empty global path).
func ancestorPaths(cmdPath string) []string {
	if cmdPath == "" {
		return nil
	}
	toks := strings.Split(cmdPath, " ")
	paths := make([]string, 0, len(toks))
	for i := len(toks); i >= 1; i-- {
		paths = append(paths, strings.Join(toks[:i], " "))
	}
	return paths
}

// valueSuggestions resolves a flag's value candidates via the value-source ladder:
// explicit completer > File (path completion, shell-delegated) > enumerable validator
// (a validator that exposes its accepted set, e.g. validation.IsOneOf) > legacy
// AcceptedValues.
func (p *Parser) valueSuggestions(ctx CompletionContext) []Suggestion {
	fi, ok := p.getFlagInCommandPath(ctx.ValueFlag, ctx.Command)
	if !ok || fi.Argument == nil {
		return nil
	}
	arg := fi.Argument
	if arg.Completer != nil {
		return arg.Completer(CompleterContext{Command: ctx.Command, Prefix: ctx.Prefix, Parser: p})
	}
	if arg.TypeOf == types.File {
		return nil // file completion is delegated to the shell stub (Phase 4)
	}
	// Any validator that can enumerate its accepted set (Enumerable) drives completion —
	// so validation.IsOneOf both validates AND completes, from one declaration.
	for _, v := range arg.Validators {
		if e, ok := v.(validation.Enumerable); ok {
			cands := e.Candidates()
			out := make([]Suggestion, 0, len(cands))
			for _, c := range cands {
				out = append(out, Suggestion{Value: c})
			}
			return out
		}
	}
	if len(arg.AcceptedValues) > 0 { // deprecated, but honoured while it exists
		out := make([]Suggestion, 0, len(arg.AcceptedValues))
		for _, v := range arg.AcceptedValues {
			out = append(out, Suggestion{Value: v.Pattern, Description: v.Description})
		}
		return out
	}
	return nil
}

func filterByPrefix(in []Suggestion, prefix string) []Suggestion {
	if prefix == "" {
		return in
	}
	out := in[:0:0]
	for _, s := range in {
		if strings.HasPrefix(s.Value, prefix) {
			out = append(out, s)
		}
	}
	return out
}

// completionStateSnapshot holds the transient parse state replaced for a completion
// parse so the real parser is left untouched.
type completionStateSnapshot struct {
	errors          []error
	options         map[string]string
	rawArgs         map[string]string
	repeatedFlags   map[string]bool
	positionalArgs  []PositionalArgument
	secureArguments *orderedmap.OrderedMap[string, *types.Secure]
	commandOptions  *orderedmap.OrderedMap[string, bool]
	allowUnknown    bool
	completionPath  string
}

func (p *Parser) beginCompletionParse() completionStateSnapshot {
	s := completionStateSnapshot{
		errors:          p.errors,
		options:         p.options,
		rawArgs:         p.rawArgs,
		repeatedFlags:   p.repeatedFlags,
		positionalArgs:  p.positionalArgs,
		secureArguments: p.secureArguments,
		commandOptions:  p.commandOptions,
		allowUnknown:    p.allowUnknownFlags,
		completionPath:  p.completionPath,
	}
	p.errors = nil
	p.options = map[string]string{}
	p.rawArgs = map[string]string{}
	p.repeatedFlags = map[string]bool{}
	p.positionalArgs = nil
	p.secureArguments = orderedmap.NewOrderedMap[string, *types.Secure]()
	p.commandOptions = orderedmap.NewOrderedMap[string, bool]()
	p.allowUnknownFlags = true // partial flags must not error
	p.completionMode = true
	p.completionPath = ""
	return s
}

func (p *Parser) endCompletionParse(s completionStateSnapshot) {
	p.completionMode = false
	p.errors = s.errors
	p.options = s.options
	p.rawArgs = s.rawArgs
	p.repeatedFlags = s.repeatedFlags
	p.positionalArgs = s.positionalArgs
	p.secureArguments = s.secureArguments
	p.commandOptions = s.commandOptions
	p.allowUnknownFlags = s.allowUnknown
	p.completionPath = s.completionPath
}
