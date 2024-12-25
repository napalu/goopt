package goopt

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/types/orderedmap"
	"github.com/napalu/goopt/types/queue"
	"github.com/napalu/goopt/util"
)

type Bindable interface {
	~string | int8 | int16 | int32 | int64 | ~int | uint8 | uint16 | uint32 | uint64 | ~uint | float32 | float64 |
		bool | time.Time | []string | []int8 | []int16 | []int32 | []int64 | ~[]int | []uint8 | []uint16 | []uint32 |
		[]uint64 | ~[]uint | []float32 | []float64 | []bool | []time.Time
}

// PrettyPrintConfig is used to print the list of accepted commands as a tree in PrintCommandsUsing and PrintCommands
type PrettyPrintConfig struct {
	// NewCommandPrefix precedes the start of a new command
	NewCommandPrefix string
	// DefaultPrefix precedes sub-commands by default
	DefaultPrefix string
	// TerminalPrefix precedes terminal, i.e. Command structs which don't have sub-commands
	TerminalPrefix string
	// InnerLevelBindPrefix is used for indentation. The indentation is repeated for each Level under the
	//  command root. The Command root is at Level 0. Each sub-command increases root Level by 1.
	InnerLevelBindPrefix string
	// OuterLevelBindPrefix is used for indentation after InnerLevelBindPrefix has been rendered. The indentation is repeated for each Level under the
	//  command root. The Command root is at Level 0. Each sub-command increases root Level by 1.
	OuterLevelBindPrefix string
}

// RequiredIfFunc used to specify if an option is required when a particular Command or Flag is specified
type RequiredIfFunc func(cmdLine *Parser, optionName string) (bool, string)

// ListDelimiterFunc signature to match when supplying a user-defined function to check for the runes which form list delimiters.
// Defaults to ',' || r == '|' || r == ' '.
type ListDelimiterFunc func(matchOn rune) bool

// ConfigureCmdLineFunc is used when defining CommandLineOption options
type ConfigureCmdLineFunc func(cmdLine *Parser, err *error)

// ConfigureArgumentFunc is used when defining Flag arguments
type ConfigureArgumentFunc func(argument *Argument, err *error)

// ConfigureCommandFunc is used when defining Command options
type ConfigureCommandFunc func(command *Command)

// FilterFunc used to "filter" (change/evaluate) flag values - see AddFilter/GetPreValidationFilter/HasPreValidationFilter
type FilterFunc func(string) string

// CommandFunc callback - optionally specified as part of the Command structure gets called when matched on Parse()
type CommandFunc func(cmdLine *Parser, command *Command) error

// ValueSetFunc callback - optionally specified as part of the Argument structure to 'bind' variables to a Flag
// Used to set the value of a Flag to a custom structure.
type ValueSetFunc func(flag, value string, customStruct interface{})

// NameConversionFunc converts a field name to a command/flag name
type NameConversionFunc func(string) string

// Built-in conversion strategies
var (
	// ToKebabCase converts a string to kebab case "my-command-name"
	ToKebabCase = func(s string) string {
		return strcase.ToKebab(s)
	}

	// ToSnakeCase converts a string to snake case "my_command_name"
	ToSnakeCase = func(s string) string {
		// convert "MyCommandName" to "my_command_name"
		return strcase.ToSnake(s)
	}

	// ToScreamingSnake converts a string to screaming snake case "MY_COMMAND_NAME"
	ToScreamingSnake = func(s string) string {
		return strcase.ToScreamingSnake(s)
	}

	// ToLowerCamel converts a string to lower camel case "myCommandName"
	ToLowerCamel = func(s string) string {
		// convert "MyCommandName" to "myCommandName"
		return strcase.ToLowerCamel(s)
	}

	// ToLowerCase converts a string to lower case "mycommandname"
	ToLowerCase = func(s string) string {
		return strings.ToLower(s)
	}

	DefaultCommandNameConverter = ToLowerCase
	DefaultFlagNameConverter    = ToLowerCamel
	DefaultEnvNameConverter     = ToScreamingSnake
)

// OptionType used to define Flag types (such as Standalone, Single, Chained)
type OptionType int

const (
	// Single denotes a Flag accepting a string value
	Single OptionType = 0
	// Chained denotes a Flag accepting a string value which should be evaluated as a list (split on ' ', '|' and ',')
	Chained OptionType = 1
	// Standalone denotes a boolean Flag (does not accept a value)
	Standalone OptionType = 2
	// File denotes a Flag which is evaluated as a path (the content of the file is treated as the value)
	File OptionType = 3
	// Empty denotes a Flag which is not set - this is internally used to indicate that a Flag is not set
	Empty OptionType = 4
)

// PatternValue is used to define an acceptable value for a Flag. The 'pattern' argument is compiled to a regular expression
// and the description argument is used to provide a human-readable description of the pattern.
type PatternValue struct {
	Pattern     string
	Description string
	value       *regexp.Regexp
}

