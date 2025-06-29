// Package errors provides common type definitions for the goopt library.
// This file contains constants for all translation keys used throughout the library.
package errs

// Prefix for all goopt translation keys
const (
	prefixKey = "goopt"
)

// Error prefixes
const (
	ErrorPrefixKey    = prefixKey + ".error"
	ParseErrorPathKey = ErrorPrefixKey + ".parse"
)

// CoreErrors contains keys for core parser error messages
const (
	// Core parser errors
	ErrUnsupportedTypeKey              = ErrorPrefixKey + ".unsupported_type"
	ErrCommandNotFoundKey              = ErrorPrefixKey + ".command_not_found"
	ErrCommandNoCallbackKey            = ErrorPrefixKey + ".command_not_found_or_no_callback"
	ErrFlagNotFoundKey                 = ErrorPrefixKey + ".flag_not_found"
	ErrPosixIncompatibleKey            = ErrorPrefixKey + ".posix_incompatible"
	ErrValidationFailedKey             = ErrorPrefixKey + ".validation_failed"
	ErrBindNilKey                      = ErrorPrefixKey + ".bind_nil"
	ErrNonPointerVarKey                = ErrorPrefixKey + ".non_pointer_var"
	ErrRequiredFlagKey                 = ErrorPrefixKey + ".required_flag"
	ErrRequiredPositionalFlagKey       = ErrorPrefixKey + ".required_positional_flag"
	ErrInvalidArgumentTypeKey          = ErrorPrefixKey + ".invalid_argument_type"
	ErrFlagValueNotRetrievedKey        = ErrorPrefixKey + ".flag_value_not_retrieved"
	ErrEmptyArgumentPrefixListKey      = ErrorPrefixKey + ".empty_argument_prefix_list"
	ErrEmptyFlagKey                    = ErrorPrefixKey + ".empty_flag"
	ErrFlagAlreadyExistsKey            = ErrorPrefixKey + ".flag_already_exists"
	ErrFlagDoesNotExistKey             = ErrorPrefixKey + ".flag_does_not_exist"
	ErrPosixShortFormKey               = ErrorPrefixKey + ".posix__short_form_incompatible"
	ErrShortFlagConflictKey            = ErrorPrefixKey + ".short_flag_conflict"
	ErrShortFlagConflictKeyContext     = ErrorPrefixKey + ".short_flag_conflict_context"
	ErrInvalidListDelimiterFuncKey     = ErrorPrefixKey + ".invalid_list_delimiter_func"
	ErrBindInvalidValueKey             = ErrorPrefixKey + ".bind_invalid_value_field"
	ErrPointerExpectedKey              = ErrorPrefixKey + ".pointer_to_variable_expected"
	ErrOptionNotSetKey                 = ErrorPrefixKey + ".option_not_set"
	ErrLanguageUnavailableKey          = ErrorPrefixKey + ".language_not_available"
	ErrShortFlagUndefinedKey           = ErrorPrefixKey + ".short_flag_not_defined"
	ErrDependencyOnEmptyFlagKey        = ErrorPrefixKey + ".dependency_on_empty_flag"
	ErrRemoveDependencyFromEmptyKey    = ErrorPrefixKey + ".remove_dependency_from_empty_flag"
	ErrSettingBoundValueKey            = ErrorPrefixKey + ".setting_bound_variable_value"
	ErrCommandCallbackErrorKey         = ErrorPrefixKey + ".command_callback_error"
	ErrNegativeCapacityKey             = ErrorPrefixKey + ".negative_capacity"
	ErrUnsupportedTypeConversionKey    = ErrorPrefixKey + ".unsupported_type_conversion"
	ErrNoPreValidationFiltersKey       = ErrorPrefixKey + ".no_pre_validation_filters"
	ErrNoPostValidationFiltersKey      = ErrorPrefixKey + ".no_post_validation_filters"
	ErrEmptyCommandPathKey             = ErrorPrefixKey + ".empty_command_path"
	ErrRecursionDepthExceededKey       = ErrorPrefixKey + ".recursion_depth_exceeded"
	ErrConfiguringParserKey            = ErrorPrefixKey + ".configuring_parser"
	ErrFieldBindingKey                 = ErrorPrefixKey + ".field_binding"
	ErrUnwrappingValueKey              = ErrorPrefixKey + ".unwrapping_value"
	ErrOnlyStructsCanBeTaggedKey       = ErrorPrefixKey + ".only_structs_can_be_tagged"
	ErrProcessingFieldWithPrefixKey    = ErrorPrefixKey + ".processing_field_with_prefix"
	ErrProcessingFieldKey              = ErrorPrefixKey + ".processing_field"
	ErrProcessingFlagKey               = ErrorPrefixKey + ".processing_flag"
	ErrProcessingSliceFieldKey         = ErrorPrefixKey + ".processing_slice_field"
	ErrProcessingNestedStructKey       = ErrorPrefixKey + ".processing_nested_struct"
	ErrProcessingCommandKey            = ErrorPrefixKey + ".processing_command"
	ErrUnmarshallingTagKey             = ErrorPrefixKey + ".unmarshalling_tag"
	ErrNilPointerKey                   = ErrorPrefixKey + ".nil_pointer"
	ErrNoValidTagsKey                  = ErrorPrefixKey + ".no_valid_tags"
	ErrInvalidAttributeForTypeKey      = ErrorPrefixKey + ".invalid_attribute_for_type"
	ErrMissingPropertyOnLevelKey       = ErrorPrefixKey + ".missing_property_on_level"
	ErrWrappedKey                      = ErrorPrefixKey + ".wrapped"
	ErrNotFoundPathForFlagKey          = ErrorPrefixKey + ".not_found_path_for_flag"
	ErrNotFilePathForFlagKey           = ErrorPrefixKey + ".not_file_path_for_flag"
	ErrFlagFileOperationKey            = ErrorPrefixKey + ".flag_file_operation"
	ErrFlagExpectsValueKey             = ErrorPrefixKey + ".flag_expects_value"
	ErrCommandExpectsSubcommandKey     = ErrorPrefixKey + ".command_expects_subcommand"
	ErrInvalidArgumentKey              = ErrorPrefixKey + ".invalid_argument"
	ErrNoValuesKey                     = ErrorPrefixKey + ".no_values"
	ErrSecureFlagExpectsValueKey       = ErrorPrefixKey + ".secure_flag_expects_value"
	ErrCircularDependencyKey           = ErrorPrefixKey + ".circular_dependency"
	ErrDependencyNotFoundKey           = ErrorPrefixKey + ".dependency_not_specified"
	ErrDependencyValueNotSpecifiedKey  = ErrorPrefixKey + ".dependency_value_not_specified"
	ErrMissingArgumentInfoKey          = ErrorPrefixKey + ".missing_argument_info"
	ErrIndexOutOfBoundsKey             = ErrorPrefixKey + ".index_out_of_bounds"
	ErrUnknownFlagKey                  = ErrorPrefixKey + ".unknown_flag"
	ErrUnknownFlagWithSuggestionsKey   = ErrorPrefixKey + ".unknown_flag_with_suggestions"
	ErrPositionMustBeNonNegativeKey    = ErrorPrefixKey + ".position_must_be_non_negative"
	ErrPositionalArgumentNotFoundKey   = ErrorPrefixKey + ".positional_argument_not_found"
	ErrUnknownFlagInCommandPathKey     = ErrorPrefixKey + ".unknown_flag_in_command_path"
	ErrInvalidTagFormatKey             = ErrorPrefixKey + ".invalid_tag_format"
	ErrInvalidKindKey                  = ErrorPrefixKey + ".invalid_kind"
	ErrRegexCompileKey                 = ErrorPrefixKey + ".regex.compile"
	ErrFileOperationKey                = ErrorPrefixKey + ".file.operation"
	ErrNotAttachedToTerminalKey        = ErrorPrefixKey + ".not_attached_to_terminal"
	ErrCallbackOnNonTerminalCommandKey = ErrorPrefixKey + ".callback_on_non_terminal_command"
)

