package util

import (
	"math"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
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

func TestParseNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected Number
		valid    bool
	}{
		// Integers
		{"123", Number{Int: 123, IsInt: true}, true},
		{"-456", Number{Int: -456, IsInt: true, IsNegative: true}, true},
		{"0x1a", Number{Int: 26, IsInt: true}, true},                                              // hex
		{"0755", Number{Int: 493, IsInt: true}, true},                                             // octal
		{"0b1010", Number{Int: 10, IsInt: true}, true},                                            // binary
		{"9223372036854775807", Number{Int: math.MaxInt64, IsInt: true}, true},                    // max int64
		{"-9223372036854775808", Number{Int: math.MinInt64, IsInt: true, IsNegative: true}, true}, // min int64

		// Floats
		{"123.45", Number{Float: 123.45, IsFloat: true}, true},
		{"-678.90", Number{Float: -678.90, IsFloat: true, IsNegative: true}, true},
		{"1e6", Number{Float: 1e6, IsFloat: true}, true},
		{"2.5e-3", Number{Float: 0.0025, IsFloat: true}, true},
		{"NaN", Number{Float: math.NaN(), IsFloat: true}, true},
		{"Inf", Number{Float: math.Inf(1), IsFloat: true}, true},
		{"-Inf", Number{Float: math.Inf(-1), IsFloat: true, IsNegative: true}, true},

		// Complex
		{"1+2i", Number{Complex: 1 + 2i, IsComplex: true}, true},
		{"-3.4-5.6i", Number{Complex: -3.4 - 5.6i, IsComplex: true, IsNegative: true}, true},
		{"7i", Number{Complex: complex(0, 7), IsComplex: true, IsNegative: false}, true},
		{"-8i", Number{Complex: complex(0, -8), IsComplex: true, IsNegative: false}, true},

		// Edge Cases
		{"", Number{}, false},
		{"abc", Number{}, false},
		{"12.3.4", Number{}, false},
		{"0xghij", Number{}, false}, // invalid hex
		{"--123", Number{}, false},  // double negative
		{"12+3", Number{}, false},   // invalid complex
		{"漢字", Number{}, false},     // non-ASCII
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual, ok := ParseNumeric(tt.input)

			assert.Equal(t, tt.valid, ok, "Validity mismatch")
			if !tt.valid {
				return
			}

			// Check type flags
			assert.Equal(t, tt.expected.IsInt, actual.IsInt, "IsInt mismatch")
			assert.Equal(t, tt.expected.IsFloat, actual.IsFloat, "IsFloat mismatch")
			assert.Equal(t, tt.expected.IsComplex, actual.IsComplex, "IsComplex mismatch")
			assert.Equal(t, tt.expected.IsNegative, actual.IsNegative, "IsNegative mismatch")

			// Value comparisons with tolerance
			const epsilon = 1e-9
			switch {
			case actual.IsInt:
				assert.Equal(t, tt.expected.Int, actual.Int, "Int value mismatch")
			case actual.IsFloat:
				if math.IsNaN(tt.expected.Float) {
					assert.True(t, math.IsNaN(actual.Float), "Expected NaN")
				} else if math.IsInf(tt.expected.Float, 0) {
					assert.True(t, math.IsInf(actual.Float, int(math.Copysign(1, tt.expected.Float))),
						"Infinity sign mismatch")
				} else {
					assert.InEpsilon(t, tt.expected.Float, actual.Float, epsilon,
						"Float value mismatch")
				}
			case actual.IsComplex:
				// Real part
				expectedReal := real(tt.expected.Complex)
				actualReal := real(actual.Complex)
				if expectedReal == 0 {
					assert.Equal(t, expectedReal, actualReal, "Complex real part should be zero")
				} else {
					assert.InEpsilon(t, expectedReal, actualReal, epsilon, "Complex real part mismatch")
				}

				// Imaginary part
				expectedImag := imag(tt.expected.Complex)
				actualImag := imag(actual.Complex)
				if expectedImag == 0 {
					assert.Equal(t, expectedImag, actualImag, "Complex imaginary part should be zero")
				} else {
					assert.InEpsilon(t, expectedImag, actualImag, epsilon, "Complex imaginary part mismatch")
				}
			}

			// UTF-8 validity check
			assert.True(t, utf8.ValidString(tt.input), "Test input should be valid UTF-8")
		})
	}
}
