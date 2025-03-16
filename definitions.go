package goopt

import (
	"io"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/i18n"
	"github.com/napalu/goopt/types"
	"github.com/napalu/goopt/types/orderedmap"
	"github.com/napalu/goopt/types/queue"
	"github.com/napalu/goopt/util"
	"golang.org/x/text/language"
)

type Bindable interface {
	~string | int8 | int16 | int32 | int64 | ~int | uint8 | uint16 | uint32 | uint64 | ~uint | float32 | float64 |
		bool | time.Time | time.Duration | []string | []int8 | []int16 | []int32 | []int64 | ~[]int | []uint8 | []uint16 | []uint32 |
		[]uint64 | ~[]uint | []float32 | []float64 | []bool | []time.Time | []time.Duration
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
	ToKebabCase = strcase.ToKebab

	// ToSnakeCase converts a string to snake case "my_command_name"
	ToSnakeCase = strcase.ToSnake

	// ToScreamingSnake converts a string to screaming snake case "MY_COMMAND_NAME"
	ToScreamingSnake = strcase.ToScreamingSnake

	// ToLowerCamel converts a string to lower camel case "myCommandName"
	ToLowerCamel = strcase.ToLowerCamel

	// ToLowerCase converts a string to lower case "mycommandname"
	ToLowerCase = strings.ToLower

	DefaultCommandNameConverter = ToLowerCase
	DefaultFlagNameConverter    = ToLowerCamel
)

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
	Position int       // Position in the command line
	ArgPos   int       // Position in the argument list
	Value    string    // The actual value
	Argument *Argument // Reference to the argument definition, if this was bound
}

// Command defines commands and sub-commands
type Command struct {
	Name           string
	NameKey        string
	Subcommands    []Command
	Callback       CommandFunc
	Description    string
	DescriptionKey string
	topLevel       bool
	path           string
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
	listFunc             types.ListDelimiterFunc
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
	secureArguments      *orderedmap.OrderedMap[string, *types.Secure]
	envNameConverter     NameConversionFunc
	commandNameConverter NameConversionFunc
	flagNameConverter    NameConversionFunc
	terminalReader       util.TerminalReader
	stderr               io.Writer
	stdout               io.Writer
	maxDependencyDepth   int
	i18n                 *i18n.Bundle
	userI18n             *i18n.Bundle
	currentLanguage      language.Tag
	renderer             Renderer
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

// Renderer is an interface for rendering Goopt structures
type Renderer interface {
	FlagName(f *Argument) string
	FlagDescription(f *Argument) string
	FlagUsage(f *Argument) string
	CommandName(c *Command) string
	CommandDescription(c *Command) string
	CommandUsage(c *Command) string
}

const (
	FmtErrorWithString = "%w: %s"
)

type commandCallback struct {
	callback  CommandFunc
	arguments []any
}

// DefaultMaxDependencyDepth is the default maximum depth for flag dependencies
const DefaultMaxDependencyDepth = 10
