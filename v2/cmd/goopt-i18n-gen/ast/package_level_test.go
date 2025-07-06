package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageLevelTRDeclaration(t *testing.T) {
	// Create a temp directory
	tempDir := t.TempDir()

	// Create first file with TR declaration
	file1 := filepath.Join(tempDir, "file1.go")
	file1Content := `package main

import (
	"github.com/napalu/goopt/v2/i18n"
)

var TR = func() i18n.Translator {
	panic("TODO: Implement TR() - return your i18n.Translator instance")
}

func function1() {
	msg := "Hello from file1"
}`

	if err := os.WriteFile(file1, []byte(file1Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create second file without TR declaration
	file2 := filepath.Join(tempDir, "file2.go")
	file2Content := `package main

func function2() {
	msg := "Hello from file2"
}`

	if err := os.WriteFile(file2, []byte(file2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test transforming file2 - it should NOT add TR declaration
	stringMap := map[string]string{
		`"Hello from file2"`: "messages.Keys.Hello2",
	}

	ft := NewFormatTransformer(stringMap)
	ft.SetTransformMode("all")
	ft.SetTranslatorPattern("TR().T")
	ft.SetMessagePackagePath("messages")

	// Read file2
	src, err := os.ReadFile(file2)
	if err != nil {
		t.Fatal(err)
	}

	// Transform file2
	result, err := ft.TransformFile(file2, src)
	if err != nil {
		t.Fatalf("TransformFile failed: %v", err)
	}

	resultStr := string(result)

	// Check that TR declaration was NOT added
	if strings.Contains(resultStr, "var TR =") {
		t.Errorf("TR declaration should not be added to file2 when it exists in file1")
	}

	// Check that the transformation still happened
	if !strings.Contains(resultStr, "TR().T(messages.Keys.Hello2)") {
		t.Errorf("String transformation should still occur")
	}

	// Check that i18n import was NOT added (since TR exists in another file)
	if strings.Contains(resultStr, `"github.com/napalu/goopt/v2/i18n"`) {
		t.Errorf("i18n import should not be added when TR is declared in another file in the package")
	}

	// Test transforming a file in a different package/directory
	otherDir := filepath.Join(tempDir, "other")
	os.Mkdir(otherDir, 0755)

	file3 := filepath.Join(otherDir, "file3.go")
	file3Content := `package other

func function3() {
	msg := "Hello from file3"
}`

	if err := os.WriteFile(file3, []byte(file3Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Transform file3 - it SHOULD add TR declaration (different package)
	stringMap3 := map[string]string{
		`"Hello from file3"`: "messages.Keys.Hello3",
	}

	ft3 := NewFormatTransformer(stringMap3)
	ft3.SetTransformMode("all")
	ft3.SetTranslatorPattern("TR().T")
	ft3.SetMessagePackagePath("messages")

	src3, err := os.ReadFile(file3)
	if err != nil {
		t.Fatal(err)
	}

	result3, err := ft3.TransformFile(file3, src3)
	if err != nil {
		t.Fatalf("TransformFile failed: %v", err)
	}

	result3Str := string(result3)

	// Check that TR declaration WAS added (different package)
	if !strings.Contains(result3Str, "var TR =") {
		t.Errorf("TR declaration should be added to file3 (different package)")
	}

	// Test transforming a file that already has TR declaration
	file4 := filepath.Join(tempDir, "file4.go")
	file4Content := `package main

import (
	"github.com/napalu/goopt/v2/i18n"
)

var TR = func() i18n.Translator {
	panic("TODO: Implement TR() - return your i18n.Translator instance")
}

func function4() {
	msg := "Hello from file4"
}`

	if err := os.WriteFile(file4, []byte(file4Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Transform file4 - it should NOT add another TR declaration
	stringMap4 := map[string]string{
		`"Hello from file4"`: "messages.Keys.Hello4",
	}

	ft4 := NewFormatTransformer(stringMap4)
	ft4.SetTransformMode("all")
	ft4.SetTranslatorPattern("TR().T")
	ft4.SetMessagePackagePath("messages")

	src4, err := os.ReadFile(file4)
	if err != nil {
		t.Fatal(err)
	}

	result4, err := ft4.TransformFile(file4, src4)
	if err != nil {
		t.Fatalf("TransformFile failed: %v", err)
	}

	result4Str := string(result4)

	// Count occurrences of "var TR ="
	trCount := strings.Count(result4Str, "var TR =")
	if trCount != 1 {
		t.Errorf("Expected exactly 1 TR declaration, got %d", trCount)
	}
}
