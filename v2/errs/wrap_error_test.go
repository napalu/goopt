package errs

import (
	"errors"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/stretchr/testify/assert"
)

func TestWrapProcessingError(t *testing.T) {
	baseErr := errors.New("base error")

	t.Run("wraps unwrapped error", func(t *testing.T) {
		result := WrapOnce(baseErr, ErrProcessingField, "fieldName")

		// Should be wrapped with ErrProcessingField
		var wrapped *i18n.TrError
		assert.True(t, errors.As(result, &wrapped))
		assert.Equal(t, ErrProcessingField.Key(), wrapped.Key())

		// Should contain the field name in args
		assert.Equal(t, []interface{}{"fieldName"}, wrapped.Args())

		// Should wrap the original error
		assert.True(t, errors.Is(result, baseErr))
	})

	t.Run("does not double wrap with same error type", func(t *testing.T) {
		// First wrap
		wrapped := WrapOnce(baseErr, ErrProcessingField, "field1")

		// Try to wrap again with same error type
		result := WrapOnce(wrapped, ErrProcessingField, "field2")

		// Should return the original wrapped error unchanged
		assert.Equal(t, wrapped, result)

		// Args should still be from first wrap
		var errWithProvider *i18n.TrError
		assert.True(t, errors.As(result, &errWithProvider))
		assert.Equal(t, []interface{}{"field1"}, errWithProvider.Args())
	})

	t.Run("wraps with different error type", func(t *testing.T) {
		// First wrap with ErrProcessingField
		wrapped := WrapOnce(baseErr, ErrProcessingField, "fieldName")

		// Wrap with different error type
		result := WrapOnce(wrapped, ErrProcessingCommand, "commandName")

		// Should have both wrappers
		var cmdErr *i18n.TrError
		assert.True(t, errors.As(result, &cmdErr))
		assert.Equal(t, ErrProcessingCommand.Key(), cmdErr.Key())

		// Should still be able to find the inner error
		var fieldErr *i18n.TrError
		assert.True(t, errors.As(result, &fieldErr))

		// Should still wrap the original base error
		assert.True(t, errors.Is(result, baseErr))
	})

	t.Run("handles multiple field arguments", func(t *testing.T) {
		result := WrapOnce(baseErr, ErrProcessingField, "field1", "field2", "field3")

		var wrapped *i18n.TrError
		assert.True(t, errors.As(result, &wrapped))
		assert.Equal(t, []interface{}{"field1", "field2", "field3"}, wrapped.Args())
	})

	t.Run("handles no field arguments", func(t *testing.T) {
		result := WrapOnce(baseErr, ErrProcessingField)

		var wrapped *i18n.TrError
		assert.True(t, errors.As(result, &wrapped))
		assert.Empty(t, wrapped.Args())
	})

	t.Run("preserves error chain", func(t *testing.T) {
		// Create a chain: baseErr -> ErrInvalidArgument -> ErrProcessingField
		innerWrapped := ErrInvalidArgument.Wrap(baseErr).WithArgs("value")
		result := WrapOnce(innerWrapped, ErrProcessingField, "fieldName")

		// Should be able to find all errors in the chain
		assert.True(t, errors.Is(result, baseErr))

		var invalidValueErr *i18n.TrError
		assert.True(t, errors.As(result, &invalidValueErr))

		var processingErr *i18n.TrError
		assert.True(t, errors.As(result, &processingErr))
		assert.Equal(t, ErrProcessingField.Key(), processingErr.Key())
	})

	t.Run("works with custom TranslatableError implementations", func(t *testing.T) {
		// Assuming you have other TranslatableError types
		result := WrapOnce(baseErr, ErrUnknownFlag, "myFlag")

		var wrapped *i18n.TrError
		assert.True(t, errors.As(result, &wrapped))
		assert.Equal(t, ErrUnknownFlag.Key(), wrapped.Key())
		assert.Equal(t, []interface{}{"myFlag"}, wrapped.Args())
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		// This test assumes WrapOnce handles nil gracefully
		// You might want to add nil check in the function
		result := WrapOnce(nil, ErrProcessingField, "fieldName")
		assert.Nil(t, result)
	})
}

// Test with a more complex scenario
func TestWrapProcessingError_ComplexScenario(t *testing.T) {
	// Simulate a real-world scenario with nested operations

	t.Run("nested struct processing error chain", func(t *testing.T) {
		// Start with a validation error
		validationErr := errors.New("value must be positive")

		// Wrap as invalid argument error
		invalidErr := ErrInvalidArgument.Wrap(validationErr).WithArgs(-5)

		// Try to wrap as processing field error
		fieldErr := WrapOnce(invalidErr, ErrProcessingField, "config.port")

		// Try to wrap again as processing field (should not double wrap)
		fieldErr2 := WrapOnce(fieldErr, ErrProcessingField, "config.port.value")

		// Should be the same error (no double wrapping)
		assert.Equal(t, fieldErr, fieldErr2)

		// But can wrap with different error type
		structErr := WrapOnce(fieldErr, ErrProcessingNestedStruct, "config")
		assert.NotEqual(t, fieldErr, structErr)

		// Verify the chain
		assert.True(t, errors.Is(structErr, validationErr))

		var errStruct *i18n.TrError
		assert.True(t, errors.As(structErr, &errStruct))
		assert.Equal(t, ErrProcessingNestedStruct.Key(), errStruct.Key())
	})
}

// Benchmark to ensure the function is performant
func BenchmarkWrapProcessingError(b *testing.B) {
	baseErr := errors.New("base error")

	b.Run("wrap new error", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = WrapOnce(baseErr, ErrProcessingField, "fieldName")
		}
	})

	b.Run("check already wrapped", func(b *testing.B) {
		wrapped := WrapOnce(baseErr, ErrProcessingField, "fieldName")
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = WrapOnce(wrapped, ErrProcessingField, "fieldName2")
		}
	})
}
