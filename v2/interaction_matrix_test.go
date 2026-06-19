package goopt

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/i18n/locales/ar"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/validation"
	"golang.org/x/text/language"
)

// Interaction-matrix tests deliberately "hammer the edges": they cross each
// constraint FEATURE against the DIMENSIONS where goopt bugs have historically
// clustered — flag scope (global / command-scoped / inherited), cross-command
// isolation, and rendering across locales — rather than testing each feature only
// in its primary (global) dimension. The cells, not the features, are the target.

// --- the WHERE axis: flag scope ---

type matrixScope struct {
	name     string
	flagPath []string // command path the constrained flags are registered under (nil = global)
	invoke   []string // command tokens that should activate them
}

var matrixScopes = []matrixScope{
	{"global", nil, nil},
	{"command", []string{"cmd"}, []string{"cmd"}},
	{"inherited", []string{"cmd"}, []string{"cmd", "sub"}}, // flag on parent, invoked via subcommand
}

// registerMatrixCommands sets up `cmd` (with subcommand `sub`) plus a sibling
// `other`, giving every scope and the cross-command isolation check what it needs.
func registerMatrixCommands(t *testing.T, p *Parser, s matrixScope) {
	t.Helper()
	var cmd *Command
	if len(s.invoke) > 1 {
		// inherited scope: `cmd` owns the subcommand we invoke through, so the
		// flags registered on `cmd` are reached via `cmd sub`.
		cmd = NewCommand(WithName(s.invoke[0]), WithSubcommands(NewCommand(WithName(s.invoke[1]))))
	} else {
		// command scope: `cmd` is terminal (a mandatory subcommand would make a
		// bare `cmd` invocation invalid).
		cmd = NewCommand(WithName("cmd"))
	}
	if err := p.AddCommand(cmd); err != nil {
		t.Fatalf("add cmd: %v", err)
	}
	if err := p.AddCommand(NewCommand(WithName("other"))); err != nil {
		t.Fatalf("add other: %v", err)
	}
}

// --- the WHAT axis: constraint features ---

type matrixFeature struct {
	name    string
	setup   func(t *testing.T, p *Parser, path []string)
	violate []string // flag tokens (after the command tokens) that must trip the constraint
	satisfy []string // flag tokens that must satisfy it
	wantErr error    // sentinel expected on violate (nil => "any error", e.g. validators)
	warning bool     // constraint surfaces as a Warning (GetWarnings), not an Error
}

var matrixFeatures = []matrixFeature{
	{
		name: "required",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", NewArg(WithType(types.Single), WithRequired(true)), path...)
		},
		violate: nil,
		satisfy: []string{"--alpha", "v"},
		wantErr: errs.ErrRequiredFlag,
	},
	{
		name: "validator",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", NewArg(WithType(types.Single), WithValidators(validation.MinLength(5))), path...)
		},
		violate: []string{"--alpha", "ab"},
		satisfy: []string{"--alpha", "abcde"},
		wantErr: nil, // validators wrap their own error; assert presence
	},
	{
		name: "mutex",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", newStandalone(WithMutex("g")), path...)
			mustAddFlag(t, p, "beta", newStandalone(WithMutex("g")), path...)
		},
		violate: []string{"--alpha", "--beta"},
		satisfy: []string{"--alpha"},
		wantErr: errs.ErrMutexViolation,
	},
	{
		name: "exactlyone",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", newStandalone(WithExactlyOne("g")), path...)
			mustAddFlag(t, p, "beta", newStandalone(WithExactlyOne("g")), path...)
		},
		violate: nil,
		satisfy: []string{"--alpha"},
		wantErr: errs.ErrExactlyOneRequired,
	},
	{
		name: "conflicts",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", newStandalone(WithConflicts("beta")), path...)
			mustAddFlag(t, p, "beta", newStandalone(), path...)
		},
		violate: []string{"--alpha", "--beta"},
		satisfy: []string{"--alpha"},
		wantErr: errs.ErrConflictingFlags,
	},
	{
		name: "requires",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", newStandalone(WithRequires("beta")), path...)
			mustAddFlag(t, p, "beta", newStandalone(), path...)
		},
		violate: []string{"--alpha"},
		satisfy: []string{"--alpha", "--beta"},
		wantErr: errs.ErrFlagRequires,
	},
	{
		name: "requiredOn",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "trigger", newStandalone(), path...)
			mustAddFlag(t, p, "token", NewArg(WithRequiredOn("trigger")), path...)
		},
		violate: []string{"--trigger"},
		satisfy: []string{"--trigger", "--token", "x"},
		wantErr: errs.ErrRequiredWhen,
	},
	{
		name: "positional",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "pos0", NewArg(WithType(types.Single), WithPosition(0), WithRequired(true)), path...)
		},
		violate: nil,
		satisfy: []string{"thevalue"},
		wantErr: errs.ErrRequiredPositionalFlag,
	},
	{
		name: "dependsOn",
		setup: func(t *testing.T, p *Parser, path []string) {
			mustAddFlag(t, p, "alpha", newStandalone(WithDependentFlags([]string{"beta"})), path...)
			mustAddFlag(t, p, "beta", newStandalone(), path...)
		},
		violate: []string{"--alpha"},
		satisfy: []string{"--alpha", "--beta"},
		warning: true, // dependency unmet is warning-level, not an error
	},
}

