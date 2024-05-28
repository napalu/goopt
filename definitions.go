package goopt

import (
	"errors"
	"github.com/ef-ds/deque"
	"github.com/napalu/goopt/types/orderedmap"
	"regexp"
	"time"
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
	// LevelBindPrefix is used for indentation. The indentation is repeated for each Level under the
	//  command root. The Command root is at Level 0. Each sub-command increases root Level by 1.
	LevelBindPrefix string
}

// RequiredIfFunc used to specify if an option is required when a particular Command or Flag is specified
type RequiredIfFunc func(cmdLine *CmdLineOption, optionName string) (bool, string)

// ListDelimiterFunc signature to match when supplying a user-defined function to check for the runes which form list delimiters.
// Defaults to ',' || r == '|' || r == ' '.
type ListDelimiterFunc func(matchOn rune) bool

// ConfigureCmdLineFunc is used to enable a fluent interface when defining options
type ConfigureCmdLineFunc func(cmdLine *CmdLineOption, err *error)

// ConfigureArgumentFunc is used to enable a fluent interface when defining arguments
type ConfigureArgumentFunc func(argument *Argument, err *error)

// ConfigureCommandFunc is used to enable a fluent interface when defining commands
type ConfigureCommandFunc func(command *Command)

// FilterFunc used to "filter" (change/evaluate) flag values - see AddFilter/GetPreValidationFilter/HasPreValidationFilter
type FilterFunc func(string) string

// CommandFunc callback - optionally specified as part of the Command structure gets called when matched on Parse()
type CommandFunc func(cmdLine *CmdLineOption, command *Command, value string) error

// ValueSetFunc callback - optionally specified as part of the Argument structure to 'bind' variables to a Flag
// Used to set the value of a Flag to a custom structure.
type ValueSetFunc func(flag, value string, customStruct interface{})

// OptionType used to define Flag types (such as Standalone, Single, Chained)
type OptionType int

const (
	// Single denotes a Flag accepting a string value
	Single OptionType = 0
	// Chained denotes a Flag accepting a string value which should be evaluated as a list (split on ' ', '|' and ',')
	Chained OptionType = 1
	// Standalone denotes a boolean Flag (does not accept a value)
	Standalone OptionType = 2
)

type PatternValue struct {
	Pattern     string
	Description string
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

// PathValue denotes Path/value Command pairs where the Path represents the keys of all Command / sub-command
// at which a value is stored
// Example:
//
//	in the structure Command{Name : "Test", Subcommands: []Command{{Name: "User"}}}
//	the Path to User would consist of "Test User"
type PathValue struct {
	Path  string
	Value string
}

// LiterateRegex used to provide human descriptions of regular expression
type LiterateRegex struct {
	value   *regexp.Regexp
	explain string
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
	AcceptedValues []LiterateRegex
	DependsOn      []string
	OfValue        []string
	Secure         Secure
	Short          string
	DefaultValue   string
}

// Command defines commands and sub-commands
type Command struct {
	Name         string
	Subcommands  []Command
	Callback     CommandFunc
	Description  string
	DefaultValue string
	Required     bool
	path         string
}

// CmdLineOption opaque struct used in all Flag/Command manipulation
type CmdLineOption struct {
	posixCompatible bool
	prefixes        []rune
	listFunc        ListDelimiterFunc
	acceptedFlags   *orderedmap.OrderedMap[string, *Argument]
	lookup          map[string]string
	options         map[string]string
	errors          []error
	bind            map[string]any
	customBind      map[string]ValueSetFunc
	//registeredCommands map[string]Command
	registeredCommands *orderedmap.OrderedMap[string, Command]
	commandOptions     *orderedmap.OrderedMap[string, path]
	positionalArgs     []PositionalArgument
	rawArgs            map[string]bool
	callbackQueue      *deque.Deque
	callbackResults    map[string]error
	secureArguments    *orderedmap.OrderedMap[string, *Secure]
}

var (
	ErrUnsupportedTypeConversion = errors.New("unsupported type conversion")
	ErrCommandNotFound           = errors.New("command not found")
	ErrFlagNotFound              = errors.New("flag not found")
	ErrPosixIncompatible         = errors.New("posix incompatible")
	ErrValidationFailed          = errors.New("validation failed")
)

type path struct {
	value         string
	isTerminating bool
}

type commandCallback struct {
	callback  CommandFunc
	arguments []any
}

type parseState struct {
	endOf int
	skip  int
	pos   int
}
