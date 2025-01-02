package parse

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/napalu/goopt/types"
	"github.com/napalu/goopt/util"
)

// DependencyMap maps flag names to their allowed values
// empty slice means any value is acceptable
type DependencyMap map[string][]string

// Common error messages
const (
	errEmptyInput        = "empty %s"
	errMalformedBraces   = "malformed braces in: %s"
	errUnmatchedBrackets = "unmatched brackets in: %s"
	errInvalidFormat     = "invalid format"
	errEmptyKey          = "empty key in: %s"
	errMissingValue      = "missing or empty %s in: %s"
	errDuplicateFlag     = "duplicate flag: %s"
	errEmptyValue        = "empty value in: %s"
	errBothValues        = "cannot specify both 'value' and 'values' in: %s"
)

// PatternValue parses a pattern value in format {pattern:xyz,desc:abc}
//
// Escape sequences:
//
//   - Special characters:
//     \,  -> , (comma)
//     \:  -> : (colon)
//     \{  -> { (left brace)
//     \}  -> } (right brace)
//     \   -> ' ' (space)
//
//   - Quotes:
//     \"  -> " (double quote)
//     \'  -> ' (single quote)
//
//   - Backslashes:
//     \\  -> \ (single backslash)
//     \\\ -> \\ (escaped backslash followed by char)
//
// Examples:
//
//	{pattern:a\,b,desc:Values a\, b}     -> pattern="a,b" desc="Values a, b"
//	{pattern:C:\\Windows,desc:Path}      -> pattern="C:\Windows" desc="Path"
//	{pattern:\w+\:\d+,desc:Key\: Value}  -> pattern="w+:d+" desc="Key: Value"
func PatternValue(input string) (*types.PatternValue, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "pattern value")
	}

	// Check for proper braces
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}
	input = strings.Trim(input, "{}")

	// Parse key-value pairs while preserving escaped characters
	parts := make(map[string]string)
	var current strings.Builder
	var escaped bool
	var lastChar rune

	input = input + "," // Add trailing comma to simplify parsing
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			switch ch {
			case '\\':
				if lastChar == '\\' {
					current.WriteByte('\\') // Double backslash becomes single
				}
			case ',', '{', '}', ':':
				current.WriteByte(ch) // Escaped special chars
			case '"', '\'':
				current.WriteByte(ch) // Escaped quotes
			default:
				if lastChar == '\\' {
					current.WriteByte('\\') // Preserve backslash in patterns
				}
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			lastChar = '\\'
			continue
		}

		if ch == ',' && !escaped {
			part := current.String()
			key, value, found := strings.Cut(strings.TrimSpace(part), ":")
			if !found {
				return nil, errors.New(errInvalidFormat)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" {
				return nil, fmt.Errorf(errEmptyKey, input)
			}
			parts[key] = value
			current.Reset()
			continue
		}

		current.WriteByte(ch)
		lastChar = rune(ch)
	}

	pattern, ok := parts["pattern"]
	if !ok || pattern == "" {
		return nil, fmt.Errorf(errMissingValue, "pattern", input)
	}

	desc, ok := parts["desc"]
	if !ok || desc == "" {
		return nil, fmt.Errorf(errMissingValue, "desc", input)
	}

	return &types.PatternValue{
		Pattern:     pattern,
		Description: desc,
	}, nil
}

