// Package migration provides codegen helpers for migrating source to goopt/v2 API
// changes. It is intended to be run once during an upgrade and removed afterwards.
//
// The v2 validator-interface change made validation.Validator (an interface) the
// currency for WithValidator(s)/SetValidators/AddFlagValidators/etc. A bare inline
// func(string) error no longer satisfies it. This tool rewrites such call sites by
// wrapping the func literal in validation.Custom(...) (which returns a Validator), and
// renames []validation.ValidatorFunc slice literals to []validation.Validator. It only
// rewrites what it can prove syntactically; anything it can't (e.g. a func-typed
// variable) the compiler will still flag for a manual wrap.
package migration

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const validationPkgPath = "github.com/napalu/goopt/v2/validation"

// validatorSetters are the function/method names whose func-literal arguments must be
// wrapped after the validator-interface change.
var validatorSetters = map[string]bool{
	"WithValidator":      true,
	"WithValidators":     true,
	"SetValidators":      true,
	"WithFlagValidators": true,
	"AddFlagValidators":  true,
	"SetFlagValidators":  true,
}

// WrapValidatorsInSource applies the validator migration to Go source and returns the
// rewritten source, whether anything changed, and any parse/format error.
func WrapValidatorsInSource(src []byte) ([]byte, bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, false, fmt.Errorf("parse: %w", err)
	}

	alias := validationAlias(file) // existing import alias, or "" if not imported
	wrapAlias := alias
	if wrapAlias == "" {
		wrapAlias = "validation"
	}

	modified := false
	wrappedAny := false

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if validatorSetters[calleeName(node.Fun)] {
				for i, arg := range node.Args {
					// Only func LITERALS are unambiguously bare funcs needing the wrap.
					// An already-wrapped validation.ValidatorFunc(...) is a CallExpr, so
					// this is naturally idempotent.
					if fl, ok := arg.(*ast.FuncLit); ok {
						node.Args[i] = wrapInCustom(fl, wrapAlias)
						modified, wrappedAny = true, true
					}
				}
			}
		case *ast.ArrayType:
			// []validation.ValidatorFunc -> []validation.Validator
			if renameValidatorFuncType(node.Elt) {
				modified = true
			}
		}
		return true
	})

	if wrappedAny && alias == "" {
		ensureImport(file, validationPkgPath)
		modified = true
	}

	if !modified {
		return src, false, nil
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, false, fmt.Errorf("format: %w", err)
	}
	return buf.Bytes(), true, nil
}

// PreviewFile returns the rewritten source for a file without writing it (dry run).
func PreviewFile(filename string) (out []byte, changed bool, err error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, false, err
	}
	return WrapValidatorsInSource(src)
}

// ConvertFile rewrites a file in place if it needs the validator migration. When backup
// is true a "<file>.bak" copy is written first. It reports whether the file changed.
func ConvertFile(filename string, backup bool) (changed bool, err error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}
	out, changed, err := WrapValidatorsInSource(src)
	if err != nil {
		return false, fmt.Errorf("%s: %w", filename, err)
	}
	if !changed {
		return false, nil
	}
	if backup {
		if err := os.WriteFile(filename+".bak", src, 0o644); err != nil {
			return false, fmt.Errorf("backup %s: %w", filename, err)
		}
	}
	if err := os.WriteFile(filename, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", filename, err)
	}
	return true, nil
}

// ConvertDir walks dir (recursively when recursive is true), converting every .go file
// that needs the validator migration. It skips vendor, .git, and dot-directories. It
// returns the list of files it changed.
func ConvertDir(dir string, recursive, backup bool) (changed []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != dir && (!recursive || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		did, e := ConvertFile(path, backup)
		if e != nil {
			return e
		}
		if did {
			changed = append(changed, path)
		}
		return nil
	})
	return changed, err
}

// WalkPreview walks dir like ConvertDir but writes nothing: it calls report(path) for
// every .go file that WOULD change. Used for dry-run on a directory.
func WalkPreview(dir string, recursive bool, report func(path string)) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != dir && (!recursive || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		_, changed, err := PreviewFile(path)
		if err != nil {
			return err
		}
		if changed {
			report(path)
		}
		return nil
	})
}

// --- AST helpers ---

// calleeName returns the final identifier of a call's function expression:
// WithValidator -> "WithValidator"; goopt.WithValidator -> "WithValidator";
// p.AddFlagValidators -> "AddFlagValidators".
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// wrapInCustom builds `<alias>.Custom(<funcLit>)` — the idiomatic way to add a custom
// validator (validation.Custom accepts a func and returns a Validator), matching what
// hand-written v2 code uses. (validation.ValidatorFunc(...) would be equally valid; we
// emit Custom so generated and hand-written code land on one idiom.)
func wrapInCustom(fl *ast.FuncLit, alias string) ast.Expr {
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(alias),
			Sel: ast.NewIdent("Custom"),
		},
		Args: []ast.Expr{fl},
	}
}

// renameValidatorFuncType rewrites a `validation.ValidatorFunc` (or bare `ValidatorFunc`
// under a dot-import) element type to `...Validator`. Returns whether it changed.
func renameValidatorFuncType(elt ast.Expr) bool {
	switch e := elt.(type) {
	case *ast.SelectorExpr:
		if e.Sel.Name == "ValidatorFunc" {
			e.Sel.Name = "Validator"
			return true
		}
	case *ast.Ident:
		if e.Name == "ValidatorFunc" {
			e.Name = "Validator"
			return true
		}
	}
	return false
}

// validationAlias returns the local name the validation package is imported under, or ""
// if it isn't imported.
func validationAlias(file *ast.File) string {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == validationPkgPath {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return "validation"
		}
	}
	return ""
}

// ensureImport adds an import for path (default-named) to the file's first import block,
// creating one if necessary.
func ensureImport(file *ast.File, path string) {
	spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			gd.Specs = append(gd.Specs, spec)
			file.Imports = append(file.Imports, spec)
			return
		}
	}
	gd := &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}}
	file.Decls = append([]ast.Decl{gd}, file.Decls...)
	file.Imports = append(file.Imports, spec)
}
