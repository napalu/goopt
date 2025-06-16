package validation

import (
	"fmt"
	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"regexp"
	"strconv"
	"strings"
)

// IntRangeValidator validates integer values within a range
type IntRangeValidator struct {
	ValidatorMetadata
	min, max int
}

func (v *IntRangeValidator) Validate(value string) error {
	num, err := strconv.Atoi(value)
	if err != nil {
		return errs.ErrValueMustBeInteger.WithArgs(value)
	}
	if num < v.min || num > v.max {
		return errs.ErrValueBetween.WithArgs(v.min, v.max, value)
	}
	return nil
}

// NewIntRangeValidator creates a new integer range validator
func NewIntRangeValidator(min, max int) Validator {
	return &IntRangeValidator{
		ValidatorMetadata: ValidatorMetadata{
			name:          fmt.Sprintf("int-range[%d,%d]", min, max),
			validatorType: "range",
			description:   fmt.Sprintf("Integer between %d and %d", min, max),
			isConverted:   false,
		},
		min: min,
		max: max,
	}
}

// RegexValidator validates against a regex pattern
type RegexValidator struct {
	ValidatorMetadata
	pattern   string
	compiled  *regexp.Regexp
	descOrKey string
	provider  i18n.MessageProvider
}

func (v *RegexValidator) Validate(value string) error {
	if !v.compiled.MatchString(value) {
		desc := v.descOrKey
		if desc == "" {
			desc = v.pattern
		} else if v.provider != nil {
			// Try to translate
			translated := v.provider.GetMessage(desc)
			if translated != desc {
				desc = translated
			}
		}
		return errs.ErrPatternMatch.WithArgs(desc, value)
	}
	return nil
}

// NewRegexValidator creates a new regex validator
func NewRegexValidator(pattern, descriptionOrKey string, provider i18n.MessageProvider) (Validator, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errs.ErrInvalidValidator.WithArgs(pattern, err)
	}

	return &RegexValidator{
		ValidatorMetadata: ValidatorMetadata{
			name:          fmt.Sprintf("regex[%s]", pattern),
			validatorType: "pattern",
			description:   descriptionOrKey,
			isConverted:   false,
		},
		pattern:   pattern,
		compiled:  re,
		descOrKey: descriptionOrKey,
		provider:  provider,
	}, nil
}

// CompositeValidator combines multiple validators with a specific logic (All/Any)
type CompositeValidator struct {
	ValidatorMetadata
	validators []Validator
	logic      string // "all" or "any"
}

func (v *CompositeValidator) Validate(value string) error {
	// Handle empty validators case
	if len(v.validators) == 0 {
		return nil // No validators means always pass
	}

	if v.logic == "all" {
		for _, validator := range v.validators {
			if err := validator.Validate(value); err != nil {
				return err
			}
		}
		return nil
	}

	// logic == "any"
	var errors []string
	for _, validator := range v.validators {
		if err := validator.Validate(value); err == nil {
			return nil
		} else {
			errors = append(errors, err.Error())
		}
	}

	if v.logic == "any" {
		return errs.ErrValidationCombinedFailed.WithArgs(strings.Join(errors, " OR "))
	}

	return errs.ErrValidationCombinedFailed.WithArgs(strings.Join(errors, "; "))
}

// NewAllValidator creates a validator where all sub-validators must pass
func NewAllValidator(validators ...Validator) Validator {
	names := make([]string, len(validators))
	for i, v := range validators {
		names[i] = v.Name()
	}

	return &CompositeValidator{
		ValidatorMetadata: ValidatorMetadata{
			name:          fmt.Sprintf("all[%s]", strings.Join(names, ",")),
			validatorType: "composite",
			description:   "All validators must pass",
			isConverted:   false,
		},
		validators: validators,
		logic:      "all",
	}
}

// NewAnyValidator creates a validator where at least one sub-validator must pass
func NewAnyValidator(validators ...Validator) Validator {
	names := make([]string, len(validators))
	for i, v := range validators {
		names[i] = v.Name()
	}

	return &CompositeValidator{
		ValidatorMetadata: ValidatorMetadata{
			name:          fmt.Sprintf("any[%s]", strings.Join(names, ",")),
			validatorType: "composite",
			description:   "At least one validator must pass",
			isConverted:   false,
		},
		validators: validators,
		logic:      "any",
	}
}

// Migration helpers to create Validator versions of existing validators

// IntRangeV creates a Validator version of IntRange
func IntRangeV(min, max int) Validator {
	return NewIntRangeValidator(min, max)
}