// PatternValues parses multiple pattern values
func PatternValues(input string) ([]types.PatternValue, error) {
	var result []types.PatternValue

	// Handle empty input
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "pattern values")
	}

	// Initial brace validation
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}

	// Split patterns while preserving escaped characters
	var current strings.Builder
	var escaped bool
	var braceCount int

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			current.WriteByte('\\')
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		switch ch {
		case '{':
			braceCount++
			current.WriteByte(ch)
		case '}':
			braceCount--
			current.WriteByte(ch)
			if braceCount == 0 && i+1 < len(input) && input[i+1] == ',' {
				pv, err := PatternValue(current.String())
				if err != nil {
					return nil, err
				}
				result = append(result, *pv)
				current.Reset()
				i++ // Skip the comma
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Handle the last pattern
	if current.Len() > 0 {
		pv, err := PatternValue(current.String())
		if err != nil {
			return nil, err
		}
		result = append(result, *pv)
	}

	return result, nil
}

// Dependency format rules:
//   - Must be enclosed in braces: {flag:name,...}
//   - Must have a flag field
//   - Can have either 'value' or 'values' field, but not both
//   - 'values' field must be enclosed in brackets [...]
//
// Escape sequences in values:
//   - Special characters:
//     \,  -> , (comma in value or values list)
//     \:  -> : (colon in value)
//     \[  -> [ (left bracket in value)
//     \]  -> ] (right bracket in value)
//
//   - Quotes:
//     \"  -> " (double quote)
//     \'  -> ' (single quote)
//
//   - Backslashes:
//     \\  -> \ (single backslash)
//
// Examples:
//   {flag:log,value:info\,debug}     -> flag="log" values=["info,debug"]
//   {flag:path,value:C:\\Windows}    -> flag="path" values=["C:\Windows"]
//   {flag:tags,values:[a\,b,c\,d]}   -> flag="tags" values=["a,b","c,d"]
//   {flag:cmd,values:[[a\,b],[c\,d]} -> flag="cmd" values=["[a,b]","[c,d]"]

// Dependencies parses multiple dependency entries in format {flag:a,value:1},{flag:b,values:[1,2]}
// using the dependency format rules defined above.
func Dependencies(input string) (DependencyMap, error) {
	result := make(DependencyMap)

	// Handle empty input
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "dependency")
	}

	// Initial brace validation
	if !strings.HasPrefix(strings.TrimSpace(input), "{") || !strings.HasSuffix(strings.TrimSpace(input), "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}

	// Split dependencies while preserving brackets
	var (
		entries                  []string
		current                  strings.Builder
		bracketCount, braceCount int
	)

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case '[':
			bracketCount++
			current.WriteByte(ch)
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return nil, fmt.Errorf(errUnmatchedBrackets, input)
			}
			current.WriteByte(ch)
		case '{':
			braceCount++
			current.WriteByte(ch)
		case '}':
			braceCount--
			if braceCount < 0 {
				return nil, fmt.Errorf(errMalformedBraces, input)
			}
			current.WriteByte(ch)
		case ',':
			if bracketCount == 0 && braceCount == 0 && i > 0 && input[i-1] == '}' && i+1 < len(input) && input[i+1] == '{' {
				entries = append(entries, current.String())
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	if bracketCount != 0 {
		return nil, fmt.Errorf(errUnmatchedBrackets, input)
	}
	if braceCount != 0 {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}

	if current.Len() > 0 {
		entries = append(entries, current.String())
	}

	// Check for duplicate flags
	seenFlags := make(map[string]bool)
	for _, entry := range entries {
		flag, values, err := Dependency(entry)
		if err != nil {
			return nil, err
		}
		if seenFlags[flag] {
			return nil, fmt.Errorf(errDuplicateFlag, flag)
		}
		seenFlags[flag] = true
		result[flag] = values
	}

	return result, nil
}

