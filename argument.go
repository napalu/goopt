package goopt

import (
	"fmt"
	"strings"

	"github.com/napalu/goopt/types"
)

// Argument defines a command-line Flag
type Argument struct {
	Description    string
	TypeOf         types.OptionType
	Required       bool
	RequiredIf     RequiredIfFunc
	PreFilter      FilterFunc
	PostFilter     FilterFunc
	AcceptedValues []types.PatternValue
	DependsOn      []string // Deprecated: use DependencyMap instead - will be removed in v2.0.0
	OfValue        []string // Deprecated: use DependencyMap instead - will be removed in v2.0.0
	DependencyMap  map[string][]string
	Secure         types.Secure
	Short          string
	DefaultValue   string
	Capacity       int // For slices, the capacity of the slice, ignored for other types
	Position       *int
}

// NewArgument convenience initialization method to describe Flags. Alternatively, Use NewArg to
// configure Argument using option functions.
func NewArgument(shortFlag string, description string, typeOf types.OptionType, required bool, secure types.Secure, defaultValue string) *Argument {
	return &Argument{
		Description:  description,
		TypeOf:       typeOf,
		Required:     required,
		DependsOn:    []string{},
		OfValue:      []string{},
		Secure:       secure,
		Short:        shortFlag,
		DefaultValue: defaultValue,
	}
}

// NewArg convenience initialization method to configure flags
func NewArg(configs ...ConfigureArgumentFunc) *Argument {
	argument := &Argument{}
	for _, config := range configs {
		config(argument, nil)
	}

	return argument
}

// Set configures the Argument instance with the provided ConfigureArgumentFunc(s),
// and returns an error if a configuration results in an error.
//
// Usage example:
//
//	arg := &Argument{}
//	err := arg.Set(
//	    WithDescription("example argument"),
//	    WithType(Standalone),
//	    SetRequired(true),
//	)
//	if err != nil {
//	    // handle error
//	}
func (a *Argument) Set(configs ...ConfigureArgumentFunc) error {
	a.ensureInit()
	var err error
	for _, config := range configs {
		config(a, &err)
		if err != nil {
			return err
		}
	}
	return nil
}

// String returns a string representation of the Argument instance
func (a *Argument) String() string {
	return strings.TrimLeft(fmt.Sprintf("%s %s %s", a.short(), a.description(), a.required()), " ")
}

func (a *Argument) ensureInit() {
	if a.DependsOn == nil {
		a.DependsOn = []string{}
	}
	if a.OfValue == nil {
		a.OfValue = []string{}
	}
	if a.AcceptedValues == nil {
		a.AcceptedValues = []types.PatternValue{}
	}
	if a.DependencyMap == nil {
		a.DependencyMap = map[string][]string{}
	}
}

func (a *Argument) default_() string {
	return a.DefaultValue
}

func (a *Argument) isPositional() bool {
	return a.Position != nil
}

func (a *Argument) short() string {
	if a.Short == "" {
		return ""
	}

	return "or -" + a.Short
}

func (a *Argument) required() string {
	requiredOrOptional := "optional"
	if a.Required {
		requiredOrOptional = "required"
	} else if a.RequiredIf != nil {
		requiredOrOptional = "conditional"
	}

	return "(" + requiredOrOptional + ")"
}

func (a *Argument) description() string {
	d := a.default_()
	if d != "" {
		return fmt.Sprintf("\"%s\" (defaults to: %s)", a.Description, d)

	}

	return fmt.Sprintf("\"%s\"", a.Description)
}
