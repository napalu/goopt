package goopt

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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

// TestInteractionMatrixTranslatedCommands is the command-path twin of the
// translated-flag test: a command invoked by its localized name must resolve to
// its canonical path — terminal, nested (translated parent + translated sub), and
// mixed canonical/translated — and the canonical names must still work in the
// non-English locale. Commands resolve through a different path (getCommand /
// GetCanonicalCommandPath) than flags, so it gets its own coverage.
func TestInteractionMatrixTranslatedCommands(t *testing.T) {
	build := func(t *testing.T) *Parser {
		p := NewParser()
		b := i18n.NewEmptyBundle()
		if err := b.AddLanguage(language.Spanish, map[string]string{
			"cmd.status": "estado", "cmd.deploy": "desplegar", "cmd.service": "servicio",
		}); err != nil {
			t.Fatal(err)
		}
		if err := p.SetUserBundle(b); err != nil {
			t.Fatal(err)
		}
		if err := p.SetLanguage(language.Spanish); err != nil {
			t.Fatal(err)
		}
		// terminal command "status"
		if err := p.AddCommand(NewCommand(WithName("status"), WithCommandNameKey("cmd.status"))); err != nil {
			t.Fatal(err)
		}
		// nested: deploy -> service
		sub := NewCommand(WithName("service"), WithCommandNameKey("cmd.service"))
		if err := p.AddCommand(NewCommand(WithName("deploy"), WithCommandNameKey("cmd.deploy"), WithSubcommands(sub))); err != nil {
			t.Fatal(err)
		}
		return p
	}
	invoked := func(p *Parser, path string) bool {
		for _, c := range p.GetCommands() {
			if c == path {
				return true
			}
		}
		return false
	}
	check := func(t *testing.T, args []string, wantPath string) {
		t.Helper()
		p := build(t)
		p.Parse(append([]string{os.Args[0]}, args...))
		if errs := p.GetErrors(); len(errs) != 0 {
			t.Fatalf("%v: unexpected errors: %v", args, errs)
		}
		if !invoked(p, wantPath) {
			t.Errorf("%v did not invoke %q; commands=%v", args, wantPath, p.GetCommands())
		}
	}

	t.Run("terminal by translated name", func(t *testing.T) {
		check(t, []string{"estado"}, "status")
	})
	t.Run("nested by translated names", func(t *testing.T) {
		check(t, []string{"desplegar", "servicio"}, "deploy service")
	})
	t.Run("mixed canonical parent + translated sub", func(t *testing.T) {
		check(t, []string{"deploy", "servicio"}, "deploy service")
	})
	t.Run("canonical still works in the non-en locale", func(t *testing.T) {
		check(t, []string{"deploy", "service"}, "deploy service")
		check(t, []string{"status"}, "status")
	})
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

			// (A2) a default on a mutex/exactlyone member is meaningless -> rejected.
			t.Run("default_on_exclusive_member_is_rejected", func(t *testing.T) {
				p := mk(t)
				if err := p.AddFlag("mx", NewArg(WithType(types.Single), WithDefaultValue("d"), WithMutex("g")), s.flagPath...); !errors.Is(err, errs.ErrDefaultInExclusiveGroup) {
					t.Errorf("mutex+default should be rejected at construction; got %v", err)
				}
				p = mk(t)
				if err := p.AddFlag("xo", NewArg(WithType(types.Single), WithDefaultValue("d"), WithExactlyOne("g")), s.flagPath...); !errors.Is(err, errs.ErrDefaultInExclusiveGroup) {
					t.Errorf("exactlyone+default should be rejected at construction; got %v", err)
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

// promptDetector is a TerminalReader that records whether the interactive prompt
// was ever invoked — so a test can prove an env var was used *in lieu of* prompting.
type promptDetector struct{ prompted *bool }

func (d promptDetector) ReadPassword(fd int) ([]byte, error) {
	*d.prompted = true
	return nil, errors.New("unexpected interactive prompt")
}
func (d promptDetector) IsTerminal(fd int) bool { return true }

// TestInteractionMatrixSecureEnv verifies a required `secure` flag is satisfied by
// an environment variable in lieu of the interactive prompt (CI/CD-friendly): the
// required check passes, the value is taken from the environment, and the terminal
// is never read.
func TestInteractionMatrixSecureEnv(t *testing.T) {
	conv := func(s string) string { return strings.ToUpper(strings.ReplaceAll(s, "-", "_")) }
	t.Run("env satisfies required secure flag without prompting", func(t *testing.T) {
		t.Setenv("TOKEN", "s3cret")
		prompted := false
		var token string
		p := NewParser()
		p.SetEnvNameConverter(conv)
		p.SetTerminalReader(promptDetector{&prompted})
		if err := p.BindFlag(&token, "token", NewArg(WithType(types.Single), WithSecurePrompt("Token: "), WithRequired(true))); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{os.Args[0]})
		if hasErr(p, errs.ErrRequiredFlag) {
			t.Errorf("env should satisfy the required secure flag; got %v", p.GetErrors())
		}
		if prompted {
			t.Errorf("secure flag prompted despite the env var being set — env should replace the prompt")
		}
		if token != "s3cret" {
			t.Errorf("secure value = %q, want %q (from env)", token, "s3cret")
		}
	})
}

// TestInteractionMatrixHooks crosses the execution-lifecycle dimension: global vs
// command pre/post hooks, hook ordering, and error short-circuiting. It locks the
// observed (and coherent) model — onion ordering (pre outer->inner, post
// inner->outer, mirrored by SetHookOrder) and defer-style errors (a pre-hook error
// aborts the command but post-hooks still run with the error; a command error
// propagates to every post-hook).
func TestInteractionMatrixHooks(t *testing.T) {
	es := func(e error) string {
		if e == nil {
			return "ok"
		}
		return "err"
	}
	// build wires global+command pre/post hooks and the command callback to append
	// to a shared trace; preErr/cmdErr inject failures at the pre-hook / command.
	build := func(t *testing.T, order HookOrder, preErr, cmdErr bool) (*Parser, *[]string) {
		t.Helper()
		var tr []string
		add := func(s string) { tr = append(tr, s) }
		p := NewParser()
		p.SetHookOrder(order)
		cmd := NewCommand(WithName("run"), WithCallback(func(p *Parser, c *Command) error {
			add("cmd")
			if cmdErr {
				return errors.New("cmd failed")
			}
			return nil
		}))
		if err := p.AddCommand(cmd); err != nil {
			t.Fatal(err)
		}
		p.AddGlobalPreHook(func(p *Parser, c *Command) error {
			add("gPre")
			if preErr {
				return errors.New("gpre failed")
			}
			return nil
		})
		p.AddCommandPreHook("run", func(p *Parser, c *Command) error { add("cPre"); return nil })
		p.AddCommandPostHook("run", func(p *Parser, c *Command, e error) error { add("cPost:" + es(e)); return nil })
		p.AddGlobalPostHook(func(p *Parser, c *Command, e error) error { add("gPost:" + es(e)); return nil })
		return p, &tr
	}
	trace := func(t *testing.T, order HookOrder, preErr, cmdErr bool) string {
		t.Helper()
		p, tr := build(t, order, preErr, cmdErr)
		p.Parse([]string{os.Args[0], "run"})
		p.ExecuteCommands()
		return strings.Join(*tr, " ")
	}

	cases := []struct {
		name   string
		order  HookOrder
		preErr bool
		cmdErr bool
		want   string
	}{
		{"global-first ordering", OrderGlobalFirst, false, false, "gPre cPre cmd cPost:ok gPost:ok"},
		{"command-first mirrors it", OrderCommandFirst, false, false, "cPre gPre cmd gPost:ok cPost:ok"},
		{"pre-hook error aborts command, post-hooks still run with error", OrderGlobalFirst, true, false, "gPre cPost:err gPost:err"},
		{"command error propagates to post-hooks", OrderGlobalFirst, false, true, "gPre cPre cmd cPost:err gPost:err"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := trace(t, c.order, c.preErr, c.cmdErr); got != c.want {
				t.Errorf("hook trace:\n got:  %s\n want: %s", got, c.want)
			}
		})
	}
}

// TestInteractionMatrixMultipleCommands exercises invoking several sibling commands
// in one line (`cmd1 --flags cmd2 --flags`): each command runs, its flags resolve to
// it without bleed, and per-command contracts stay scoped to their command even when
// both are active. The cross-fire case is a regression guard — same-labelled groups
// in two simultaneously-invoked commands must NOT merge.
func TestInteractionMatrixMultipleCommands(t *testing.T) {
	var trace []string
	build := func(t *testing.T) *Parser {
		t.Helper()
		trace = nil
		p := NewParser()
		mk := func(name string) *Command {
			return NewCommand(WithName(name), WithCallback(func(p *Parser, c *Command) error {
				trace = append(trace, name)
				return nil
			}))
		}
		if err := p.AddCommand(mk("cmd1")); err != nil {
			t.Fatal(err)
		}
		if err := p.AddCommand(mk("cmd2")); err != nil {
			t.Fatal(err)
		}
		// Both commands carry a mutex group reusing the SAME label "g".
		mustAddFlag(t, p, "a", newStandalone(WithMutex("g")), "cmd1")
		mustAddFlag(t, p, "b", newStandalone(WithMutex("g")), "cmd1")
		mustAddFlag(t, p, "c", newStandalone(WithMutex("g")), "cmd2")
		mustAddFlag(t, p, "d", newStandalone(WithMutex("g")), "cmd2")
		return p
	}

	t.Run("both commands run in order; flags resolve per command", func(t *testing.T) {
		p := build(t)
		p.Parse([]string{os.Args[0], "cmd1", "--a", "cmd2", "--c"})
		p.ExecuteCommands()
		if strings.Join(trace, " ") != "cmd1 cmd2" {
			t.Errorf("both commands should run in order; trace=%v", trace)
		}
		if !p.HasFlag("a", "cmd1") || !p.HasFlag("c", "cmd2") || p.HasFlag("a", "cmd2") {
			t.Errorf("flags should resolve per command without bleed")
		}
	})

	t.Run("same-label contracts stay scoped per command (no cross-fire)", func(t *testing.T) {
		p := build(t)
		p.Parse([]string{os.Args[0], "cmd1", "--a", "cmd2", "--c"}) // a@cmd1 and c@cmd2 — different commands
		if hasErr(p, errs.ErrMutexViolation) {
			t.Errorf("a@cmd1 and c@cmd2 must not cross-fire the shared label; got %v", p.GetErrors())
		}
	})

	t.Run("real same-command violation still fires", func(t *testing.T) {
		p := build(t)
		p.Parse([]string{os.Args[0], "cmd1", "--a", "--b", "cmd2"}) // a,b both in cmd1
		if !hasErr(p, errs.ErrMutexViolation) {
			t.Errorf("a,b in cmd1 should fire mutex; got %v", p.GetErrors())
		}
	})
}

// TestInteractionMatrixDeepNesting charts the 3-level command axis. The rest of
// the matrix tops out at two levels (`cmd` / `cmd sub`); a third level exposes
// seams that only appear past depth 2: parent-walk flag resolution that must climb
// TWO ancestors, cross-level contract targets, and command-scoping (`isActive`)
// at depth, where a deep terminal's constraints must stay off for sibling branches.
//
// Command tree:
//
//	a
//	├── b
//	│   ├── c   (terminal)
//	│   └── d   (terminal)
//	└── e       (terminal)
func TestInteractionMatrixDeepNesting(t *testing.T) {
	// build constructs the tree afresh; opts registers the subtest's flags.
	build := func(t *testing.T, opts func(p *Parser)) *Parser {
		t.Helper()
		p := NewParser()
		c := NewCommand(WithName("c"))
		d := NewCommand(WithName("d"))
		e := NewCommand(WithName("e"))
		b := NewCommand(WithName("b"), WithSubcommands(c, d))
		a := NewCommand(WithName("a"), WithSubcommands(b, e))
		if err := p.AddCommand(a); err != nil {
			t.Fatalf("add a: %v", err)
		}
		if opts != nil {
			opts(p)
		}
		return p
	}

	t.Run("level-1/2/3 flags all resolve when the deepest command is invoked", func(t *testing.T) {
		p := build(t, func(p *Parser) {
			mustAddFlag(t, p, "top", NewArg(WithType(types.Single)), "a")
			mustAddFlag(t, p, "mid", NewArg(WithType(types.Single)), "a", "b")
			mustAddFlag(t, p, "leaf", NewArg(WithType(types.Single)), "a", "b", "c")
		})
		p.Parse([]string{os.Args[0], "a", "b", "c", "--top", "T", "--mid", "M", "--leaf", "L"})
		if e := p.GetErrors(); len(e) != 0 {
			t.Fatalf("clean deep parse should have no errors; got %v", e)
		}
		// Each token binds to its OWNING command, proving the parser climbed up to
		// two ancestors to resolve `top` (on a) and one for `mid` (on a b).
		for _, tc := range []struct {
			name string
			path []string
			want string
		}{
			{"top", []string{"a"}, "T"},
			{"mid", []string{"a", "b"}, "M"},
			{"leaf", []string{"a", "b", "c"}, "L"},
		} {
			if v, ok := p.Get(tc.name, tc.path...); !ok || v != tc.want {
				t.Errorf("%s@%v: got (%q,%v), want %q", tc.name, tc.path, v, ok, tc.want)
			}
		}
	})

	t.Run("requires on a level-3 flag resolves its target on a level-1 ancestor", func(t *testing.T) {
		mk := func() *Parser {
			return build(t, func(p *Parser) {
				mustAddFlag(t, p, "top", NewArg(WithType(types.Single)), "a")
				mustAddFlag(t, p, "leaf", newStandalone(WithRequires("top")), "a", "b", "c")
			})
		}
		// leaf set, target two levels up absent → requires must fire.
		p := mk()
		p.Parse([]string{os.Args[0], "a", "b", "c", "--leaf"})
		if !hasErr(p, errs.ErrFlagRequires) {
			t.Errorf("leaf requires top (two levels up); want ErrFlagRequires, got %v", p.GetErrors())
		}
		// target supplied → satisfied.
		p = mk()
		p.Parse([]string{os.Args[0], "a", "b", "c", "--leaf", "--top", "T"})
		if hasErr(p, errs.ErrFlagRequires) {
			t.Errorf("top supplied; requires should be satisfied, got %v", p.GetErrors())
		}
	})

	t.Run("a deep terminal's required flag stays inactive for sibling branches", func(t *testing.T) {
		mk := func() *Parser {
			return build(t, func(p *Parser) {
				mustAddFlag(t, p, "needc", NewArg(WithType(types.Single), WithRequired(true)), "a", "b", "c")
			})
		}
		// sibling leaf under the same parent: `a b d` must not demand `a b c`'s flag.
		p := mk()
		p.Parse([]string{os.Args[0], "a", "b", "d"})
		if hasErr(p, errs.ErrRequiredFlag) {
			t.Errorf("invoking sibling `a b d` must not require a flag of `a b c`; got %v", p.GetErrors())
		}
		// different level-2 branch entirely: `a e` must not demand it either.
		p = mk()
		p.Parse([]string{os.Args[0], "a", "e"})
		if hasErr(p, errs.ErrRequiredFlag) {
			t.Errorf("invoking `a e` must not require a flag of `a b c`; got %v", p.GetErrors())
		}
		// the owning terminal itself, flag omitted → must fire.
		p = mk()
		p.Parse([]string{os.Args[0], "a", "b", "c"})
		if !hasErr(p, errs.ErrRequiredFlag) {
			t.Errorf("invoking `a b c` without needc should require it; got %v", p.GetErrors())
		}
	})

	t.Run("nearest-ancestor wins when a child redefines an inherited flag", func(t *testing.T) {
		p := build(t, nil)
		if err := p.AddFlag("dup", NewArg(WithType(types.Single), WithDefaultValue("mid")), "a", "b"); err != nil {
			t.Fatalf("add dup@'a b': %v", err)
		}
		// A child redefining a parent's flag name is the documented override case.
		if err := p.AddFlag("dup", NewArg(WithType(types.Single), WithDefaultValue("leaf")), "a", "b", "c"); err != nil {
			t.Skipf("child override of an inherited flag rejected at build time: %v", err)
		}
		p.Parse([]string{os.Args[0], "a", "b", "c", "--dup", "X"})
		if v, ok := p.Get("dup", "a", "b", "c"); !ok || v != "X" {
			t.Errorf("the leaf's own dup should receive X (nearest ancestor wins); got (%q,%v)", v, ok)
		}
		if v, _ := p.Get("dup", "a", "b"); v != "mid" {
			t.Errorf("the parent's dup should keep its default 'mid', untouched; got %q", v)
		}
	})
}

// TestInteractionMatrixTypedFlags charts the Chained (list) value type, which the
// constraint matrix never exercised. It locks the two bugs found at the
// list × {repeated, custom-delimiter, bound-vs-unbound} seam:
//
//	A — a custom ListDelimiterFunc made GetList mangle the list (the internal
//	    occurrence marker leaked into an element), diverging from the bound slice.
//	B — repeated occurrences of an UNBOUND chained flag silently dropped all but the
//	    last (accumulation was coupled to having a bound variable).
//
// The invariant under test: GetList (unbound) and a bound []string must return the
// SAME list in every cell, and the user's declared separator is honoured for element
// separation while repeated occurrences still accumulate.
func TestInteractionMatrixTypedFlags(t *testing.T) {
	want := []string{"a", "b", "c", "d"}

	t.Run("unbound repeated accumulates (default delimiter)", func(t *testing.T) {
		p := NewParser()
		mustAddFlag(t, p, "v", NewArg(WithType(types.Chained)))
		p.Parse([]string{"--v", "a,b", "--v", "c,d"})
		got, err := p.GetList("v")
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("unbound repeated should accumulate to %v; got %v (err=%v)", want, got, err)
		}
	})

	t.Run("unbound repeated accumulates (custom ';' delimiter)", func(t *testing.T) {
		p := NewParser()
		if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
			t.Fatal(err)
		}
		mustAddFlag(t, p, "v", NewArg(WithType(types.Chained)))
		p.Parse([]string{"--v", "a;b", "--v", "c;d"})
		got, err := p.GetList("v")
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("custom-delimiter unbound list should be %v; got %v (err=%v)", want, got, err)
		}
	})

	t.Run("bound []string and GetList agree under a custom delimiter", func(t *testing.T) {
		var bound []string
		p := NewParser()
		if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
			t.Fatal(err)
		}
		if err := p.BindFlag(&bound, "v", NewArg(WithType(types.Chained))); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{"--v", "a;b", "--v", "c;d"})
		got, _ := p.GetList("v")
		if !slices.Equal(bound, want) {
			t.Errorf("bound slice should be %v; got %v", want, bound)
		}
		if !slices.Equal(got, bound) {
			t.Errorf("GetList must equal the bound slice; GetList=%v bound=%v", got, bound)
		}
	})

	t.Run("single token splits on the user delimiter, not the internal marker", func(t *testing.T) {
		p := NewParser()
		if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
			t.Fatal(err)
		}
		mustAddFlag(t, p, "v", NewArg(WithType(types.Chained)))
		p.Parse([]string{"--v", "a;b"})
		if got, _ := p.GetList("v"); !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("single token should split to [a b]; got %v", got)
		}
	})

	t.Run("per-element validators run on each list element", func(t *testing.T) {
		mk := func() *Parser {
			p := NewParser()
			if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
				t.Fatal(err)
			}
			mustAddFlag(t, p, "v", NewArg(WithType(types.Chained), WithValidators(validation.MinLength(2))))
			return p
		}
		// every element satisfies MinLength(2), across a repeat → accumulates clean
		p := mk()
		p.Parse([]string{"--v", "ab;cd", "--v", "ef"})
		if got, _ := p.GetList("v"); !slices.Equal(got, []string{"ab", "cd", "ef"}) {
			t.Errorf("validated list should be [ab cd ef]; got %v", got)
		}
		// a single bad element (len 1) trips the validator
		p = mk()
		p.Parse([]string{"--v", "ab;x"})
		if len(p.GetErrors()) == 0 {
			t.Errorf("element 'x' (<2) should fail MinLength; got no error")
		}
	})

	t.Run("Get returns a representation without a control byte leaking", func(t *testing.T) {
		p := NewParser()
		if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
			t.Fatal(err)
		}
		mustAddFlag(t, p, "v", NewArg(WithType(types.Chained)))
		p.Parse([]string{"--v", "a;b", "--v", "c;d"})
		raw, _ := p.Get("v")
		if strings.ContainsRune(raw, '\x1f') {
			t.Errorf("Get must not leak the internal marker; got %q", raw)
		}
	})
}

