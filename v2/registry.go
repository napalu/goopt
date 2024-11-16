package v2

import (
	"fmt"
	"github.com/napalu/goopt/v2/compare"
	"github.com/napalu/goopt/v2/types/orderedmap"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type Registry interface {
	AddFlag(flag string, argument *Argument, commandPath ...string) error
	AddCommand(cmd *Command) error
	SetFlagValue(flag string, value string, commandPath ...string) error
	GetFlagValue(flag string, commandPath ...string) (string, error)
	BindFlag(flag string, argument *Argument, bindPtr interface{}, commandPath ...string) error
	AddPositionalArg(argument PositionalArgument) error
	IsFlag(token string) bool
	IsCommand(token string) bool
	GetArgument(token string, commandPath ...string) (*FlagInfo, error)
	GetCommand(token string) (*Command, error)
}

type defaultRegistry struct {
	acceptedFlags      *orderedmap.OrderedMap[string, *FlagInfo]
	registeredCommands *orderedmap.OrderedMap[string, *Command]
	flagLookup         map[string]string
	optionValues       map[string]string
	rawArgs            map[string]string
	bind               map[string]any
	positionalArgs     []PositionalArgument
	prefixes           []rune
	posixCompatible    bool
}

func newDefaultRegistry(posix bool) Registry {
	dr := &defaultRegistry{
		acceptedFlags:      orderedmap.NewOrderedMap[string, *FlagInfo](),
		flagLookup:         make(map[string]string),
		registeredCommands: orderedmap.NewOrderedMap[string, *Command](),
		prefixes:           []rune{'-'},
		bind:               make(map[string]any),
		optionValues:       make(map[string]string),
		posixCompatible:    posix,
	}

	return dr
}

// AddFlag is used to define a Flag. A Flag represents a command line option
// with a "long" name and an optional "short" form prefixed by '-', '--' or '/'.
// This version supports both global flags and command-specific flags using the optional commandPath argument.
func (d *defaultRegistry) AddFlag(flag string, argument *Argument, commandPath ...string) error {
	argument.ensureInit()

	if flag == "" {
		return fmt.Errorf("can't set empty flag")
	}

	// Use the helper function to generate the lookup key
	lookupFlag := buildPathFlag(flag, commandPath...)

	// Ensure no duplicate flags for the same command path or globally
	if _, exists := d.acceptedFlags.Get(lookupFlag); exists {
		return fmt.Errorf("flag '%s' already exists for the given command path", lookupFlag)
	}

	if lenS := len(argument.Short); lenS > 0 {
		if d.posixCompatible && lenS > 1 {
			return fmt.Errorf("%w: flag %s has short form %s which is not posix compatible (length > 1)", ErrPosixIncompatible, flag, argument.Short)
		}

		// Check for short flag conflicts only for global flags
		if len(commandPath) == 0 { // Global flag
			if arg, exists := d.flagLookup[argument.Short]; exists {
				return fmt.Errorf("short flag '%s' on global flag %s already exists as %v", argument.Short, flag, arg)
			}
		}

		d.flagLookup[argument.Short] = flag
	}

	d.acceptedFlags.Set(lookupFlag, &FlagInfo{
		Argument:    argument,
		CommandPath: strings.Join(commandPath, " "), // Keep track of the command path
	})

	return nil
}

func (d *defaultRegistry) AddCommand(cmd *Command) error {
	// Validate the command hierarchy and ensure unique paths
	if ok, err := d.validateCommand(cmd, 0, 100); !ok {
		return err
	}

	// Add the command and all its subcommands to registeredCommands
	d.registerCommandRecursive(cmd)

	return nil
}

func (d *defaultRegistry) SetFlagValue(flag string, value string, commandPath ...string) error {
	mainKey := d.flagOrShortFlag(flag, commandPath...)
	key := ""
	_, found := d.optionValues[flag]
	if found {
		d.optionValues[mainKey] = value
		key = mainKey
	} else {
		d.optionValues[flag] = value
		key = flag
	}
	flagInfo, err := d.GetArgument(key)
	if err != nil {
		return err
	}

	if flagInfo.Argument.TypeOf == File {
		path := d.rawArgs[key]
		if path == "" {
			path = flagInfo.Argument.DefaultValue
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		err = os.WriteFile(abs, []byte(value), 0600)
		if err != nil {
			return err
		}

	}

	return nil
}

func (d *defaultRegistry) GetFlagValue(flag string, commandPath ...string) (string, error) {
	mainKey := d.flagOrShortFlag(flag, commandPath...)
	val, found := d.optionValues[mainKey]
	if !found {
		return "", fmt.Errorf("flag '%s' does not exist", flag)
	}

	return val, nil
}

func (d *defaultRegistry) BindFlag(flag string, argument *Argument, bindPtr interface{}, commandPath ...string) error {
	if bindPtr == nil {
		return fmt.Errorf("can't bind flag to nil CmdLine pointer")
	}
	if ok, err := canConvert(bindPtr, argument.TypeOf); !ok {
		return err
	}

	if reflect.ValueOf(bindPtr).Kind() != reflect.Ptr {
		return fmt.Errorf("BindFlag only accepts pointer types")
	}

	if err := d.AddFlag(flag, argument, commandPath...); err != nil {
		return err
	}

	lookupFlag := buildPathFlag(flag, commandPath...)

	// Bind the flag to the variable
	d.bind[lookupFlag] = bindPtr

	return nil
}

func (d *defaultRegistry) AddPositionalArg(argument PositionalArgument) error {
	if argument.Position < 0 || argument.Position >= len(d.rawArgs) {
		return fmt.Errorf("invalid position %d for argument %s", argument.Position, argument.Value)
	}

	d.positionalArgs = append(d.positionalArgs, argument)

	return nil
}

func (d *defaultRegistry) IsFlag(token string) bool {
	return compare.HasPrefix(token, d.prefixes)
}

func (d *defaultRegistry) IsCommand(token string) bool {
	_, ok := d.registeredCommands.Get(token)

	return ok
}

func (d *defaultRegistry) GetArgument(token string, commandPath ...string) (*FlagInfo, error) {
	mainKey := d.flagOrShortFlag(token, commandPath...)
	flagInfo, ok := d.acceptedFlags.Get(mainKey)
	if !ok {
		return nil, fmt.Errorf("no flag found for %s", mainKey)
	}

	flagInfo.MainKey = mainKey

	return flagInfo, nil
}

func (d *defaultRegistry) GetCommand(token string) (*Command, error) {
	if cmd, ok := d.registeredCommands.Get(token); ok {
		return cmd, nil
	} else {
		return nil, fmt.Errorf("command %s not found", token)
	}
}

func (d *defaultRegistry) validateCommand(cmdArg *Command, level, maxDepth int) (bool, error) {
	if level > maxDepth {
		return false, fmt.Errorf("max command depth of %d exceeded", maxDepth)
	}

	var commandType string
	if level > 0 {
		commandType = "sub-command"
	} else {
		commandType = "command"
	}
	if cmdArg.Name == "" {
		return false, fmt.Errorf("the 'Name' property is missing from %s on Level %d: %+v", commandType, level, cmdArg)
	}

	if level == 0 {
		cmdArg.Path = cmdArg.Name
	}

	for i := 0; i < len(cmdArg.Subcommands); i++ {
		cmdArg.Subcommands[i].Path = cmdArg.Path + " " + cmdArg.Subcommands[i].Name
		if ok, err := d.validateCommand(&cmdArg.Subcommands[i], level+1, maxDepth); err != nil {
			return ok, err
		}
	}

	return true, nil
}

func (d *defaultRegistry) registerCommandRecursive(cmd *Command) {
	// Add the current command to the map
	cmd.TopLevel = strings.Count(cmd.Path, " ") == 0
	d.registeredCommands.Set(cmd.Path, cmd)

	// Recursively register all subcommands
	for i := range cmd.Subcommands {
		subCmd := &cmd.Subcommands[i]
		d.registerCommandRecursive(subCmd)
	}

}

func (d *defaultRegistry) flagOrShortFlag(flag string, commandPath ...string) string {
	flag = strings.TrimLeftFunc(flag, func(r rune) bool {
		for _, s := range d.prefixes {
			if r == s {
				return true
			}
		}

		return false
	})
	pathFlag := buildPathFlag(flag, commandPath...)
	_, pathFound := d.acceptedFlags.Get(pathFlag)
	if !pathFound {
		globalFlag := splitPathFlag(flag)[0]
		_, found := d.acceptedFlags.Get(globalFlag)
		if found {
			return globalFlag
		}
		item, found := d.flagLookup[flag]
		if found {
			pathFlag = buildPathFlag(item, commandPath...)
			if _, found := d.acceptedFlags.Get(pathFlag); found {
				return pathFlag
			}
			return item
		}
	}

	return pathFlag
}
