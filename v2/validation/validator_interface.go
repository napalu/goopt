package validation

import (
	"strings"

	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/types"
)

// Validator is the interface for all validators
type Validator interface {
	// Validate checks if the value is valid
	Validate(value string) error

	// Name returns a unique name/ID for this validator
	Name() string

	// Type returns the type of validator (e.g., "range", "pattern", "converted-accepted-values")
	Type() string

	// Description returns a human-readable description of what this validator checks
	Description() string

	// IsConvertedFromAcceptedValues returns true if this validator was created from AcceptedValues
	IsConvertedFromAcceptedValues() bool
}

// ValidatorMetadata provides common metadata fields for validators
type ValidatorMetadata struct {
	name          string
	validatorType string
	description   string
	isConverted   bool
}

func (m ValidatorMetadata) Name() string {
	return m.name
}

func (m ValidatorMetadata) Type() string {
	return m.validatorType
}

func (m ValidatorMetadata) Description() string {
	return m.description
}

func (m ValidatorMetadata) IsConvertedFromAcceptedValues() bool {
	return m.isConverted
}

// FuncValidator wraps a validation function to implement the Validator interface
type FuncValidator struct {
	ValidatorMetadata
	fn func(string) error
}

func (f *FuncValidator) Validate(value string) error {
	return f.fn(value)
}

// NewFuncValidator creates a Validator from a validation function
func NewFuncValidator(name, validatorType, description string, fn func(string) error) Validator {
	return &FuncValidator{
		ValidatorMetadata: ValidatorMetadata{
			name:          name,
			validatorType: validatorType,
			description:   description,
			isConverted:   false,
		},
		fn: fn,
	}
}

// ConvertedAcceptedValuesValidator is a special validator for converted AcceptedValues
type ConvertedAcceptedValuesValidator struct {
	ValidatorMetadata
	patterns []types.PatternValue
	provider i18n.MessageProvider
}

func (v *ConvertedAcceptedValuesValidator) Validate(value string) error {
	// Check if any pattern matches
	for _, pv := range v.patterns {
		if pv.Compiled != nil && pv.Compiled.MatchString(value) {
			return nil
		}
	}

	// No match - build error with translated descriptions
	var descriptions []string
	for _, pv := range v.patterns {
		desc := pv.Description
		if desc == "" {
			desc = pv.Pattern
		} else if v.provider != nil {
			// Try to translate - if it returns the same string, it's not a translation key
			translated := v.provider.GetMessage(desc)
			if translated != desc {
				desc = translated
			}
		}
		descriptions = append(descriptions, desc)
	}

	// Return error in legacy format
	return errs.ErrInvalidArgument.WithArgs(value, "flag", strings.Join(descriptions, ", "))
}

// NewConvertedAcceptedValuesValidator creates a validator from AcceptedValues
func NewConvertedAcceptedValuesValidator(patterns []types.PatternValue, provider i18n.MessageProvider) Validator {
	return &ConvertedAcceptedValuesValidator{
		ValidatorMetadata: ValidatorMetadata{
			name:          "accepted-values",
			validatorType: "pattern-list",
			description:   "Validates against accepted value patterns",
			isConverted:   true,
		},
		patterns: patterns,
		provider: provider,
	}
}
