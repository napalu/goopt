package parse

import (
	"reflect"
	"strings"
	"testing"
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
			name:  "simple position",
			input: "0",
			want:  &PositionData{Index: 0},
		},
		{
			name:  "legacy format",
			input: "{idx:0}",
			want:  &PositionData{Index: 0},
		},
		{
			name:      "negative index",
			input:     "-1",
			wantErr:   true,
			errString: "index must be non-negative",
		},
		{
			name:      "invalid format",
			input:     "invalid",
			wantErr:   true,
			errString: "invalid index value",
		},
		{
			name:      "empty input",
			input:     "",
			wantErr:   true,
			errString: "empty position",
		},
		{
			name:  "whitespace handling",
			input: "  42  ",
			want:  &PositionData{Index: 42},
		},
		{
			name:  "legacy whitespace handling",
			input: "  {  idx  :  42  }  ",
			want:  &PositionData{Index: 42},
		},
		{
			name:      "malformed legacy format",
			input:     "{idx:0",
			wantErr:   true,
			errString: "malformed braces",
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