func newMatrixParser(t *testing.T, f matrixFeature, s matrixScope) *Parser {
	t.Helper()
	p := NewParser()
	if len(s.flagPath) > 0 {
		registerMatrixCommands(t, p, s)
	}
	f.setup(t, p, s.flagPath)
	return p
}

// matrixArgs builds an argv exactly as a real program receives it: os.Args[0]
// (the executable) followed by the command tokens and flags. Parse strips the
// leading os.Args[0] via pruneExecPathFromArgs, just as it does in production —
// using a fake program name here would leave it to be mis-bound as a positional.
func matrixArgs(invoke, flags []string) []string {
	args := append([]string{os.Args[0]}, invoke...)
	return append(args, flags...)
}

func constraintTripped(p *Parser, f matrixFeature) bool {
	if f.warning {
		return len(p.GetWarnings()) > 0
	}
	if f.wantErr != nil {
		return hasErr(p, f.wantErr)
	}
	return len(p.GetErrors()) > 0
}

// diagnostics returns the rendered user-facing messages for a feature's outcome
// (warnings for warning-level features, errors otherwise).
func diagnostics(p *Parser, f matrixFeature) []string {
	if f.warning {
		return p.GetWarnings()
	}
	var msgs []string
	for _, e := range p.GetErrors() {
		msgs = append(msgs, e.Error())
	}
	return msgs
}

func assertConstraint(t *testing.T, p *Parser, f matrixFeature, want bool, label string) {
	t.Helper()
	if got := constraintTripped(p, f); got != want {
		t.Errorf("[%s] constraint tripped=%v, want %v; errors=%v", label, got, want, p.GetErrors())
	}
}

// matrixKnownGaps records (feature/scope) cells that are intentionally not
// supported, with the design rationale. Skipping — rather than deleting the cell —
// keeps the decision visible and greppable.
var matrixKnownGaps = map[string]string{
	"positional/inherited": "BY DESIGN: positionals are command-local — they bind to their direct command only and are not inherited by subcommands. Unlike name-keyed flags, positionals are index-keyed, so inheritance would risk silent mis-binding (index collisions across levels) and erode the unknown-argument guard. For a value shared across subcommands, use a flag (which inherits) or declare the positional per-subcommand.",
}

// TestInteractionMatrix crosses every constraint feature with every scope and
// asserts: it fires on violation, stays quiet when satisfied, and — for
// command-scoped flags — does NOT fire when a different command is invoked.
func TestInteractionMatrix(t *testing.T) {
	for _, f := range matrixFeatures {
		for _, s := range matrixScopes {
			t.Run(f.name+"/"+s.name, func(t *testing.T) {
				if reason, gap := matrixKnownGaps[f.name+"/"+s.name]; gap {
					t.Skip(reason)
				}
				// 1. violate -> the constraint must fire
				p := newMatrixParser(t, f, s)
				p.Parse(matrixArgs(s.invoke, f.violate))
				assertConstraint(t, p, f, true, "violate")

				// 2. satisfy -> it must not fire
				p = newMatrixParser(t, f, s)
				p.Parse(matrixArgs(s.invoke, f.satisfy))
				assertConstraint(t, p, f, false, "satisfy")

				// 3. cross-command isolation: invoking a DIFFERENT command must not
				//    trip a constraint declared on this command's flags.
				if len(s.flagPath) > 0 {
					p = newMatrixParser(t, f, s)
					p.Parse([]string{"app", "other"})
					assertConstraint(t, p, f, false, "isolation(other)")
				}
			})
		}
	}
}

