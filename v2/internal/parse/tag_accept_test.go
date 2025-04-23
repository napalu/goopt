package parse

import (
	"reflect"
	"testing"

	"github.com/napalu/goopt/v2/types"
)

func TestParsePatternValues(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []types.PatternValue
		wantErr bool
	}{
		{
			name:  "single pattern",
			input: "{pattern:json,desc:JSON format}",
			want: []types.PatternValue{{
				Pattern:     "json",
				Description: "JSON format",
			}},
		},
		{
			name:  "multiple patterns",
			input: "{pattern:json,desc:JSON format},{pattern:yaml,desc:YAML format}",
			want: []types.PatternValue{
				{Pattern: "json", Description: "JSON format"},
				{Pattern: "yaml", Description: "YAML format"},
			},
		},
		{
			name:  "regex with escapes",
			input: "{pattern:(?i)^(?:ALL|INFO|ERROR|WARN|DEBUG|NONE)$,desc:one of (ALL\\, INFO\\, ERROR\\, WARN\\, DEBUG\\, NONE) - case-insensitive}",
			want: []types.PatternValue{{
				Pattern:     "(?i)^(?:ALL|INFO|ERROR|WARN|DEBUG|NONE)$",
				Description: "one of (ALL, INFO, ERROR, WARN, DEBUG, NONE) - case-insensitive",
			}},
		},
		{
			name:  "multiple with escapes",
			input: "{pattern:a\\,b,desc:Values a\\, b},{pattern:c\\,d,desc:Values c\\, d}",
			want: []types.PatternValue{
				{Pattern: "a,b", Description: "Values a, b"},
				{Pattern: "c,d", Description: "Values c, d"},
			},
		},
		{
			name:    "invalid format",
			input:   "{pattern:json}",
			wantErr: true,
		},
		{
			name:    "missing pattern",
			input:   "{desc:JSON format}",
			wantErr: true,
		},
		{
			name:  "pattern with multiple escapes",
			input: "{pattern:a\\,b\\,c,desc:Values with\\, multiple\\, commas}",
			want: []types.PatternValue{{
				Pattern:     "a,b,c",
				Description: "Values with, multiple, commas",
			}},
		},
		{
			name:  "pattern with escaped braces",
			input: "{pattern:\\{\\},desc:Literal braces}",
			want: []types.PatternValue{{
				Pattern:     "{}",
				Description: "Literal braces",
			}},
		},
		{
			name:    "multiple with empty values between",
			input:   "{pattern:a,desc:A},,{pattern:b,desc:B}",
			wantErr: true,
		},
		{
			name:  "pattern with escaped quotes",
			input: "{pattern:\\\",desc:Quote},{pattern:\\',desc:Single quote}",
			want: []types.PatternValue{
				{Pattern: "\"", Description: "Quote"},
				{Pattern: "'", Description: "Single quote"},
			},
		},
		{
			name:  "pattern with regex special chars",
			input: "{pattern:^\\w+@\\w+\\.\\w+$,desc:Email regex}",
			want: []types.PatternValue{{
				Pattern:     `^\w+@\w+\.\w+$`,
				Description: "Email regex",
			}},
		},
		{
			name:  "pattern with multiple escaped backslashes",
			input: "{pattern:C:\\\\Windows\\\\System32,desc:Windows path}",
			want: []types.PatternValue{{
				Pattern:     `C:\Windows\System32`,
				Description: "Windows path",
			}},
		},
		{
			name:    "empty pattern with spaces",
			input:   "{pattern:   ,desc:test}",
			wantErr: true,
		},
		{
			name:    "empty desc with spaces",
			input:   "{pattern:test,desc:   }",
			wantErr: true,
		},
		{
			name:    "unterminated escape",
			input:   "{pattern:test\\,desc:Description\\}",
			wantErr: true,
		},
		{
			name:  "consecutive escapes",
			input: "{pattern:a\\\\\\,b,desc:Backslash and comma}",
			want: []types.PatternValue{{
				Pattern:     `a\,b`,
				Description: "Backslash and comma",
			}},
		},
		{
			name:  "mixed escapes in description",
			input: "{pattern:[a-z]+,desc:Letters (a\\, b\\, c\\\\d\\\\e)}",
			want: []types.PatternValue{{
				Pattern:     "[a-z]+",
				Description: `Letters (a, b, c\d\e)`,
			}},
		},
		{
			name:  "escaped colon",
			input: "{pattern:key\\:value,desc:Contains colon}",
			want: []types.PatternValue{{
				Pattern:     "key:value",
				Description: "Contains colon",
			}},
		},
		{
			name:  "multiple patterns with mixed escapes",
			input: "{pattern:\\\\\\,\\\\,desc:Backslash\\, comma},{pattern:\\\"\\',desc:Quotes}",
			want: []types.PatternValue{
				{Pattern: `\,\`, Description: "Backslash, comma"},
				{Pattern: `"'`, Description: "Quotes"},
			},
		},
		{
			name:  "spaces",
			input: "{pattern:a b c,desc:space spaces}",
			want: []types.PatternValue{{
				Pattern:     "a b c",
				Description: "space spaces",
			}},
		},
		{
			name:    "trailing backslash",
			input:   "{pattern:test\\,desc:Description}\\",
			wantErr: true,
		},
		{
			name:    "escaped empty values",
			input:   "{pattern:\\,desc:\\}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PatternValues(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PatternValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PatternValues() = %v, want %v", got, tt.want)
			}
		})
	}
}
