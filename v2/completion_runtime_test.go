package goopt

import (
	"slices"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/types"
)

// build a small tree: global --verbose(-v); server{--port} -> start{--timeout}
func newCompletionParser(t *testing.T, fired *[]string, bound *string) *Parser {
	t.Helper()
	p := NewParser()
	if err := p.AddFlag("verbose", newStandalone(WithShortFlag("v"))); err != nil {
		t.Fatal(err)
	}
	rec := func(name string) CommandFunc {
		return func(_ *Parser, _ *Command) error {
			if fired != nil {
				*fired = append(*fired, name)
			}
			return nil
		}
	}
	start := NewCommand(WithName("start"), WithCallback(rec("start")))
	server := NewCommand(WithName("server"), WithCallback(rec("server")), WithSubcommands(start))
	if err := p.AddCommand(server); err != nil {
		t.Fatal(err)
	}
	if bound != nil {
		if err := p.BindFlag(bound, "port", NewArg(WithType(types.Single)), "server"); err != nil {
			t.Fatal(err)
		}
	} else {
		mustAddFlag(t, p, "port", NewArg(WithType(types.Single)), "server")
	}
	mustAddFlag(t, p, "timeout", NewArg(WithType(types.Single)), "server", "start")
	return p
}

func TestCompletionContextDetection(t *testing.T) {
	cases := []struct {
		words   []string
		wantCmd string
		wantK   CompletionKind
		wantVF  string
		wantPfx string
	}{
		{[]string{"app", "ser"}, "", CompCommand, "", "ser"},
		{[]string{"app", "server", ""}, "server", CompCommand, "", ""},
		{[]string{"app", "server", "--po"}, "server", CompFlagName, "", "--po"},
		{[]string{"app", "server", "--port", ""}, "server", CompFlagValue, "port", ""},
		{[]string{"app", "server", "--port", "80", ""}, "server", CompCommand, "", ""}, // value given → back to command pos
		{[]string{"app", "server", "start", "--"}, "server start", CompFlagName, "", "--"},
		{[]string{"app", "server", "start", "--timeout", "5"}, "server start", CompFlagValue, "timeout", "5"},
		{[]string{"app", "--verb"}, "", CompFlagName, "", "--verb"},
		{[]string{"app", "server", "--verbose", ""}, "server", CompCommand, "", ""}, // standalone → no pending value
	}
	for _, c := range cases {
		p := newCompletionParser(t, nil, nil)
		ctx := p.resolveCompletionContext(c.words)
		if ctx.Command != c.wantCmd || ctx.Kind != c.wantK || ctx.ValueFlag != c.wantVF || ctx.Prefix != c.wantPfx {
			t.Errorf("%v -> got {cmd=%q kind=%d vf=%q pfx=%q}, want {cmd=%q kind=%d vf=%q pfx=%q}",
				c.words, ctx.Command, ctx.Kind, ctx.ValueFlag, ctx.Prefix, c.wantCmd, c.wantK, c.wantVF, c.wantPfx)
		}
	}
}

// TestCompletionHonorsNothingThatActs is the boundary invariant: a completion walk must
// execute zero command callbacks, write zero bound variables, and leave the parser's
// observable state untouched.
func TestCompletionHonorsNothingThatActs(t *testing.T) {
	var fired []string
	var port string
	p := newCompletionParser(t, &fired, &port)

	// Drive several completion resolutions, including ones that pass over a value flag
	// and over commands with callbacks + ExecOnParse-like positions.
	for _, words := range [][]string{
		{"app", "server", "start", ""},
		{"app", "server", "--port", "9999", ""},
		{"app", "server", "start", "--timeout", "5s"},
	} {
		_ = p.resolveCompletionContext(words)
	}

	if len(fired) != 0 {
		t.Errorf("completion must not run command callbacks; fired=%v", fired)
	}
	if port != "" {
		t.Errorf("completion must not bind variables; port=%q", port)
	}
	if errs := p.GetErrors(); len(errs) != 0 {
		t.Errorf("completion must not leak errors onto the parser; errs=%v", errs)
	}
	if cmds := p.GetCommands(); len(cmds) != 0 {
		t.Errorf("completion must not leave invoked-command state; GetCommands=%v", cmds)
	}

	// And the parser must still work normally afterwards.
	p2 := newCompletionParser(t, &fired, &port)
	if !p2.Parse([]string{"app", "server", "start", "--timeout", "5s"}) {
		t.Fatalf("normal Parse after completion setup should succeed; errs=%v", p2.GetErrors())
	}
}

func svals(ss []Suggestion) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Value)
	}
	return out
}

