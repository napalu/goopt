package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptionType_String(t *testing.T) {
	tests := []struct {
		opt      OptionType
		expected string
	}{
		{Single, "single"},
		{Chained, "chained"},
		{Standalone, "standalone"},
		{File, "file"},
		{OptionType(99), "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.opt.String())
		})
	}
}

func TestPatternValue_Describe(t *testing.T) {
	tests := []struct {
		name     string
		pv       PatternValue
		expected string
	}{
		{
			name: "with description",
			pv: PatternValue{
				Pattern:     "test-pattern",
				Description: "This is a test pattern",
			},
			expected: "This is a test pattern",
		},
		{
			name: "without description",
			pv: PatternValue{
				Pattern: "test-pattern",
			},
			expected: "test-pattern",
		},
		{
			name: "empty pattern with description",
			pv: PatternValue{
				Pattern:     "",
				Description: "Empty pattern",
			},
			expected: "Empty pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pv.Describe())
		})
	}
}
