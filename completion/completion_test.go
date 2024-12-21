package completion

import (
	"fmt"
	"strings"
	"testing"
)

func getTestCompletionData() CompletionData {
	return CompletionData{
		Commands: []string{"test", "test sub"},
		Flags:    []string{"--global-flag", "--verbose"},
		CommandFlags: map[string][]string{
			"test": {"--test-flag", "--required-flag"},
		},
		Descriptions: map[string]string{
			"--global-flag":        "A global flag",
			"--verbose":            "Verbose output",
			"test@--test-flag":     "A test flag",
			"test@--required-flag": "A required flag (required)",
		},
		CommandDescriptions: map[string]string{
			"test":     "Test command",
			"test sub": "Test subcommand",
		},
		FlagValues: map[string][]CompletionValue{
			"--global-flag": {
				{Pattern: "value1", Description: "First value"},
				{Pattern: "value2", Description: "Second value"},
			},
			"test@--test-flag": {
				{Pattern: "test1", Description: "Test value 1"},
				{Pattern: "test2", Description: "Test value 2"},
			},
		},
	}
}

func TestBashCompletion(t *testing.T) {
	data := getTestCompletionData()
	gen := &BashGenerator{}
	result := gen.Generate("testapp", data)

	// Test expected content
	expectations := []string{
		"function __testapp_completion",
		"--global-flag",
		"--verbose",
		"test[Test command]",
		"value1[First value]",
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
		}
	}
}

// completion/completion_test.go (Bash test part)

func TestBashCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "required flags",
			data: CompletionData{
				Flags: []string{"--required"},
				Descriptions: map[string]string{
					"--required": "(required) Required flag",
				},
			},
			expected: []string{
				"flags+=(--required[(required) Required flag])",
			},
		},
		{
			name: "nested commands",
			data: CompletionData{
				Commands: []string{"db", "db backup", "db restore"},
				CommandDescriptions: map[string]string{
					"db":         "Database operations",
					"db backup":  "Backup database",
					"db restore": "Restore database",
				},
			},
			expected: []string{
				`cmd="${c%%[*}"`,
				`subcmd="${c##* }"`,
				`case "${cmd}" in`,
				"db)",
				`COMPREPLY+=( $(compgen -W "db backup" "db restore" -- "$subcmd") )`,
			},
		},
		{
			name: "flag values with patterns",
			data: CompletionData{
				Flags: []string{"--log-level"},
				FlagValues: map[string][]CompletionValue{
					"--log-level": {
						{Pattern: "debug", Description: "Debug logging"},
						{Pattern: "info", Description: "Info logging"},
						{Pattern: "error", Description: "Error logging"},
					},
				},
			},
			expected: []string{
				"--log-level)",
				"local vals=(debug[Debug logging] info[Info logging] error[Error logging])",
				`COMPREPLY=( $(compgen -W "${vals[*]%%[*}" -- "$cur") )`,
			},
		},
		{
			name: "command specific flags",
			data: CompletionData{
				Commands: []string{"serve"},
				CommandFlags: map[string][]string{
					"serve": {"--port", "--host"},
				},
				Descriptions: map[string]string{
					"serve@--port": "Port to listen on",
					"serve@--host": "Host to bind to",
				},
			},
			expected: []string{
				"serve)",
				"local cmd_flags=(--port[Port to listen on] --host[Host to bind to])",
				`flags+=("${cmd_flags[@]}")`,
			},
		},
		{
			name: "global and command flags together",
			data: CompletionData{
				Flags:    []string{"--verbose", "--config"},
				Commands: []string{"build"},
				CommandFlags: map[string][]string{
					"build": {"--target"},
				},
				Descriptions: map[string]string{
					"--verbose":      "Verbose output",
					"--config":       "Config file",
					"build@--target": "Build target",
				},
			},
			expected: []string{
				"# Global flags",
				"flags+=(--verbose[Verbose output])",
				"flags+=(--config[Config file])",
				"build)",
				"local cmd_flags=(--target[Build target])",
				`flags+=("${cmd_flags[@]}")`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &BashGenerator{}
			result := gen.Generate("testapp", tt.data)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected completion to contain %q", expected)
					t.Logf("Actual content:\n%s", result)
				}
			}
		})
	}
}

func TestZshCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "required flags",
			data: CompletionData{
				Flags: []string{"--required"},
				Descriptions: map[string]string{
					"--required": "(required) Required flag",
				},
			},
			expected: []string{
				"*--required[(required) Required flag]",
			},
		},
		{
			name: "nested commands",
			data: CompletionData{
				Commands: []string{"db", "db backup", "db restore"},
				CommandDescriptions: map[string]string{
					"db":         "Database operations",
					"db backup":  "Backup database",
					"db restore": "Restore database",
				},
			},
			expected: []string{
				"_values 'commands'",
				"'db[Database operations]'",
				"'db\\ backup[Backup database]'",
				"'db\\ restore[Restore database]'",
			},
		},
		{
			name: "flag values with patterns",
			data: CompletionData{
				Flags: []string{"--log-level"},
				FlagValues: map[string][]CompletionValue{
					"--log-level": {
						{Pattern: "debug", Description: "Debug logging"},
						{Pattern: "info", Description: "Info logging"},
						{Pattern: "error", Description: "Error logging"},
					},
				},
			},
			expected: []string{
				"*--log-level:value:(debug\\:Debug\\ logging info\\:Info\\ logging error\\:Error\\ logging)",
			},
		},
		{
			name: "command specific flags",
			data: CompletionData{
				Commands: []string{"serve"},
				CommandFlags: map[string][]string{
					"serve": {"--port", "--host"},
				},
				Descriptions: map[string]string{
					"serve@--port": "Port to listen on",
					"serve@--host": "Host to bind to",
				},
			},
			expected: []string{
				"serve)",
				"_arguments",
				"*--port[Port to listen on]",
				"*--host[Host to bind to]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &ZshGenerator{}
			result := gen.Generate("testapp", tt.data)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected completion to contain %q", expected)
					t.Logf("Actual content:\n%s", result)
				}
			}
		})
	}
}

func TestFishCompletion(t *testing.T) {
	data := getTestCompletionData()
	gen := &FishGenerator{}
	result := gen.Generate("testapp", data)

	expectations := []string{
		"complete -c testapp",
		`complete -c testapp -l global-flag -d 'A global flag'`,
		`complete -c testapp -f -n '__fish_use_subcommand' -a 'test' -d 'Test command'`,
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	}
}

// completion/completion_test.go
// completion/completion_test.go
func TestFishCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "global flags",
			data: CompletionData{
				Flags: []string{"--global-flag"},
				Descriptions: map[string]string{
					"--global-flag": "A global flag",
				},
			},
			expected: []string{
				"complete -c testapp -l global-flag -d 'A global flag'",
			},
		},
		{
			name: "commands and subcommands",
			data: CompletionData{
				Commands: []string{"test", "test sub"},
				CommandDescriptions: map[string]string{
					"test":     "Test command",
					"test sub": "Test subcommand",
				},
			},
			expected: []string{
				"complete -c testapp -f -n '__fish_use_subcommand' -a 'test' -d 'Test command'",
				"complete -c testapp -f -n '__fish_seen_subcommand_from test' -a 'sub' -d 'Test subcommand'",
			},
		},
		{
			name: "flag values",
			data: CompletionData{
				Flags: []string{"--global-flag"},
				FlagValues: map[string][]CompletionValue{
					"--global-flag": {
						{Pattern: "value1", Description: "First value"},
						{Pattern: "value2", Description: "Second value"},
					},
				},
			},
			expected: []string{
				"complete -c testapp -f -n '__fish_seen_argument -l global-flag' -a 'value1' -d 'First value'",
				"complete -c testapp -f -n '__fish_seen_argument -l global-flag' -a 'value2' -d 'Second value'",
			},
		},
		{
			name: "command flags",
			data: CompletionData{
				Commands: []string{"test"},
				CommandFlags: map[string][]string{
					"test": {"--test-flag", "--required-flag"},
				},
				Descriptions: map[string]string{
					"test@--test-flag":     "A test flag",
					"test@--required-flag": "A required flag (required)",
				},
			},
			expected: []string{
				"complete -c testapp -f -n '__fish_seen_subcommand_from test' -l test-flag -d 'A test flag'",
				"complete -c testapp -f -n '__fish_seen_subcommand_from test' -l required-flag -d 'A required flag (required)'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &FishGenerator{}
			result := gen.Generate("testapp", tt.data)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected completion to contain %q", expected)
					t.Logf("Actual content:\n%s", result)
				}
			}
		})
	}
}

