package goopt

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/napalu/goopt/parse"
	"github.com/napalu/goopt/types"
	"github.com/stretchr/testify/assert"
)

func FuzzParseFlags(f *testing.F) {
	// Seed corpus with edge cases
	f.Add("-a2こんにちは")
	f.Add("--long")            // Empty value
	f.Add("-vxffile")          // Bundled flags with attached value
	f.Add("-- value")          // Empty flag name
	f.Add("   --spaces ok   ") // Leading/trailing spaces
	f.Add("-漢字=こんにちは こんにち")    // Unicode
	f.Add("0")
	f.Add("-")
	f.Add("-a \\'-xtra\\'")
	f.Add("-a -xtra 000000")
	f.Add("-a -xtra -123.45")
	f.Fuzz(func(t *testing.T, rawArgs string) {
		// Normalize input
		args, err := parse.Split(rawArgs)
		if err != nil {
			return
		}
		if len(args) == 0 {
			return // Skip empty input
		}

		// Setup parser with common flags
		p := NewParser()
		//p.SetPosix(true)
		p.AddFlag("a", NewArg(WithShortFlag("a")))
		p.AddFlag("xtra", NewArg(WithShortFlag("x"), WithType(types.Standalone)))
		p.AddFlag("verbose", NewArg(WithShortFlag("v"), WithType(types.Standalone)))
		p.AddFlag("file", NewArg(WithShortFlag("f")))
		p.AddFlag("long", NewArg(WithDescription("xxxxx")))
		p.AddFlag("spaces", NewArg())
		p.AddFlag("漢字", NewArg())

		if strings.Contains(rawArgs, "-a \\'-xtra\\'") {
			t.Logf("rawArgs: %s", rawArgs)
		}
		// Parse and validate invariants
		ok := p.Parse(args) && p.GetErrorCount() == 0

		// Invariant 1: Parser state consistency
		if ok {

			// Retrieved values match input
			for _, arg := range p.parseState.Args() {
				if p.isFlag(arg) {
					name := strings.TrimLeftFunc(arg, p.prefixFunc)
					if len(name) == 0 {
						continue
					}
					_, exists := p.Get(name)
					if !exists {
						t.Logf("not found %s", name)
					}
					assert.True(t, exists, "Flag %s not found", name)
				}
			}
		} else {
			// Don't assert on positional args - failure could be flag validation
			// Instead, check help text remains valid
			b := bytes.NewBuffer(nil)
			p.PrintUsage(b)
			assert.NotContains(t, b.String(), "%!MISSING")
		}

		// Invariant 2: No panics
		// Implicitly verified by fuzzing framework

		// Invariant 3: Help text remains valid
		b := bytes.NewBuffer(nil)
		p.PrintUsage(b)
		assert.NotContains(t, b.String(), "%!MISSING")
	})
}

func FuzzPositionalArgs(f *testing.F) {
	f.Add("a.txt", "b.pdf")     // Mixed flags and positionals
	f.Add("--", "-invalid")     // POSIX end-of-options
	f.Add("漢字.txt", "--utf8=✓") // Unicode values

	f.Fuzz(func(t *testing.T, arg1, arg2 string) {
		p := NewParser()
		p.AddFlag("file1", NewArg(WithType(types.File), WithPosition(0)))
		p.AddFlag("file2", NewArg(WithType(types.File), WithPosition(1)))

		args := []string{arg1, arg2, "-v"}
		ok := p.Parse(args)

		if ok {
			f1, _ := p.Get("file1")
			f2, _ := p.Get("file2")
			assert.Equal(t, arg1, f1)
			assert.Equal(t, arg2, f2)
		}
	})
}

func FuzzEnvVars(f *testing.F) {
	f.Add("FLAG", "VAL%UE!")     // Special chars
	f.Add("FLAG_漢字", "こんにちは")    // Unicode
	f.Add("FLAG_漢字_2", "こんに!ちは") // Unicode
	f.Fuzz(func(t *testing.T, key, value string) {
		t.Setenv(key, value)

		p := NewParser()
		p.SetEnvNameConverter(func(s string) string {
			return s
		})
		p.AddFlag(key, NewArg())

		// Should read from environment
		ok := p.Parse([]string{})

		if ok {
			v, _ := p.Get(key)
			assert.Equal(t, value, v)
		}
	})
}

func FuzzHelpText(f *testing.F) {
	f.Add("--flag", "desc!@#$%^&*()") // Special chars
	f.Add("--漢字", "説明")               // Unicode

	f.Fuzz(func(t *testing.T, flag, desc string) {
		p := NewParser()
		p.AddFlag(flag, NewArg(WithDescription(desc)))

		b := bytes.Buffer{}
		p.PrintUsage(&b)
		help := b.String()

		// Critical invariants
		assert.False(t, strings.Contains(help, "%!"),
			"Help text contains formatting errors")
		assert.True(t, utf8.ValidString(help),
			"Help text contains invalid UTF-8")
		assert.NotContains(t, help, "\x00",
			"Help text contains null bytes")
	})
}
