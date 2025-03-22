package parse

import (
	"errors"
	"reflect"
	"testing"

	"github.com/napalu/goopt/errs"
)

func TestPosition(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *PositionData
		wantErr bool
		err     error
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
			name:    "negative index",
			input:   "-1",
			wantErr: true,
			err:     errs.ErrParseNegativeIndex.WithArgs(-1),
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
			err:     errs.ErrParseInt.WithArgs("invalid", 64),
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
			err:     errs.ErrParseMissingValue.WithArgs("position", ""),
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
			name:    "malformed legacy format",
			input:   "{idx:0",
			wantErr: true,
			err:     errs.ErrParseMalformedBraces.WithArgs("{idx:0"),
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
				if !errors.Is(err, tt.err) {
					t.Errorf("Position() error = %v, want error containing %v", err, tt.err)
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
