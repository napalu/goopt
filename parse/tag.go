package parse

import (
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
	errInvalidFormat     = "invalid format in: %s"
	errEmptyKey          = "empty key in: %s"
	errMissingValue      = "missing or empty %s in: %s"
	errDuplicateFlag     = "duplicate flag: %s"
	errEmptyValue        = "empty value in: %s"
	errBothValues        = "cannot specify both 'value' and 'values' in: %s"
)

// TypeOfFlagFromString converts a string to a types.OptionType
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
		t = util.UnwrapType(f.Type)
	case reflect.Type:
		if f == nil {
			return types.Empty
		}
		t = util.UnwrapType(f)
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
				return nil, fmt.Errorf("invalid 'depends' value in field %s: %w", field.Name, err)
			}
			config.DependsOn = deps
		case "capacity":
			cap, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid capacity value '%s' in %q: %w", value, field.Name, err)
			}
			if cap < 0 {
				return nil, fmt.Errorf("negative capacity not allowed in %q: %d", field.Name, cap)
			}
			config.Capacity = cap
		case "pos":
			posData, err := Position(value)
			if err != nil {
				return nil, fmt.Errorf("invalid position in field %s: %w", field.Name, err)
			}
			config.Position = &posData.Index
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