// Dependency parses a single dependency entry using the same dependency format rules as Dependencies.
func Dependency(input string) (string, []string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, fmt.Errorf(errEmptyInput, "dependency")
	}

	// Validate braces
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return "", nil, fmt.Errorf(errMalformedBraces, input)
	}
	input = strings.Trim(input, "{} \r\n")

	// Parse key-value pairs while preserving brackets
	parts := make(map[string]string)
	var current strings.Builder
	var bracketCount int

	input = input + "," // Add trailing comma to simplify parsing
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case '[':
			bracketCount++
			current.WriteByte(ch)
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return "", nil, fmt.Errorf(errUnmatchedBrackets, input)
			}
			current.WriteByte(ch)
		case ',':
			if bracketCount == 0 {
				part := current.String()
				key, value, found := strings.Cut(strings.TrimSpace(part), ":")
				if !found {
					if strings.Contains(part, "=") {
						return "", nil, errors.New(errInvalidFormat)
					}
					return "", nil, fmt.Errorf(errMalformedBraces, input)
				}
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key == "" {
					return "", nil, fmt.Errorf(errEmptyKey, part)
				}
				parts[key] = value
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	if bracketCount != 0 {
		return "", nil, fmt.Errorf(errUnmatchedBrackets, input)
	}

	flag, ok := parts["flag"]
	if !ok || flag == "" {
		return "", nil, fmt.Errorf(errMissingValue, "flag", input)
	}

	// Handle values
	if value, hasValue := parts["value"]; hasValue {
		if _, hasValues := parts["values"]; hasValues {
			return "", nil, fmt.Errorf(errBothValues, input)
		}
		if value == "" {
			return "", nil, fmt.Errorf(errEmptyValue, input)
		}
		return flag, []string{value}, nil
	}

	if values, hasValues := parts["values"]; hasValues {
		if len(values) > 1 && values[0] == '[' && values[len(values)-1] == ']' {
			values = values[1 : len(values)-1]
		}
		if values == "" {
			return flag, nil, nil
		}

		// Handle values with escaped characters
		var valueList []string
		var current strings.Builder
		var escaped bool
		var bracketCount int
		var lastChar rune

		values = values + "," // Add trailing comma to simplify parsing
		for i := 0; i < len(values); i++ {
			ch := values[i]
			if escaped {
				switch ch {
				case '\\':
					if lastChar == '\\' {
						current.WriteByte('\\') // Double backslash becomes single
					}
				case ',':
					current.WriteByte(',') // Escaped comma
				default:
					if lastChar == '\\' {
						current.WriteByte('\\') // Preserve backslash in paths
					}
					current.WriteByte(ch)
				}
				escaped = false
				continue
			}

			if ch == '\\' {
				escaped = true
				lastChar = '\\'
				continue
			}

			if ch == '[' {
				bracketCount++
				current.WriteByte(ch)
			} else if ch == ']' {
				bracketCount--
				current.WriteByte(ch)
				// If we're back to bracket level 0 and next char is comma,
				// this is a completely nested structure
				if bracketCount == 0 && i+1 < len(values) && values[i+1] == ',' {
					if v := strings.TrimSpace(current.String()); v != "" {
						// For double-bracketed items, remove one level
						if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
							v = v[1 : len(v)-1]
						}
						valueList = append(valueList, v)
					}
					current.Reset()
					i++ // Skip the comma
					continue
				}
			} else if ch == ',' && !escaped && bracketCount == 0 {
				if v := strings.TrimSpace(current.String()); v != "" {
					valueList = append(valueList, v)
				}
				current.Reset()
				continue
			} else {
				current.WriteByte(ch)
			}

			lastChar = rune(ch)
		}

		// Don't forget the last value
		if v := strings.TrimSpace(current.String()); v != "" {
			// For double-bracketed items, remove one level
			if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
				v = v[1 : len(v)-1]
			}
			valueList = append(valueList, v)
		}

		return flag, valueList, nil
	}

	return flag, nil, nil
}

func TypeOfFlagFromString(s string) types.OptionType {
	switch strings.ToUpper(s) {
	case "STANDALONE":
		return types.Standalone
	case "CHAINED":
		return types.Chained
	case "FILE":
		return types.File
	case "SINGLE":
		return types.Single
	default:
		return types.Empty
	}
}

func TypeOfFlagToString(t types.OptionType) string {
	switch t {
	case types.Standalone:
		return "standalone"
	case types.Single:
		return "single"
	case types.Chained:
		return "chained"
	case types.File:
		return "file"
	default:
		return "empty"
	}
}

func LegacyUnmarshalTagFormat(field reflect.StructField) (*types.TagConfig, error) {
	foundLegacyTag := false

	config := &types.TagConfig{
		Kind: types.KindFlag,
	}

	tagNames := []string{
		"long", "short", "description", "required", "type", "default",
		"secure", "prompt", "path", "accepted", "depends",
	}

	for _, tag := range tagNames {
		value, ok := field.Tag.Lookup(tag)
		if !ok {
			continue
		}

		foundLegacyTag = true
		switch tag {
		case "long":
			config.Name = value
		case "short":
			config.Short = value
		case "description":
			config.Description = value
		case "type":
			config.TypeOf = TypeOfFlagFromString(value)
		case "default":
			config.Default = value
		case "required":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'required' tag value for field %s: %w", field.Name, err)
			}
			config.Required = boolVal
		case "secure":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'secure' tag value for field %s: %w", field.Name, err)
			}
			if boolVal {
				config.Secure = types.Secure{IsSecure: boolVal}
			}
		case "prompt":
			if config.Secure.IsSecure {
				config.Secure.Prompt = value
			}
		case "path":
			config.Path = value
		case "accepted":
			patterns, err := PatternValues(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'accepted' tag value for field %s: %w", field.Name, err)
			}
			// Convert to PatternValue
			config.AcceptedValues = make([]types.PatternValue, len(patterns))
			for i, p := range patterns {
				pv, err := compilePattern(p, field.Name)
				if err != nil {
					return nil, err
				}
				config.AcceptedValues[i] = *pv
			}
		case "depends":
			deps, err := Dependencies(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'depends' tag value for field %s: %w", field.Name, err)
			}
			config.DependsOn = deps
		default:
			return nil, fmt.Errorf("unrecognized tag '%s' on field %s", tag, field.Name)
		}
	}

	if !foundLegacyTag {
		return nil, nil
	}

	if config.TypeOf == types.Empty {
		config.TypeOf = InferFieldType(field)
	}

	return config, nil
}

