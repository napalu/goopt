package errors

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var (
	// ErrBaseFileNotFound is returned when the base file cannot be found
	ErrBaseFileNotFound = i18n.NewError(messages.Keys.AppError.BaseFileNotFound)

	// ErrCommandFailed is returned when a command execution fails
	ErrCommandFailed = i18n.NewError(messages.Keys.AppError.CommandFailed)

	// ErrErrorProcessingFile is returned when there's an error processing a file
	ErrErrorProcessingFile = i18n.NewError(messages.Keys.AppError.ErrorProcessingFile)

	// ErrFailedToCreateDir is returned when directory creation fails
	ErrFailedToCreateDir = i18n.NewError(messages.Keys.AppError.FailedToCreateDir)

	// ErrFailedToCreateDirectory is returned when directory creation fails
	ErrFailedToCreateDirectory = i18n.NewError(messages.Keys.AppError.FailedToCreateDirectory)

	// ErrFailedToCreateFile is returned when file creation fails
	ErrFailedToCreateFile = i18n.NewError(messages.Keys.AppError.FailedToCreateFile)

	// ErrFailedToCreateOutputDir is returned when output directory creation fails
	ErrFailedToCreateOutputDir = i18n.NewError(messages.Keys.AppError.FailedToCreateOutputDir)

	// ErrFailedToExecuteTemplate is returned when template execution fails
	ErrFailedToExecuteTemplate = i18n.NewError(messages.Keys.AppError.FailedToExecuteTemplate)

	// ErrFailedToExpandFilePatterns is returned when expanding file patterns fails
	ErrFailedToExpandFilePatterns = i18n.NewError(messages.Keys.AppError.FailedToExpandFilePatterns)

	// ErrFailedToExpandInput is returned when expanding input fails
	ErrFailedToExpandInput = i18n.NewError(messages.Keys.AppError.FailedToExpandInput)

	// ErrFailedToExpandPattern is returned when expanding a pattern fails
	ErrFailedToExpandPattern = i18n.NewError(messages.Keys.AppError.FailedToExpandPattern)

	// ErrFailedToExpandReferencePatterns is returned when expanding reference patterns fails
	ErrFailedToExpandReferencePatterns = i18n.NewError(messages.Keys.AppError.FailedToExpandReferencePatterns)

	// ErrFailedToExpandTargetPatterns is returned when expanding target patterns fails
	ErrFailedToExpandTargetPatterns = i18n.NewError(messages.Keys.AppError.FailedToExpandTargetPatterns)

	// ErrFailedToGeneratePackage is returned when package generation fails
	ErrFailedToGeneratePackage = i18n.NewError(messages.Keys.AppError.FailedToGeneratePackage)

	// ErrFailedToGetConfig is returned when getting configuration fails
	ErrFailedToGetConfig = i18n.NewError(messages.Keys.AppError.FailedToGetConfig)

	// ErrFailedToLoadFile is returned when loading a file fails
	ErrFailedToLoadFile = i18n.NewError(messages.Keys.AppError.FailedToLoadFile)

	// ErrFailedToLoadReferenceFile is returned when loading a reference file fails
	ErrFailedToLoadReferenceFile = i18n.NewError(messages.Keys.AppError.FailedToLoadReferenceFile)

	// ErrFailedToLoadTargetFile is returned when loading a target file fails
	ErrFailedToLoadTargetFile = i18n.NewError(messages.Keys.AppError.FailedToLoadTargetFile)

	// ErrFailedToMarshal is returned when marshaling data fails
	ErrFailedToMarshal = i18n.NewError(messages.Keys.AppError.FailedToMarshal)

	// ErrFailedToMarshalJson is returned when marshaling to JSON fails
	ErrFailedToMarshalJson = i18n.NewError(messages.Keys.AppError.FailedToMarshalJson)

	// ErrFailedToParseFile is returned when parsing a file fails
	ErrFailedToParseFile = i18n.NewError(messages.Keys.AppError.FailedToParseFile)

	// ErrFailedToParseJson is returned when parsing JSON fails
	ErrFailedToParseJson = i18n.NewError(messages.Keys.AppError.FailedToParseJson)

	// ErrFailedToParseJsonSimple is returned when simple JSON parsing fails
	ErrFailedToParseJsonSimple = i18n.NewError(messages.Keys.AppError.FailedToParseJsonSimple)

	// ErrFailedToParseTemplate is returned when parsing a template fails
	ErrFailedToParseTemplate = i18n.NewError(messages.Keys.AppError.FailedToParseTemplate)

	// ErrFailedToPrepareInput is returned when preparing input fails
	ErrFailedToPrepareInput = i18n.NewError(messages.Keys.AppError.FailedToPrepareInput)

	// ErrFailedToReadFile is returned when reading a file fails
	ErrFailedToReadFile = i18n.NewError(messages.Keys.AppError.FailedToReadFile)

	// ErrFailedToReadInput is returned when reading input fails
	ErrFailedToReadInput = i18n.NewError(messages.Keys.AppError.FailedToReadInput)

	// ErrFailedToSaveFile is returned when saving a file fails
	ErrFailedToSaveFile = i18n.NewError(messages.Keys.AppError.FailedToSaveFile)

	// ErrFailedToTransformFile is returned when transforming a file fails
	ErrFailedToTransformFile = i18n.NewError(messages.Keys.AppError.FailedToTransformFile)

	// ErrFailedToUpdateFile is returned when updating a file fails
	ErrFailedToUpdateFile = i18n.NewError(messages.Keys.AppError.FailedToUpdateFile)

	// ErrFailedToWriteFile is returned when writing a file fails
	ErrFailedToWriteFile = i18n.NewError(messages.Keys.AppError.FailedToWriteFile)

	// ErrFailedToWriteJson is returned when writing JSON fails
	ErrFailedToWriteJson = i18n.NewError(messages.Keys.AppError.FailedToWriteJson)

	// ErrFailedToWriteOutput is returned when writing output fails
	ErrFailedToWriteOutput = i18n.NewError(messages.Keys.AppError.FailedToWriteOutput)

	// ErrGoModNotFound is returned when go.mod file is not found
	ErrGoModNotFound = i18n.NewError(messages.Keys.AppError.GoModNotFound)

	// ErrInvalidPattern is returned when a pattern is invalid
	ErrInvalidPattern = i18n.NewError(messages.Keys.AppError.InvalidPattern)

	// ErrKeyAlreadyExists is returned when a key already exists
	ErrKeyAlreadyExists = i18n.NewError(messages.Keys.AppError.KeyAlreadyExists)

	// ErrModuleDirectiveNotFound is returned when module directive is not found
	ErrModuleDirectiveNotFound = i18n.NewError(messages.Keys.AppError.ModuleDirectiveNotFound)

	// ErrNoFiles is returned when no files are found
	ErrNoFiles = i18n.NewError(messages.Keys.AppError.NoFiles)

	// ErrNoReferenceFiles is returned when no reference files are found
	ErrNoReferenceFiles = i18n.NewError(messages.Keys.AppError.NoReferenceFiles)

	// ErrNoTargetFiles is returned when no target files are found
	ErrNoTargetFiles = i18n.NewError(messages.Keys.AppError.NoTargetFiles)

	// ErrParseError is returned when a parse error occurs
	ErrParseError = i18n.NewError(messages.Keys.AppError.ParseError)

	// ErrSyncRequiresAtLeastTwoFiles is returned when sync command is called with less than 2 files
	ErrSyncRequiresAtLeastTwoFiles = i18n.NewError(messages.Keys.AppError.SyncRequiresAtLeastTwoFiles)

	// ErrUnknownLanguageCode is returned when an unknown language code is encountered
	ErrUnknownLanguageCode = i18n.NewError(messages.Keys.AppError.UnknownLanguageCode)

	// ErrValidationFailed is returned when validation fails
	ErrValidationFailed = i18n.NewError(messages.Keys.AppError.ValidationFailed)

	// Add command specific errors

	// ErrNoKeys is returned when no keys are specified for the add command
	ErrNoKeys = i18n.NewError(messages.Keys.AppAdd.NoKeys)

	// ErrBothSingleAndFile is returned when both single key and from-file options are specified
	ErrBothSingleAndFile = i18n.NewError(messages.Keys.AppAdd.BothSingleAndFile)

	// ErrMissingValue is returned when a key is specified without a value
	ErrMissingValue = i18n.NewError(messages.Keys.AppAdd.MissingValue)

	// ErrInvalidMode is returned when an invalid mode is specified
	ErrInvalidMode = i18n.NewError(messages.Keys.AppAdd.InvalidMode)

	// ErrFailedReadKeysFile is returned when failed to read keys file
	ErrFailedReadKeysFile = i18n.NewError(messages.Keys.AppAdd.FailedReadKeysFile)

	// ErrFailedParseKeysFile is returned when failed to parse keys file
	ErrFailedParseKeysFile = i18n.NewError(messages.Keys.AppAdd.FailedParseKeysFile)

	// ErrKeyExistsError is returned when a key already exists in error mode
	ErrKeyExistsError = i18n.NewError(messages.Keys.AppAdd.KeyExistsError)

	// Extract command specific errors

	// ErrInvalidRegex is returned when an invalid regex pattern is provided
	ErrInvalidRegex = i18n.NewError(messages.Keys.AppExtract.InvalidRegex)

	// ErrGlobError is returned when glob pattern expansion fails
	ErrGlobError = i18n.NewError(messages.Keys.AppExtract.GlobError)

	// ErrUpdateError is returned when file update fails
	ErrUpdateError = i18n.NewError(messages.Keys.AppExtract.UpdateError)

	// ErrFileError is returned when file operation fails
	ErrFileError = i18n.NewError(messages.Keys.AppExtract.FileError)
)