// TestInteractionMatrixRendering crosses each feature's violation with locale
// rendering (incl. RTL Arabic) and asserts the user-facing message is well-formed:
// no %!(NOVERB/BADINDEX), no doubled or mixed quotes.
func TestInteractionMatrixRendering(t *testing.T) {
	langs := []struct {
		tag  language.Tag
		load bool
	}{
		{language.English, false},
		{language.German, false}, // default-loaded
		{ar.Tag, true},           // must be loaded explicitly (RTL)
	}
	for _, f := range matrixFeatures {
		for _, l := range langs {
			t.Run(f.name+"/"+l.tag.String(), func(t *testing.T) {
				p := NewParser()
				if l.load {
					if err := p.GetSystemBundle().LoadFromString(ar.Tag, ar.SystemTranslations); err != nil {
						t.Fatalf("load %s: %v", l.tag, err)
					}
				}
				if err := p.SetLanguage(l.tag); err != nil {
					t.Fatalf("set language %s: %v", l.tag, err)
				}
				cmdScope := matrixScopes[1] // command-scoped (terminal cmd)
				registerMatrixCommands(t, p, cmdScope)
				f.setup(t, p, cmdScope.flagPath)
				p.Parse(matrixArgs(cmdScope.invoke, f.violate))

				msgs := diagnostics(p, f)
				if len(msgs) == 0 {
					t.Fatalf("[%s] expected the violation to produce a diagnostic to render", l.tag)
				}
				for _, msg := range msgs {
					for _, bad := range []string{"%!", "''", `"'`, `'"`} {
						if strings.Contains(msg, bad) {
							t.Errorf("[%s] %s: malformed render %q (marker %q)", l.tag, f.name, msg, bad)
						}
					}
				}
			})
		}
	}
}

// TestInteractionMatrixTranslatedNames crosses translated flag names (nameKey)
// with scope: a flag invoked by its localized name must resolve to the canonical
// flag whether global, command-scoped, or inherited — the i18n × parsing × commands
// seam. The canonical name must keep working in the same locale.
func TestInteractionMatrixTranslatedNames(t *testing.T) {
	for _, s := range matrixScopes {
		t.Run(s.name, func(t *testing.T) {
			build := func() *Parser {
				p := NewParser()
				b := i18n.NewEmptyBundle()
				if err := b.AddLanguage(language.Spanish, map[string]string{"flag.color": "tonalidad"}); err != nil {
					t.Fatal(err)
				}
				if err := p.SetUserBundle(b); err != nil {
					t.Fatal(err)
				}
				if err := p.SetLanguage(language.Spanish); err != nil {
					t.Fatal(err)
				}
				if len(s.flagPath) > 0 {
					registerMatrixCommands(t, p, s)
				}
				mustAddFlag(t, p, "color", NewArg(WithType(types.Single), WithNameKey("flag.color")), s.flagPath...)
				return p
			}

			// Invoked by its translated name -> must set the canonical flag.
			p := build()
			p.Parse(matrixArgs(s.invoke, []string{"--tonalidad", "red"}))
			if errs := p.GetErrors(); len(errs) != 0 {
				t.Fatalf("translated name: unexpected errors: %v", errs)
			}
			if !p.HasFlag("color", s.flagPath...) {
				t.Errorf("translated --tonalidad did not set canonical 'color'")
			}

			// Canonical name must still work in the same locale.
			p = build()
			p.Parse(matrixArgs(s.invoke, []string{"--color", "blue"}))
			if errs := p.GetErrors(); len(errs) != 0 {
				t.Fatalf("canonical name: unexpected errors: %v", errs)
			}
			if !p.HasFlag("color", s.flagPath...) {
				t.Errorf("canonical --color did not set 'color'")
			}
		})
	}
}

