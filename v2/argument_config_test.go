package goopt

import (
	"testing"

	"github.com/napalu/goopt/v2/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetValidators(t *testing.T) {
	t.Run("SetValidators replaces all validators", func(t *testing.T) {
		// Create arg with initial validators
		arg := NewArg(
			WithValidator(validation.MinLength(5)),
			WithValidator(validation.MaxLength(10)),
		)

		// Verify initial validators
		assert.Len(t, arg.Validators, 2)

		// Use SetValidators to replace them
		arg2 := NewArg(
			SetValidators(
				validation.Email(),
				validation.MinLength(3),
			),
		)

		// Verify validators were replaced (not added)
		assert.Len(t, arg2.Validators, 2)

		// Test with a valid email
		err := arg2.Validators[0]("test@example.com")
		assert.NoError(t, err)

		// Test with invalid email
		err = arg2.Validators[0]("not-an-email")
		assert.Error(t, err)
	})

	t.Run("SetValidators with empty list clears validators", func(t *testing.T) {
		// Create arg with validators then clear them
		arg := NewArg(
			WithValidator(validation.MinLength(5)),
			SetValidators(), // This should clear all validators
		)

		// Verify validators were cleared
		assert.Len(t, arg.Validators, 0)
	})

	t.Run("SetValidators in NewArgE", func(t *testing.T) {
		// Test with NewArgE for error handling
		arg, err := NewArgE(
			WithValidator(validation.MinLength(5)),
			SetValidators(validation.Email()), // Replace with email validator
		)
		assert.NoError(t, err)
		assert.Len(t, arg.Validators, 1)

		// Verify it's the email validator
		err = arg.Validators[0]("user@example.com")
		assert.NoError(t, err)
	})
}