func TestPowerShellCompletion(t *testing.T) {
	data := getTestCompletionData()
	gen := &PowerShellGenerator{}
	result := gen.Generate("testapp", data)

	expectations := []string{
		"Register-ArgumentCompleter -Native -CommandName testapp",
		"[CompletionResult]::new('--global-flag'",
		"[CompletionResult]::new('test', 'test', [CompletionResultType]::Command",
		"[CompletionResult]::new('value1', 'value1', [CompletionResultType]::ParameterValue",
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
		}
	}
}

// completion/completion_test.go (PowerShell test part)

func TestPowerShellCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "required flags",
			data: CompletionData{
				Flags: []string{"--required"},
				Descriptions: map[string]string{
					"--required": "(required) Required flag",
				},
			},
			expected: []string{
				"Register-ArgumentCompleter -Native -CommandName testapp -ScriptBlock",
				"[CompletionResult]::new('--required', 'required', [CompletionResultType]::ParameterName, '(required) Required flag')",
			},
		},
		{
			name: "nested commands",
			data: CompletionData{
				Commands: []string{"db", "db backup", "db restore"},
				CommandDescriptions: map[string]string{
					"db":         "Database operations",
					"db backup":  "Backup database",
					"db restore": "Restore database",
				},
			},
			expected: []string{
				"if ($wordToComplete -eq '') {",
				"[CompletionResult]::new('db', 'db', [CompletionResultType]::Command, 'Database operations')",
				"[CompletionResult]::new('db backup', 'db backup', [CompletionResultType]::Command, 'Backup database')",
				"[CompletionResult]::new('db restore', 'db restore', [CompletionResultType]::Command, 'Restore database')",
			},
		},
		{
			name: "flag values with patterns",
			data: CompletionData{
				Flags: []string{"--log-level"},
				FlagValues: map[string][]CompletionValue{
					"--log-level": {
						{Pattern: "debug", Description: "Debug logging"},
						{Pattern: "info", Description: "Info logging"},
						{Pattern: "error", Description: "Error logging"},
					},
				},
			},
			expected: []string{
				"if ($wordToComplete -eq '--log-level') {",
				"[CompletionResult]::new('debug', 'debug', [CompletionResultType]::ParameterValue, 'Debug logging')",
				"[CompletionResult]::new('info', 'info', [CompletionResultType]::ParameterValue, 'Info logging')",
				"[CompletionResult]::new('error', 'error', [CompletionResultType]::ParameterValue, 'Error logging')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &PowerShellGenerator{}
			result := gen.Generate("testapp", tt.data)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected completion to contain %q", expected)
					t.Logf("Actual content:\n%s", result)
				}
			}
		})
	}
}

func TestGetGenerator(t *testing.T) {
	tests := []struct {
		shell    string
		expected Generator
	}{
		{"bash", &BashGenerator{}},
		{"zsh", &ZshGenerator{}},
		{"fish", &FishGenerator{}},
		{"powershell", &PowerShellGenerator{}},
		{"unknown", &BashGenerator{}}, // defaults to bash
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			gen := GetGenerator(tt.shell)
			if gen == nil {
				t.Errorf("GetGenerator(%q) returned nil", tt.shell)
			}
			// Check type matches expected
			expectedType := strings.TrimPrefix(fmt.Sprintf("%T", tt.expected), "*completion.")
			gotType := strings.TrimPrefix(fmt.Sprintf("%T", gen), "*completion.")
			if gotType != expectedType {
				t.Errorf("GetGenerator(%q) = %T, want %T", tt.shell, gen, tt.expected)
			}
		})
	}
}

// Helper function tests
func TestEscapeDescription(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple text", "simple text"},
		{"text with 'quotes'", `text with \'quotes\'`},
		{`text with "double quotes"`, `text with \"double quotes\"`},
		{"text with `backticks`", "text with `backticks`"},
		{"text with $variables", `text with \$variables`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeBash(tt.input)
			if result != tt.expected {
				t.Errorf("escapeDescription(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
