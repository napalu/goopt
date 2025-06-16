package validation

import (
	"github.com/napalu/goopt/v2/internal/util"
	"strconv"
	"strings"

	"github.com/napalu/goopt/v2/errs"
)

// Constants for pattern formats
const (
	// Pattern format prefixes
	JSONFormatPrefix     = "{"
	JSONFormatSuffix     = "}"
	PatternPrefix        = "pattern:"
	DescriptionSeparator = ",desc:"

	// Email validators
	ValidatorEmail = "email"

	// URL validators
	ValidatorURL = "url"

	// Length validators (Unicode characters)
	ValidatorMinLength = "minlength"
	ValidatorMinLen    = "minlen"
	ValidatorMaxLength = "maxlength"
	ValidatorMaxLen    = "maxlen"
	ValidatorLength    = "length"
	ValidatorLen       = "len"

	// Byte length validators
	ValidatorMinByteLength = "minbytelength"
	ValidatorMinByteLen    = "minbytelen"
	ValidatorMaxByteLength = "maxbytelength"
	ValidatorMaxByteLen    = "maxbytelen"
	ValidatorByteLength    = "bytelength"
	ValidatorByteLen       = "bytelen"

	// Numeric range validators
	ValidatorRange    = "range"
	ValidatorIntRange = "intrange"
	ValidatorMin      = "min"
	ValidatorMax      = "max"

	// Regex validators
	ValidatorRegex        = "regex"
	ValidatorMustMatch    = "mustmatch"
	ValidatorMustNotMatch = "mustnotmatch"

	// Collection validators
	ValidatorIsOneOf    = "isoneof"
	ValidatorIsNotOneOf = "isnotoneof"

	// Type validators
	ValidatorInteger      = "integer"
	ValidatorInt          = "int"
	ValidatorFloat        = "float"
	ValidatorNumber       = "number"
	ValidatorBoolean      = "boolean"
	ValidatorBool         = "bool"
	ValidatorAlphaNumeric = "alphanumeric"
	ValidatorAlNum        = "alnum"
	ValidatorIdentifier   = "identifier"
	ValidatorID           = "id"
	ValidatorNoWhitespace = "nowhitespace"
	ValidatorNoSpace      = "nospace"
	ValidatorFileExt      = "fileext"
	ValidatorExtension    = "extension"
	ValidatorHostname     = "hostname"
	ValidatorHost         = "host"
	ValidatorIP           = "ip"
	ValidatorIPAddress    = "ipaddress"
	ValidatorPort         = "port"

	// Composite validators
	ValidatorOneOf = "oneof"
	ValidatorAll   = "all"
	ValidatorNot   = "not"
)

// ParseValidators converts validator specifications to validator functions
// Specifications can be:
// - Simple: "email", "integer", "alphanumeric"
// - With args: "minlength:5", "range:1:100", "oneof:red:green:blue"
// - Combined: "email,minlength:5"
func ParseValidators(specs []string) ([]Validator, error) {
	var validators []Validator

	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		validator, err := parseValidatorWithDepth(spec, 0)
		if err != nil {
			return nil, errs.ErrInvalidValidator.WithArgs(spec).Wrap(err)
		}
		validators = append(validators, validator)
	}

	return validators, nil
}

// parseCompositeArgs parses arguments for composite validators like oneof and all
// It handles nested validators by tracking parentheses/braces depth
func parseCompositeArgs(input string) []string {
	return parseParenthesesArgs(input)
}

