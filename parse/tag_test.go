package parse

import (
	"reflect"
	"testing"
	"time"

	"github.com/napalu/goopt/types"
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
			tag:  "name:source;desc:Source file;pos:{at:start,idx:0};required:true",
			want: &types.TagConfig{
				Kind:          types.KindFlag,
				Name:          "source",
				Description:   "Source file",
				Position:      ptr(types.AtStart),
				RelativeIndex: ptr(0),
				Required:      true,
			},
		},
		{
			name: "position only idx",
			tag:  "pos:{idx:1}",
			want: &types.TagConfig{
				Kind:          types.KindFlag,
				RelativeIndex: ptr(1),
			},
		},
		{
			name: "position with empty at",
			tag:  "pos:{at:};required:true",
			want: &types.TagConfig{
				Kind:     types.KindFlag,
				Required: true,
			},
		},
		{
			name: "multiple position tags",
			tag:  "pos:{at:start};pos:{at:end}", // Last one wins
			want: &types.TagConfig{
				Kind:     types.KindFlag,
				Position: ptr(types.AtEnd),
			},
		},
		{
			name:    "invalid position type",
			tag:     "pos:{at:middle}",
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