// TestInteractionMatrixShortFlags crosses POSIX short-flag invocation (-x) with
// scope. Short flags go through a separate parse path (parsePosixFlag) from long
// flags, so this checks they resolve — and carry their constraints — identically
// across global / command / inherited scope.
func TestInteractionMatrixShortFlags(t *testing.T) {
	for _, s := range matrixScopes {
		t.Run(s.name, func(t *testing.T) {
			build := func() *Parser {
				p := NewParser()
				if len(s.flagPath) > 0 {
					registerMatrixCommands(t, p, s)
				}
				mustAddFlag(t, p, "alpha", newStandalone(WithShortFlag("a"), WithMutex("g")), s.flagPath...)
				mustAddFlag(t, p, "beta", newStandalone(WithShortFlag("b"), WithMutex("g")), s.flagPath...)
				return p
			}

			// 1. a short flag sets its flag
			p := build()
			p.Parse(matrixArgs(s.invoke, []string{"-a"}))
			if errs := p.GetErrors(); len(errs) != 0 {
				t.Fatalf("-a: unexpected errors: %v", errs)
			}
			if !p.HasFlag("alpha", s.flagPath...) {
				t.Errorf("-a did not set 'alpha'")
			}

			// 2. short flags carry the constraint: -a -b must trip the mutex
			p = build()
			p.Parse(matrixArgs(s.invoke, []string{"-a", "-b"}))
			if !hasErr(p, errs.ErrMutexViolation) {
				t.Errorf("-a -b: expected mutex violation, got %v", p.GetErrors())
			}
		})
	}
}

// TestInteractionMatrixEnv crosses environment-variable resolution with scope. A
// required flag with no command-line value should be satisfied by its env var —
// and that resolution must work whether the flag is global, command-scoped, or
// inherited. Both a plain and a hyphenated flag name are exercised (the hyphenated
// shape is the one that historically failed to match the converter's output).
func TestInteractionMatrixEnv(t *testing.T) {
	// Maps a flag name to its env-var form: upper-case, hyphens -> underscores.
	// Idempotent on its own output, so matching is stable on both sides.
	conv := func(s string) string { return strings.ToUpper(strings.ReplaceAll(s, "-", "_")) }
	cases := []struct{ flag, env string }{
		{"envflag", "ENVFLAG"},   // plain
		{"env-flag", "ENV_FLAG"}, // hyphenated (historical-bug shape)
	}
	for _, c := range cases {
		for _, s := range matrixScopes {
			t.Run(c.flag+"/"+s.name, func(t *testing.T) {
				build := func() *Parser {
					p := NewParser()
					p.SetEnvNameConverter(conv)
					if len(s.flagPath) > 0 {
						registerMatrixCommands(t, p, s)
					}
					mustAddFlag(t, p, c.flag, NewArg(WithType(types.Single), WithRequired(true)), s.flagPath...)
					return p
				}

				// env unset, no arg -> the required flag must fire
				t.Run("unset", func(t *testing.T) {
					p := build()
					p.Parse(matrixArgs(s.invoke, nil))
					if !hasErr(p, errs.ErrRequiredFlag) {
						t.Errorf("env unset: expected required error, got %v", p.GetErrors())
					}
				})
				// env set -> it must satisfy the required flag (no error)
				t.Run("set", func(t *testing.T) {
					t.Setenv(c.env, "fromenv")
					p := build()
					p.Parse(matrixArgs(s.invoke, nil))
					if hasErr(p, errs.ErrRequiredFlag) {
						t.Errorf("env %s set: required should be satisfied, got %v", c.env, p.GetErrors())
					}
				})
			})
		}
	}
}

// envRoundtrip drives the struct-based env path: derived names where a field
// becomes a flag (FlagNameConverter) whose value can be supplied via an env var.
// Fields need tag content to register as flags (an empty `goopt:""` is "not a flag").
type envRoundtrip struct {
	GlobalVal string `goopt:"desc:global value"`
	Cmd       struct {
		CmdVal string   `goopt:"desc:command value"`
		Sub    struct{} `goopt:"kind:command"`
	} `goopt:"kind:command"`
}