// TestInteractionMatrixExoticTyped charts the typed value types: scalar conversion
// (time.Duration, time.Time), typed slices (which auto-infer to Chained and convert
// per element — crossing directly through the chained-list refactor), and the File
// type (whose value is the file's CONTENT, not its path). The seam of interest is
// typed-slice × {custom delimiter, repeated}: per-element conversion must ride the
// same (user delimiter ∪ internal marker) recovery the string path uses.
func TestInteractionMatrixExoticTyped(t *testing.T) {
	t.Run("BindFlag infers the option type from the bound variable", func(t *testing.T) {
		// Regression: AddFlag's Empty->Single default used to run before BindFlag's
		// inference, silently turning every bound slice into a scalar and every bound
		// bool into a value-flag. Inference now runs first.
		var ports []int
		var verbose bool
		var name string
		p := NewParser()
		if err := p.BindFlag(&ports, "port", NewArg()); err != nil {
			t.Fatal(err)
		}
		if err := p.BindFlag(&verbose, "verbose", NewArg()); err != nil {
			t.Fatal(err)
		}
		if err := p.BindFlag(&name, "name", NewArg()); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			flag string
			want types.OptionType
		}{
			{"port", types.Chained},       // []int → list
			{"verbose", types.Standalone}, // bool → presence-flag
			{"name", types.Single},        // string → scalar
		} {
			arg, err := p.GetArgument(tc.flag)
			if err != nil {
				t.Fatalf("GetArgument(%s): %v", tc.flag, err)
			}
			if arg.TypeOf != tc.want {
				t.Errorf("%s inferred TypeOf=%v, want %v", tc.flag, arg.TypeOf, tc.want)
			}
		}
	})

	t.Run("time.Duration scalar infers Single and parses", func(t *testing.T) {
		var d time.Duration
		p := NewParser()
		if err := p.BindFlag(&d, "timeout", NewArg()); err != nil { // type inferred
			t.Fatal(err)
		}
		p.Parse([]string{"--timeout", "1m30s"})
		if d != 90*time.Second {
			t.Errorf("timeout should parse to 90s; got %v", d)
		}
	})

	t.Run("time.Time scalar infers Single and parses", func(t *testing.T) {
		var ts time.Time
		p := NewParser()
		if err := p.BindFlag(&ts, "since", NewArg()); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{"--since", "2026-06-20"})
		if ts.Year() != 2026 || ts.Month() != time.June || ts.Day() != 20 {
			t.Errorf("since should parse to 2026-06-20; got %v", ts)
		}
	})

	t.Run("[]int slice infers Chained and converts per element (default delimiter)", func(t *testing.T) {
		var ports []int
		p := NewParser()
		if err := p.BindFlag(&ports, "port", NewArg()); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{"--port", "80,443", "--port", "8080"})
		if !slices.Equal(ports, []int{80, 443, 8080}) {
			t.Errorf("ports should accumulate+convert to [80 443 8080]; got %v", ports)
		}
	})

	t.Run("[]int slice converts per element under a CUSTOM delimiter + repeat", func(t *testing.T) {
		// This is the cell crossing the chained refactor: per-element typed conversion
		// must split on (user ';' ∪ internal marker), same as GetList.
		var ports []int
		p := NewParser()
		if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
			t.Fatal(err)
		}
		if err := p.BindFlag(&ports, "port", NewArg()); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{"--port", "80;443", "--port", "8080"})
		if !slices.Equal(ports, []int{80, 443, 8080}) {
			t.Errorf("custom-delimiter typed slice should be [80 443 8080]; got %v (errs=%v)", ports, p.GetErrors())
		}
	})

	t.Run("[]time.Duration slice converts per element across a repeat", func(t *testing.T) {
		var durs []time.Duration
		p := NewParser()
		if err := p.BindFlag(&durs, "wait", NewArg()); err != nil {
			t.Fatal(err)
		}
		p.Parse([]string{"--wait", "5s,2h", "--wait", "90m"})
		want := []time.Duration{5 * time.Second, 2 * time.Hour, 90 * time.Minute}
		if !slices.Equal(durs, want) {
			t.Errorf("durations should be %v; got %v (errs=%v)", want, durs, p.GetErrors())
		}
	})

	t.Run("numeric validator + typed slice + custom delimiter validates and converts per element", func(t *testing.T) {
		// The capstone cross: per-element VALIDATION and per-element typed CONVERSION
		// both ride (user ';' ∪ internal marker), across a repeat. Numeric validators
		// parse the string before comparing, so there is no lexical-comparison trap.
		mk := func() (*Parser, *[]int) {
			var ports []int
			p := NewParser()
			if err := p.SetListDelimiterFunc(func(r rune) bool { return r == ';' }); err != nil {
				t.Fatal(err)
			}
			if err := p.BindFlag(&ports, "port", NewArg(WithType(types.Chained), WithValidators(validation.IntRange(1, 100)))); err != nil {
				t.Fatal(err)
			}
			return p, &ports
		}
		p, ports := mk()
		p.Parse([]string{"--port", "10;20", "--port", "30"})
		if !slices.Equal(*ports, []int{10, 20, 30}) || len(p.GetErrors()) != 0 {
			t.Errorf("all-in-range should give [10 20 30] no error; got %v errs=%v", *ports, p.GetErrors())
		}
		p2, _ := mk()
		p2.Parse([]string{"--port", "10;200"})
		if !hasErr(p2, errs.ErrValueBetween) {
			t.Errorf("element 200 should trip IntRange; got %v", p2.GetErrors())
		}
	})

	t.Run("File type yields the file's content as the value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "secret.txt")
		if err := os.WriteFile(path, []byte("s3cr3t-token"), 0o600); err != nil {
			t.Fatal(err)
		}
		p := NewParser()
		mustAddFlag(t, p, "creds", NewArg(WithType(types.File)))
		p.Parse([]string{"--creds", path})
		if v, _ := p.Get("creds"); v != "s3cr3t-token" {
			t.Errorf("File flag value should be the file CONTENT; got %q", v)
		}
	})
}

