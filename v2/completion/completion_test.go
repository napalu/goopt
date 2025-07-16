package completion

import (
	"fmt"
	"strings"
	"testing"
)

func getTestCompletionData() CompletionData {
	return CompletionData{
		Commands: []string{"test", "test sub"},
		Flags: []FlagPair{
			{Long: "global-flag", Short: "g", Description: "A global flag"},
			{Long: "verbose", Short: "v", Description: "Verbose output"},
		},
		CommandFlags: map[string][]FlagPair{
			"test": {
				{Long: "test-flag", Short: "t", Description: "A test flag"},
				{Long: "required-flag", Short: "r", Description: "A required flag (required)"},
			},
		},
		CommandDescriptions: map[string]string{
			"test":     "Test command",
			"test sub": "Test subcommand",
		},
		FlagValues: map[string][]CompletionValue{
			"global-flag": {
				{Pattern: "value1", Description: "First value"},
				{Pattern: "value2", Description: "Second value"},
			},
			"test-flag": {
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

	// Test expected content without descriptions
	expectations := []string{
		"function __testapp_completion",
		"--global-flag",
		"-g",
		"--verbose",
		"-v",
		"test",
		"value1",
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
		}
	}
}

func TestBashCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "required flags",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "required", Short: "r", Description: "(required) Required flag"},
				},
			},
			expected: []string{
				`flags="$flags -r"`,
				`flags="$flags --required"`,
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
				`if [[ "$cmd" == *" "* ]]; then`,
				`local base_cmd="${cmd%% *}"`,
				`local sub_cmd="${cmd#* }"`,
				`case "$base_cmd" in`,
				"db)",
				`COMPREPLY=( $(compgen -W "backup restore" -- "$sub_cmd") )`,
			},
		},
		{
			name: "flag values with patterns",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "log-level", Short: "l", Description: "Set log level"},
				},
				FlagValues: map[string][]CompletionValue{
					"log-level": {
						{Pattern: "debug", Description: "Debug logging"},
						{Pattern: "info", Description: "Info logging"},
						{Pattern: "error", Description: "Error logging"},
					},
				},
			},
			expected: []string{
				`--log-level|-l)`,
				`COMPREPLY=( $(compgen -W "debug info error" -- "$cur") )`,
			},
		},
		{
			name: "command specific flags",
			data: CompletionData{
				CommandFlags: map[string][]FlagPair{
					"serve": {
						{Long: "port", Short: "p", Description: "Port to listen on"},
						{Long: "host", Short: "h", Description: "Host to bind to"},
					},
				},
			},
			expected: []string{
				`serve)`,
				`local cmd_flags=""`,
				`cmd_flags="$cmd_flags -p"`,
				`cmd_flags="$cmd_flags --port"`,
				`cmd_flags="$cmd_flags -h"`,
				`cmd_flags="$cmd_flags --host"`,
				`flags="$flags$cmd_flags"`,
			},
		},
		{
			name: "global and command flags together",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "verbose", Short: "v", Description: "Verbose output"},
					{Long: "config", Short: "c", Description: "Config file"},
				},
				CommandFlags: map[string][]FlagPair{
					"build": {
						{Long: "target", Short: "t", Description: "Build target"},
					},
				},
			},
			expected: []string{
				`flags="$flags -v"`,
				`flags="$flags --verbose"`,
				`flags="$flags -c"`,
				`flags="$flags --config"`,
				`build)`,
				`local cmd_flags=""`,
				`cmd_flags="$cmd_flags -t"`,
				`cmd_flags="$cmd_flags --target"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &BashGenerator{}
			result := generator.Generate("testapp", tt.data)

			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected completion to contain %q", exp)
					t.Errorf("Actual content:\n%s", result)
				}
			}
		})
	}
}
func TestZshCompletion(t *testing.T) {
	data := CompletionData{
		Commands: []string{"test", "test sub"},
		CommandDescriptions: map[string]string{
			"test":     "Test command",
			"test sub": "Test subcommand",
		},
		Flags: []FlagPair{
			{Long: "global-flag", Short: "g", Description: "A global flag"},
			{Long: "verbose", Short: "v", Description: "Verbose output"},
		},
		FlagValues: map[string][]CompletionValue{
			"global-flag": {
				{Pattern: "value1", Description: "First value"},
				{Pattern: "value2", Description: "Second value"},
			},
		},
		CommandFlags: map[string][]FlagPair{
			"test": {
				{Long: "test-flag", Short: "t", Description: "A test flag"},
				{Long: "required-flag", Short: "r", Description: "A required flag (required)"},
			},
		},
	}

	expected := []string{
		// Commands
		`"test:Test command"`,

		// Global flags with values
		`"(-g --global-flag)"{-g,--global-flag}"[A global flag]":(value1\:"First value" value2\:"Second value")`,

		// Global flags without values
		`"(-v --verbose)"{-v,--verbose}"[Verbose output]"`,

		// Command-specific flags
		`"(-t --test-flag)"{-t,--test-flag}"[A test flag]"`,
		`"(-r --required-flag)"{-r,--required-flag}"[A required flag (required)]"`,

		// Subcommands
		`"test:(sub:Test subcommand)"`,
	}

	gen := &ZshGenerator{}
	script := gen.Generate("testapp", data)
	for _, exp := range expected {
		if !strings.Contains(script, exp) {
			t.Errorf("Expected completion to contain %q", exp)
			t.Errorf("Actual content:\n%s", script)
		}
	}
}

func TestZshCompletionSpecific(t *testing.T) {
	t.Run("commands_with_descriptions", func(t *testing.T) {
		data := CompletionData{
			Commands: []string{"test", "other"},
			CommandDescriptions: map[string]string{
				"test":  "Test command",
				"other": "Other command",
			},
		}
		gen := &ZshGenerator{}
		result := gen.Generate("testapp", data)

		expected := `"test:Test command"`
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	})

	t.Run("nested_commands_with_descriptions", func(t *testing.T) {
		data := CompletionData{
			Commands: []string{"db", "db backup", "db restore"},
			CommandDescriptions: map[string]string{
				"db":         "Database operations",
				"db backup":  "Backup database",
				"db restore": "Restore database",
			},
		}
		gen := &ZshGenerator{}
		result := gen.Generate("testapp", data)

		expected := `"db:(backup:Backup database restore:Restore database)"`
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	})

	t.Run("flags_with_short_and_long_forms", func(t *testing.T) {
		data := CompletionData{
			Flags: []FlagPair{
				{Long: "verbose", Short: "v", Description: "Verbose output"},
				{Long: "config", Description: "Config file"},
			},
		}
		gen := &ZshGenerator{}
		result := gen.Generate("testapp", data)

		// Updated to match zsh convention (short flag first)
		expected := `"(-v --verbose)"{-v,--verbose}"[Verbose output]"`
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	})

	t.Run("command_specific_flags", func(t *testing.T) {
		data := CompletionData{
			Commands: []string{"test"},
			CommandFlags: map[string][]FlagPair{
				"test": {
					{Long: "test-flag", Short: "t", Description: "Test flag"},
				},
			},
		}
		gen := &ZshGenerator{}
		result := gen.Generate("testapp", data)

		// Updated to match zsh convention
		expected := `"(-t --test-flag)"{-t,--test-flag}"[Test flag]"`
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	})

	t.Run("file_type_flags", func(t *testing.T) {
		data := CompletionData{
			Flags: []FlagPair{
				{Long: "file", Short: "f", Description: "A file flag", Type: FlagTypeFile},
			},
		}
		gen := &ZshGenerator{}
		result := gen.Generate("testapp", data)

		// Updated to match zsh convention
		expected := `"(-f --file)"{-f,--file}"[A file flag]":_files`
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	})
}

func TestFishCompletion(t *testing.T) {
	data := getTestCompletionData()
	gen := &FishGenerator{}
	result := gen.Generate("testapp", data)

	expectations := []string{
		// Global flags with values
		`complete -c testapp -f -l global-flag -s g -d 'A global flag'`,
		`complete -c testapp -f -l global-flag -s g -n '__fish_seen_argument -l global-flag -s g' -a 'value1' -d 'First value'`,
		`complete -c testapp -f -l global-flag -s g -n '__fish_seen_argument -l global-flag -s g' -a 'value2' -d 'Second value'`,
		// Other global flags
		`complete -c testapp -f -l verbose -s v -d 'Verbose output'`,
		// Commands and subcommands
		`complete -c testapp -f -n '__fish_use_subcommand' -a 'test' -d 'Test command'`,
		`complete -c testapp -f -n '__fish_seen_subcommand_from test' -a 'sub' -d 'Test subcommand'`,
		// Command-specific flags with values
		`complete -c testapp -f -n '__fish_seen_subcommand_from test' -l test-flag -s t -d 'A test flag'`,
		`complete -c testapp -f -n '__fish_seen_subcommand_from test' -l test-flag -s t -n '__fish_seen_argument -l test-flag -s t' -a 'test1' -d 'Test value 1'`,
		`complete -c testapp -f -n '__fish_seen_subcommand_from test' -l test-flag -s t -n '__fish_seen_argument -l test-flag -s t' -a 'test2' -d 'Test value 2'`,
		// Required flag
		`complete -c testapp -f -n '__fish_seen_subcommand_from test' -l required-flag -s r -d 'A required flag (required)'`,
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected completion to contain %q", expected)
			t.Logf("Actual content:\n%s", result)
		}
	}
}

func TestFishCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "global_flags",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "global-flag", Short: "g", Description: "A global flag"},
				},
			},
			expected: []string{
				"complete -c testapp -f -l global-flag -s g -d 'A global flag'",
			},
		},
		{
			name: "commands_and_subcommands",
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
			name: "flag_values",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "global-flag", Short: "g", Description: "A global flag"},
				},
				FlagValues: map[string][]CompletionValue{
					"global-flag": {
						{Pattern: "value1", Description: "First value"},
						{Pattern: "value2", Description: "Second value"},
					},
				},
			},
			expected: []string{
				"complete -c testapp -f -l global-flag -s g -d 'A global flag'",
				"complete -c testapp -f -l global-flag -s g -n '__fish_seen_argument -l global-flag -s g' -a 'value1' -d 'First value'",
				"complete -c testapp -f -l global-flag -s g -n '__fish_seen_argument -l global-flag -s g' -a 'value2' -d 'Second value'",
			},
		},
		{
			name: "command_flags",
			data: CompletionData{
				Commands: []string{"test"},
				CommandFlags: map[string][]FlagPair{
					"test": {
						{Long: "test-flag", Short: "t", Description: "Test flag"},
						{Long: "required-flag", Short: "r", Description: "A required flag (required)"},
					},
				},
			},
			expected: []string{
				"complete -c testapp -f -n '__fish_seen_subcommand_from test' -l test-flag -s t -d 'Test flag'",
				"complete -c testapp -f -n '__fish_seen_subcommand_from test' -l required-flag -s r -d 'A required flag (required)'",
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

func TestPowerShellCompletionSpecific(t *testing.T) {
	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "required flags",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "required", Short: "r", Description: "(required) Required flag"},
				},
			},
			expected: []string{
				"Register-ArgumentCompleter -Native -CommandName testapp -ScriptBlock",
				"$tokens = $commandAst.CommandElements",
				"$currentToken = $tokens | Where-Object { $_.Extent.StartOffset -le $cursorPosition } | Select-Object -Last 1",
				"[CompletionResult]::new('--required', 'required', [CompletionResultType]::ParameterName, '(required) Required flag')",
				"[CompletionResult]::new('-r', 'r', [CompletionResultType]::ParameterName, '(required) Required flag')",
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
				"if ($tokens.Count -eq 1) {",
				"[CompletionResult]::new('db', 'db', [CompletionResultType]::Command, 'Database operations')",
				"switch ($tokens[1].Value) {",
				"'db' {",
				"[CompletionResult]::new('backup', 'backup', [CompletionResultType]::Command, 'Backup database')",
				"[CompletionResult]::new('restore', 'restore', [CompletionResultType]::Command, 'Restore database')",
			},
		},
		{
			name: "flag values with patterns",
			data: CompletionData{
				Flags: []FlagPair{
					{Long: "log-level", Short: "l", Description: "Set log level"},
				},
				FlagValues: map[string][]CompletionValue{
					"log-level": {
						{Pattern: "debug", Description: "Debug logging"},
						{Pattern: "info", Description: "Info logging"},
					},
				},
			},
			expected: []string{
				"switch ($currentToken.ParameterName) {",
				"{$_ -in 'log-level', 'l'} {",
				"[CompletionResult]::new('debug', 'debug', [CompletionResultType]::ParameterValue, 'Debug logging')",
				"[CompletionResult]::new('info', 'info', [CompletionResultType]::ParameterValue, 'Info logging')",
				"}",
			},
		},
		{
			name: "command specific flags",
			data: CompletionData{
				Commands: []string{"test"},
				CommandFlags: map[string][]FlagPair{
					"test": {
						{Long: "test-flag", Short: "t", Description: "Test flag"},
					},
				},
			},
			expected: []string{
				"switch ($tokens[1].Value) {",
				"test) {",
				"[CompletionResult]::new('--test-flag', 'test-flag', [CompletionResultType]::ParameterName, 'Test flag')",
				"[CompletionResult]::new('-t', 't', [CompletionResultType]::ParameterName, 'Test flag')",
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

func TestShellGenerators(t *testing.T) {
	testData := CompletionData{
		Commands: []string{"test", "test sub"},
		CommandDescriptions: map[string]string{
			"test":     "Test command",
			"test sub": "Test subcommand",
		},
		Flags: []FlagPair{
			{Long: "file", Short: "f", Description: "A file flag", Type: FlagTypeFile},
			{Long: "value", Short: "v", Description: "A value flag"},
			{Long: "bool", Short: "b", Description: "A boolean flag"},
			{Long: "list", Short: "l", Description: "A list flag"},
		},
		FlagValues: map[string][]CompletionValue{
			"value": {
				{Pattern: "val1", Description: "First value"},
				{Pattern: "val2", Description: "Second value"},
			},
		},
		CommandFlags: map[string][]FlagPair{
			"test": {
				{Long: "test-file", Short: "t", Description: "Test file flag", Type: FlagTypeFile},
				{Long: "test-value", Description: "Test value flag"},
			},
		},
	}

	tests := []struct {
		name     string
		data     CompletionData
		expected []string
	}{
		{
			name: "bash",
			data: testData,
			expected: []string{
				"_filedir",    // File completion
				"--value|-v)", // Flag with short and long form
				"COMPREPLY=( $(compgen -W \"val1 val2\" -- \"$cur\") )", // Value completion
				"commands=\"$commands test\\ sub\"",                     // Space-escaped subcommand
			},
		},
		{
			name: "zsh",
			data: testData,
			expected: []string{
				// Short option first, then long option
				`"(-f --file)"{-f,--file}"[A file flag]":_files`,
				// Value flag with both short and long forms
				`"(-v --value)"{-v,--value}"[A value flag]":(val1\:"First value" val2\:"Second value")`,
				// Command descriptions
				`"test:Test command"`,
				`"test:(sub:Test subcommand)"`,
			},
		},
		{
			name: "fish",
			data: testData,
			expected: []string{
				// Basic flag completion (order of -l and -s doesn't matter)
				`complete -c testapp -l file -s f -d 'A file flag'`,
				`complete -c testapp -f -l value -s v -d 'A value flag'`,
				// Value completion (include both long and short forms)
				`complete -c testapp -f -l value -s v -n '__fish_seen_argument -l value -s v' -a 'val1' -d 'First value'`,
				// Subcommand completion
				`complete -c testapp -f -n '__fish_seen_subcommand_from test' -a 'sub' -d 'Test subcommand'`,
			},
		},
		{
			name: "powershell",
			data: testData,
			expected: []string{
				// Parameter name completions (flags)
				`[CompletionResult]::new('--file', 'file', [CompletionResultType]::ParameterName, 'A file flag')`,
				`[CompletionResult]::new('-f', 'f', [CompletionResultType]::ParameterName, 'A file flag')`,
				`[CompletionResult]::new('--value', 'value', [CompletionResultType]::ParameterName, 'A value flag')`,

				// Parameter value completions
				`[CompletionResult]::new('val1', 'val1', [CompletionResultType]::ParameterValue, 'First value')`,
				`[CompletionResult]::new('val2', 'val2', [CompletionResultType]::ParameterValue, 'Second value')`,

				// Command completions
				`[CompletionResult]::new('test', 'test', [CompletionResultType]::Command, 'Test command')`,

				// Subcommand completions (when 'test' is the current command)
				`[CompletionResult]::new('sub', 'sub', [CompletionResultType]::Command, 'Test subcommand')`,

				// Command-specific flags
				`[CompletionResult]::new('--test-file', 'test-file', [CompletionResultType]::ParameterName, 'Test file flag')`,
				`[CompletionResult]::new('-t', 't', [CompletionResultType]::ParameterName, 'Test file flag')`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := GetGenerator(tt.name)
			if generator == nil {
				t.Fatalf("No generator found for shell: %s", tt.name)
			}

			result := generator.Generate("testapp", tt.data)
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected completion to contain %q", exp)
					t.Errorf("Generated script:\n%s", result)
				}
			}
		})
	}
}
