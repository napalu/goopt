package i18n

import (
	"errors"
	"fmt"
)

// TranslatableError represents an error that can be translated
type TranslatableError interface {
	error
	Key() string
	Args() []interface{}
	Unwrap() error
	WithArgs(args ...interface{}) TranslatableError
	Wrap(err error) TranslatableError
	Is(target error) bool
	Format(provider MessageProvider) string
}

// TrError represents a translatable error with optional formatting arguments
// and error wrapping support. It implements both the TranslatableError interface
// and the standard error interface.
//
// Example usage:
//
//	err := NewError("validation.error")
//	err = err.WithArgs("field", "value")
//	err = err.Wrap(originalError)
type TrError struct {
	// The sentinel error value for comparison with errors.Is
	sentinel error
	// The translation key
	key  string
	keyT Translatable
	// Optional format arguments
	args []interface{}
	// Optional wrapped error
	wrapped error
}

// NewError creates a new translatable error with a key
func NewError(key string) *TrError {
	sentinel := errors.New(key)
	return &TrError{
		sentinel: sentinel,
		keyT:     NewTranslatable(key),
		key:      key,
	}
}

// NewErrorWithProvider creates a new translatable error with a key and specific provider
func NewErrorWithProvider(key string, provider MessageProvider) *TrError {
	defaultMsg := provider.GetMessage(key)
	sentinel := errors.New(defaultMsg)
	return &TrError{
		sentinel: sentinel,
		key:      key,
	}
}

// Error returns the default message, formatted with args if provided
func (e *TrError) Error() string {
	return e.Format(getDefaultProvider())
}

// WithArgs returns a copy of the error with format arguments
func (e *TrError) WithArgs(args ...interface{}) TranslatableError {
	return &TrError{
		sentinel: e.sentinel,
		key:      e.key,
		args:     args,
		wrapped:  e.wrapped,
	}
}

// Wrap returns a new error that wraps another error
func (e *TrError) Wrap(err error) TranslatableError {
	return &TrError{
		sentinel: e.sentinel,
		key:      e.key,
		args:     e.args,
		wrapped:  err,
	}
}

// Is implements errors.Is for comparison with the sentinel error
func (e *TrError) Is(target error) bool {
	// Check if target is the same sentinel error
	if t, ok := target.(*TrError); ok {
		return e.sentinel == t.sentinel
	}
	// Check if target is the sentinel error directly
	return target == e.sentinel || target == e
}

// Key returns the translation key
func (e *TrError) Key() string {
	return e.key
}

// Args returns the format arguments
func (e *TrError) Args() []interface{} {
	return e.args
}

// Unwrap returns the wrapped error
func (e *TrError) Unwrap() error {
	return e.wrapped
}

func (e *TrError) Format(provider MessageProvider) string {
	msg := provider.GetMessage(e.key)
	if len(e.args) > 0 {
		translatedArgs := make([]interface{}, len(e.args))

		// Get formatter if provider supports it
		var formatter *Formatter
		if lp, ok := provider.(*LayeredMessageProvider); ok {
			formatter = lp.GetFormatter()
		}

		// Parse format string to determine which arguments need locale formatting
		// This is a simple heuristic: if we see %d, %f, etc., we keep raw values
		// If we see %s or %v, we apply locale formatting
		formatSpecifiers := parseFormatSpecifiers(msg)

		for i, arg := range e.args {
			if t, ok := arg.(Translatable); ok {
				translatedArgs[i] = t.T(provider)
			} else if formatter != nil && i < len(formatSpecifiers) {
				// Check the format specifier for this argument
				spec := formatSpecifiers[i]

				// Apply locale formatting only for %s and %v specifiers
				if spec == 's' || spec == 'v' {
					switch v := arg.(type) {
					case int:
						translatedArgs[i] = formatter.FormatInt(v)
					case int64:
						translatedArgs[i] = formatter.FormatInt64(v)
					case float64:
						translatedArgs[i] = formatter.FormatFloat(v, 2) // Default precision
					case float32:
						translatedArgs[i] = formatter.FormatFloat(float64(v), 2)
					default:
						translatedArgs[i] = arg
					}
				} else {
					// Keep raw value for %d, %f, etc.
					translatedArgs[i] = arg
				}
			} else {
				translatedArgs[i] = arg
			}
		}
		msg = fmt.Sprintf(msg, translatedArgs...)
	}

	if e.wrapped != nil {
		return fmt.Sprintf("%s: %v", msg, e.wrapped)
	}
	return msg
}

// parseFormatSpecifiers extracts the format specifier characters from a format string
// Returns a slice of runes representing the specifier for each argument in positional order
func parseFormatSpecifiers(format string) []rune {
	type specInfo struct {
		position int
		verb     rune
	}

	var specs []specInfo
	runes := []rune(format)
	nextPosition := 1 // Default position counter for non-positional args

	for i := 0; i < len(runes); i++ {
		if runes[i] == '%' && i+1 < len(runes) {
			// Skip %%
			if runes[i+1] == '%' {
				i++
				continue
			}

			// Look for format specifier
			j := i + 1
			position := 0

			// Check for positional argument notation %[n]
			if j < len(runes) && runes[j] == '[' {
				j++ // Skip [
				numStart := j
				for j < len(runes) && runes[j] >= '0' && runes[j] <= '9' {
					j++
				}
				if j < len(runes) && runes[j] == ']' {
					// Parse the position number
					for k := numStart; k < j; k++ {
						position = position*10 + int(runes[k]-'0')
					}
					j++ // Skip ]
				}
			}

			// If no position specified, use next available
			if position == 0 {
				position = nextPosition
				nextPosition++
			}

			// Skip flags, width, and precision
			for j < len(runes) && (runes[j] == '-' || runes[j] == '+' || runes[j] == ' ' ||
				runes[j] == '#' || runes[j] == '0' || (runes[j] >= '0' && runes[j] <= '9') ||
				runes[j] == '.' || runes[j] == '*') {
				j++
			}

			// Get the format verb
			if j < len(runes) {
				specs = append(specs, specInfo{position: position, verb: runes[j]})
				i = j
			}
		}
	}

	// Sort by position and extract verbs
	maxPos := 0
	for _, spec := range specs {
		if spec.position > maxPos {
			maxPos = spec.position
		}
	}

	result := make([]rune, maxPos)
	for _, spec := range specs {
		if spec.position > 0 && spec.position <= maxPos {
			result[spec.position-1] = spec.verb
		}
	}

	return result
}
