package goopt

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockTerminal struct {
	Password         []byte
	IsTerminalResult bool
	Err              error
	Stderr           io.Writer
	Stdout           io.Writer
}

func (m *MockTerminal) ReadPassword(fd int) ([]byte, error) {
	return m.Password, m.Err
}

func (m *MockTerminal) IsTerminal(fd int) bool {
	return m.IsTerminalResult
}

func TestArgumentConfigFuncs(t *testing.T) {

	tests := []struct {
		name         string
		setupFunc    func(*Parser) error
		input        string
		mockPassword string
		mockError    error
		wantParse    bool
		wantValue    string
		wantWarns    []string
		wantErrs     []error
	}{
		{
			name: "with dependency map - valid value",
			setupFunc: func(p *Parser) error {
				err := p.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
				if err != nil {
					return err
				}
				return p.AddFlag("dependent", NewArg(
					WithDependencyMap(map[string][]string{
						"main": {"value1", "value2"},
					}),
				))
			},
			input:     "--main value1 --dependent test",
			wantParse: true,
			wantValue: "value1",
			wantWarns: nil,
		},
		{
			name: "with dependency map - invalid value",
			setupFunc: func(p *Parser) error {
				err := p.AddFlag("main", NewArgument("m", "", Single, false, Secure{}, ""))
				if err != nil {
					return err
				}
				return p.AddFlag("dependent", NewArg(
					WithDependencyMap(map[string][]string{
						"main": {"value1", "value2"},
					}),
				))
			},
			input:     "--main value3 --dependent test",
			wantParse: true,
			wantValue: "value3",
			wantWarns: []string{"Flag 'dependent' depends on 'main' with value 'value1' or 'value2' which was not specified. (got 'value3')"},
		},
		{
			name: "with accepted values - valid",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("status", NewArg(
					WithAcceptedValues([]PatternValue{
						{Pattern: "active", Description: "active status"},
						{Pattern: "inactive", Description: "inactive status"},
					}),
				))
			},
			input:     "--status active",
			wantParse: true,
			wantValue: "active",
			wantWarns: nil,
		},
		{
			name: "with accepted values - invalid",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("status", NewArg(
					WithAcceptedValues([]PatternValue{
						{Pattern: "active", Description: "active status"},
						{Pattern: "inactive", Description: "inactive status"},
					}),
				))
			},
			input:     "--status pending",
			wantParse: false,
			wantWarns: nil,
			wantErrs:  []error{fmt.Errorf("invalid argument 'pending' for flag 'status'. Accepted values: active status, inactive status")},
		},
		{
			name: "with secure flag - terminal input",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("password", NewArg(
					SetSecure(true),
					WithType(Single),
				))
			},
			input:        "--password",
			mockPassword: "terminal_input",
			wantParse:    true,
			wantValue:    "terminal_input",
		},
		{
			name: "with secure prompt - terminal input",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("password", NewArg(
					SetSecure(true),
					SetSecurePrompt("Enter password:"),
					WithType(Single),
				))
			},
			input:        "--password",
			mockPassword: "prompted_input",
			wantParse:    true,
			wantValue:    "prompted_input",
		},
		{
			name: "with secure flag - terminal error",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("password", NewArg(
					SetSecure(true),
					WithType(Single),
				))
			},
			input:     "--password",
			mockError: fmt.Errorf("terminal error"),
			wantParse: false,
			wantErrs:  []error{fmt.Errorf("secure flag 'password' expects a value but we failed to obtain one: terminal error")},
		},
		{
			name: "with pre validation filter",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("upper", NewArg(
					WithPreValidationFilter(strings.ToUpper),
				))
			},
			input:     "--upper lowercase",
			wantParse: true,
			wantValue: "LOWERCASE",
		},
		{
			name: "with post validation filter",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("trim", NewArg(
					WithPostValidationFilter(strings.TrimSpace),
				))
			},
			input:     "--trim ' value '",
			wantParse: true,
			wantValue: "value",
		},
		{
			name: "with both pre and post validation filters",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("transform", NewArg(
					WithPreValidationFilter(strings.ToUpper),
					WithPostValidationFilter(strings.TrimSpace),
				))
			},
			input:     "--transform ' hello '",
			wantParse: true,
			wantValue: "HELLO",
		},
		{
			name: "with pre validation filter and accepted values",
			setupFunc: func(p *Parser) error {
				return p.AddFlag("upper", NewArg(
					WithPreValidationFilter(strings.ToUpper),
					WithAcceptedValues([]PatternValue{
						{Pattern: "^[A-Z]+$", Description: "uppercase only"},
					}),
				))
			},
			input:     "--upper lowercase",
			wantParse: true,
			wantValue: "LOWERCASE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewParser()
			err := tt.setupFunc(opts)
			assert.Nil(t, err)

			// Set up mock terminal if needed
			if tt.mockPassword != "" || tt.mockError != nil {
				originalReader := opts.SetTerminalReader(nil)
				originalStderr := opts.SetStderr(&bytes.Buffer{})
				originalStdout := opts.SetStdout(&bytes.Buffer{})
				defer func() {
					opts.SetTerminalReader(originalReader)
					opts.SetStderr(originalStderr)
					opts.SetStdout(originalStdout)
				}()
				opts.SetTerminalReader(&MockTerminal{
					Password:         []byte(tt.mockPassword),
					IsTerminalResult: true,
					Err:              tt.mockError,
				})
			}

			assert.Equal(t, tt.wantParse, opts.ParseString(tt.input))
			if tt.wantParse {
				got, ok := opts.Get(strings.Split(tt.input, " ")[0][2:])
				assert.True(t, ok)
				assert.Equal(t, tt.wantValue, got)
			}
			if tt.wantWarns != nil {
				assert.Equal(t, tt.wantWarns, opts.GetWarnings())
			}
			if tt.wantErrs != nil {
				assert.Equal(t, tt.wantErrs, opts.GetErrors())
			}
		})
	}
}