// ParseErrors contains keys for parsing and validation errors
const (
	// Parsing/validation errors
	ErrParseBoolKey              = ParseErrorPathKey + ".bool"
	ErrParseIntKey               = ParseErrorPathKey + ".int"
	ErrParseFloatKey             = ParseErrorPathKey + ".float"
	ErrParseDurationKey          = ParseErrorPathKey + ".duration"
	ErrParseTimeKey              = ParseErrorPathKey + ".time"
	ErrParseComplexKey           = ParseErrorPathKey + ".complex"
	ErrParseListKey              = ParseErrorPathKey + ".list"
	ErrParseInt64Key             = ParseErrorPathKey + ".int64"
	ErrParseInt32Key             = ParseErrorPathKey + ".int32"
	ErrParseInt16Key             = ParseErrorPathKey + ".int16"
	ErrParseInt8Key              = ParseErrorPathKey + ".int8"
	ErrParseOverflowKey          = ParseErrorPathKey + ".overflow"
	ErrParseFloat64Key           = ParseErrorPathKey + ".float64"
	ErrParseFloat32Key           = ParseErrorPathKey + ".float32"
	ErrParseUintKey              = ParseErrorPathKey + ".uint"
	ErrParseUint64Key            = ParseErrorPathKey + ".uint64"
	ErrParseUint32Key            = ParseErrorPathKey + ".uint32"
	ErrParseUint16Key            = ParseErrorPathKey + ".uint16"
	ErrParseUint8Key             = ParseErrorPathKey + ".uint8"
	ErrParseUintptrKey           = ParseErrorPathKey + ".uintptr"
	ErrParseDuplicateFlagKey     = ParseErrorPathKey + ".duplicate_flag"
	ErrParseEmptyInputKey        = ParseErrorPathKey + ".empty_input"
	ErrParseMalformedBracesKey   = ParseErrorPathKey + ".malformed_braces"
	ErrParseUnmatchedBracketsKey = ParseErrorPathKey + ".unmatched_brackets"
	ErrParseInvalidFormatKey     = ParseErrorPathKey + ".invalid_format"
	ErrParseEmptyKeyKey          = ParseErrorPathKey + ".empty_key"
	ErrParseMissingValueKey      = ParseErrorPathKey + ".missing_value"
	ErrParseNegativeIndexKey     = ParseErrorPathKey + ".negative_index"

	// Validation error keys
	ValidationErrorPathKey = ErrorPrefixKey + ".validation"

	// General validation errors
	ErrValidationCombinedFailedKey = ValidationErrorPathKey + ".combined_failed"
	ErrValueMustBeNumberKey        = ValidationErrorPathKey + ".must_be_number"

	// Email validation
	ErrInvalidEmailFormatKey = ValidationErrorPathKey + ".invalid_email_format"

	// URL validation
	ErrInvalidURLKey           = ValidationErrorPathKey + ".invalid_url"
	ErrURLSchemeMustBeOneOfKey = ValidationErrorPathKey + ".url_scheme_must_be_one_of"
	ErrURLMustHaveHostKey      = ValidationErrorPathKey + ".url_must_have_host"

	// Length validation (Unicode characters)
	ErrMinLengthKey   = ValidationErrorPathKey + ".min_length"
	ErrMaxLengthKey   = ValidationErrorPathKey + ".max_length"
	ErrExactLengthKey = ValidationErrorPathKey + ".exact_length"

	// Byte length validation
	ErrMinByteLengthKey   = ValidationErrorPathKey + ".min_byte_length"
	ErrMaxByteLengthKey   = ValidationErrorPathKey + ".max_byte_length"
	ErrExactByteLengthKey = ValidationErrorPathKey + ".exact_byte_length"

	// Numeric validation
	ErrValueBetweenKey = ValidationErrorPathKey + ".value_between"
	ErrValueAtLeastKey = ValidationErrorPathKey + ".value_at_least"
	ErrValueAtMostKey  = ValidationErrorPathKey + ".value_at_most"

	// Pattern validation
	ErrPatternMatchKey = ValidationErrorPathKey + ".pattern_match"

	// Set validation
	ErrValueMustBeOneOfKey = ValidationErrorPathKey + ".value_must_be_one_of"
	ErrValueCannotBeKey    = ValidationErrorPathKey + ".value_cannot_be"

	// Type validation
	ErrValueMustBeIntegerKey            = ValidationErrorPathKey + ".must_be_integer"
	ErrValueMustBeBooleanKey            = ValidationErrorPathKey + ".must_be_boolean"
	ErrValueMustBeAlphanumericKey       = ValidationErrorPathKey + ".must_be_alphanumeric"
	ErrValueMustBeIdentifierKey         = ValidationErrorPathKey + ".must_be_identifier"
	ErrValueMustNotContainWhitespaceKey = ValidationErrorPathKey + ".must_not_contain_whitespace"

	// File validation
	ErrFileMustHaveExtensionKey = ValidationErrorPathKey + ".file_must_have_extension"

	// Network validation
	ErrHostnameTooLongKey       = ValidationErrorPathKey + ".hostname_too_long"
	ErrInvalidHostnameFormatKey = ValidationErrorPathKey + ".invalid_hostname_format"
	ErrInvalidIPv4AddressKey    = ValidationErrorPathKey + ".invalid_ipv4_address"
	ErrValueMustBeValidIPKey    = ValidationErrorPathKey + ".must_be_valid_ip"

	// Validator parsing errors
	ErrInvalidValidatorKey                    = ValidationErrorPathKey + ".invalid_validator"
	ErrValidatorRequiresArgumentKey           = ValidationErrorPathKey + ".validator_requires_argument"
	ErrValidatorArgumentMustBeIntegerKey      = ValidationErrorPathKey + ".validator_argument_must_be_integer"
	ErrValidatorArgumentMustBeNumberKey       = ValidationErrorPathKey + ".validator_argument_must_be_number"
	ErrValidatorRequiresAtLeastOneArgumentKey = ValidationErrorPathKey + ".validator_requires_at_least_one_argument"
	ErrUnknownValidatorKey                    = ValidationErrorPathKey + ".unknown_validator"
	ErrValidatorArgumentCannotBeNegativeKey   = ValidationErrorPathKey + ".validator_argument_cannot_be_negative"
	ErrValidatorRecursionDepthExceededKey     = ValidationErrorPathKey + ".recursion_depth_exceeded"
	ErrValidatorMustUseParenthesesKey         = ValidationErrorPathKey + ".must_use_parentheses"
)
