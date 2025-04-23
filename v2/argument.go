package goopt

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/napalu/goopt/v2/types"
)

// Argument defines a command-line Flag
type Argument struct {
	NameKey        string
	Description    string
	DescriptionKey string
	TypeOf         types.OptionType
	Required       bool
	RequiredIf     RequiredIfFunc
	PreFilter      FilterFunc
	PostFilter     FilterFunc
	AcceptedValues []types.PatternValue
	DependencyMap  map[string][]string
	Secure         types.Secure
	Short          string
	DefaultValue   string
	Capacity       int // For slices, the capacity of the slice, ignored for other types
	Position       *int
	uuid           string
}

// NewArg convenience initialization method to configure flags
func NewArg(configs ...ConfigureArgumentFunc) *Argument {
	argument := &Argument{}
	for _, config := range configs {
		config(argument, nil)
	}
	argument.ensureInit()

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

func (a *Argument) ensureInit() {
	if a.AcceptedValues == nil {
		a.AcceptedValues = []types.PatternValue{}
	}
	if a.DependencyMap == nil {
		a.DependencyMap = map[string][]string{}
	}
	if a.uuid == "" {
		a.uuid = uuid.New().String()
	}
}

func (a *Argument) isPositional() bool {
	return a.Position != nil
}

func (a *Argument) GetLongName(parser *Parser) string {
	if parser == nil {
		return ""
	}

	if a.uuid != "" {
		return parser.lookup[a.uuid]
	}

	return ""
}

func (a *Argument) DisplayID() string {
	if a.Position != nil {
		return fmt.Sprintf("pos%d", *a.Position)
	}

	return fmt.Sprintf("%s-%s", a.uuid[:8], a.DescriptionKey)
}

// Equal compares two Argument variables for equality across their exported fields
func (a *Argument) Equal(other *Argument) bool {
	if a == nil || other == nil {
		return false
	}

	//nolint:SA1026 // Ignoring "unsupported type" warning as we only care about marshallable fields
	aj, _ := json.Marshal(a)
	//nolint:SA1026 // Ignoring "unsupported type" warning as we only care about marshallable fields
	oj, _ := json.Marshal(other)

	var am, om map[string]interface{}
	_ = json.Unmarshal(aj, &am)
	_ = json.Unmarshal(oj, &om)

	return reflect.DeepEqual(am, om)
}
