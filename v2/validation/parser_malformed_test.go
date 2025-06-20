package validation

import (
	"errors"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/errs"
	"github.com/stretchr/testify/assert"
)

func TestParserMalformedInput(t *testing.T) {
	t.Run("Missing closing parenthesis", func(t *testing.T) {
		tests := []string{
			"oneof(email,url",
			"all(minlength(5),maxlength(10)",
			"not(email",
			"regex(^[A-Z]+$",
		}

		for _, spec := range tests {
			t.Run(spec, func(t *testing.T) {
				_, err := parseValidator(spec)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, errs.ErrInvalidValidator), "Expected ErrInvalidValidator for: %s", spec)
			})
		}
	})

	t.Run("Missing opening parenthesis", func(t *testing.T) {
		tests := []struct {
			spec        string
			expectedErr error
		}{
			{"oneofemail,url)", errs.ErrUnknownValidator},
			{"allminlength:5,maxlength:10)", errs.ErrValidatorMustUseParentheses}, // has colon, so different error
			{"notemail)", errs.ErrUnknownValidator},
		}

		for _, tt := range tests {
			t.Run(tt.spec, func(t *testing.T) {
				_, err := parseValidator(tt.spec)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected %v for: %s, got: %v", tt.expectedErr, tt.spec, err)
			})
		}
	})

	t.Run("Empty parentheses", func(t *testing.T) {
		tests := []struct {
			spec        string
			shouldError bool
			expectedErr error
		}{
			{"oneof()", true, errs.ErrValidatorRequiresAtLeastOneArgument},
			{"all()", true, errs.ErrValidatorRequiresAtLeastOneArgument},
			{"not()", true, errs.ErrUnknownValidator}, // not() becomes empty string, treated as unknown
			{"email()", false, nil},                   // email doesn't require args
			{"minlength()", true, errs.ErrValidatorRequiresArgument},
		}

		for _, tt := range tests {
			t.Run(tt.spec, func(t *testing.T) {
				_, err := parseValidator(tt.spec)
				if tt.shouldError {
					assert.Error(t, err)
					if tt.expectedErr != nil {
						assert.True(t, errors.Is(err, tt.expectedErr), "Expected %v for: %s, got: %v", tt.expectedErr, tt.spec, err)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Nested malformed input", func(t *testing.T) {
		tests := []string{
			"oneof(email,all(minlength:5,maxlength:10)", // missing closing paren in nested
			"all(oneof(email,url),not(regex(^[A-Z]+$))", // missing closing paren at end
			"not(oneof(email,url)",                      // missing closing paren in nested
		}

		for _, spec := range tests {
			t.Run(spec, func(t *testing.T) {
				_, err := parseValidator(spec)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, errs.ErrInvalidValidator), "Expected ErrInvalidValidator for: %s", spec)
			})
		}
	})

	t.Run("Invalid nested validators", func(t *testing.T) {
		tests := []struct {
			spec        string
			expectedErr error
		}{
			{"oneof(unknown)", errs.ErrUnknownValidator},
			{"all(email,badvalidator)", errs.ErrUnknownValidator},
			{"not(invalidtype)", errs.ErrUnknownValidator},
			{"oneof(minlength)", errs.ErrValidatorRequiresArgument}, // minlength needs an arg
		}

		for _, tt := range tests {
			t.Run(tt.spec, func(t *testing.T) {
				_, err := parseValidator(tt.spec)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected %v for: %s, got: %v", tt.expectedErr, tt.spec, err)
			})
		}
	})

	t.Run("Unbalanced braces in regex", func(t *testing.T) {
		tests := []string{
			"regex:{pattern:^[A-Z]+$,desc:uppercase", // missing closing brace
			"regex:pattern:^[A-Z]+$,desc:uppercase}", // missing opening brace
			"regex:{pattern:^[A-Z]+$desc:uppercase}", // missing comma
		}

		for _, spec := range tests {
			t.Run(spec, func(t *testing.T) {
				// These should not cause infinite loops, but may cause regex compilation errors
				validator, err := parseValidator(spec)
				if err != nil {
					// Error is acceptable for malformed input
					assert.Error(t, err)
				} else {
					// If it parses, it should at least be callable
					assert.NotNil(t, validator)
					// Test that it doesn't crash
					_ = validator("test")
				}
			})
		}
	})

	t.Run("Recursion depth protection", func(t *testing.T) {
		// Create a deeply nested validator that exceeds max depth
		nested := "email"
		for i := 0; i < 15; i++ { // More than maxRecursionDepth (10)
			nested = "not(" + nested + ")"
		}

		_, err := parseValidator(nested)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errs.ErrValidatorRecursionDepthExceeded), "Expected ErrValidatorRecursionDepthExceeded")
	})

	t.Run("Circular reference attempt", func(t *testing.T) {
		// While we can't create true circular references in a single spec,
		// we can test edge cases that might cause confusion
		tests := []string{
			"oneof(oneof(email))", // redundant nesting
			"all(all(email))",     // redundant nesting
			"not(not(email))",     // double negation
		}

		for _, spec := range tests {
			t.Run(spec, func(t *testing.T) {
				validator, err := parseValidator(spec)
				assert.NoError(t, err) // These should be valid, just redundant
				assert.NotNil(t, validator)

				// Test functionality
				err = validator("user@example.com")
				assert.NoError(t, err)
			})
		}
	})

	t.Run("Invalid arguments", func(t *testing.T) {
		tests := []struct {
			spec        string
			expectedErr error
		}{
			{"minlength:abc", errs.ErrValidatorMustUseParentheses}, // old syntax should fail
			{"maxlength:-5", errs.ErrValidatorMustUseParentheses},  // old syntax should fail
			{"range:1:abc", errs.ErrValidatorMustUseParentheses},   // old syntax should fail
			{"range:abc:10", errs.ErrValidatorMustUseParentheses},  // old syntax should fail
			{"length:0.5", errs.ErrValidatorMustUseParentheses},    // old syntax should fail
			{"minlength(abc)", errs.ErrValidatorArgumentMustBeInteger},
			{"maxlength(-5)", errs.ErrValidatorArgumentCannotBeNegative},
			{"range(1,abc)", errs.ErrValidatorArgumentMustBeNumber},
			{"range(abc,10)", errs.ErrValidatorArgumentMustBeNumber},
			{"length(0.5)", errs.ErrValidatorArgumentMustBeInteger},
		}

		for _, tt := range tests {
			t.Run(tt.spec, func(t *testing.T) {
				_, err := parseValidator(tt.spec)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected %v for: %s, got: %v", tt.expectedErr, tt.spec, err)
			})
		}
	})

	t.Run("Comma-separated malformed specs", func(t *testing.T) {
		// Test that we handle malformed specs in lists properly
		specs := []string{
			"email,minlength:abc,url", // middle spec is malformed
			"unknown,email",           // first spec is malformed
			"email,badvalidator",      // last spec is malformed
		}

		for _, specList := range specs {
			t.Run(specList, func(t *testing.T) {
				specArray := strings.Split(specList, ",")
				_, err := ParseValidators(specArray)
				assert.Error(t, err)
			})
		}
	})

	t.Run("Empty and whitespace specs", func(t *testing.T) {
		tests := []struct {
			specs    []string
			expected int // expected number of validators
		}{
			{[]string{""}, 0},                 // empty string should be ignored
			{[]string{"  "}, 0},               // whitespace should be ignored
			{[]string{"email", "", "url"}, 2}, // empty in middle should be ignored
			{[]string{"", "email", ""}, 1},    // empty at start/end should be ignored
		}

		for _, tt := range tests {
			t.Run(strings.Join(tt.specs, ","), func(t *testing.T) {
				validators, err := ParseValidators(tt.specs)
				assert.NoError(t, err)
				assert.Len(t, validators, tt.expected)
			})
		}
	})
}

