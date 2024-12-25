package parse

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePatternValues(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []TagPatternValue
		wantErr bool
	}{
		{
			name:  "single pattern",
			input: "{pattern:json,desc:JSON format}",
			want: []TagPatternValue{{
				Pattern:     "json",
				Description: "JSON format",
			}},
		},
		{
			name:  "multiple patterns",
			input: "{pattern:json,desc:JSON format},{pattern:yaml,desc:YAML format}",
			want: []TagPatternValue{
				{Pattern: "json", Description: "JSON format"},
				{Pattern: "yaml", Description: "YAML format"},
			},
		},
		{
			name:  "regex with escapes",
			input: "{pattern:(?i)^(?:ALL|INFO|ERROR|WARN|DEBUG|NONE)$,desc:one of (ALL\\, INFO\\, ERROR\\, WARN\\, DEBUG\\, NONE) - case-insensitive}",
			want: []TagPatternValue{{
				Pattern:     "(?i)^(?:ALL|INFO|ERROR|WARN|DEBUG|NONE)$",
				Description: "one of (ALL, INFO, ERROR, WARN, DEBUG, NONE) - case-insensitive",
			}},
		},
		{
			name:  "multiple with escapes",
			input: "{pattern:a\\,b,desc:Values a\\, b},{pattern:c\\,d,desc:Values c\\, d}",
			want: []TagPatternValue{
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
			want: []TagPatternValue{{
				Pattern:     "a,b,c",
				Description: "Values with, multiple, commas",
			}},
		},
		{
			name:  "pattern with escaped braces",
			input: "{pattern:\\{\\},desc:Literal braces}",
			want: []TagPatternValue{{
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
			want: []TagPatternValue{
				{Pattern: "\"", Description: "Quote"},
				{Pattern: "'", Description: "Single quote"},
			},
		},
		{
			name:  "pattern with regex special chars",
			input: "{pattern:^\\w+@\\w+\\.\\w+$,desc:Email regex}",
			want: []TagPatternValue{{
				Pattern:     `^\w+@\w+\.\w+$`,
				Description: "Email regex",
			}},
		},
		{
			name:  "pattern with multiple escaped backslashes",
			input: "{pattern:C:\\\\Windows\\\\System32,desc:Windows path}",
			want: []TagPatternValue{{
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
			want: []TagPatternValue{{
				Pattern:     `a\,b`,
				Description: "Backslash and comma",
			}},
		},
		{
			name:  "mixed escapes in description",
			input: "{pattern:[a-z]+,desc:Letters (a\\, b\\, c\\\\d\\\\e)}",
			want: []TagPatternValue{{
				Pattern:     "[a-z]+",
				Description: `Letters (a, b, c\d\e)`,
			}},
		},
		{
			name:  "escaped colon",
			input: "{pattern:key\\:value,desc:Contains colon}",
			want: []TagPatternValue{{
				Pattern:     "key:value",
				Description: "Contains colon",
			}},
		},
		{
			name:  "multiple patterns with mixed escapes",
			input: "{pattern:\\\\\\,\\\\,desc:Backslash\\, comma},{pattern:\\\"\\',desc:Quotes}",
			want: []TagPatternValue{
				{Pattern: `\,\`, Description: "Backslash, comma"},
				{Pattern: `"'`, Description: "Quotes"},
			},
		},
		{
			name:  "spaces",
			input: "{pattern:a b c,desc:space spaces}",
			want: []TagPatternValue{{
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

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      DependencyMap
		wantErr   bool
		errString string
	}{
		// Single dependency cases
		{
			name:  "simple flag",
			input: "{flag:input}",
			want:  DependencyMap{"input": nil},
		},
		{
			name:  "flag with value",
			input: "{flag:type,value:json}",
			want:  DependencyMap{"type": []string{"json"}},
		},
		{
			name:  "flag with multiple values",
			input: "{flag:env,values:[prod,stage]}",
			want:  DependencyMap{"env": []string{"prod", "stage"}},
		},
		{
			name:  "empty values list",
			input: "{flag:env,values:[]}",
			want:  DependencyMap{"env": nil},
		},
		{
			name:  "values with whitespace",
			input: "{flag:env,values:[ prod , stage ]}",
			want:  DependencyMap{"env": []string{"prod", "stage"}},
		},
		// Multiple dependencies
		{
			name:  "multiple dependencies",
			input: "{flag:format,value:json},{flag:compress,value:true}",
			want: DependencyMap{
				"format":   []string{"json"},
				"compress": []string{"true"},
			},
		},
		// Error cases
		{
			name:      "empty input",
			input:     "",
			wantErr:   true,
			errString: "empty dependency",
		},
		{
			name:      "malformed braces",
			input:     "flag:input}",
			wantErr:   true,
			errString: "malformed braces",
		},
		{
			name:      "missing flag",
			input:     "{value:json}",
			wantErr:   true,
			errString: "missing or empty flag",
		},
		{
			name:      "both value and values",
			input:     "{flag:env,value:prod,values:[stage]}",
			wantErr:   true,
			errString: "cannot specify both",
		},
		{
			name:      "empty value",
			input:     "{flag:type,value:}",
			wantErr:   true,
			errString: "empty value",
		},
		{
			name:      "invalid format",
			input:     "{flag=input}",
			wantErr:   true,
			errString: "invalid format",
		},
		// Edge cases for brackets and values
		{
			name:  "nested brackets",
			input: "{flag:env,values:[[prod\\,stage],[dev\\,test]]}",
			want:  DependencyMap{"env": []string{"[prod,stage]", "[dev,test]"}},
		},
		{
			name:  "simple list",
			input: "{flag:env,values:[prod,stage]}",
			want:  DependencyMap{"env": []string{"prod", "stage"}},
		},
		{
			name:  "values with special chars",
			input: "{flag:path,values:[/usr/bin,C:\\Program Files]}",
			want:  DependencyMap{"path": []string{"/usr/bin", "C:\\Program Files"}},
		},
		{
			name:  "multiple dependencies with complex values",
			input: "{flag:env,values:[prod,stage]},{flag:path,value:/usr/bin},{flag:opts,values:[key=val,x=y]}",
			want: DependencyMap{
				"env":  []string{"prod", "stage"},
				"path": []string{"/usr/bin"},
				"opts": []string{"key=val", "x=y"},
			},
		},
		// Edge cases for whitespace
		{
			name:  "excessive whitespace",
			input: "  {  flag  :  env  ,  values  : [  prod  ,  stage  ]  }  ",
			want:  DependencyMap{"env": []string{"prod", "stage"}},
		},
		{
			name:  "newlines in input",
			input: "{\nflag:env,\nvalues:[\nprod,\nstage\n]\n}",
			want:  DependencyMap{"env": []string{"prod", "stage"}},
		},
		// Error cases
		{
			name:      "unmatched brackets in values",
			input:     "{flag:env,values:[prod,stage}",
			wantErr:   true,
			errString: "unmatched brackets",
		},
		{
			name:      "nested unmatched brackets",
			input:     "{flag:env,values:[[prod,stage]}",
			wantErr:   true,
			errString: "unmatched brackets",
		},
		{
			name:      "duplicate flags",
			input:     "{flag:env,value:prod},{flag:env,value:stage}",
			wantErr:   true,
			errString: "duplicate flag",
		},
		{
			name:  "empty parts between commas",
			input: "{flag:env,values:[,prod,,stage,]}",
			want:  DependencyMap{"env": []string{"prod", "stage"}},
		},
		{
			name:  "escaped characters",
			input: `{flag:path,value:C:\path\with\backslashes}`,
			want:  DependencyMap{"path": []string{`C:\path\with\backslashes`}},
		},
		{
			name:  "values with escaped commas",
			input: `{flag:env,values:[prod\,stage,dev\,test]}`,
			want:  DependencyMap{"env": []string{"prod,stage", "dev,test"}},
		},
		{
			name:  "mixed normal and escaped commas",
			input: `{flag:env,values:[prod\,stage,dev,test\,local]}`,
			want:  DependencyMap{"env": []string{"prod,stage", "dev", "test,local"}},
		},
		{
			name:  "escaped backslash and comma",
			input: `{flag:path,values:[C:\\path\\with\\commas\,in\,it]}`,
			want:  DependencyMap{"path": []string{`C:\path\with\commas,in,it`}},
		},
		// Bracket handling cases
		{
			name:  "nested brackets with multiple levels",
			input: "{flag:env,values:[[prod\\,stage],[dev\\,test],[[qa\\,1,qa\\,2]]]}",
			want:  DependencyMap{"env": []string{"[prod,stage]", "[dev,test]", "[qa,1,qa,2]"}},
		},
		{
			name:  "brackets in middle of value",
			input: "{flag:env,values:[pre[test]post,dev]}",
			want:  DependencyMap{"env": []string{"pre[test]post", "dev"}},
		},
		{
			name:  "empty brackets",
			input: "{flag:env,values:[[]]}",
			want:  DependencyMap{"env": []string{"[]"}},
		},
		{
			name:  "multiple escaped characters",
			input: "{flag:path,values:[C:\\\\Program Files\\\\[x86],/usr/local/[bin]]}",
			want:  DependencyMap{"path": []string{`C:\Program Files\[x86]`, `/usr/local/[bin]`}},
		},
		{
			name:      "unbalanced nested brackets",
			input:     "{flag:env,values:[[test]}",
			wantErr:   true,
			errString: "unmatched brackets",
		},
		{
			name:      "missing outer brackets",
			input:     "{flag:env,values:a,b,c}",
			wantErr:   true,
			errString: "malformed braces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Dependencies(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Dependencies() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("Dependencies() error = %v, want error containing %v", err, tt.errString)
				}
				return
			}
			if err != nil {
				t.Errorf("Dependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}