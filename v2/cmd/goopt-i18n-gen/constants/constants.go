package constants

// DefaultTODOPrefix is the default prefix used for untranslated entries
const DefaultTODOPrefix = "[TODO]"

// CommentTODO is the prefix used for TODO comments in source files
const CommentTODO = "i18n-todo"

// CommentDone is the prefix used for completed i18n comments
const CommentDone = "i18n-done"

// CommentSkip is the prefix used for strings to skip
const CommentSkip = "i18n-skip"

// GeneratedFileHeader is the header for generated files
const GeneratedFileHeader = "// Code generated by goopt-i18n-gen. DO NOT EDIT."

// DefaultPackageName is the default package name for generated files
const DefaultPackageName = "messages"

// DefaultKeyPrefix is the default prefix for generated keys
const DefaultKeyPrefix = "app"

// DefaultBackupDir is the default directory for backup files
const DefaultBackupDir = ".goopt-i18n-backup"

// DefaultMinStringLength is the default minimum string length for extraction
const DefaultMinStringLength = 2

// DefaultTranslatorPattern is the default translator pattern
const DefaultTranslatorPattern = "tr.T"