func TestParserEdgeCases(t *testing.T) {
	t.Run("Unicode in validator names", func(t *testing.T) {
		// Validator names should be ASCII, but test Unicode gracefully fails
		tests := []string{
			"émäil",     // accented characters
			"验证器",       // Chinese characters
			"валидатор", // Cyrillic
		}

		for _, spec := range tests {
			t.Run(spec, func(t *testing.T) {
				_, err := parseValidator(spec)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, errs.ErrUnknownValidator), "Expected ErrUnknownValidator for: %s", spec)
			})
		}
	})

	t.Run("Very long validator specs", func(t *testing.T) {
		// Test performance and memory usage with long specs
		longPattern := strings.Repeat("a", 10000)
		spec := "regex:" + longPattern

		validator, err := parseValidator(spec)
		if err != nil {
			// Error is acceptable for very long patterns
			assert.Error(t, err)
		} else {
			// If it parses, it should work
			assert.NotNil(t, validator)
		}
	})

	t.Run("Special characters in arguments", func(t *testing.T) {
		tests := []struct {
			spec       string
			shouldWork bool
			expectErr  error
		}{
			{"regex(^[a-z\\d]+$)", true, nil},                                      // escaped characters
			{"regex(^[a-z)]+$)", true, nil},                                        // parenthesis in pattern
			{"regex(^[a-z,]+$)", true, nil},                                        // comma in pattern
			{"isoneof(val1,val2,val3)", true, nil},                                 // normal comma separation
			{"regex:^[a-z\\d]+$", false, errs.ErrValidatorMustUseParentheses},      // old syntax
			{"isoneof:val1,val2,val3", false, errs.ErrValidatorMustUseParentheses}, // old syntax
			{"isoneof:val1:val2:val3", false, errs.ErrValidatorMustUseParentheses}, // old syntax with colons
		}

		for _, tt := range tests {
			t.Run(tt.spec, func(t *testing.T) {
				validator, err := parseValidator(tt.spec)
				if tt.shouldWork {
					assert.NoError(t, err)
					assert.NotNil(t, validator)
				} else {
					assert.Error(t, err)
					if tt.expectErr != nil {
						assert.True(t, errors.Is(err, tt.expectErr), "Expected %v for: %s, got: %v", tt.expectErr, tt.spec, err)
					}
				}
			})
		}
	})
}
