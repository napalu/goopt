// Package types provides common type definitions for the goopt library.
// This file contains constants for all translation keys used throughout the library.
package types

// Prefix for all goopt translation keys
const (
	PrefixKey = "goopt"
)

// Error prefixes
const (
	ErrorPrefixKey    = PrefixKey + ".error"
	ParseErrorPathKey = ErrorPrefixKey + ".parse"
	WarningPrefixKey  = PrefixKey + ".warning"
	MessagePrefixKey  = PrefixKey + ".msg"
)

// CoreErrors contains keys for core parser error messages
const (
	// Core parser errors
	ErrUnsupportedTypeKey           = ErrorPrefixKey + ".unsupported_type"
	ErrCommandNotFoundKey           = ErrorPrefixKey + ".command_not_found"
	ErrCommandNoCallbackKey         = ErrorPrefixKey + ".command_not_found_or_no_callback"
	ErrFlagNotFoundKey              = ErrorPrefixKey + ".flag_not_found"
	ErrPosixIncompatibleKey         = ErrorPrefixKey + ".posix_incompatible"
	ErrValidationFailedKey          = ErrorPrefixKey + ".validation_failed"
	ErrBindNilKey                   = ErrorPrefixKey + ".bind_nil"
	ErrNonPointerVarKey             = ErrorPrefixKey + ".non_pointer_var"
	ErrRequiredFlagKey              = ErrorPrefixKey + ".required_flag"
	ErrRequiredPositionalFlagKey    = ErrorPrefixKey + ".required_positional_flag"
	ErrInvalidArgumentTypeKey       = ErrorPrefixKey + ".invalid_argument_type"
	ErrFlagValueNotRetrievedKey     = ErrorPrefixKey + ".flag_value_not_retrieved"
	ErrEmptyArgumentPrefixListKey   = ErrorPrefixKey + ".empty_argument_prefix_list"
	ErrEmptyFlagKey                 = ErrorPrefixKey + ".empty_flag"
	ErrFlagAlreadyExistsKey         = ErrorPrefixKey + ".flag_already_exists"
	ErrPosixShortFormKey            = ErrorPrefixKey + ".posix__short_form_incompatible"
	ErrShortFlagConflictKey         = ErrorPrefixKey + ".short_flag_conflict"
	ErrInvalidListDelimiterFuncKey  = ErrorPrefixKey + ".invalid_list_delimiter_func"
	ErrBindInvalidValueKey          = ErrorPrefixKey + ".bind_invalid_value_field"
	ErrPointerExpectedKey           = ErrorPrefixKey + ".pointer_to_variable_expected"
	ErrOptionNotSetKey              = ErrorPrefixKey + ".option_not_set"
	ErrLanguageUnavailableKey       = ErrorPrefixKey + ".language_not_available"
	ErrShortFlagUndefinedKey        = ErrorPrefixKey + ".short_flag_not_defined"
	ErrDependencyOnEmptyFlagKey     = ErrorPrefixKey + ".dependency_on_empty_flag"
	ErrRemoveDependencyFromEmptyKey = ErrorPrefixKey + ".remove_dependency_from_empty_flag"
	ErrSettingBoundValueKey         = ErrorPrefixKey + ".setting_bound_variable_value"
	ErrCommandCallbackErrorKey      = ErrorPrefixKey + ".command_callback_error"
	ErrNegativeCapacityKey          = ErrorPrefixKey + ".negative_capacity"
	ErrUnsupportedTypeConversionKey = ErrorPrefixKey + ".unsupported_type_conversion"
	ErrFieldBindingKey              = ErrorPrefixKey + ".field_binding"
	ErrNilPointerKey                = ErrorPrefixKey + ".nil_pointer"
	ErrNoValidTagsKey               = ErrorPrefixKey + ".no_valid_tags"
	ErrInvalidAttributeForTypeKey   = ErrorPrefixKey + ".invalid_attribute_for_type"
	ErrWrappedKey                   = ErrorPrefixKey + ".wrapped"
)

// ParseErrors contains keys for parsing and validation errors
const (
	// Parsing/validation errors
	ErrParseBoolKey     = ParseErrorPathKey + ".bool"
	ErrParseIntKey      = ParseErrorPathKey + ".int"
	ErrParseFloatKey    = ParseErrorPathKey + ".float"
	ErrRegexCompileKey  = ErrorPrefixKey + ".regex.compile"
	ErrFileOperationKey = ErrorPrefixKey + ".file.operation"
	ErrParseDurationKey = ParseErrorPathKey + ".duration"
	ErrParseTimeKey     = ParseErrorPathKey + ".time"
	ErrParseComplexKey  = ParseErrorPathKey + ".complex"
	ErrParseListKey     = ParseErrorPathKey + ".list"
	ErrParseInt64Key    = ParseErrorPathKey + ".int64"
	ErrParseInt32Key    = ParseErrorPathKey + ".int32"
	ErrParseInt16Key    = ParseErrorPathKey + ".int16"
	ErrParseInt8Key     = ParseErrorPathKey + ".int8"
	ErrParseOverflowKey = ParseErrorPathKey + ".overflow"
	ErrParseFloat64Key  = ParseErrorPathKey + ".float64"
	ErrParseFloat32Key  = ParseErrorPathKey + ".float32"
	ErrParseUintKey     = ParseErrorPathKey + ".uint"
	ErrParseUint64Key   = ParseErrorPathKey + ".uint64"
	ErrParseUint32Key   = ParseErrorPathKey + ".uint32"
	ErrParseUint16Key   = ParseErrorPathKey + ".uint16"
	ErrParseUint8Key    = ParseErrorPathKey + ".uint8"
	ErrParseUintptrKey  = ParseErrorPathKey + ".uintptr"
)

// Warnings contains keys for warning messages
const (
	WarnDependencyNotSpecifiedKey      = WarningPrefixKey + ".dependency_not_specified"
	WarnDependencyValueNotSpecifiedKey = WarningPrefixKey + ".dependency_value_not_specified"
)

// UIMessages contains keys for user interface messages
const (
	MsgOptionalKey    = MessagePrefixKey + ".optional"
	MsgRequiredKey    = MessagePrefixKey + ".required"
	MsgConditionalKey = MessagePrefixKey + ".conditional"
	MsgDefaultsToKey  = MessagePrefixKey + ".defaults_to"
	MsgPositionalKey  = MessagePrefixKey + ".positional"
	MsgOrKey          = MessagePrefixKey + ".or"
)

// Help message keys - placeholders for future expansion
const (
	HelpUsageKey    = PrefixKey + ".help.usage"
	HelpCommandsKey = PrefixKey + ".help.commands"
	HelpFlagsKey    = PrefixKey + ".help.flags"
	HelpExamplesKey = PrefixKey + ".help.examples"
	HelpFooterKey   = PrefixKey + ".help.footer"
)
