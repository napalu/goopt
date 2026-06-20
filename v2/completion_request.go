package goopt

import (
	"io"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/types"
)

// CompletionDirective tells the shell stub how to treat the result beyond the literal
// suggestions (mirrors the small, proven Cobra idea). It is emitted as a trailing
// ":<n>" line so the stub can act on it.
type CompletionDirective int

const (
	DirectiveDefault        CompletionDirective = iota // use the listed suggestions
	DirectiveFileCompletion                            // let the shell complete file paths
)

// completionSentinel is the hidden subcommand a generated completion stub invokes:
//
//	myapp __complete <shell> <word>...
//
// The words are the command line being completed (including the program name and the
// final, possibly-empty, cursor token).
const completionSentinel = "__complete"

// Suggestions is the result of a completion request: the candidates plus the shell that
// asked, so it can render itself in that shell's protocol.
type Suggestions struct {
	Shell     string
	Items     []Suggestion
	Directive CompletionDirective
}

// CompletionRequest reports whether args is a completion invocation (`<prog> __complete
// <shell> <word>...`) and, if so, returns the computed suggestions. It runs the parser
// in completion mode (resolution only — no callbacks, binding, validation, or output)
// and never mutates the parser. When args is a normal invocation it returns false with
// no work done, so it is cheap to call unconditionally before Parse:
//
//	if sugg, ok := parser.CompletionRequest(os.Args); ok {
//	    sugg.WriteTo(os.Stdout)
//	    os.Exit(0) // the caller owns the exit — goopt only computed
//	}
//	parser.Parse(os.Args)
func (p *Parser) CompletionRequest(args []string) (Suggestions, bool) {
	if len(args) < 3 || args[1] != completionSentinel {
		return Suggestions{}, false
	}
	shell := args[2]
	words := args[3:] // the actual command line being completed
	ctx := p.resolveCompletionContext(words)
	return Suggestions{Shell: shell, Items: p.Suggest(ctx), Directive: p.completionDirective(ctx)}, true
}

// completionDirective returns DirectiveFileCompletion when the cursor is the value of a
// File-type flag, so the shell completes paths (something a value list can't express).
func (p *Parser) completionDirective(ctx CompletionContext) CompletionDirective {
	if ctx.Kind != CompFlagValue {
		return DirectiveDefault
	}
	if fi, ok := p.getFlagInCommandPath(ctx.ValueFlag, ctx.Command); ok && fi.Argument != nil {
		if fi.Argument.TypeOf == types.File {
			return DirectiveFileCompletion
		}
	}
	return DirectiveDefault
}

// HandleCompletion is the one-liner: if args is a completion request, compute and write
// the suggestions in the requested shell's protocol and return true (the caller should
// then exit); otherwise return false and do nothing.
func (p *Parser) HandleCompletion(args []string, w io.Writer) bool {
	s, ok := p.CompletionRequest(args)
	if !ok {
		return false
	}
	_, _ = s.WriteTo(w)
	return true
}

// WriteTo renders the suggestions in the shell's expected protocol. This is the ONLY
// shell-specific surface in the runtime path: bash wants bare values (one per line);
// zsh and fish accept "value<TAB>description".
func (s Suggestions) WriteTo(w io.Writer) (int64, error) {
	var b strings.Builder
	withDesc := s.Shell == "zsh" || s.Shell == "fish" || s.Shell == "powershell"
	for _, item := range s.Items {
		b.WriteString(item.Value)
		if withDesc && item.Description != "" {
			b.WriteByte('\t')
			b.WriteString(item.Description)
		}
		b.WriteByte('\n')
	}
	// Trailing directive line, read by the stub (e.g. ":1" → complete files).
	b.WriteString(":")
	b.WriteString(strconv.Itoa(int(s.Directive)))
	b.WriteByte('\n')
	n, err := io.WriteString(w, b.String())
	return int64(n), err
}

// GenerateCompletionStub returns the small, stable shell script the user installs. It
// does NOT encode the command tree — it forwards every <TAB> to `<prog> __complete
// <shell> ...`, so the live parser computes suggestions. This is the entire
// shell-specific surface of runtime completion (contrast the full static scripts).
func (p *Parser) GenerateCompletionStub(shell, programName string) (string, error) {
	var stub string
	switch shell {
	case "bash":
		stub = bashStub
	case "zsh":
		stub = zshStub
	case "fish":
		stub = fishStub
	case "powershell":
		stub = powershellStub
	default:
		return "", errs.ErrUnsupportedShell.WithArgs(shell)
	}
	return strings.ReplaceAll(stub, "{{PROG}}", programName), nil
}

// bashStub forwards completion to `<prog> __complete bash <words...>` and honours the
// trailing directive line (:1 → file completion).
const bashStub = `_{{PROG}}_complete() {
    local out directive suggestions
    out="$("${COMP_WORDS[0]}" __complete bash "${COMP_WORDS[@]}" 2>/dev/null)"
    directive="${out##*$'\n'}"
    suggestions="${out%$'\n'*}"
    if [[ "$directive" == ":1" ]]; then
        compopt -o default 2>/dev/null
        COMPREPLY=()
        return
    fi
    COMPREPLY=($(compgen -W "${suggestions}" -- "${COMP_WORDS[COMP_CWORD]}"))
}
complete -F _{{PROG}}_complete {{PROG}}
`

// zshStub forwards to `__complete zsh` (value<TAB>desc lines) and renders via
// _describe; the trailing ":1" directive switches to file completion.
const zshStub = `#compdef {{PROG}}
_{{PROG}}_complete() {
    local out directive
    local -a lines comps
    lines=("${(@f)$(${words[1]} __complete zsh "${words[@]}" 2>/dev/null)}")
    directive="${lines[-1]}"
    lines=("${lines[1,-2]}")
    if [[ "$directive" == ":1" ]]; then
        _files
        return
    fi
    local line
    for line in "${lines[@]}"; do
        comps+=("${line//$'\t'/:}")
    done
    _describe -t values '{{PROG}}' comps
}
compdef _{{PROG}}_complete {{PROG}}
`

// fishStub forwards to `__complete fish`; fish renders value<TAB>desc natively. The
// trailing directive line is filtered out.
const fishStub = `function __{{PROG}}_complete
    set -l tokens (commandline -opc) (commandline -ct)
    {{PROG}} __complete fish $tokens 2>/dev/null | string match -v -r '^:[0-9]+$'
end
complete -c {{PROG}} -f -a '(__{{PROG}}_complete)'
`

// powershellStub forwards to `__complete powershell`, drops the directive line, and
// maps value<TAB>desc to CompletionResult entries.
const powershellStub = `Register-ArgumentCompleter -Native -CommandName {{PROG}} -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $tokens = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
    & $tokens[0] __complete powershell @tokens 2>$null |
        Where-Object { $_ -notmatch '^:[0-9]+$' } |
        ForEach-Object {
            $parts = $_ -split "` + "`t" + `", 2
            $desc = if ($parts.Length -gt 1) { $parts[1] } else { $parts[0] }
            [System.Management.Automation.CompletionResult]::new($parts[0], $parts[0], 'ParameterValue', $desc)
        }
}
`