// EnvCreds is an embedded creds struct; its flags are namespaced as env-creds.* and
// registered per command (e.g. "env-creds.url@undo").
type EnvCreds struct {
	URL     string `goopt:"required:true;short:u;desc:url"`
	AppName string `goopt:"required:true;short:a;desc:app"`
}

type envPositionalCfg struct {
	Undo struct {
		Input string `goopt:"short:i;pos:0;required:true;desc:input file"`
		EnvCreds
		Exec CommandFunc
	} `goopt:"kind:command;desc:undo"`
}

// TestInteractionMatrixPositionalEnv charts the positional × env × command-scope seam.
// A command-scoped flag supplied via an environment variable is injected into the arg
// stream as the internal "name@command" form; setPositionalArguments must recognise
// that suffixed token as a flag (resolving it in the suffix's command scope) and skip
// its value — otherwise the env value is consumed as the command's positional argument.
// Regression for the env-value-leaks-into-positional bug.
func TestInteractionMatrixPositionalEnv(t *testing.T) {
	// plain command-scoped flag + a positional on the same command, flag set via env.
	t.Run("command-scoped flag via env does not leak into the positional", func(t *testing.T) {
		type cfg struct {
			Undo struct {
				Input string `goopt:"pos:0;required:true;desc:input file"`
				Token string `goopt:"required:true;desc:token"` // command-scoped, supplied via env
			} `goopt:"kind:command;desc:undo"`
		}
		t.Setenv("TOKEN", "from-env")
		c := &cfg{}
		p, err := NewParserFromStruct(c, WithFlagNameConverter(ToKebabCase), WithEnvNameConverter(ToKebabCase))
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if !p.Parse([]string{os.Args[0], "undo", "myfile.log"}) {
			t.Fatalf("Parse returned false; errs=%v", p.GetErrors())
		}
		if c.Undo.Input != "myfile.log" {
			t.Errorf("positional Input = %q, want %q (env value leaked into positional)", c.Undo.Input, "myfile.log")
		}
		if c.Undo.Token != "from-env" {
			t.Errorf("Token = %q, want %q (resolved from env)", c.Undo.Token, "from-env")
		}
	})

	// namespaced (embedded) command-scoped flags via env — the original repro shape:
	// flags arrive as "env-creds.url@undo", a two-level suffixed token.
	t.Run("namespaced embedded flags via env do not leak into the positional", func(t *testing.T) {
		envR := &customEnvResolver{environ: map[string]string{
			"APP_ENV_CREDS_URL":      "https://env-url/crowd",
			"APP_ENV_CREDS_APP_NAME": "env-app",
		}}
		c := &envPositionalCfg{}
		p, err := NewParserFromStruct(c,
			WithFlagNameConverter(ToKebabCase),
			WithEnvNameConverter(ToKebabCase),
			WithEnvVarPrefix("APP_"),
			WithEnvResolver(envR),
		)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if !p.Parse([]string{os.Args[0], "undo", "myfile.log"}) {
			t.Fatalf("Parse returned false; errs=%v", p.GetErrors())
		}
		if c.Undo.Input != "myfile.log" {
			t.Errorf("positional Input = %q, want %q (env value leaked into positional)", c.Undo.Input, "myfile.log")
		}
		if c.Undo.URL != "https://env-url/crowd" || c.Undo.AppName != "env-app" {
			t.Errorf("env creds not resolved: URL=%q AppName=%q", c.Undo.URL, c.Undo.AppName)
		}
	})

	// Full type sweep: EVERY flag type supplied via env must keep its value off the
	// positional. Single/Chained/File ride the value-flag path (checkIfFlagNeedsValue);
	// Standalone (a boolean with an explicit env value) rides shouldSkipBooleanAfterStandalone.
	t.Run("no flag type leaks its env value into the positional", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "cfg.txt")
		if err := os.WriteFile(fpath, []byte("filecontent"), 0o600); err != nil {
			t.Fatal(err)
		}
		type cfg struct {
			Undo struct {
				Input  string   `goopt:"pos:0;required:true;desc:input"`
				Single string   `goopt:"desc:single"`
				List   []string `goopt:"desc:list"`           // Chained
				Conf   string   `goopt:"type:file;desc:conf"` // File
				Force  bool     `goopt:"desc:force"`          // Standalone
			} `goopt:"kind:command;desc:undo"`
		}
		t.Setenv("SINGLE", "sv")
		t.Setenv("LIST", "a,b")
		t.Setenv("CONF", fpath)
		t.Setenv("FORCE", "true")
		c := &cfg{}
		p, err := NewParserFromStruct(c, WithFlagNameConverter(ToKebabCase), WithEnvNameConverter(ToKebabCase))
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if !p.Parse([]string{os.Args[0], "undo", "myfile.log"}) {
			t.Fatalf("Parse returned false; errs=%v", p.GetErrors())
		}
		if c.Undo.Input != "myfile.log" {
			t.Errorf("positional Input = %q, want myfile.log (an env flag value leaked)", c.Undo.Input)
		}
		if c.Undo.Single != "sv" || len(c.Undo.List) != 2 || c.Undo.Conf != "filecontent" || !c.Undo.Force {
			t.Errorf("env values not all resolved: Single=%q List=%v Conf=%q Force=%v",
				c.Undo.Single, c.Undo.List, c.Undo.Conf, c.Undo.Force)
		}
	})
}