// parseParenthesesArgs parses the new intuitive syntax: oneof(email,url,integer)
// It preserves nested parentheses and braces
func parseParenthesesArgs(input string) []string {
	var args []string
	var current strings.Builder
	parenDepth := 0
	braceDepth := 0

	for i := 0; i < len(input); i++ {
		ch := input[i]

		switch ch {
		case '(':
			parenDepth++
			current.WriteByte(ch)
		case ')':
			parenDepth--
			current.WriteByte(ch)
		case '{':
			braceDepth++
			current.WriteByte(ch)
		case '}':
			braceDepth--
			current.WriteByte(ch)
		case ',':
			// Only split on commas at depth 0 (not inside nested validators or braces)
			if parenDepth == 0 && braceDepth == 0 {
				if current.Len() > 0 {
					args = append(args, strings.TrimSpace(current.String()))
					current.Reset()
				}
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Add the last argument
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	return args
}

const maxRecursionDepth = 10 // Prevent infinite recursion

func parseValidator(spec string) (Validator, error) {
	return parseValidatorWithDepth(spec, 0)
}

func parseValidatorWithDepth(spec string, depth int) (Validator, error) {
	if depth > maxRecursionDepth {
		return nil, errs.ErrValidatorRecursionDepthExceeded
	}
	// Check for parentheses syntax
	if parenIndex := strings.Index(spec, "("); parenIndex != -1 {
		// Parentheses syntax: validator(args)
		name := spec[:parenIndex]
		if !strings.HasSuffix(spec, ")") {
			return nil, errs.ErrInvalidValidator.WithArgs(spec, "missing closing parenthesis")
		}
		argsStr := spec[parenIndex+1 : len(spec)-1]

		var args []string
		switch strings.ToLower(name) {
		case ValidatorOneOf, ValidatorAll:
			args = parseCompositeArgs(argsStr)
		case ValidatorNot:
			args = []string{argsStr}
		default:
			// For validators that might have special comma handling
			switch strings.ToLower(name) {
			case ValidatorRegex:
				// Regex supports multiple formats:
				// 1. regex(pattern) - just the pattern
				// 2. regex(pattern:xxx,desc:xxx) - explicit pattern and description
				// 3. regex({pattern:xxx,desc:xxx}) - JSON-like format (backward compat)
				if argsStr != "" {
					args = []string{argsStr}
				}
			case ValidatorMustMatch, ValidatorMustNotMatch:
				// These take a single pattern argument that might contain commas
				if argsStr != "" {
					args = []string{argsStr}
				}
			default:
				// For other validators with parentheses, split by comma
				if argsStr != "" {
					args = strings.Split(argsStr, ",")
					for i := range args {
						args[i] = strings.TrimSpace(args[i])
					}
				}
			}
		}

		return createValidatorWithDepth(name, args, depth)
	}

	// For validators without parentheses, check if they have arguments (colon syntax)
	if strings.Contains(spec, ":") {
		// Validators with arguments MUST use parentheses syntax
		return nil, errs.ErrValidatorMustUseParentheses.WithArgs(spec)
	}

	// Simple validators without arguments (like "email", "integer", etc.)
	return createValidatorWithDepth(spec, nil, depth)
}

func createValidatorWithDepth(name string, args []string, depth int) (Validator, error) {
	// Use EqualFold for case-insensitive comparison that handles Unicode correctly
	switch {
	case strings.EqualFold(name, ValidatorEmail):
		return Email(), nil
	case strings.EqualFold(name, ValidatorURL):
		return URL(args...), nil
	case strings.EqualFold(name, ValidatorMinLength) || strings.EqualFold(name, ValidatorMinLen):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMinLength, 1)
		}
		minL, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeInteger.WithArgs(ValidatorMinLength)
		}
		if minL < 0 {
			return nil, errs.ErrValidatorArgumentCannotBeNegative.WithArgs(ValidatorMinLength)
		}
		return MinLength(minL), nil
	case strings.EqualFold(name, ValidatorMaxLength) || strings.EqualFold(name, ValidatorMaxLen):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMaxLength, 1)
		}
		maxL, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeInteger.WithArgs(ValidatorMaxLength)
		}
		if maxL < 0 {
			return nil, errs.ErrValidatorArgumentCannotBeNegative.WithArgs(ValidatorMaxLength)
		}
		return MaxLength(maxL), nil
	case strings.EqualFold(name, ValidatorLength) || strings.EqualFold(name, ValidatorLen):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorLength, 1)
		}
		exact, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeInteger.WithArgs(ValidatorLength)
		}
		if exact < 0 {
			return nil, errs.ErrValidatorArgumentCannotBeNegative.WithArgs(ValidatorLength)
		}
		return Length(exact), nil
	case strings.EqualFold(name, ValidatorMinByteLength) || strings.EqualFold(name, ValidatorMinByteLen):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMinByteLength, 1)
		}
		minB, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeInteger.WithArgs(ValidatorMinByteLength)
		}
		if minB < 0 {
			return nil, errs.ErrValidatorArgumentCannotBeNegative.WithArgs(ValidatorMinByteLength)
		}
		return MinByteLength(minB), nil
	case strings.EqualFold(name, ValidatorMaxByteLength) || strings.EqualFold(name, ValidatorMaxByteLen):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMaxByteLength, 1)
		}
		maxB, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeInteger.WithArgs(ValidatorMaxByteLength)
		}
		if maxB < 0 {
			return nil, errs.ErrValidatorArgumentCannotBeNegative.WithArgs(ValidatorMaxByteLength)
		}
		return MaxByteLength(maxB), nil
	case strings.EqualFold(name, ValidatorByteLength) || strings.EqualFold(name, ValidatorByteLen):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorByteLength, 1)
		}
		exactB, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeInteger.WithArgs(ValidatorByteLength)
		}
		if exactB < 0 {
			return nil, errs.ErrValidatorArgumentCannotBeNegative.WithArgs(ValidatorByteLength)
		}
		return ByteLength(exactB), nil
	case strings.EqualFold(name, ValidatorRange):
		if len(args) != 2 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorRange, 2)
		}
		startR, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeNumber.WithArgs("range min")
		}
		endR, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeNumber.WithArgs("range max")
		}
		return Range(startR, endR), nil
	case strings.EqualFold(name, ValidatorIntRange):
		if len(args) != 2 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorRange, 2)
		}
		var startR, endR int
		err := util.ConvertString(args[0], &startR, name, func(matchOn rune) bool {
			if matchOn == ',' {
				return true
			}

			return false
		})
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeNumber.WithArgs("range min")
		}
		err = util.ConvertString(args[1], &endR, name, func(matchOn rune) bool {
			if matchOn == ',' {
				return true
			}

			return false
		})
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeNumber.WithArgs("range max")
		}
		return IntRange(startR, endR), nil
	case strings.EqualFold(name, ValidatorMin):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMin, 1)
		}
		minF, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeNumber.WithArgs(ValidatorMin)
		}
		return Min(minF), nil
	case strings.EqualFold(name, ValidatorMax):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMax, 1)
		}
		maxF, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return nil, errs.ErrValidatorArgumentMustBeNumber.WithArgs(ValidatorMax)
		}
		return Max(maxF), nil
	// Regex validators
	case strings.EqualFold(name, ValidatorRegex):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorRegex, 1)
		}

		arg := args[0]

		// Support multiple formats:
		// 1. regex(pattern) - pattern is used as description
		// 2. regex(pattern:xxx,desc:xxx) - explicit pattern and description
		// 3. regex({pattern:xxx,desc:xxx}) - JSON-like format for backward compatibility

		// Check for structured formats
		if strings.HasPrefix(arg, JSONFormatPrefix) && strings.HasSuffix(arg, JSONFormatSuffix) {
			// JSON-like format - use RegexSpec
			v, err := RegexSpec(arg)
			if err != nil {
				return nil, err
			}
			return v, nil
		} else if strings.HasPrefix(arg, PatternPrefix) {
			// New structured format: pattern:xxx,desc:xxx
			parts := strings.SplitN(arg, DescriptionSeparator, 2)
			if len(parts) == 2 {
				pattern := strings.TrimPrefix(parts[0], PatternPrefix)
				desc := parts[1]
				return Regex(pattern, desc)
			}
			// No desc part, just pattern
			pattern := strings.TrimPrefix(arg, PatternPrefix)
			return Regex(pattern, pattern)
		} else {
			// Plain pattern - use pattern as description
			return Regex(arg, arg)
		}
	case strings.EqualFold(name, ValidatorMustMatch):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMustMatch, 1)
		}
		v, err := RegexSpec(args[0])
		if err != nil {
			return nil, err
		}
		return v, nil
	case strings.EqualFold(name, ValidatorMustNotMatch):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorMustNotMatch, 1)
		}
		// Use Not composition with RegexSpec
		v, err := RegexSpec(args[0])
		if err != nil {
			return nil, err
		}
		return Not(v), nil

	// String matching validators
	case strings.EqualFold(name, ValidatorIsOneOf):
		if len(args) == 0 {
			return nil, errs.ErrValidatorRequiresAtLeastOneArgument.WithArgs(ValidatorIsOneOf)
		}
		return IsOneOf(args...), nil
	case strings.EqualFold(name, ValidatorIsNotOneOf):
		if len(args) == 0 {
			return nil, errs.ErrValidatorRequiresAtLeastOneArgument.WithArgs(ValidatorIsNotOneOf)
		}
		return IsNotOneOf(args...), nil
	case strings.EqualFold(name, ValidatorInteger) || strings.EqualFold(name, ValidatorInt):
		return Integer(), nil
	case strings.EqualFold(name, ValidatorFloat) || strings.EqualFold(name, ValidatorNumber):
		return Float(), nil
	case strings.EqualFold(name, ValidatorBoolean) || strings.EqualFold(name, ValidatorBool):
		return Boolean(), nil
	case strings.EqualFold(name, ValidatorAlphaNumeric) || strings.EqualFold(name, ValidatorAlNum):
		return AlphaNumeric(), nil
	case strings.EqualFold(name, ValidatorIdentifier) || strings.EqualFold(name, ValidatorID):
		return Identifier(), nil
	case strings.EqualFold(name, ValidatorNoWhitespace) || strings.EqualFold(name, ValidatorNoSpace):
		return NoWhitespace(), nil
	case strings.EqualFold(name, ValidatorFileExt) || strings.EqualFold(name, ValidatorExtension):
		if len(args) == 0 {
			return nil, errs.ErrValidatorRequiresAtLeastOneArgument.WithArgs(ValidatorFileExt)
		}
		return FileExtension(args...), nil
	case strings.EqualFold(name, ValidatorHostname) || strings.EqualFold(name, ValidatorHost):
		return Hostname(), nil
	case strings.EqualFold(name, ValidatorIP) || strings.EqualFold(name, ValidatorIPAddress):
		return IP(), nil
	case strings.EqualFold(name, ValidatorPort):
		return Port(), nil

	// Compositional validators
	case strings.EqualFold(name, ValidatorOneOf):
		if len(args) == 0 {
			return nil, errs.ErrValidatorRequiresAtLeastOneArgument.WithArgs(ValidatorOneOf)
		}
		// Parse each validator spec and compose with OneOf
		var subValidators []Validator
		for _, arg := range args {
			subValidator, err := parseValidatorWithDepth(arg, depth+1)
			if err != nil {
				return nil, err
			}
			subValidators = append(subValidators, subValidator)
		}
		return OneOf(subValidators...), nil
	case strings.EqualFold(name, ValidatorAll):
		if len(args) == 0 {
			return nil, errs.ErrValidatorRequiresAtLeastOneArgument.WithArgs(ValidatorAll)
		}
		// Parse each validator spec and compose with All
		var subValidators []Validator
		for _, arg := range args {
			subValidator, err := parseValidatorWithDepth(arg, depth+1)
			if err != nil {
				return nil, err
			}
			subValidators = append(subValidators, subValidator)
		}
		return All(subValidators...), nil
	case strings.EqualFold(name, ValidatorNot):
		if len(args) != 1 {
			return nil, errs.ErrValidatorRequiresArgument.WithArgs(ValidatorNot, 1)
		}
		// Parse the validator spec and negate it
		subValidator, err := parseValidatorWithDepth(args[0], depth+1)
		if err != nil {
			return nil, err
		}
		return Not(subValidator), nil
	default:
		return nil, errs.ErrUnknownValidator.WithArgs(name)
	}
}