// TestInteractionMatrixEnvStruct exercises the round-trip a real app wires: a field
// becomes a flag via the FlagNameConverter, and an env var resolves to it via the
// EnvNameConverter. By design the two converters must be the SAME — env matching
// runs the env converter over both the env var name and the (already flag-named)
// flag, so they match only when normalized to the same form. With ToKebabCase on
// both, env `GLOBAL_VAL` and flag `global-val` both normalize to `global-val`.
// Verified across global, command, and inherited scope.
func TestInteractionMatrixEnvStruct(t *testing.T) {
	newP := func(t *testing.T, cfg *envRoundtrip) *Parser {
		t.Helper()
		p, err := NewParserFromStruct(cfg,
			WithFlagNameConverter(ToKebabCase), // GlobalVal -> global-val
			WithEnvNameConverter(ToKebabCase),  // same converter, by design: GLOBAL_VAL & global-val both -> global-val
		)
		if err != nil {
			t.Fatalf("NewParserFromStruct: %v", err)
		}
		return p
	}

	// GlobalVal -> --global-val -> $GLOBAL_VAL (global scope)
	t.Run("global", func(t *testing.T) {
		t.Setenv("GLOBAL_VAL", "fromenv")
		cfg := &envRoundtrip{}
		p := newP(t, cfg)
		p.Parse([]string{os.Args[0]})
		if cfg.GlobalVal != "fromenv" {
			t.Errorf("GlobalVal = %q, want %q (round-trip GlobalVal->global-val->GLOBAL_VAL); errs=%v",
				cfg.GlobalVal, "fromenv", p.GetErrors())
		}
	})

	// Cmd.CmdVal -> --cmd-val -> $CMD_VAL, resolved when `cmd` is invoked.
	t.Run("command", func(t *testing.T) {
		t.Setenv("CMD_VAL", "fromenv")
		cfg := &envRoundtrip{}
		p := newP(t, cfg)
		p.Parse([]string{os.Args[0], "cmd"})
		if cfg.Cmd.CmdVal != "fromenv" {
			t.Errorf("Cmd.CmdVal = %q, want %q; errs=%v", cfg.Cmd.CmdVal, "fromenv", p.GetErrors())
		}
	})

	// Same field reached via a subcommand invocation (inherited scope).
	t.Run("inherited", func(t *testing.T) {
		t.Setenv("CMD_VAL", "fromenv")
		cfg := &envRoundtrip{}
		p := newP(t, cfg)
		p.Parse([]string{os.Args[0], "cmd", "sub"})
		if cfg.Cmd.CmdVal != "fromenv" {
			t.Errorf("Cmd.CmdVal = %q, want %q (inherited via `cmd sub`); errs=%v",
				cfg.Cmd.CmdVal, "fromenv", p.GetErrors())
		}
	})
}

