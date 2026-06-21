package migration

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// compiles checks the output is at least syntactically valid Go.
func mustParse(t *testing.T, src []byte) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments); err != nil {
		t.Fatalf("output does not parse: %v\n---\n%s", err, src)
	}
}

func TestWrapInlineFuncLiteral(t *testing.T) {
	in := `package x

import (
	"github.com/napalu/goopt/v2"
)

func build() {
	goopt.NewArg(goopt.WithValidators(func(s string) error {
		if s == "" {
			return nil
		}
		return nil
	}))
}
`
	out, changed, err := WrapValidatorsInSource([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected a change")
	}
	s := string(out)
	mustParse(t, out)
	if !strings.Contains(s, "validation.Custom(func(s string) error") {
		t.Errorf("inline func should be wrapped; got:\n%s", s)
	}
	if !strings.Contains(s, validationPkgPath) {
		t.Errorf("validation import should be added; got:\n%s", s)
	}
}

func TestRenameSliceLiteral(t *testing.T) {
	in := `package x

import "github.com/napalu/goopt/v2/validation"

var v = []validation.ValidatorFunc{validation.MinLength(3)}
`
	out, changed, err := WrapValidatorsInSource([]byte(in))
	if err != nil || !changed {
		t.Fatalf("expected change; changed=%v err=%v", changed, err)
	}
	if !strings.Contains(string(out), "[]validation.Validator{") {
		t.Errorf("slice type should be renamed; got:\n%s", out)
	}
}

func TestLeavesGoodCodeAlone(t *testing.T) {
	// IsOneOf and MinLength already satisfy Validator — nothing to wrap.
	in := `package x

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/validation"
)

func build() {
	goopt.NewArg(goopt.WithValidators(validation.IsOneOf("a", "b"), validation.MinLength(3)))
}
`
	_, changed, err := WrapValidatorsInSource([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Errorf("validators that already satisfy Validator must not be touched")
	}
}

func TestIdempotent(t *testing.T) {
	in := `package x

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/validation"
)

func build() {
	goopt.NewArg(goopt.WithValidator(validation.Custom(func(s string) error { return nil })))
}
`
	_, changed, err := WrapValidatorsInSource([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Errorf("already-wrapped func must not be wrapped again")
	}
}

func TestRespectsImportAlias(t *testing.T) {
	in := `package x

import (
	"github.com/napalu/goopt/v2"
	v "github.com/napalu/goopt/v2/validation"
)

func build() {
	goopt.NewArg(goopt.WithValidator(func(s string) error { return nil }))
	_ = v.MinLength
}
`
	out, changed, err := WrapValidatorsInSource([]byte(in))
	if err != nil || !changed {
		t.Fatalf("expected change; changed=%v err=%v", changed, err)
	}
	if !strings.Contains(string(out), "v.Custom(func(s string) error") {
		t.Errorf("should wrap using the existing import alias 'v'; got:\n%s", out)
	}
}

func TestMethodReceiverSetter(t *testing.T) {
	// p.AddFlagValidators(...) — method form, matched by final selector name.
	in := `package x

func build(p interface{ AddFlagValidators(string, ...interface{}) error }) {
	_ = p.AddFlagValidators("flag", func(s string) error { return nil })
}
`
	out, changed, _ := WrapValidatorsInSource([]byte(in))
	if !changed || !strings.Contains(string(out), "validation.Custom(func(s string) error") {
		t.Errorf("method-form setter func should be wrapped; got:\n%s", out)
	}
}
