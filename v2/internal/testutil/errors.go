package testutil

import (
	"errors"
	"reflect"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
)

// CompareErrors compares two slices of errors using errors.Is
// It returns true if the errors match and false otherwise.
// It also reports detailed differences through the testing.T instance.
func CompareErrors(t *testing.T, want, got []error) bool {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("error count mismatch:\ngot:  %d errors\nwant: %d errors", len(got), len(want))
		if len(got) > 0 {
			t.Errorf("got errors: %v", got)
		}
		if len(want) > 0 {
			t.Errorf("want errors: %v", want)
		}
		return false
	}

	for i, wantErr := range want {
		if !errors.Is(got[i], wantErr) {
			t.Errorf("error %d mismatch:\ngot:  %v\nwant: %v", i, got[i], wantErr)
			return false
		}
	}
	return true
}

// AssertError is a helper for comparing single errors
func AssertError(t *testing.T, got, want error) bool {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error mismatch:\ngot:  %v\nwant: %v", got, want)
		return false
	}
	return true
}

// AssertNoErrors checks that the error slice is empty
func AssertNoErrors(t *testing.T, errors []error) bool {
	t.Helper()
	return len(errors) == 0
}

// AssertErrorIs checks if the error is of the expected type and has expected arguments
func AssertErrorIs(t *testing.T, got error, want i18n.TranslatableError) bool {
	t.Helper()

	if !errors.Is(got, want) {
		t.Errorf("error type mismatch:\ngot:  %T\nwant: %T", got, want)
		return false
	}

	// Check arguments if it's a TranslatableError
	if te, ok := got.(i18n.TranslatableError); ok {
		wantArgs := want.Args()
		gotArgs := te.Args()

		if !reflect.DeepEqual(gotArgs, wantArgs) {
			t.Errorf("error arguments mismatch:\ngot:  %v\nwant: %v", gotArgs, wantArgs)
			return false
		}
	}

	return true
}

// AssertErrorChain verifies a chain of wrapped errors
func AssertErrorChain(t *testing.T, got error, wantChain ...error) bool {
	t.Helper()

	current := got
	for _, want := range wantChain {
		if current == nil {
			t.Errorf("error chain too short, expected %v", want)
			return false
		}

		if !errors.Is(current, want) {
			t.Errorf("error in chain mismatch:\ngot:  %v\nwant: %v", current, want)
			return false
		}

		current = errors.Unwrap(current)
	}

	if current != nil {
		t.Errorf("error chain too long, unexpected: %v", current)
		return false
	}

	return true
}
