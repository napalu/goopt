package parse

import (
	"fmt"
	"strconv"
	"strings"
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
		return nil, fmt.Errorf(errEmptyInput, "position")
	}

	// Handle legacy format
	if strings.HasPrefix(input, "{") {
		if !strings.HasSuffix(input, "}") {
			return nil, fmt.Errorf(errMalformedBraces, input)
		}

		// Extract content between braces and split by ':'
		content := strings.TrimSpace(input[1 : len(input)-1])
		parts := strings.Split(content, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf(errInvalidFormat, input)
		}

		// Check for 'idx' prefix (case insensitive, with whitespace)
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "idx") {
			return nil, fmt.Errorf(errInvalidFormat, input)
		}

		// Parse the number
		return parseIndex(parts[1])
	}

	// New format: just the number
	return parseIndex(input)
}

func parseIndex(value string) (*PositionData, error) {
	value = strings.TrimSpace(value)
	idx, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("invalid index value: %s", value)
	}
	if idx < 0 {
		return nil, fmt.Errorf("index must be non-negative")
	}
	return &PositionData{Index: idx}, nil
}