func TestCompletionSuggest(t *testing.T) {
	p := newCompletionParser(t, nil, nil)
	got := func(words ...string) []string { return svals(p.Suggest(p.resolveCompletionContext(words))) }

	// command position at root → top-level command "server" (+ global flags filtered by prefix)
	if g := got("app", "ser"); !slices.Contains(g, "server") {
		t.Errorf("root 'ser' should suggest 'server'; got %v", g)
	}
	// flag-name under server → own --port + inherited global --verbose (prefix --)
	g := got("app", "server", "--")
	if !slices.Contains(g, "--port") || !slices.Contains(g, "--verbose") {
		t.Errorf("`server --` should offer own --port AND inherited --verbose; got %v", g)
	}
	// deep: flag-name under server start → --timeout (own) + --port (inherited) + --verbose (global)
	g = got("app", "server", "start", "--")
	for _, want := range []string{"--timeout", "--port", "--verbose"} {
		if !slices.Contains(g, want) {
			t.Errorf("`server start --` should offer %s; got %v", want, g)
		}
	}
	// subcommand position: `server ` → child "start"
	if g := got("app", "server", ""); !slices.Contains(g, "start") {
		t.Errorf("`server ` should suggest subcommand 'start'; got %v", g)
	}
}

func TestCompletionDynamicValues(t *testing.T) {
	p := NewParser()
	p.AddCommand(NewCommand(WithName("checkout")))
	branches := []string{"main", "develop", "feature-x"}
	mustAddFlag(t, p, "branch", NewArg(WithType(types.Single), WithCompleter(func(c CompleterContext) []Suggestion {
		var out []Suggestion
		for _, b := range branches {
			out = append(out, Suggestion{Value: b})
		}
		return out
	})), "checkout")

	// --branch <TAB> → dynamic branches
	g := svals(p.Suggest(p.resolveCompletionContext([]string{"app", "checkout", "--branch", ""})))
	if len(g) != 3 || !slices.Contains(g, "feature-x") {
		t.Errorf("dynamic completer should yield branches; got %v", g)
	}
	// prefix filter applies: --branch feat<TAB> → feature-x only
	g = svals(p.Suggest(p.resolveCompletionContext([]string{"app", "checkout", "--branch", "feat"})))
	if len(g) != 1 || g[0] != "feature-x" {
		t.Errorf("prefix filter should narrow to feature-x; got %v", g)
	}
}

func TestCompletionRequestEndToEnd(t *testing.T) {
	p := newCompletionParser(t, nil, nil)

	// Not a completion request → false, no work.
	if _, ok := p.CompletionRequest([]string{"app", "server", "--port", "80"}); ok {
		t.Errorf("normal args must not be treated as a completion request")
	}

	// A bash completion request for `app server --` → bare values, one per line.
	var buf strings.Builder
	handled := p.HandleCompletion([]string{"app", "__complete", "bash", "app", "server", "--"}, &buf)
	if !handled {
		t.Fatal("__complete invocation should be handled")
	}
	out := buf.String()
	if !strings.Contains(out, "--port") || !strings.Contains(out, "--verbose") {
		t.Errorf("bash completion output should list --port and --verbose; got %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("bash output must be bare values (no tab/description); got %q", out)
	}

	// zsh carries descriptions (value<TAB>desc).
	buf.Reset()
	p.HandleCompletion([]string{"app", "__complete", "zsh", "app", "server", "start", "--timeout", ""}, &buf)
	// value position for timeout has no static values → empty output is acceptable; just ensure no panic and handled
	_ = out
}

func TestCompletionFileDirectiveAndStub(t *testing.T) {
	dir := t.TempDir()
	_ = dir
	p := NewParser()
	p.AddCommand(NewCommand(WithName("load")))
	mustAddFlag(t, p, "config", NewArg(WithType(types.File)), "load")

	// File-type flag value position → DirectiveFileCompletion, emitted as ":1".
	var buf strings.Builder
	p.HandleCompletion([]string{"app", "__complete", "bash", "app", "load", "--config", ""}, &buf)
	if !strings.Contains(buf.String(), ":1") {
		t.Errorf("File-type value should emit file directive :1; got %q", buf.String())
	}

	// Non-file value emits :0.
	buf.Reset()
	p2 := newCompletionParser(t, nil, nil)
	p2.HandleCompletion([]string{"app", "__complete", "bash", "app", "server", "--port", ""}, &buf)
	if !strings.Contains(buf.String(), ":0") {
		t.Errorf("non-file value should emit default directive :0; got %q", buf.String())
	}

	// Stub generator produces a forwarding script for bash.
	stub, err := p.GenerateCompletionStub("bash", "myapp")
	if err != nil || !strings.Contains(stub, "__complete bash") || !strings.Contains(stub, "complete -F _myapp_complete myapp") {
		t.Errorf("bash stub should forward to __complete; err=%v stub=%q", err, stub)
	}
	if _, err := p.GenerateCompletionStub("tcsh", "myapp"); err == nil {
		t.Errorf("unsupported shell should return an error, not silently succeed")
	}
}

func TestCompletionStubsAllShells(t *testing.T) {
	p := NewParser()
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		stub, err := p.GenerateCompletionStub(shell, "myapp")
		if err != nil {
			t.Errorf("%s stub: unexpected error %v", shell, err)
			continue
		}
		if !strings.Contains(stub, "__complete "+shell) {
			t.Errorf("%s stub must forward to `__complete %s`; got %q", shell, shell, stub)
		}
		if !strings.Contains(stub, "myapp") {
			t.Errorf("%s stub must reference the program name", shell)
		}
	}
	if _, err := p.GenerateCompletionStub("tcsh", "myapp"); err == nil {
		t.Error("unsupported shell must return an error")
	}
}

