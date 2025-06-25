package parse

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/types"
)

// DependencyMap maps flag names to their allowed values
// empty slice means any value is acceptable
type DependencyMap map[string][]string

// Common error messages

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
		k, value, found := strings.Cut(part, ":")
		if !found {
			return nil, errs.ErrInvalidTagFormat.WithArgs(part)
		}

		switch key := strings.ToLower(k); key {
		case "desckey":
			config.DescriptionKey = value
		case "namekey":
			config.NameKey = value
		case "kind":
			switch types.Kind(value) {
			case types.KindFlag, types.KindCommand, types.KindEmpty:
				config.Kind = types.Kind(value)
			default:
				return nil, errs.ErrInvalidKind.WithArgs(value)
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
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'required'", field.Name, value)
			}
			config.Required = boolVal
		case "secure":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'secure'", field.Name, value)
			}
			if boolVal {
				config.Secure = types.Secure{IsSecure: boolVal}
			}
		case "prompt":
			if config.Secure.IsSecure {
				config.Secure.Prompt = value
			}
		case "help":
			//
		case "path":
			config.Path = value
		case "accepted":
			patterns, err := PatternValues(value)
			if err != nil {
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'accepted'", field.Name, value)
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
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'depends'", field.Name, value)
			}
			config.DependsOn = deps
		case "capacity":
			kap, err := strconv.Atoi(value)
			if err != nil {
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'capacity'", field.Name, value)
			}
			if kap < 0 {
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'capacity'", field.Name, value)
			}
			config.Capacity = kap
		case "pos":
			posData, err := Position(value)
			if err != nil {
				return nil, errs.ErrInvalidAttributeForType.WithArgs("'position'", field.Name, value)
			}
			config.Position = &posData.Index
		case "validators":
			config.Validators = ValidatorSpecs(value)
		default:
			return nil, errs.ErrInvalidAttributeForType.WithArgs(key, field.Name)
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
		return nil, errs.ErrInvalidAttributeForType.WithArgs("'accepted'", fieldName, p.Pattern)
	}

	return &types.PatternValue{
		Pattern:     p.Pattern,
		Description: p.Description,
		Compiled:    re,
	}, nil
}
