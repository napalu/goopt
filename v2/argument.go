package goopt

import (
	"fmt"
	"reflect"

	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/validation"

	"github.com/napalu/goopt/v2/types"
)

// Argument defines a command-line Flag
type Argument struct {
	NameKey        string
	Description    string
	DescriptionKey string
	TypeOf         types.OptionType
	IsHelp         bool
	Required       bool
	RequiredIf     RequiredIfFunc
	PreFilter      FilterFunc
	PostFilter     FilterFunc
	Validators     []validation.ValidatorFunc
	AcceptedValues []types.PatternValue
	DependencyMap  map[string][]string
	Secure         types.Secure
	Short          string
	DefaultValue   string
	Capacity       int // For slices, the capacity of the slice, ignored for other types
	Position       *int
	uniqueID       string
}

// NewArg convenience initialization method to configure flags.
// Note: This function ignores configuration errors for backward compatibility.
// Use NewArgE if you need error handling.
func NewArg(configs ...ConfigureArgumentFunc) *Argument {
	argument := &Argument{}
	for _, config := range configs {
		config(argument, nil)
	}
	argument.ensureInit()

	return argument
}

// NewArgE creates a new Argument with error handling.
// Returns an error if any configuration function fails.
func NewArgE(configs ...ConfigureArgumentFunc) (*Argument, error) {
	argument := &Argument{}
	var err error
	for _, config := range configs {
		config(argument, &err)
		if err != nil {
			return nil, err
		}
	}
	argument.ensureInit()

	return argument, nil
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

func (a *Argument) ensureInit() {
	if a.AcceptedValues == nil {
		a.AcceptedValues = []types.PatternValue{}
	}
	if a.DependencyMap == nil {
		a.DependencyMap = map[string][]string{}
	}
	if a.Validators == nil {
		a.Validators = []validation.ValidatorFunc{}
	}
	if a.uniqueID == "" {
		a.uniqueID = util.UniqueID("arg")
	}
}

func (a *Argument) isPositional() bool {
	return a.Position != nil
}

func (a *Argument) GetLongName(parser *Parser) string {
	if parser == nil {
		return ""
	}

	if a.uniqueID != "" {
		return parser.lookup[a.uniqueID]
	}

	return ""
}

func (a *Argument) DisplayID() string {
	if a.Position != nil {
		return fmt.Sprintf("pos%d", *a.Position)
	}

	return fmt.Sprintf("%s-%s", a.uniqueID[:8], a.DescriptionKey)
}

type comparableArgument struct {
	NameKey        string
	Description    string
	DescriptionKey string
	TypeOf         types.OptionType
	IsHelp         bool
	Required       bool
	Validators     []validation.ValidatorFunc
	AcceptedValues []types.PatternValue
	DependencyMap  map[string][]string
	Secure         types.Secure
	Short          string
	DefaultValue   string
	Capacity       int // For slices, the capacity of the slice, ignored for other types
	Position       *int
}

func toComparable(a *Argument) comparableArgument {
	return comparableArgument{
		NameKey:        a.NameKey,
		Description:    a.Description,
		DescriptionKey: a.DescriptionKey,
		TypeOf:         a.TypeOf,
		IsHelp:         a.IsHelp,
		Required:       a.Required,
		Validators:     normalizeSlice(a.Validators),
		AcceptedValues: normalizeSlice(a.AcceptedValues),
		DependencyMap:  normalizeMap(a.DependencyMap),
		Secure:         a.Secure,
		Short:          a.Short,
		DefaultValue:   a.DefaultValue,
		Capacity:       a.Capacity,
		Position:       normalizePosition(a.Position),
	}
}

func normalizeSlice[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func normalizeMap(m map[string][]string) map[string][]string {
	if m == nil {
		return map[string][]string{}
	}
	return m
}

func normalizePosition(p *int) *int {
	if p == nil {
		i := 0
		return &i
	}
	return p
}

// Equal compares two Argument variables for equality across their exported fields
func (a *Argument) Equal(other *Argument) bool {
	if a == nil || other == nil {
		return false
	}

	ca := toComparable(a)
	cb := toComparable(other)

	return reflect.DeepEqual(ca, cb)
}
