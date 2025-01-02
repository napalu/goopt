package util

import (
	"testing"
)

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		x, y     interface{} // Use interface{} to test multiple types
		expected interface{}
		typ      string // For test description
	}{
		{"int smaller first", 1, 2, 1, "int"},
		{"int smaller second", 5, 3, 3, "int"},
		{"int equal", 4, 4, 4, "int"},
		{"int negative", -5, -2, -5, "int"},

		{"float32 smaller first", float32(1.5), float32(2.5), float32(1.5), "float32"},
		{"float32 smaller second", float32(5.5), float32(3.5), float32(3.5), "float32"},
		{"float32 negative", float32(-5.5), float32(-2.5), float32(-5.5), "float32"},

		{"float64 smaller first", 1.5, 2.5, 1.5, "float64"},
		{"float64 smaller second", 5.5, 3.5, 3.5, "float64"},
		{"float64 negative", -5.5, -2.5, -5.5, "float64"},

		{"uint8 smaller first", uint8(1), uint8(2), uint8(1), "uint8"},
		{"uint8 smaller second", uint8(5), uint8(3), uint8(3), "uint8"},

		{"int64 smaller first", int64(1), int64(2), int64(1), "int64"},
		{"int64 smaller second", int64(5), int64(3), int64(3), "int64"},
		{"int64 negative", int64(-5), int64(-2), int64(-5), "int64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}

			// Type-specific tests using switch
			switch x := tt.x.(type) {
			case int:
				result = Min(x, tt.y.(int))
			case float32:
				result = Min(x, tt.y.(float32))
			case float64:
				result = Min(x, tt.y.(float64))
			case uint8:
				result = Min(x, tt.y.(uint8))
			case int64:
				result = Min(x, tt.y.(int64))
			}

			if result != tt.expected {
				t.Errorf("Min(%v, %v) = %v; want %v (%s)", tt.x, tt.y, result, tt.expected, tt.typ)
			}
		})
	}
}
