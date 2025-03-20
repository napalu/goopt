package parse

import (
	"strconv"
	"strings"

	"github.com/napalu/goopt/errs"
)

// PositionData represents a parsed position configuration
type PositionData struct {
	Index int // Sequential index for positional argument
}

// Position parses a position tag value in both formats:
// - New format: "N" (just the number)
// - Legacy format: "{idx:N}"
func Position(input string) (*PositionData, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errs.ErrParseMissingValue.WithArgs("position", input)
	}

	// Handle legacy format
	if strings.HasPrefix(input, "{") {
		if !strings.HasSuffix(input, "}") {
			return nil, errs.ErrParseMalformedBraces.WithArgs(input)
		}

		// Extract content between braces and split by ':'
		content := strings.TrimSpace(input[1 : len(input)-1])
		parts := strings.Split(content, ":")
		if len(parts) != 2 {
			return nil, errs.ErrParseInvalidFormat.WithArgs(input)
		}

		// Check for 'idx' prefix (case insensitive, with whitespace)
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "idx") {
			return nil, errs.ErrParseInvalidFormat.WithArgs(input)
		}

		// Parse the number
		return parseIndex(parts[1])
	}

	// New format: just the number
	return parseIndex(input)
}

func parseIndex(value string) (*PositionData, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errs.ErrParseMissingValue.WithArgs("position", value)
	}
	idx, err := strconv.Atoi(value)
	if err != nil {
		return nil, errs.ErrParseInt.WithArgs(value, 64).Wrap(err)
	}
	if idx < 0 {
		return nil, errs.ErrParseNegativeIndex.WithArgs(idx)
	}
	return &PositionData{Index: idx}, nil
}