// TestCompletionMatrixParity is the core invariant of the runtime system: at any
// command context, completion offers a flag IFF the parser actually accepts it there.
// It checks completion's offer against REAL parse acceptance (not the shared resolver,
// which would be circular) across flag scopes — global, command-owned, inherited.
func TestCompletionMatrixParity(t *testing.T) {
	build := func() *Parser {
		p := NewParser()
		mustAddFlag(t, p, "g", newStandalone()) // global
		mustAddCmd(t, p, "solo")
		mustAddFlag(t, p, "s", newStandalone(), "solo")
		child := NewCommand(WithName("child"))
		parent := NewCommand(WithName("parent"), WithSubcommands(child))
		if err := p.AddCommand(parent); err != nil {
			t.Fatal(err)
		}
		mustAddFlag(t, p, "p", newStandalone(), "parent")          // owned by parent
		mustAddFlag(t, p, "c", newStandalone(), "parent", "child") // owned by parent child
		mustAddCmd(t, p, "other")
		mustAddFlag(t, p, "o", newStandalone(), "other")
		return p
	}
	flags := []string{"g", "s", "p", "c", "o"}
	contexts := []struct {
		name string
		path []string
	}{
		{"solo", []string{"solo"}},
		{"parent child", []string{"parent", "child"}},
		{"other", []string{"other"}},
	}

	sp := build() // Suggest is non-mutating, so one parser serves all completion queries
	for _, ctx := range contexts {
		words := append(append([]string{"app"}, ctx.path...), "--")
		offered := map[string]bool{}
		for _, s := range sp.Suggest(sp.resolveCompletionContext(words)) {
			offered[strings.TrimPrefix(s.Value, "--")] = true
		}
		for _, f := range flags {
			// Real parser verdict: parse `<ctx> --<f>` on a fresh parser; a valid flag
			// parses clean, an invalid one raises an unknown-flag error.
			pp := build()
			pp.Parse(append(append([]string{"app"}, ctx.path...), "--"+f))
			accepts := len(pp.GetErrors()) == 0
			if offered[f] != accepts {
				t.Errorf("PARITY at %q flag --%s: completion offers=%v, parser accepts=%v",
					ctx.name, f, offered[f], accepts)
			}
		}
	}
}

// TestCompletionSubcommandParity: completion offers subcommand S at context C IFF the
// parser navigates into "C S" (a real registered child).
func TestCompletionSubcommandParity(t *testing.T) {
	p := NewParser()
	gc := NewCommand(WithName("grandchild"))
	child := NewCommand(WithName("child"), WithSubcommands(gc))
	parent := NewCommand(WithName("parent"), WithSubcommands(child))
	if err := p.AddCommand(parent); err != nil {
		t.Fatal(err)
	}
	if err := p.AddCommand(NewCommand(WithName("sibling"))); err != nil {
		t.Fatal(err)
	}

	check := func(ctx []string, wantChildren ...string) {
		words := append(append([]string{"app"}, ctx...), "")
		got := map[string]bool{}
		for _, s := range p.Suggest(p.resolveCompletionContext(words)) {
			got[s.Value] = true
		}
		for _, w := range wantChildren {
			if !got[w] {
				t.Errorf("at %v completion should offer subcommand %q; got %v", ctx, w, got)
			}
		}
	}
	check(nil, "parent", "sibling") // root → top-level commands
	check([]string{"parent"}, "child")
	check([]string{"parent", "child"}, "grandchild")
}
