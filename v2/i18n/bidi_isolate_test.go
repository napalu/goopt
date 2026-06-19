package i18n

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/text/language"
)

const (
	fsi = "\u2068" // First Strong Isolate
	pdi = "\u2069" // Pop Directional Isolate
)

// newBidiProvider builds a provider whose default bundle carries the test keys
// in both an LTR (English) and RTL (Arabic) language.
func newBidiProvider() *LayeredMessageProvider {
	b := NewEmptyBundle()
	for _, lang := range []language.Tag{language.English, language.Arabic} {
		b.AddLanguage(lang, map[string]string{
			"test.flag_msg": "flag %[1]s expects a value",
			"test.pos_msg":  "positional %[1]s at index %[2]d",
			"test.wrap_msg": "operation failed",
		})
	}
	return NewLayeredMessageProvider(b, nil, nil)
}

func TestBidiIsolation_StringArg(t *testing.T) {
	p := newBidiProvider()
	e := NewError("test.flag_msg").WithArgs("crowd.app-pass")

	p.SetDefaultLanguage(language.Arabic)
	if got := e.Format(p); !strings.Contains(got, fsi+"crowd.app-pass"+pdi) {
		t.Fatalf("RTL: arg not bidi-isolated: %q", got)
	}

	p.SetDefaultLanguage(language.English)
	got := e.Format(p)
	if strings.Contains(got, fsi) || strings.Contains(got, pdi) {
		t.Fatalf("LTR: unexpected isolation controls: %q", got)
	}
	if got != "flag crowd.app-pass expects a value" {
		t.Fatalf("LTR: output changed: %q", got)
	}
}

func TestBidiIsolation_WrappedReason(t *testing.T) {
	p := newBidiProvider()
	e := NewError("test.wrap_msg").Wrap(errors.New("not attached to a terminal"))

	p.SetDefaultLanguage(language.Arabic)
	if got := e.Format(p); !strings.Contains(got, fsi+"not attached to a terminal"+pdi) {
		t.Fatalf("RTL: wrapped reason not isolated: %q", got)
	}

	p.SetDefaultLanguage(language.English)
	got := e.Format(p)
	if strings.Contains(got, fsi) {
		t.Fatalf("LTR: unexpected isolation controls: %q", got)
	}
	if got != "operation failed: not attached to a terminal" {
		t.Fatalf("LTR: output changed: %q", got)
	}
}

// Numeric args must NOT be isolated — wrapping a %[2]d value in FSI/PDI would
// turn it into a string and break the verb.
func TestBidiIsolation_NumericArgUnaffected(t *testing.T) {
	p := newBidiProvider()
	e := NewError("test.pos_msg").WithArgs("input", 3)

	p.SetDefaultLanguage(language.Arabic)
	got := e.Format(p)
	if strings.Contains(got, "%!") {
		t.Fatalf("format verb leak (numeric arg isolated?): %q", got)
	}
	if !strings.Contains(got, fsi+"input"+pdi) {
		t.Fatalf("string arg not isolated: %q", got)
	}
	if !strings.Contains(got, "index 3") {
		t.Fatalf("numeric arg malformed: %q", got)
	}
}
