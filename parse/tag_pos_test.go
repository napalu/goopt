package parse

import (
	"reflect"
	"strings"
	"testing"

	"github.com/napalu/goopt/types"
)

func TestPosition(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *PositionData
		wantErr   bool
		errString string
	}{
		{
			name:  "start position with index",
			input: "pos:{at:start,idx:0}",
			want: &PositionData{
				At:  ptr(types.AtStart),
				Idx: ptr(0),
			},
		},
		{
			name:  "end position without index",
			input: "pos:{at:end}",
			want: &PositionData{
				At:  ptr(types.AtEnd),
				Idx: nil,
			},
		},
		{
			name:  "only index",
			input: "pos:{at:,idx:1}",
			want: &PositionData{
				At:  nil,
				Idx: ptr(1),
			},
		},
		{
			name:      "invalid position type",
			input:     "pos:{at:middle,idx:0}",
			wantErr:   true,
			errString: "invalid position type",
		},
		{
			name:      "negative index",
			input:     "pos:{at:start,idx:-1}",
			wantErr:   true,
			errString: "index must be non-negative",
		},
		{
			name:      "malformed braces",
			input:     "pos:at:start,idx:0}",
			wantErr:   true,
			errString: "malformed braces",
		},
		{
			name:      "empty input",
			input:     "",
			wantErr:   true,
			errString: "empty position",
		},
		{
			name:      "invalid format",
			input:     "pos:{at=start}",
			wantErr:   true,
			errString: "invalid format",
		},
		{
			name:  "empty position type with index",
			input: "pos:{at:,idx:1}",
			want: &PositionData{
				At:  nil,
				Idx: ptr(1),
			},
		},
		{
			name:  "empty position type with spaces and index",
			input: "pos:{at: ,idx:1}",
			want: &PositionData{
				At:  nil,
				Idx: ptr(1),
			},
		},
		{
			name:  "only empty position type",
			input: "pos:{at:}",
			want: &PositionData{
				At:  nil,
				Idx: nil,
			},
		},
		{
			name:  "excessive whitespace",
			input: "  pos:{  at  :  start  ,  idx  :  0  }  ",
			want: &PositionData{
				At:  ptr(types.AtStart),
				Idx: ptr(0),
			},
		},
		{
			name:  "newlines in input",
			input: "pos:{\nat:start,\nidx:0\n}",
			want: &PositionData{
				At:  ptr(types.AtStart),
				Idx: ptr(0),
			},
		},
		{
			name:  "large index value",
			input: "pos:{at:end,idx:999999}",
			want: &PositionData{
				At:  ptr(types.AtEnd),
				Idx: ptr(999999),
			},
		},
		{
			name:      "index overflow",
			input:     "pos:{at:end,idx:9999999999999999999}",
			wantErr:   true,
			errString: "invalid index value",
		},
		{
			name:      "missing colon",
			input:     "pos:{at start,idx:0}",
			wantErr:   true,
			errString: "malformed braces",
		},
		{
			name:  "extra fields",
			input: "pos:{at:start,idx:0,extra:value}",
			want: &PositionData{
				At:  ptr(types.AtStart),
				Idx: ptr(0),
			},
		},
		{
			name:  "duplicate fields",
			input: "pos:{at:start,at:end,idx:0}",
			want: &PositionData{
				At:  ptr(types.AtEnd),
				Idx: ptr(0),
			},
		},
		{
			name:  "empty braces",
			input: "pos:{}",
			want:  &PositionData{},
		},
		{
			name:      "missing closing brace",
			input:     "pos:{at:start",
			wantErr:   true,
			errString: "malformed braces",
		},
		{
			name:      "missing opening brace",
			input:     "pos:at:start}",
			wantErr:   true,
			errString: "malformed braces",
		},
		{
			name:  "case insensitive START",
			input: "pos:{at:START,idx:0}",
			want: &PositionData{
				At:  ptr(types.AtStart),
				Idx: ptr(0),
			},
		},
		{
			name:  "case insensitive END",
			input: "pos:{at:EnD}",
			want: &PositionData{
				At:  ptr(types.AtEnd),
				Idx: nil,
			},
		},
		{
			name:      "invalid position with number",
			input:     "pos:{at:start1}",
			wantErr:   true,
			errString: "invalid position type",
		},
		{
			name:      "invalid position with special chars",
			input:     "pos:{at:start@end}",
			wantErr:   true,
			errString: "invalid position type",
		},
		{
			name:  "multiple idx values",
			input: "pos:{at:start,idx:0,idx:1}",
			want: &PositionData{
				At:  ptr(types.AtStart),
				Idx: ptr(1),
			},
		},
		{
			name:  "multiple at values",
			input: "pos:{at:start,at:end}",
			want: &PositionData{
				At:  ptr(types.AtEnd),
				Idx: nil,
			},
		},
		{
			name:  "idx without at",
			input: "pos:{idx:0}",
			want: &PositionData{
				At:  nil,
				Idx: ptr(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Position(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Position() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("Position() error = %v, want error containing %v", err, tt.errString)
				}
				return
			}
			if err != nil {
				t.Errorf("Position() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Position() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPositionType_String(t *testing.T) {
	tests := []struct {
		name string
		p    types.PositionType
		want string
	}{
		{
			name: "start position",
			p:    types.AtStart,
			want: "start",
		},
		{
			name: "end position",
			p:    types.AtEnd,
			want: "end",
		},
		{
			name: "unknown position",
			p:    types.PositionType(999),
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("PositionType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePositionType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      types.PositionType
		wantErr   bool
		errString string
	}{
		{
			name:  "start lowercase",
			input: "start",
			want:  types.AtStart,
		},
		{
			name:  "end lowercase",
			input: "end",
			want:  types.AtEnd,
		},
		{
			name:  "start uppercase",
			input: "START",
			want:  types.AtStart,
		},
		{
			name:  "end uppercase",
			input: "END",
			want:  types.AtEnd,
		},
		{
			name:  "start mixed case",
			input: "StArT",
			want:  types.AtStart,
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  0,
		},
		{
			name:      "invalid position",
			input:     "middle",
			wantErr:   true,
			errString: "invalid position type",
		},
		{
			name:      "invalid with spaces",
			input:     "  invalid  ",
			wantErr:   true,
			errString: "invalid position type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePositionType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePositionType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("parsePositionType() error = %v, want error containing %v", err, tt.errString)
				}
				return
			}
			if got != tt.want {
				t.Errorf("parsePositionType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to create pointers
func ptr[T any](v T) *T {
	return &v
}
