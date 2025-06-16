package goopt

import (
	"testing"

	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/validation"
	"github.com/stretchr/testify/assert"
)

func TestNewArgWithInvalidRegex(t *testing.T) {
	t.Run("NewArg ignores regex errors", func(t *testing.T) {
		// This will create an Argument but the regex compilation error is ignored
		arg := NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[invalid regex", Description: "This regex is invalid"},
			}),
		)

		// The argument is created but AcceptedValues[0].Compiled will be nil
		assert.NotNil(t, arg)
		assert.Len(t, arg.AcceptedValues, 1)
		assert.Nil(t, arg.AcceptedValues[0].Compiled)
	})

	t.Run("NewArgE returns error for invalid regex", func(t *testing.T) {
		// This will return an error for invalid regex
		arg, err := NewArgE(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[invalid regex", Description: "This regex is invalid"},
			}),
		)

		assert.Nil(t, arg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing closing ]")
	})

	t.Run("NewArgE succeeds with valid regex", func(t *testing.T) {
		arg, err := NewArgE(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: `^[0-9]+$`, Description: "Numbers only"},
				{Pattern: `^[a-z]+$`, Description: "Lowercase letters only"},
			}),
		)

		assert.NoError(t, err)
		assert.NotNil(t, arg)
		assert.Len(t, arg.AcceptedValues, 2)
		assert.NotNil(t, arg.AcceptedValues[0].Compiled)
		assert.NotNil(t, arg.AcceptedValues[1].Compiled)
	})

	t.Run("AddFlag with NewArg and invalid regex", func(t *testing.T) {
		parser := NewParser()

		// This will succeed even with invalid regex because NewArg ignores errors
		err := parser.AddFlag("test", NewArg(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[invalid", Description: "Invalid pattern"},
			}),
		))

		assert.NoError(t, err) // AddFlag succeeds

		// Parsing will fail because no patterns will match (they're all nil)
		success := parser.Parse([]string{"--test", "value"})
		assert.False(t, success) // Fails because no valid patterns to match against
	})

	t.Run("AddFlag with NewArgE and invalid regex", func(t *testing.T) {
		// First create the argument with error handling
		arg, err := NewArgE(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[invalid", Description: "Invalid pattern"},
			}),
		)

		// We get the error at creation time
		assert.Error(t, err)
		assert.Nil(t, arg)

		// So we don't even try to add it to the parser
		// If we had a valid arg, we would do:
		// parser := NewParser()
		// err = parser.AddFlag("test", arg)
	})

	t.Run("Argument.Set also handles errors properly", func(t *testing.T) {
		arg := &Argument{}
		err := arg.Set(
			WithType(types.Single),
			WithAcceptedValues([]types.PatternValue{
				{Pattern: "[invalid", Description: "Invalid pattern"},
			}),
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing closing ]")
	})
}

func TestNewArgEWithValidators(t *testing.T) {
	t.Run("can add validators with NewArgE", func(t *testing.T) {
		arg, err := NewArgE(
			WithType(types.Single),
			WithValidators(validation.HasPrefix("test-")),
		)

		assert.NoError(t, err)
		assert.NotNil(t, arg)
		assert.Len(t, arg.Validators, 1)
	})
}