// TestInteractionMatrixDefaults crosses default-value semantics with scope, locking
// three behaviors (and checking scope doesn't change them):
//   - `required` + `default` is a contradictory declaration, rejected at
//     construction (Design B) — a default makes the flag never "missing";
//   - a bad default is NOT run through validators (defaults are trusted);
//   - a provided value IS validated and overrides the default.
func TestInteractionMatrixDefaults(t *testing.T) {
	for _, s := range matrixScopes {
		t.Run(s.name, func(t *testing.T) {
			mk := func(t *testing.T) *Parser {
				p := NewParser()
				if len(s.flagPath) > 0 {
					registerMatrixCommands(t, p, s)
				}
				return p
			}

			// (A) required + default is contradictory -> rejected at construction.
			t.Run("required_plus_default_is_rejected", func(t *testing.T) {
				p := mk(t)
				err := p.AddFlag("req", NewArg(WithType(types.Single), WithDefaultValue("d"), WithRequired(true)), s.flagPath...)
				if !errors.Is(err, errs.ErrRequiredWithDefault) {
					t.Errorf("required+default should be rejected at construction; got %v", err)
				}
			})

			// (B) bad default, no value -> validators are NOT run on the default.
			t.Run("bad_default_bypasses_validators", func(t *testing.T) {
				p := mk(t)
				mustAddFlag(t, p, "val", NewArg(WithType(types.Single), WithDefaultValue("ab"), WithValidators(validation.MinLength(5))), s.flagPath...)
				p.Parse(matrixArgs(s.invoke, nil))
				if len(p.GetErrors()) != 0 {
					t.Errorf("a bad default should bypass validators (no error); got %v", p.GetErrors())
				}
			})

			// (C) a provided value that violates the validator IS caught (and the
			//     value path runs validation even though a default exists).
			t.Run("provided_value_is_validated", func(t *testing.T) {
				p := mk(t)
				mustAddFlag(t, p, "val", NewArg(WithType(types.Single), WithDefaultValue("ab"), WithValidators(validation.MinLength(5))), s.flagPath...)
				p.Parse(matrixArgs(s.invoke, []string{"--val", "abc"})) // len 3 < 5
				if len(p.GetErrors()) == 0 {
					t.Errorf("a provided value violating the validator should error; got none")
				}
			})
		})
	}
}

// TestInteractionMatrixPrecedence verifies the DOCUMENTED configuration precedence
// (default < env < config(ParseWithDefaults) < command line) actually holds in the
// implementation — in particular config-over-env, which the injection mechanism
// (both arrive as synthetic args) does not obviously guarantee.
func TestInteractionMatrixPrecedence(t *testing.T) {
	conv := func(s string) string { return strings.ToUpper(strings.ReplaceAll(s, "-", "_")) }
	build := func() *Parser {
		p := NewParser()
		p.SetEnvNameConverter(conv)
		_ = p.AddFlag("port", NewArg(WithType(types.Single), WithDefaultValue("8080")))
		return p
	}
	get := func(p *Parser) string { return p.GetOrDefault("port", "") }

	t.Run("default only", func(t *testing.T) {
		p := build()
		p.Parse([]string{os.Args[0]})
		if g := get(p); g != "8080" {
			t.Errorf("got %q, want 8080 (default)", g)
		}
	})
	t.Run("env over default", func(t *testing.T) {
		t.Setenv("PORT", "9000")
		p := build()
		p.Parse([]string{os.Args[0]})
		if g := get(p); g != "9000" {
			t.Errorf("got %q, want 9000 (env > default)", g)
		}
	})
	t.Run("config over env", func(t *testing.T) {
		t.Setenv("PORT", "9000")
		p := build()
		p.ParseWithDefaults(map[string]string{"port": "7000"}, []string{os.Args[0]})
		if g := get(p); g != "7000" {
			t.Errorf("got %q, want 7000 (config > env per docs)", g)
		}
	})
	t.Run("command line over all", func(t *testing.T) {
		t.Setenv("PORT", "9000")
		p := build()
		p.ParseWithDefaults(map[string]string{"port": "7000"}, []string{os.Args[0], "--port", "3000"})
		if g := get(p); g != "3000" {
			t.Errorf("got %q, want 3000 (cmdline > all)", g)
		}
	})
}

// TestInteractionMatrixBindDefault locks that a configured default is written to
// the bound target for BOTH direct BindFlag and struct binding (BindFlag used to
// leave the bound var empty), and that a provided value overrides it.
func TestInteractionMatrixBindDefault(t *testing.T) {
	t.Run("BindFlag writes default to bound var", func(t *testing.T) {
		var v string
		p := NewParser()
		if err := p.BindFlag(&v, "f", NewArg(WithType(types.Single), WithDefaultValue("d"))); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{os.Args[0]})
		if v != "d" {
			t.Errorf("BindFlag default: bound=%q, want %q", v, "d")
		}
	})
	t.Run("provided value overrides the default", func(t *testing.T) {
		var v string
		p := NewParser()
		if err := p.BindFlag(&v, "f", NewArg(WithType(types.Single), WithDefaultValue("d"))); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{os.Args[0], "--f", "x"})
		if v != "x" {
			t.Errorf("override: bound=%q, want %q", v, "x")
		}
	})
}
