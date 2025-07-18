// Package errs provides translatable error definitions for goopt.
// All errors use the global default bundle for translations, which allows
// adding new languages but ensures consistency across all parser instances.
package errs

import (
	"errors"
	"github.com/napalu/goopt/v2/i18n"
)

// Core parser errors
var (
	ErrUnsupportedType              = i18n.NewError(ErrUnsupportedTypeKey)
	ErrCommandNotFound              = i18n.NewError(ErrCommandNotFoundKey)
	ErrCommandNoCallback            = i18n.NewError(ErrCommandNoCallbackKey)
	ErrFlagNotFound                 = i18n.NewError(ErrFlagNotFoundKey)
	ErrPosixIncompatible            = i18n.NewError(ErrPosixIncompatibleKey)
	ErrValidationFailed             = i18n.NewError(ErrValidationFailedKey)
	ErrBindNil                      = i18n.NewError(ErrBindNilKey)
	ErrNonPointerVar                = i18n.NewError(ErrNonPointerVarKey)
	ErrRequiredFlag                 = i18n.NewError(ErrRequiredFlagKey)
	ErrRequiredPositionalFlag       = i18n.NewError(ErrRequiredPositionalFlagKey)
	ErrInvalidArgumentType          = i18n.NewError(ErrInvalidArgumentTypeKey)
	ErrFlagValueNotRetrieved        = i18n.NewError(ErrFlagValueNotRetrievedKey)
	ErrEmptyArgumentPrefixList      = i18n.NewError(ErrEmptyArgumentPrefixListKey)
	ErrEmptyFlag                    = i18n.NewError(ErrEmptyFlagKey)
	ErrFlagAlreadyExists            = i18n.NewError(ErrFlagAlreadyExistsKey)
	ErrFlagDoesNotExist             = i18n.NewError(ErrFlagDoesNotExistKey)
	ErrPosixShortForm               = i18n.NewError(ErrPosixShortFormKey)
	ErrShortFlagConflict            = i18n.NewError(ErrShortFlagConflictKey)
	ErrShortFlagConflictContext     = i18n.NewError(ErrShortFlagConflictKeyContext)
	ErrInvalidListDelimiterFunc     = i18n.NewError(ErrInvalidListDelimiterFuncKey)
	ErrBindInvalidValue             = i18n.NewError(ErrBindInvalidValueKey)
	ErrPointerExpected              = i18n.NewError(ErrPointerExpectedKey)
	ErrOptionNotSet                 = i18n.NewError(ErrOptionNotSetKey)
	ErrLanguageUnavailable          = i18n.NewError(ErrLanguageUnavailableKey)
	ErrShortFlagUndefined           = i18n.NewError(ErrShortFlagUndefinedKey)
	ErrDependencyOnEmptyFlag        = i18n.NewError(ErrDependencyOnEmptyFlagKey)
	ErrRemoveDependencyFromEmpty    = i18n.NewError(ErrRemoveDependencyFromEmptyKey)
	ErrSettingBoundValue            = i18n.NewError(ErrSettingBoundValueKey)
	ErrCommandCallbackError         = i18n.NewError(ErrCommandCallbackErrorKey)
	ErrNegativeCapacity             = i18n.NewError(ErrNegativeCapacityKey)
	ErrUnsupportedTypeConversion    = i18n.NewError(ErrUnsupportedTypeConversionKey)
	ErrNoPreValidationFilters       = i18n.NewError(ErrNoPreValidationFiltersKey)
	ErrNoPostValidationFilters      = i18n.NewError(ErrNoPostValidationFiltersKey)
	ErrEmptyCommandPath             = i18n.NewError(ErrEmptyCommandPathKey)
	ErrRecursionDepthExceeded       = i18n.NewError(ErrRecursionDepthExceededKey)
	ErrConfiguringParser            = i18n.NewError(ErrConfiguringParserKey)
	ErrFieldBinding                 = i18n.NewError(ErrFieldBindingKey)
	ErrUnwrappingValue              = i18n.NewError(ErrUnwrappingValueKey)
	ErrOnlyStructsCanBeTagged       = i18n.NewError(ErrOnlyStructsCanBeTaggedKey)
	ErrProcessingFieldWithPrefix    = i18n.NewError(ErrProcessingFieldWithPrefixKey)
	ErrProcessingField              = i18n.NewError(ErrProcessingFieldKey)
	ErrProcessingFlag               = i18n.NewError(ErrProcessingFlagKey)
	ErrProcessingSliceField         = i18n.NewError(ErrProcessingSliceFieldKey)
	ErrProcessingNestedStruct       = i18n.NewError(ErrProcessingNestedStructKey)
	ErrProcessingCommand            = i18n.NewError(ErrProcessingCommandKey)
	ErrUnmarshallingTag             = i18n.NewError(ErrUnmarshallingTagKey)
	ErrMissingPropertyOnLevel       = i18n.NewError(ErrMissingPropertyOnLevelKey)
	ErrNilPointer                   = i18n.NewError(ErrNilPointerKey)
	ErrNoValidTags                  = i18n.NewError(ErrNoValidTagsKey)
	ErrInvalidAttributeForType      = i18n.NewError(ErrInvalidAttributeForTypeKey)
	ErrNotFoundPathForFlag          = i18n.NewError(ErrNotFoundPathForFlagKey)
	ErrNotFilePathForFlag           = i18n.NewError(ErrNotFilePathForFlagKey)
	ErrFlagFileOperation            = i18n.NewError(ErrFlagFileOperationKey)
	ErrFlagExpectsValue             = i18n.NewError(ErrFlagExpectsValueKey)
	ErrCommandExpectsSubcommand     = i18n.NewError(ErrCommandExpectsSubcommandKey)
	ErrSecureFlagExpectsValue       = i18n.NewError(ErrSecureFlagExpectsValueKey)
	ErrInvalidArgument              = i18n.NewError(ErrInvalidArgumentKey)
	ErrNoValues                     = i18n.NewError(ErrNoValuesKey)
	ErrCircularDependency           = i18n.NewError(ErrCircularDependencyKey)
	ErrDependencyNotFound           = i18n.NewError(ErrDependencyNotFoundKey)
	ErrDependencyValueNotSpecified  = i18n.NewError(ErrDependencyValueNotSpecifiedKey)
	ErrMissingArgumentInfo          = i18n.NewError(ErrMissingArgumentInfoKey)
	ErrIndexOutOfBounds             = i18n.NewError(ErrIndexOutOfBoundsKey)
	ErrUnknownFlag                  = i18n.NewError(ErrUnknownFlagKey)
	ErrUnknownFlagWithSuggestions   = i18n.NewError(ErrUnknownFlagWithSuggestionsKey)
	ErrPositionMustBeNonNegative    = i18n.NewError(ErrPositionMustBeNonNegativeKey)
	ErrPositionalArgumentNotFound   = i18n.NewError(ErrPositionalArgumentNotFoundKey)
	ErrUnknownFlagInCommandPath     = i18n.NewError(ErrUnknownFlagInCommandPathKey)
	ErrInvalidTagFormat             = i18n.NewError(ErrInvalidTagFormatKey)
	ErrInvalidKind                  = i18n.NewError(ErrInvalidKindKey)
	ErrNotAttachedToTerminal        = i18n.NewError(ErrNotAttachedToTerminalKey)
	ErrCallbackOnNonTerminalCommand = i18n.NewError(ErrCallbackOnNonTerminalCommandKey)
)

