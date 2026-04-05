package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNumeric(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNum Number
		wantOk  bool
	}{
		{
			name:    "positive integer",
			input:   "42",
			wantNum: Number{Int: 42, IsInt: true},
			wantOk:  true,
		},
		{
			name:    "negative integer",
			input:   "-123",
			wantNum: Number{Int: -123, IsInt: true, IsNegative: true},
			wantOk:  true,
		},
		{
			name:    "positive float",
			input:   "3.14",
			wantNum: Number{Float: 3.14, IsFloat: true},
			wantOk:  true,
		},
		{
			name:    "negative float",
			input:   "-2.5",
			wantNum: Number{Float: -2.5, IsFloat: true, IsNegative: true},
			wantOk:  true,
		},
		{
			name:    "hexadecimal",
			input:   "0xFF",
			wantNum: Number{Int: 255, IsInt: true},
			wantOk:  true,
		},
		{
			name:    "binary",
			input:   "0b1010",
			wantNum: Number{Int: 10, IsInt: true},
			wantOk:  true,
		},
		{
			name:    "scientific notation",
			input:   "1e3",
			wantNum: Number{Float: 1000.0, IsFloat: true},
			wantOk:  true,
		},
		{
			name:    "invalid input",
			input:   "not-a-number",
			wantNum: Number{},
			wantOk:  false,
		},
		{
			name:    "empty string",
			input:   "",
			wantNum: Number{},
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, gotOk := ParseNumeric(tt.input)

			assert.Equal(t, tt.wantOk, gotOk)
			if tt.wantOk {
				assert.Equal(t, tt.wantNum.IsInt, gotNum.IsInt)
				assert.Equal(t, tt.wantNum.IsFloat, gotNum.IsFloat)
				assert.Equal(t, tt.wantNum.IsNegative, gotNum.IsNegative)
				if gotNum.IsInt {
					assert.Equal(t, tt.wantNum.Int, gotNum.Int)
				}
				if gotNum.IsFloat {
					assert.InDelta(t, tt.wantNum.Float, gotNum.Float, 0.0001)
				}
			}
		})
	}
}
