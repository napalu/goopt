package goopt

import (
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/napalu/goopt/v2/env"

	"github.com/napalu/goopt/v2/input"

	"github.com/iancoleman/strcase"
	"github.com/napalu/goopt/v2/i18n"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/types/orderedmap"
	"github.com/napalu/goopt/v2/types/queue"
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

// PositionalArgument describes command-line arguments which were not matched as flags, flag values, command or command values.
type PositionalArgument struct {
	Position int       // Position in the command line
	ArgPos   int       // Position in the argument list
	Value    string    // The actual value
	Argument *Argument // Reference to the argument definition, if this was bound
}

// Command defines commands and sub-commands
type Command struct {
	Name             string
	NameKey          string
	Subcommands      []Command
	Callback         CommandFunc
	ExecOnParse      bool
	Description      string
	DescriptionKey   string
	topLevel         bool
	path             string
	callbackLocation reflect.Value // stores reference to a field which may contain a CommandFunc in the future
}

// FlagInfo is used to store information about a flag
type FlagInfo struct {
	Argument    *Argument
	CommandPath string // The path of the command that owns this flag
}

// HelpBehavior defines when help should go to stdout vs stderr
type HelpBehavior int

const (
	HelpBehaviorStdout HelpBehavior = iota // Always use stdout (default)
	HelpBehaviorSmart                      // stdout for --help, stderr for errors
	HelpBehaviorStderr                     // Always use stderr
)

// HelpStyle defines different help output formats
type HelpStyle int

const (
	HelpStyleFlat         HelpStyle = iota // PrintUsage
	HelpStyleGrouped                       // PrintUsageWithGroups
	HelpStyleGroupedClean                  // PrintUsageWithGroups, clean (no ** markers)
	HelpStyleCompact                       // Deduplicated, minimal
	HelpStyleHierarchical                  // Command-focused, drill-down
	HelpStyleSmart                         // Auto-detect based on CLI size
)

// HelpConfig allows customization of help output (applies to "raw" --help outpout)
type HelpConfig struct {
	Style            HelpStyle
	ShowDefaults     bool
	ShowShortFlags   bool
	ShowRequired     bool
	ShowDescription  bool
	MaxGlobals       int
	MaxWidth         int
	GroupSharedFlags bool
	CompactThreshold int // Number of flags before switching to compact mode
}

// DefaultHelpConfig provides sensible defaults
var DefaultHelpConfig = HelpConfig{
	Style:            HelpStyleSmart,
	ShowDefaults:     true, // Show default values by default
	ShowShortFlags:   true,
	ShowRequired:     true,
	ShowDescription:  true, // Show descriptions by default (essential for help!)
	MaxWidth:         80,
	MaxGlobals:       15,
	GroupSharedFlags: true,
	CompactThreshold: 20,
}

// Parser opaque struct used in all Flag/Command manipulation
type Parser struct {
	posixCompatible           bool
	prefixes                  []rune
	listFunc                  types.ListDelimiterFunc
	acceptedFlags             *orderedmap.OrderedMap[string, *FlagInfo]
	lookup                    map[string]string
	options                   map[string]string
	errors                    []error
	bind                      map[string]any
	customBind                map[string]ValueSetFunc
	registeredCommands        *orderedmap.OrderedMap[string, *Command]
	commandOptions            *orderedmap.OrderedMap[string, bool]
	positionalArgs            []PositionalArgument
	rawArgs                   map[string]string
	repeatedFlags             map[string]bool
	callbackQueue             *queue.Q[*Command]
	callbackResults           map[string]error
	callbackOnParse           bool // *during* parse process
	callbackOnParseComplete   bool // *after* parse process
	secureArguments           *orderedmap.OrderedMap[string, *types.Secure]
	envNameConverter          NameConversionFunc
	commandNameConverter      NameConversionFunc
	flagNameConverter         NameConversionFunc
	terminalReader            input.TerminalReader
	stderr                    io.Writer
	stdout                    io.Writer
	maxDependencyDepth        int
	defaultBundle             *i18n.Bundle // Immutable default bundle
	systemBundle              *i18n.Bundle // Parser-specific overrides
	userI18n                  *i18n.Bundle // User-provided bundle
	layeredProvider           *i18n.LayeredMessageProvider
	renderer                  Renderer
	structCtx                 any
	suggestionsFormatter      SuggestionsFormatter
	helpConfig                HelpConfig
	prettyPrintConfig         *PrettyPrintConfig
	helpBehavior              HelpBehavior
	autoHelp                  bool
	helpFlags                 []string
	helpExecuted              bool
	helpEndFunc               EndShowHelpHookFunc
	autoRegisteredHelp        map[string]bool
	version                   string
	versionFunc               func() string
	versionFormatter          func(string) string
	versionFlags              []string
	autoVersion               bool
	showVersionInHelp         bool
	versionExecuted           bool
	autoRegisteredVersion     map[string]bool
	autoLanguage              bool
	checkSystemLocale         bool
	languageEnvVar            string
	languageFlags             []string
	autoRegisteredLanguage    map[string]bool
	globalPreHooks            []PreHookFunc
	globalPostHooks           []PostHookFunc
	commandPreHooks           map[string][]PreHookFunc
	commandPostHooks          map[string][]PostHookFunc
	envResolver               env.Resolver
	hookOrder                 HookOrder
	validationHook            ValidationHookFunc
	translationRegistry       *JITTranslationRegistry
	flagSuggestionThreshold   int  // Maximum Levenshtein distance for flag suggestions (default: 2)
	cmdSuggestionThreshold    int  // Maximum Levenshtein distance for command suggestions (default: 2)
	allowUnknownFlags         bool // If true, don't generate errors for unknown flags
	treatUnknownAsPositionals bool // If true, treat unknown flags and their values as positionals
	mu                        sync.Mutex
	envVarPrefix              string // Prefix for environment variables
}

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
	PositionalUsage(f *Argument, position int) string
	CommandName(c *Command) string
	CommandDescription(c *Command) string
	CommandUsage(c *Command) string
}

// DefaultMaxDependencyDepth is the default maximum depth for flag dependencies
const DefaultMaxDependencyDepth = 10

// PreHookFunc is called before command execution
type PreHookFunc func(p *Parser, cmd *Command) error

// PostHookFunc is called after command execution
type PostHookFunc func(p *Parser, cmd *Command, cmdErr error) error

type EndShowHelpHookFunc func() error

// SuggestionsFormatter formats suggestions for display in error messages
type SuggestionsFormatter func(suggestions []string) string

// HookOrder defines the order in which hooks are executed
type HookOrder int

const (
	// OrderGlobalFirst executes global hooks before command-specific hooks
	OrderGlobalFirst HookOrder = iota
	// OrderCommandFirst executes command-specific hooks before global hooks
	OrderCommandFirst
)

type ValidationHookFunc func(*Parser) error
