package parse

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/napalu/goopt/types"
)

// PositionData represents a parsed position configuration
type PositionData struct {
	At  *types.PositionType // Position type (start/end)
	Idx *int                // Relative index within the position
}

// Position parses a single position entry in format pos:{at:start,idx:0}
func Position(input string) (*PositionData, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "position")
	}

	// Validate format
	if !strings.HasPrefix(input, "pos:{") || !strings.HasSuffix(input, "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}
	input = strings.TrimPrefix(input, "pos:")
	input = strings.Trim(input, "{} \r\n")

	// Handle empty braces case
	if input == "" {
		return &PositionData{}, nil
	}

	// Parse key-value pairs
	parts := make(map[string]string)
	var current strings.Builder

	input = input + "," // Add trailing comma to simplify parsing
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if ch == ',' {
			part := current.String()
			key, value, found := strings.Cut(strings.TrimSpace(part), ":")
			if !found {
				if strings.Contains(part, "=") {
					return nil, errors.New(errInvalidFormat)
				}
				return nil, fmt.Errorf(errMalformedBraces, input)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" {
				return nil, fmt.Errorf(errEmptyKey, part)
			}
			parts[key] = value
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}

	data := &PositionData{}

	// Handle 'at' field
	if atStr, ok := parts["at"]; ok && atStr != "" {
		pos, err := parsePositionType(atStr)
		if err != nil {
			return nil, err
		}
		data.At = &pos
	}

	// Handle 'idx' field
	if idxStr, ok := parts["idx"]; ok && idxStr != "" {
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, fmt.Errorf("invalid index value: %s", idxStr)
		}
		if idx < 0 {
			return nil, fmt.Errorf("index must be non-negative: %d", idx)
		}
		data.Idx = &idx
	}

	return data, nil
}

func parsePositionType(pos string) (types.PositionType, error) {
	pos = strings.TrimSpace(pos)
	if pos == "" {
		return 0, nil // Empty position type is valid
	}

	switch strings.ToLower(pos) {
	case "start":
		return types.AtStart, nil
	case "end":
		return types.AtEnd, nil
	default:
		return 0, fmt.Errorf("invalid position type: %s", pos)
	}
}
