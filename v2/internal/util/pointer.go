package util

import (
	"reflect"

	"github.com/napalu/goopt/v2/errs"
)

// UnwrapValue recursively unwraps pointer and returns the underlying value
// Returns the zero Value if a nil pointer is encountered
func UnwrapValue(v reflect.Value) (reflect.Value, error) {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, errs.ErrNilPointer
		}
		v = v.Elem()
	}
	return v, nil
}

// UnwrapType recursively unwraps pointer.go types and returns the underlying type
func UnwrapType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func NewOfType[T any](value T) *T {
	return &value
}
