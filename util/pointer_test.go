package util

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnwrapValue(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "nil pointer",
			input:   (*string)(nil),
			wantErr: true,
		},
		{
			name:    "single pointer",
			input:   NewOfType("test"),
			wantErr: false,
		},
		{
			name:    "double pointer",
			input:   NewOfType(NewOfType("test")),
			wantErr: false,
		},
		{
			name:    "non-pointer",
			input:   "test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := reflect.ValueOf(tt.input)
			unwrapped, err := UnwrapValue(val)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, "test", unwrapped.Interface())
		})
	}
}

func TestUnwrapType(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected reflect.Kind
	}{
		{
			name:     "string",
			input:    "test",
			expected: reflect.String,
		},
		{
			name:     "pointer to string",
			input:    NewOfType("test"),
			expected: reflect.String,
		},
		{
			name:     "pointer to pointer to string",
			input:    NewOfType(NewOfType("test")),
			expected: reflect.String,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(tt.input)
			unwrapped := UnwrapType(typ)
			assert.Equal(t, tt.expected, unwrapped.Kind())
		})
	}
}