// ClearConfig allows to selectively clear a set of CmdLineOption configuration data
type ClearConfig struct {
	// KeepOptions: keep key/value options seen on command line
	KeepOptions bool
	// KeepErrors: keep errors generated during previous Parse
	KeepErrors bool
	// KeepAcceptedValues: keep value guards
	KeepAcceptedValues bool
	// KeepFilters: Keep filters set during previous configuration
	KeepFilters bool
	// KeepCommands: keep key/value commands seen on command line
	KeepCommands bool
	// KeepPositional: keep positional arguments seen on command line
	// a positional argument is defined as anything passed on the command-line
	// which was not processed as either a flag, a flag value, a command
	// or a command value
	KeepPositional bool
}

// PositionalArgument describes command-line arguments which were not matched as flags, flag values, command or command values.
type PositionalArgument struct {
	Position int
	Value    string
}

// KeyValue denotes Key Value option pairs (used in GetOptions)
type KeyValue struct {
	Key   string
	Value string
}

// Secure set to Secure to true to solicit non-echoed user input from stdin.
// If Prompt is empty a "password :" prompt will be displayed. Set to the desired value to override.
type Secure struct {
	IsSecure bool
	Prompt   string
}

// Argument defines a command-line Flag
type Argument struct {
	Description    string
	TypeOf         OptionType
	Required       bool
	RequiredIf     RequiredIfFunc
	PreFilter      FilterFunc
	PostFilter     FilterFunc
	AcceptedValues []PatternValue
	DependsOn      []string // Deprecated: use DependencyMap instead - will be removed in v2.0.0
	OfValue        []string // Deprecated: use DependencyMap instead - will be removed in v2.0.0
	DependencyMap  map[string][]string
	Secure         Secure
	Short          string
	DefaultValue   string
}

// Command defines commands and sub-commands
type Command struct {
	Name        string
	Subcommands []Command
	Callback    CommandFunc
	Description string
	TopLevel    bool
	Required    bool
	Path        string
}

// FlagInfo is used to store information about a flag
type FlagInfo struct {
	Argument    *Argument
	CommandPath string // The path of the command that owns this flag
}

// Parser opaque struct used in all Flag/Command manipulation
type Parser struct {
	posixCompatible      bool
	prefixes             []rune
	listFunc             ListDelimiterFunc
	acceptedFlags        *orderedmap.OrderedMap[string, *FlagInfo]
	lookup               map[string]string
	options              map[string]string
	errors               []error
	bind                 map[string]any
	customBind           map[string]ValueSetFunc
	registeredCommands   *orderedmap.OrderedMap[string, *Command]
	commandOptions       *orderedmap.OrderedMap[string, bool]
	positionalArgs       []PositionalArgument
	rawArgs              map[string]string
	callbackQueue        *queue.Q[commandCallback]
	callbackResults      map[string]error
	callbackOnParse      bool
	secureArguments      *orderedmap.OrderedMap[string, *Secure]
	envNameConverter     NameConversionFunc
	commandNameConverter NameConversionFunc
	flagNameConverter    NameConversionFunc
	terminalReader       util.TerminalReader
	stderr               io.Writer
	stdout               io.Writer
}

// CmdLineOption is an alias for Parser.
//
// Deprecated: Use Parser instead. This type will be removed in v2.0.0.
type CmdLineOption = Parser

// CompletionData is used to store information for command line completion
type CompletionData struct {
	Commands            []string                    // Available commands
	Flags               []string                    // Global flags
	CommandFlags        map[string][]string         // Flags per command
	Descriptions        map[string]string           // Descriptions for commands/flags
	FlagValues          map[string][]CompletionData // Values for flags
	CommandDescriptions map[string]string           // Descriptions specific to commands
}

var (
	ErrUnsupportedTypeConversion = errors.New("unsupported type conversion")
	ErrCommandNotFound           = errors.New("command not found")
	ErrFlagNotFound              = errors.New("flag not found")
	ErrPosixIncompatible         = errors.New("posix incompatible")
	ErrValidationFailed          = errors.New("validation failed")
	ErrBindNilPointer            = errors.New("can't bind flag to nil")
	ErrVariableNotAPointer       = errors.New("variable is not a pointer")
)

const (
	FmtErrorWithString = "%w: %s"
)

type commandCallback struct {
	callback  CommandFunc
	arguments []any
}

type kind string

const (
	kindFlag    kind = "flag"
	kindCommand kind = "command"
	kindEmpty   kind = ""
)

type tagConfig struct {
	kind           kind
	name           string
	short          string
	typeOf         OptionType
	description    string
	default_       string
	required       bool
	secure         Secure
	path           string
	acceptedValues []PatternValue
	dependsOn      map[string][]string
}
