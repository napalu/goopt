package parse

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/napalu/goopt/v2/internal/util"
	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestParser_InferFieldType(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected types.OptionType
	}{
		{
			name: "struct field bool",
			input: reflect.StructField{
				Name: "TestBool",
				Type: reflect.TypeOf(bool(false)),
			},
			expected: types.Standalone,
		},
		{
			name:     "reflect type bool",
			input:    reflect.TypeOf(bool(false)),
			expected: types.Standalone,
		},
		{
			name: "struct field string slice",
			input: reflect.StructField{
				Name: "TestStrings",
				Type: reflect.TypeOf([]string{}),
			},
			expected: types.Chained,
		},
		{
			name:     "reflect type string slice",
			input:    reflect.TypeOf([]string{}),
			expected: types.Chained,
		},
		{
			name: "struct field time.Duration",
			input: reflect.StructField{
				Name: "TestDuration",
				Type: reflect.TypeOf(time.Duration(0)),
			},
			expected: types.Single,
		},
		{
			name:     "reflect type time.Duration",
			input:    reflect.TypeOf(time.Duration(0)),
			expected: types.Single,
		},
		{
			name: "struct field time.Time",
			input: reflect.StructField{
				Name: "TestTime",
				Type: reflect.TypeOf(time.Time{}),
			},
			expected: types.Single,
		},
		{
			name:     "reflect type time.Time",
			input:    reflect.TypeOf(time.Time{}),
			expected: types.Single,
		},
		{
			name:     "nil type",
			input:    nil,
			expected: types.Empty,
		},
		{
			name:     "unsupported type",
			input:    reflect.TypeOf(struct{}{}),
			expected: types.Empty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InferFieldType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnmarshalTagFormat_Capacity(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		field   reflect.StructField
		want    *types.TagConfig
		wantErr bool
	}{
		{
			name: "valid capacity",
			tag:  "name:items;capacity:5",
			field: reflect.StructField{
				Name: "Items",
				Type: reflect.TypeOf([]string{}),
			},
			want: &types.TagConfig{
				Name:     "items",
				Capacity: 5,
				Kind:     types.KindFlag,
				TypeOf:   types.Chained,
			},
		},
		{
			name: "zero capacity",
			tag:  "name:items;capacity:0",
			field: reflect.StructField{
				Name: "Items",
				Type: reflect.TypeOf([]string{}),
			},
			want: &types.TagConfig{
				Name:     "items",
				Capacity: 0,
				Kind:     types.KindFlag,
				TypeOf:   types.Chained,
			},
		},
		{
			name: "negative capacity",
			tag:  "name:items;capacity:-1",
			field: reflect.StructField{
				Name: "Items",
				Type: reflect.TypeOf([]string{}),
			},
			wantErr: true,
		},
		{
			name: "invalid capacity",
			tag:  "name:items;capacity:abc",
			field: reflect.StructField{
				Name: "Items",
				Type: reflect.TypeOf([]string{}),
			},
			wantErr: true,
		},
		{
			name: "with other tags",
			tag:  "name:items;capacity:3;required:true",
			field: reflect.StructField{
				Name: "Items",
				Type: reflect.TypeOf([]string{}),
			},
			want: &types.TagConfig{
				Name:     "items",
				Required: true,
				Capacity: 3,
				Kind:     types.KindFlag,
				TypeOf:   types.Chained,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalTagFormat(tt.tag, tt.field)

			if tt.wantErr {
				if err == nil {
					t.Error("UnmarshalTagFormat() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Errorf("UnmarshalTagFormat() error = %v", err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalTagFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalTagFormat_Position(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		want    *types.TagConfig
		wantErr bool
	}{
		{
			name: "position with other tags",
			tag:  "pos:0;required:true",
			want: &types.TagConfig{
				Kind:     types.KindFlag,
				Position: util.NewOfType(0),
				Required: true,
			},
		},
		{
			name: "legacy position format",
			tag:  "pos:{idx:0}",
			want: &types.TagConfig{
				Kind:     types.KindFlag,
				Position: util.NewOfType(0),
			},
		},
		{
			name: "multiple position tags",
			tag:  "pos:0;pos:1", // Last one wins
			want: &types.TagConfig{
				Kind:     types.KindFlag,
				Position: util.NewOfType(1),
			},
		},
		{
			name:    "invalid position",
			tag:     "pos:-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalTagFormat(tt.tag, reflect.StructField{})
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalTagFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalTagFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyUnmarshalTagFormat_SupportedTags(t *testing.T) {
	tests := []struct {
		name    string
		field   reflect.StructField
		want    *types.TagConfig
		wantErr bool
	}{
		{
			name: "all supported tags",
			field: reflect.StructField{
				Name: "Config",
				Type: reflect.TypeOf(""),
				Tag: reflect.StructTag(
					`long:"config" short:"c" description:"config file" ` +
						`type:"file" default:"/etc/config" required:"true" ` +
						`secure:"true" prompt:"Enter config path" path:"config" ` +
						`accepted:"{pattern:json|yaml,desc:Format type}" ` +
						`depends:"{flag:output,values:[json,yaml]}"`,
				),
			},
			want: &types.TagConfig{
				Name:        "config",
				Short:       "c",
				Description: "config file",
				TypeOf:      types.File,
				Default:     "/etc/config",
				Required:    true,
				Secure:      types.Secure{IsSecure: true, Prompt: "Enter config path"},
				Path:        "config",
				AcceptedValues: []types.PatternValue{
					{Pattern: "json|yaml", Description: "Format type", Compiled: regexp.MustCompile("json|yaml")},
				},
				DependsOn: map[string][]string{
					"output": {"json", "yaml"},
				},
				Kind: types.KindFlag,
			},
		},
		{
			name: "minimal tags",
			field: reflect.StructField{
				Name: "Verbose",
				Type: reflect.TypeOf(false),
				Tag:  reflect.StructTag(`long:"verbose"`),
			},
			want: &types.TagConfig{
				Name:   "verbose",
				Kind:   types.KindFlag,
				TypeOf: types.Standalone,
			},
		},
		{
			name: "chained type with description",
			field: reflect.StructField{
				Name: "Files",
				Type: reflect.TypeOf([]string{}),
				Tag:  reflect.StructTag(`long:"files" type:"chained" description:"input files"`),
			},
			want: &types.TagConfig{
				Name:        "files",
				Description: "input files",
				Kind:        types.KindFlag,
				TypeOf:      types.Chained,
			},
		},
		{
			name: "secure input with prompt",
			field: reflect.StructField{
				Name: "Password",
				Type: reflect.TypeOf(""),
				Tag:  reflect.StructTag(`long:"password" secure:"true" prompt:"Enter password"`),
			},
			want: &types.TagConfig{
				Name:   "password",
				Kind:   types.KindFlag,
				TypeOf: types.Single,
				Secure: types.Secure{
					IsSecure: true,
					Prompt:   "Enter password",
				},
			},
		},
		{
			name: "accepted values with description",
			field: reflect.StructField{
				Name: "Format",
				Type: reflect.TypeOf(""),
				Tag:  reflect.StructTag(`long:"format" accepted:"{pattern:json|yaml|text,desc:Output format}"`),
			},
			want: &types.TagConfig{
				Name:   "format",
				Kind:   types.KindFlag,
				TypeOf: types.Single,
				AcceptedValues: []types.PatternValue{
					{Pattern: "json|yaml|text", Description: "Output format", Compiled: regexp.MustCompile("json|yaml|text")},
				},
			},
		},
		{
			name: "dependencies",
			field: reflect.StructField{
				Name: "Compress",
				Type: reflect.TypeOf(false),
				Tag:  reflect.StructTag(`long:"compress" depends:"{flag:format,values:[json,yaml]}"`),
			},
			want: &types.TagConfig{
				Name:   "compress",
				Kind:   types.KindFlag,
				TypeOf: types.Standalone,
				DependsOn: map[string][]string{
					"format": {"json", "yaml"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LegacyUnmarshalTagFormat(tt.field)

			if tt.wantErr {
				if err == nil {
					t.Error("LegacyUnmarshalTagFormat() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Errorf("LegacyUnmarshalTagFormat() error = %v", err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LegacyUnmarshalTagFormat() = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalTagFormat(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		field    reflect.StructField
		wantErr  bool
		wantKind types.Kind
		wantName string
		want     *types.TagConfig
	}{
		{
			name: "valid kind",
			tag:  "kind:flag;name:test;short:t;type:single;desc:description;default:defaultVal;required:false;secure:true;prompt:promptText;path:pathValue;accepted:{pattern:^one$,desc:''},{pattern:^two$,desc:''}",
			field: reflect.StructField{
				Name: "TestField",
				Type: reflect.TypeOf(types.TagConfig{}),
			},
			wantErr:  false,
			wantKind: types.KindFlag,
			wantName: "test",
			want: &types.TagConfig{
				Kind:        types.KindFlag,
				Name:        "test",
				Short:       "t",
				Default:     "defaultVal",
				Description: "description",
				TypeOf:      types.Single,
				Path:        "pathValue",
				Secure: types.Secure{
					IsSecure: true,
					Prompt:   "promptText",
				},
				AcceptedValues: []types.PatternValue{{Pattern: "^one$", Description: "''", Compiled: regexp.MustCompile("^one$")}, {Pattern: "^two$", Description: "''", Compiled: regexp.MustCompile("^two$")}},
			},
		},
		{
			name: "invalid kind",
			tag:  "kind:invalid;name:test;short:t;type:single;desc:description;default:defaultVal;required:false;secure:false;prompt:promptText;path:pathValue;accepted:{pattern:^one$,desc:''},{pattern:^two$,desc:''}",
			field: reflect.StructField{
				Name: "TestField",
				Type: reflect.TypeOf(types.TagConfig{}),
			},
			wantErr:  true,
			wantKind: types.KindEmpty,
			wantName: "test",
		},
		{
			name: "empty tag",
			tag:  "",
			field: reflect.StructField{
				Name: "TestField",
				Type: reflect.TypeOf(types.TagConfig{}),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := UnmarshalTagFormat(tt.tag, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalTagFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if config != nil {
				if config.Kind != tt.wantKind {
					t.Errorf("UnmarshalTagFormat() got Kind = %v, want %v", config.Kind, tt.wantKind)
				}
				if config.Name != tt.wantName {
					t.Errorf("UnmarshalTagFormat() got Name = %v, want %v", config.Name, tt.wantName)
				}
				if !equals(config, tt.want) {
					t.Errorf("UnmarshalTagFormat() got %v, want %v", config, tt.want)
				}
			}
		})
	}
}

func equals(t, other *types.TagConfig) bool {
	if t == nil || other == nil {
		return t == other
	}
	if !reflect.DeepEqual(t.Kind, other.Kind) {
		return false
	}
	if t.Name != other.Name {
		return false
	}
	if t.Short != other.Short {
		return false
	}
	if t.TypeOf != other.TypeOf {
		return false
	}
	if t.Description != other.Description {
		return false
	}
	if t.Default != other.Default {
		return false
	}
	if t.Required != other.Required {
		return false
	}
	if t.Secure != other.Secure {
		return false
	}
	if t.Path != other.Path {
		return false
	}
	if !reflect.DeepEqual(t.AcceptedValues, other.AcceptedValues) {
		return false
	}
	if !reflect.DeepEqual(t.DependsOn, other.DependsOn) {
		return false
	}
	if t.Capacity != other.Capacity {
		return false
	}
	if t.Position != other.Position {
		return false
	}
	return true
}
