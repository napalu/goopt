package parse

import (
	"errors"
	"reflect"
	"testing"

	"github.com/napalu/goopt/errs"
)

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    DependencyMap
		wantErr bool
		err     error
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
			name:    "empty input",
			input:   "",
			wantErr: true,
			err:     errs.ErrParseMissingValue.WithArgs("dependency", ""),
		},
		{
			name:    "malformed braces",
			input:   "flag:input}",
			wantErr: true,
			err:     errs.ErrParseMalformedBraces.WithArgs("flag:input}"),
		},
		{
			name:    "missing flag",
			input:   "{value:json}",
			wantErr: true,
			err:     errs.ErrParseMissingValue.WithArgs("flag", ""),
		},
		{
			name:    "both value and values",
			input:   "{flag:env,value:prod,values:[stage]}",
			wantErr: true,
			err:     errs.ErrParseInvalidFormat.WithArgs("{flag:env,value:prod,values:[stage]}"),
		},
		{
			name:    "empty value",
			input:   "{flag:type,value:}",
			wantErr: true,
			err:     errs.ErrParseMissingValue.WithArgs("value", ""),
		},
		{
			name:    "invalid format",
			input:   "{flag=input}",
			wantErr: true,
			err:     errs.ErrParseInvalidFormat.WithArgs("{flag=input}"),
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
			name:    "unmatched brackets in values",
			input:   "{flag:env,values:[prod,stage}",
			wantErr: true,
			err:     errs.ErrParseUnmatchedBrackets.WithArgs("{flag:env,values:[prod,stage}"),
		},
		{
			name:    "nested unmatched brackets",
			input:   "{flag:env,values:[[prod,stage]}",
			wantErr: true,
			err:     errs.ErrParseUnmatchedBrackets.WithArgs("{flag:env,values:[[prod,stage]}"),
		},
		{
			name:    "duplicate flags",
			input:   "{flag:env,value:prod},{flag:env,value:stage}",
			wantErr: true,
			err:     errs.ErrParseDuplicateFlag.WithArgs("flag", "env"),
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
			name:    "unbalanced nested brackets",
			input:   "{flag:env,values:[[test]}",
			wantErr: true,
			err:     errs.ErrParseUnmatchedBrackets.WithArgs("{flag:env,values:[[test}"),
		},
		{
			name:    "missing outer brackets",
			input:   "{flag:env,values:a,b,c}",
			wantErr: true,
			err:     errs.ErrParseMalformedBraces.WithArgs("{flag:env,values:a,b,c}"),
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
				if !errors.Is(err, tt.err) {
					t.Errorf("Dependencies() error = %v, want error  %v", err, tt.err)
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