func InferFieldType(field interface{}) types.OptionType {
	var t reflect.Type

	switch f := field.(type) {
	case reflect.StructField:
		if f.Type == nil {
			return types.Empty
		}
		t = f.Type
	case reflect.Type:
		if f == nil {
			return types.Empty
		}
		t = f
	default:
		return types.Empty
	}

	switch t.Kind() {
	case reflect.Bool:
		return types.Standalone
	case reflect.Slice, reflect.Array:
		// Create a pointer to a slice of the element type
		slicePtr := reflect.New(t).Interface()
		if ok, _ := util.CanConvert(slicePtr, types.Chained); ok {
			return types.Chained
		}
		return types.Empty
	case reflect.String, reflect.Int, reflect.Int64, reflect.Float64, reflect.Float32,
		reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		return types.Single
	default:
		if t == reflect.TypeOf(time.Duration(0)) ||
			t == reflect.TypeOf(time.Time{}) {
			return types.Single
		}
		return types.Empty
	}
}

func UnmarshalTagFormat(tag string, field reflect.StructField) (*types.TagConfig, error) {
	config := &types.TagConfig{}
	parts := strings.Split(tag, ";")

	for _, part := range parts {
		key, value, found := strings.Cut(part, ":")
		if !found {
			return nil, fmt.Errorf("invalid tag format in field %s: %s", field.Name, part)
		}

		switch key {
		case "kind":
			switch types.Kind(value) {
			case types.KindFlag, types.KindCommand, types.KindEmpty:
				config.Kind = types.Kind(value)
			default:
				return nil, fmt.Errorf("invalid kind in field %s: %s (must be 'command', 'flag', or empty)",
					field.Name, value)
			}
		case "name":
			config.Name = value
		case "short":
			config.Short = value
		case "type":
			config.TypeOf = TypeOfFlagFromString(value)
		case "desc":
			config.Description = value
		case "default":
			config.Default = value
		case "required":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'required' value in field %s: %w", field.Name, err)
			}
			config.Required = boolVal
		case "secure":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'secure' value in field %s: %w", field.Name, err)
			}
			if boolVal {
				config.Secure = types.Secure{IsSecure: boolVal}
			}
		case "prompt":
			if config.Secure.IsSecure {
				config.Secure.Prompt = value
			}
		case "path":
			config.Path = value
		case "accepted":
			patterns, err := PatternValues(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'accepted' value in field %s: %w", field.Name, err)
			}
			for i, p := range patterns {
				config.AcceptedValues = make([]types.PatternValue, len(patterns))
				pv, err := compilePattern(p, field.Name)
				if err != nil {
					return nil, err
				}
				config.AcceptedValues[i] = *pv
			}
		case "depends":
			deps, err := Dependencies(value)
			if err != nil {
				return nil, fmt.Errorf("invalid 'depends' value in field %s: %w", field.Name, err)
			}
			config.DependsOn = deps
		default:
			return nil, fmt.Errorf("unrecognized key '%s' in field %s", key, field.Name)
		}
	}

	// If kind is empty, treat as flag
	if config.Kind == types.KindEmpty {
		config.Kind = types.KindFlag
	}

	if config.TypeOf == types.Empty && config.Kind != types.KindCommand {
		config.TypeOf = InferFieldType(field)
	}

	return config, nil
}

func compilePattern(p types.PatternValue, fieldName string) (*types.PatternValue, error) {
	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid 'accepted' value in field %s: %w", fieldName, err)
	}

	return &types.PatternValue{
		Pattern:     p.Pattern,
		Description: p.Description,
		Compiled:    re,
	}, nil
}
