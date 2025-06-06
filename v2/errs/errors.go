package errs

import (
	"github.com/napalu/goopt/v2/i18n"
	"sync"
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
	ErrCircularDependency           = i18n.NewError(ErrCircularDependencyKey)
	ErrDependencyNotFound           = i18n.NewError(ErrDependencyNotFoundKey)
	ErrDependencyValueNotSpecified  = i18n.NewError(ErrDependencyValueNotSpecifiedKey)
	ErrMissingArgumentInfo          = i18n.NewError(ErrMissingArgumentInfoKey)
	ErrIndexOutOfBounds             = i18n.NewError(ErrIndexOutOfBoundsKey)
	ErrUnknownFlag                  = i18n.NewError(ErrUnknownFlagKey)
	ErrPositionMustBeNonNegative    = i18n.NewError(ErrPositionMustBeNonNegativeKey)
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
)

type builtInErrors struct {
	mu  sync.Mutex
	All []i18n.TranslatableError
}

var sysErrors = &builtInErrors{
	All: []i18n.TranslatableError{
		ErrUnsupportedType,
		ErrCommandNotFound,
		ErrCommandNoCallback,
		ErrFlagNotFound,
		ErrPosixIncompatible,
		ErrValidationFailed,
		ErrBindNil,
		ErrNonPointerVar,
		ErrRequiredFlag,
		ErrRequiredPositionalFlag,
		ErrInvalidArgumentType,
		ErrFlagValueNotRetrieved,
		ErrEmptyArgumentPrefixList,
		ErrEmptyFlag,
		ErrFlagAlreadyExists,
		ErrPosixShortForm,
		ErrShortFlagConflict,
		ErrInvalidListDelimiterFunc,
		ErrBindInvalidValue,
		ErrPointerExpected,
		ErrOptionNotSet,
		ErrLanguageUnavailable,
		ErrShortFlagUndefined,
		ErrDependencyOnEmptyFlag,
		ErrRemoveDependencyFromEmpty,
		ErrSettingBoundValue,
		ErrCommandCallbackError,
		ErrNegativeCapacity,
		ErrUnsupportedTypeConversion,
		ErrNoPreValidationFilters,
		ErrNoPostValidationFilters,
		ErrEmptyCommandPath,
		ErrRecursionDepthExceeded,
		ErrConfiguringParser,
		ErrFieldBinding,
		ErrUnwrappingValue,
		ErrOnlyStructsCanBeTagged,
		ErrProcessingFieldWithPrefix,
		ErrProcessingField,
		ErrProcessingFlag,
		ErrProcessingSliceField,
		ErrProcessingNestedStruct,
		ErrProcessingCommand,
		ErrUnmarshallingTag,
		ErrMissingPropertyOnLevel,
		ErrNilPointer,
		ErrNoValidTags,
		ErrInvalidAttributeForType,
		ErrNotFoundPathForFlag,
		ErrNotFilePathForFlag,
		ErrFlagFileOperation,
		ErrFlagExpectsValue,
		ErrCommandExpectsSubcommand,
		ErrSecureFlagExpectsValue,
		ErrInvalidArgument,
		ErrCircularDependency,
		ErrDependencyNotFound,
		ErrDependencyValueNotSpecified,
		ErrMissingArgumentInfo,
		ErrIndexOutOfBounds,
		ErrUnknownFlag,
		ErrPositionMustBeNonNegative,
		ErrUnknownFlagInCommandPath,
		ErrInvalidTagFormat,
		ErrInvalidKind,
		ErrNotAttachedToTerminal,
		ErrParseBool,
		ErrParseInt,
		ErrParseFloat,
		ErrRegexCompile,
		ErrFileOperation,
		ErrParseDuration,
		ErrParseTime,
		ErrParseComplex,
		ErrParseList,
		ErrParseInt64,
		ErrParseInt32,
		ErrParseInt16,
		ErrParseInt8,
		ErrParseOverflow,
		ErrParseFloat64,
		ErrParseFloat32,
		ErrParseUint,
		ErrParseUint64,
		ErrParseUint32,
		ErrParseUint16,
		ErrParseUint8,
		ErrParseUintptr,
		ErrWrapped,
		ErrParseDuplicateFlag,
		ErrParseEmptyInput,
		ErrParseMalformedBraces,
		ErrParseUnmatchedBrackets,
		ErrParseInvalidFormat,
		ErrParseEmptyKey,
		ErrParseMissingValue,
		ErrParseNegativeIndex,
	},
}

// UpdateMessageProvider updates the default message provider for all built-in errors.
// This is useful for setting a custom message provider for all built-in errors.
//
// Example:
//
//	provider := i18n.NewBundleMessageProvider(bundle)
//	errs.UpdateMessageProvider(provider)
//
//	// Now all built-in errors will use the custom message provider.
func UpdateMessageProvider(provider i18n.MessageProvider) {
	i18n.SetDefaultMessageProvider(provider)
	sysErrors.mu.Lock()
	for _, e := range sysErrors.All {
		e.SetProvider(provider)
	}
	sysErrors.mu.Unlock()
}