// Parsing/validation errors
var (
	ErrParseBool              = i18n.NewError(ErrParseBoolKey)
	ErrParseInt               = i18n.NewError(ErrParseIntKey)
	ErrParseFloat             = i18n.NewError(ErrParseFloatKey)
	ErrRegexCompile           = i18n.NewError(ErrRegexCompileKey)
	ErrFileOperation          = i18n.NewError(ErrFileOperationKey)
	ErrParseDuration          = i18n.NewError(ErrParseDurationKey)
	ErrParseTime              = i18n.NewError(ErrParseTimeKey)
	ErrParseComplex           = i18n.NewError(ErrParseComplexKey)
	ErrParseList              = i18n.NewError(ErrParseListKey)
	ErrParseInt64             = i18n.NewError(ErrParseInt64Key)
	ErrParseInt32             = i18n.NewError(ErrParseInt32Key)
	ErrParseInt16             = i18n.NewError(ErrParseInt16Key)
	ErrParseInt8              = i18n.NewError(ErrParseInt8Key)
	ErrParseOverflow          = i18n.NewError(ErrParseOverflowKey)
	ErrParseFloat64           = i18n.NewError(ErrParseFloat64Key)
	ErrParseFloat32           = i18n.NewError(ErrParseFloat32Key)
	ErrParseUint              = i18n.NewError(ErrParseUintKey)
	ErrParseUint64            = i18n.NewError(ErrParseUint64Key)
	ErrParseUint32            = i18n.NewError(ErrParseUint32Key)
	ErrParseUint16            = i18n.NewError(ErrParseUint16Key)
	ErrParseUint8             = i18n.NewError(ErrParseUint8Key)
	ErrParseUintptr           = i18n.NewError(ErrParseUintptrKey)
	ErrWrapped                = i18n.NewError(ErrWrappedKey)
	ErrParseDuplicateFlag     = i18n.NewError(ErrParseDuplicateFlagKey)
	ErrParseEmptyInput        = i18n.NewError(ErrParseEmptyInputKey)
	ErrParseMalformedBraces   = i18n.NewError(ErrParseMalformedBracesKey)
	ErrParseUnmatchedBrackets = i18n.NewError(ErrParseUnmatchedBracketsKey)
	ErrParseInvalidFormat     = i18n.NewError(ErrParseInvalidFormatKey)
	ErrParseEmptyKey          = i18n.NewError(ErrParseEmptyKeyKey)
	ErrParseMissingValue      = i18n.NewError(ErrParseMissingValueKey)
	ErrParseNegativeIndex     = i18n.NewError(ErrParseNegativeIndexKey)

	// Validation errors
	ErrValidationCombinedFailed      = i18n.NewError(ErrValidationCombinedFailedKey)
	ErrValueMustBeNumber             = i18n.NewError(ErrValueMustBeNumberKey)
	ErrInvalidEmailFormat            = i18n.NewError(ErrInvalidEmailFormatKey)
	ErrInvalidURL                    = i18n.NewError(ErrInvalidURLKey)
	ErrURLSchemeMustBeOneOf          = i18n.NewError(ErrURLSchemeMustBeOneOfKey)
	ErrURLMustHaveHost               = i18n.NewError(ErrURLMustHaveHostKey)
	ErrMinLength                     = i18n.NewError(ErrMinLengthKey)
	ErrMaxLength                     = i18n.NewError(ErrMaxLengthKey)
	ErrExactLength                   = i18n.NewError(ErrExactLengthKey)
	ErrMinByteLength                 = i18n.NewError(ErrMinByteLengthKey)
	ErrMaxByteLength                 = i18n.NewError(ErrMaxByteLengthKey)
	ErrExactByteLength               = i18n.NewError(ErrExactByteLengthKey)
	ErrValueBetween                  = i18n.NewError(ErrValueBetweenKey)
	ErrValueAtLeast                  = i18n.NewError(ErrValueAtLeastKey)
	ErrValueAtMost                   = i18n.NewError(ErrValueAtMostKey)
	ErrPatternMatch                  = i18n.NewError(ErrPatternMatchKey)
	ErrValueMustBeOneOf              = i18n.NewError(ErrValueMustBeOneOfKey)
	ErrValueCannotBe                 = i18n.NewError(ErrValueCannotBeKey)
	ErrValueMustBeInteger            = i18n.NewError(ErrValueMustBeIntegerKey)
	ErrValueMustBeBoolean            = i18n.NewError(ErrValueMustBeBooleanKey)
	ErrValueMustBeAlphanumeric       = i18n.NewError(ErrValueMustBeAlphanumericKey)
	ErrValueMustBeIdentifier         = i18n.NewError(ErrValueMustBeIdentifierKey)
	ErrValueMustNotContainWhitespace = i18n.NewError(ErrValueMustNotContainWhitespaceKey)
	ErrFileMustHaveExtension         = i18n.NewError(ErrFileMustHaveExtensionKey)
	ErrHostnameTooLong               = i18n.NewError(ErrHostnameTooLongKey)
	ErrInvalidHostnameFormat         = i18n.NewError(ErrInvalidHostnameFormatKey)
	ErrInvalidIPv4Address            = i18n.NewError(ErrInvalidIPv4AddressKey)
	ErrValueMustBeValidIP            = i18n.NewError(ErrValueMustBeValidIPKey)

	// Validator parsing errors
	ErrInvalidValidator                    = i18n.NewError(ErrInvalidValidatorKey)
	ErrValidatorRequiresArgument           = i18n.NewError(ErrValidatorRequiresArgumentKey)
	ErrValidatorArgumentMustBeInteger      = i18n.NewError(ErrValidatorArgumentMustBeIntegerKey)
	ErrValidatorArgumentMustBeNumber       = i18n.NewError(ErrValidatorArgumentMustBeNumberKey)
	ErrValidatorRequiresAtLeastOneArgument = i18n.NewError(ErrValidatorRequiresAtLeastOneArgumentKey)
	ErrUnknownValidator                    = i18n.NewError(ErrUnknownValidatorKey)
	ErrValidatorArgumentCannotBeNegative   = i18n.NewError(ErrValidatorArgumentCannotBeNegativeKey)
	ErrValidatorRecursionDepthExceeded     = i18n.NewError(ErrValidatorRecursionDepthExceededKey)
	ErrValidatorMustUseParentheses         = i18n.NewError(ErrValidatorMustUseParenthesesKey)
)

func WrapOnce[T i18n.TranslatableError](err error, wrapper T, fieldOrFlags ...any) error {
	if err == nil {
		return nil
	}

	// Check if the outermost error is already the same type as wrapper
	if errors.Is(err, wrapper) {
		// Already wrapped with the same error type
		return err
	}

	// Convert []string to []interface{} for WithArgs
	args := make([]interface{}, len(fieldOrFlags))
	for i, v := range fieldOrFlags {
		args[i] = v
	}

	return wrapper.WithArgs(args...).Wrap(err)
}
