package goopt

import (
	"errors"
	"testing"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/types"
)

func TestParseContracts(t *testing.T) {
	cs, err := parseContracts([]string{"mutex(src)", "conflicts(a,b)"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cs) != 2 || cs[0].Kind != ContractMutex || cs[1].Kind != ContractConflicts {
		t.Fatalf("unexpected contracts: %+v", cs)
	}
	if cs[0].Targets[0] != "src" {
		t.Fatalf("mutex group = %v, want src", cs[0].Targets)
	}
	if len(cs[1].Targets) != 2 {
		t.Fatalf("conflicts targets = %v, want 2", cs[1].Targets)
	}

	for _, bad := range []string{"mutex", "nope(x)", "mutex(a,b)", "conflicts()"} {
		if _, err := parseContracts([]string{bad}); err == nil {
			t.Errorf("expected parse error for %q", bad)
		}
	}
}

func newStandalone(configs ...ConfigureArgumentFunc) *Argument {
	return NewArg(append([]ConfigureArgumentFunc{WithType(types.Standalone)}, configs...)...)
}

func hasErr(p *Parser, target error) bool {
	for _, e := range p.GetErrors() {
		if errors.Is(e, target) {
			return true
		}
	}
	return false
}

func TestContractMutex(t *testing.T) {
	build := func(t *testing.T) *Parser {
		p := NewParser()
		if err := p.AddFlag("alpha", newStandalone(WithMutex("mode"))); err != nil {
			t.Fatal(err)
		}
		if err := p.AddFlag("beta", newStandalone(WithMutex("mode"))); err != nil {
			t.Fatal(err)
		}
		return p
	}

	p := build(t)
	p.Parse([]string{"app", "--alpha", "--beta"})
	if !hasErr(p, errs.ErrMutexViolation) {
		t.Fatalf("both set: expected mutex violation, got %v", p.GetErrors())
	}

	p = build(t)
	p.Parse([]string{"app", "--alpha"})
	if hasErr(p, errs.ErrMutexViolation) {
		t.Fatalf("one set: unexpected violation: %v", p.GetErrors())
	}
}

func TestContractMutexSingletonGuard(t *testing.T) {
	p := NewParser()
	if err := p.AddFlag("solo", newStandalone(WithMutex("typo"))); err != nil {
		t.Fatal(err)
	}
	p.Parse([]string{"app"})
	if !hasErr(p, errs.ErrSingletonContractGroup) {
		t.Fatalf("expected singleton-group guard, got %v", p.GetErrors())
	}
}

func TestContractConflicts(t *testing.T) {
	build := func(t *testing.T) *Parser {
		p := NewParser()
		if err := p.AddFlag("alpha", newStandalone(WithConflicts("beta"))); err != nil {
			t.Fatal(err)
		}
		if err := p.AddFlag("beta", newStandalone()); err != nil {
			t.Fatal(err)
		}
		return p
	}

	p := build(t)
	p.Parse([]string{"app", "--alpha", "--beta"})
	if !hasErr(p, errs.ErrConflictingFlags) {
		t.Fatalf("both set: expected conflict, got %v", p.GetErrors())
	}

	p = build(t)
	p.Parse([]string{"app", "--alpha"})
	if hasErr(p, errs.ErrConflictingFlags) {
		t.Fatalf("one set: unexpected conflict: %v", p.GetErrors())
	}
}

func TestContractRequires(t *testing.T) {
	build := func(t *testing.T) *Parser {
		p := NewParser()
		if err := p.AddFlag("a", newStandalone(WithRequires("b"))); err != nil {
			t.Fatal(err)
		}
		if err := p.AddFlag("b", newStandalone()); err != nil {
			t.Fatal(err)
		}
		return p
	}

	p := build(t)
	p.Parse([]string{"app", "--a"}) // a set, b missing
	if !hasErr(p, errs.ErrFlagRequires) {
		t.Fatalf("a set, b missing: expected requires error, got %v", p.GetErrors())
	}
	t.Logf("requires msg: %v", p.GetErrors())

	p = build(t)
	p.Parse([]string{"app", "--a", "--b"})
	if hasErr(p, errs.ErrFlagRequires) {
		t.Fatalf("a+b set: unexpected error: %v", p.GetErrors())
	}

	p = build(t)
	p.Parse([]string{"app"}) // a not set -> requires not enforced
	if hasErr(p, errs.ErrFlagRequires) {
		t.Fatalf("a absent: unexpected error: %v", p.GetErrors())
	}
}

func TestContractRequiredOn(t *testing.T) {
	build := func(t *testing.T) *Parser {
		p := NewParser()
		if err := p.AddFlag("verbose", newStandalone()); err != nil {
			t.Fatal(err)
		}
		if err := p.AddFlag("token", NewArg(WithRequiredOn("verbose"))); err != nil {
			t.Fatal(err)
		}
		return p
	}

	p := build(t)
	p.Parse([]string{"app", "--verbose"}) // trigger active, token missing
	if !hasErr(p, errs.ErrRequiredWhen) {
		t.Fatalf("verbose set, token missing: expected required error, got %v", p.GetErrors())
	}
	t.Logf("requiredOn msg: %v", p.GetErrors())

	p = build(t)
	p.Parse([]string{"app", "--verbose", "--token", "x"})
	if hasErr(p, errs.ErrRequiredWhen) {
		t.Fatalf("token provided: unexpected error: %v", p.GetErrors())
	}

	p = build(t)
	p.Parse([]string{"app"}) // trigger absent -> not required
	if hasErr(p, errs.ErrRequiredWhen) {
		t.Fatalf("verbose absent: unexpected required error: %v", p.GetErrors())
	}
}

func TestContractExactlyOne(t *testing.T) {
	build := func(t *testing.T) *Parser {
		p := NewParser()
		if err := p.AddFlag("alpha", newStandalone(WithExactlyOne("mode"))); err != nil {
			t.Fatal(err)
		}
		if err := p.AddFlag("beta", newStandalone(WithExactlyOne("mode"))); err != nil {
			t.Fatal(err)
		}
		return p
	}

	p := build(t)
	p.Parse([]string{"app"}) // none set
	if !hasErr(p, errs.ErrExactlyOneRequired) {
		t.Fatalf("none set: expected exactly-one-required, got %v", p.GetErrors())
	}
	t.Logf("exactlyone msg: %v", p.GetErrors())

	p = build(t)
	p.Parse([]string{"app", "--alpha"}) // one set
	if hasErr(p, errs.ErrExactlyOneRequired) || hasErr(p, errs.ErrMutexViolation) {
		t.Fatalf("one set: unexpected error: %v", p.GetErrors())
	}

	p = build(t)
	p.Parse([]string{"app", "--alpha", "--beta"}) // both set
	if !hasErr(p, errs.ErrMutexViolation) {
		t.Fatalf("both set: expected mutex violation, got %v", p.GetErrors())
	}
}

func TestContractSingletonBuildTime(t *testing.T) {
	type CLI struct {
		Solo bool `goopt:"name:solo;contract:mutex(typo)"`
	}
	_, err := NewParserFromStruct(&CLI{})
	if err == nil {
		t.Fatal("expected construction error for singleton mutex group")
	}
	if !errors.Is(err, errs.ErrSingletonContractGroup) {
		t.Fatalf("expected singleton error at construction, got %v", err)
	}
}

func TestContractProgrammaticAccessors(t *testing.T) {
	p := NewParser()
	if err := p.AddFlag("alpha", newStandalone()); err != nil {
		t.Fatal(err)
	}
	if err := p.AddFlag("beta", newStandalone()); err != nil {
		t.Fatal(err)
	}

	// Add via value constructors.
	if err := p.AddFlagContracts("alpha", Mutex("mode")); err != nil {
		t.Fatalf("AddFlagContracts alpha: %v", err)
	}
	if err := p.AddFlagContracts("beta", Mutex("mode")); err != nil {
		t.Fatalf("AddFlagContracts beta: %v", err)
	}

	// Get returns what we set.
	cs, err := p.GetFlagContracts("alpha")
	if err != nil || len(cs) != 1 || cs[0].Kind != ContractMutex || cs[0].Targets[0] != "mode" {
		t.Fatalf("GetFlagContracts alpha = %+v, err=%v", cs, err)
	}
	// Returned slice is a copy: mutating it must not affect parser state.
	cs[0].Targets[0] = "tampered"
	again, _ := p.GetFlagContracts("alpha")
	if again[0].Targets[0] != "mode" {
		t.Fatalf("GetFlagContracts returned an aliased slice: %+v", again)
	}

	// Enforced at parse time.
	p.Parse([]string{"app", "--alpha", "--beta"})
	if !hasErr(p, errs.ErrMutexViolation) {
		t.Fatalf("expected mutex violation, got %v", p.GetErrors())
	}

	// Set replaces; Clear removes.
	if err := p.SetFlagContracts("alpha", Conflicts("beta")); err != nil {
		t.Fatalf("SetFlagContracts: %v", err)
	}
	cs, _ = p.GetFlagContracts("alpha")
	if len(cs) != 1 || cs[0].Kind != ContractConflicts {
		t.Fatalf("after Set: %+v", cs)
	}
	if err := p.ClearFlagContracts("alpha"); err != nil {
		t.Fatalf("ClearFlagContracts: %v", err)
	}
	if cs, _ := p.GetFlagContracts("alpha"); len(cs) != 0 {
		t.Fatalf("after Clear: %+v", cs)
	}

	// Unknown flag errors on every accessor.
	for _, err := range []error{
		p.AddFlagContracts("nope", Mutex("g")),
		p.SetFlagContracts("nope", Mutex("g")),
		p.ClearFlagContracts("nope"),
	} {
		if !errors.Is(err, errs.ErrFlagDoesNotExist) {
			t.Fatalf("expected ErrFlagDoesNotExist, got %v", err)
		}
	}
	if _, err := p.GetFlagContracts("nope"); !errors.Is(err, errs.ErrFlagDoesNotExist) {
		t.Fatalf("GetFlagContracts(nope): expected ErrFlagDoesNotExist, got %v", err)
	}
}

func TestContractStructTag(t *testing.T) {
	type CLI struct {
		Group   bool `goopt:"name:group;contract:mutex(sel)"`
		Pattern bool `goopt:"name:pattern;contract:mutex(sel)"`
	}
	p, err := NewParserFromStruct(&CLI{})
	if err != nil {
		t.Fatal(err)
	}
	p.Parse([]string{"app", "--group", "--pattern"})
	if !hasErr(p, errs.ErrMutexViolation) {
		t.Fatalf("struct-tag mutex: expected violation, got %v", p.GetErrors())
	}
	// Eyeball the user-facing message.
	for _, e := range p.GetErrors() {
		if errors.Is(e, errs.ErrMutexViolation) {
			t.Logf("rendered: %s", e.Error())
		}
	}
}
