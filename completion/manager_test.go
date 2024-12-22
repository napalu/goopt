package completion

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionManager_Accept(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		programName string
		data        CompletionData
		checkScript func(t *testing.T, script string)
	}{
		{
			name:        "bash completion",
			shell:       "bash",
			programName: "mytool",
			data: CompletionData{
				Commands: []string{"test", "test sub"},
				Flags: []FlagPair{
					{Long: "verbose", Short: "v", Description: "Enable verbose output", Type: FlagTypeStandalone},
					{Long: "output", Short: "o", Description: "Output file", Type: FlagTypeFile},
				},
				CommandFlags: map[string][]FlagPair{
					"test": {
						{Long: "test-flag", Short: "t", Description: "Test flag", Type: FlagTypeStandalone},
					},
				},
				FlagValues: map[string][]CompletionValue{
					"output": {{Pattern: "file.txt", Description: "Text file"}},
					"o":      {{Pattern: "file.txt", Description: "Text file"}},
				},
				CommandDescriptions: map[string]string{
					"test":     "Test command",
					"test sub": "Test subcommand",
				},
			},
			checkScript: func(t *testing.T, script string) {
				checks := []struct {
					name    string
					content string
				}{
					{"file completion", "_filedir"}, // Changed to expect _filedir
					{"global short flag", "-v"},
					{"global long flag", "--verbose"},
					{"command short flag", "-t"},
					{"command long flag", "--test-flag"},
					{"subcommand", "test\\ sub"},
					{"command", "test"},
				}

				for _, check := range checks {
					if !strings.Contains(script, check.content) {
						t.Errorf("Missing %s: should contain %q", check.name, check.content)
					}
				}
			},
		},
		{
			name:        "zsh completion",
			shell:       "zsh",
			programName: "mytool",
			data: CompletionData{
				Commands: []string{"test", "test sub"},
				Flags: []FlagPair{
					{Long: "verbose", Short: "v", Description: "Enable verbose output"},
					{Long: "output", Short: "o", Description: "Output file", Type: FlagTypeFile},
				},
				CommandFlags: map[string][]FlagPair{
					"test": {
						{Long: "test-flag", Short: "t", Description: "Test flag"},
					},
				},
				CommandDescriptions: map[string]string{
					"test":     "Test command",
					"test sub": "Test subcommand",
				},
			},
			checkScript: func(t *testing.T, script string) {
				checks := []struct {
					name    string
					content string
				}{
					{"command with description", `"test:Test command"`},
					{"subcommand with description", `"test:(sub:Test subcommand)"`},
					{"global flag with short form", `"(-v --verbose)"{-v,--verbose}"[Enable verbose output]"`},
					{"file flag", `"(-o --output)"{-o,--output}"[Output file]":_files`},
					{"command flag", `"(-t --test-flag)"{-t,--test-flag}"[Test flag]"`},
					{"zsh compdef", "#compdef mytool"},
				}

				for _, check := range checks {
					if !strings.Contains(script, check.content) {
						t.Errorf("Missing %s: should contain %q", check.name, check.content)
					}
				}
			},
		},
		{
			name:        "fish completion",
			shell:       "fish",
			programName: "mytool",
			data: CompletionData{
				Commands: []string{"test", "test sub"},
				Flags: []FlagPair{
					{Long: "verbose", Short: "v", Description: "Enable verbose output", Type: FlagTypeStandalone},
					{Long: "output", Short: "o", Description: "Output file", Type: FlagTypeFile},
				},
				CommandFlags: map[string][]FlagPair{
					"test": {
						{Long: "test-flag", Short: "t", Description: "Test flag", Type: FlagTypeStandalone},
					},
				},
				FlagValues: map[string][]CompletionValue{
					"output": {{Pattern: "file.txt", Description: "Text file"}},
					"o":      {{Pattern: "file.txt", Description: "Text file"}},
				},
				CommandDescriptions: map[string]string{
					"test":     "Test command",
					"test sub": "Test subcommand",
				},
			},
			checkScript: func(t *testing.T, script string) {
				checks := []struct {
					name    string
					content string
				}{
					{"global flag", "-l verbose -s v -d 'Enable verbose output'"},
					{"file flag", "-l output -s o -d 'Output file'"},
					{"command flag", "-l test-flag -s t -d 'Test flag'"},
					{"file completion", "-n '__fish_seen_argument -l output -s o' -a 'file.txt' -d 'Text file'"},
					{"main command", "-n '__fish_use_subcommand' -a 'test' -d 'Test command'"},
					{"subcommand", "-n '__fish_seen_subcommand_from test' -a 'sub' -d 'Test subcommand'"},
				}

				for _, check := range checks {
					if !strings.Contains(script, check.content) {
						t.Errorf("Missing %s: should contain %q", check.name, check.content)
					}
				}
			},
		},
		{
			name:        "powershell completion",
			shell:       "powershell",
			programName: "mytool",
			data: CompletionData{
				Commands: []string{"test", "test sub"},
				Flags: []FlagPair{
					{Long: "verbose", Short: "v", Description: "Enable verbose output"},
					{Long: "output", Short: "o", Description: "Output file"},
				},
				CommandFlags: map[string][]FlagPair{
					"test": {
						{Long: "test-flag", Short: "t", Description: "Test flag"},
					},
				},
				FlagValues: map[string][]CompletionValue{
					"output": {{Pattern: "file.txt", Description: "Text file"}},
					"o":      {{Pattern: "file.txt", Description: "Text file"}},
				},
				CommandDescriptions: map[string]string{
					"test":     "Test command",
					"test sub": "Test subcommand",
				},
			},
			checkScript: func(t *testing.T, script string) {
				checks := []struct {
					name    string
					content string
				}{
					{"registration", "Register-ArgumentCompleter"},
					{"global short flag", "-v"},
					{"global long flag", "--verbose"},
					{"command short flag", "-t"},
					{"command long flag", "--test-flag"},
					{"flag description", "Enable verbose output"},
					{"command", "[CompletionResult]::new('test'"},
					{"subcommand", "[CompletionResult]::new('sub', 'sub', [CompletionResultType]::Command, 'Test subcommand')"},
					{"flag value", "[CompletionResult]::new('file.txt', 'file.txt', [CompletionResultType]::ParameterValue,"},
				}

				for _, check := range checks {
					if !strings.Contains(script, check.content) {
						t.Errorf("Missing %s: should contain %q", check.name, check.content)
					}
				}
			},
		},
		{
			name:        "empty data",
			shell:       "bash",
			programName: "mytool",
			data:        CompletionData{},
			checkScript: func(t *testing.T, script string) {
				if script == "" {
					t.Error("Script should not be empty even with empty data")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := GetGenerator(tt.shell)
			if generator == nil {
				t.Fatalf("Failed to get generator for shell: %s", tt.shell)
			}

			script := generator.Generate(tt.programName, tt.data)
			if script == "" {
				t.Error("Generate() returned empty script")
			}

			if tt.checkScript != nil {
				tt.checkScript(t, script)
			}
		})
	}
}

func TestCompletionManager_getShellFileConventions(t *testing.T) {
	tests := []struct {
		shell         string
		wantPrefix    string
		wantExtension string
		wantErr       bool
	}{
		{"bash", "", "", false},
		{"zsh", "_", "", false},
		{"fish", "", ".fish", false},
		{"powershell", "", ".ps1", false},
		{"invalid", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			manager, err := NewCompletionManager(tt.shell, "mytool")
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCompletionManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			got := manager.getShellFileConventions()
			if got.Prefix != tt.wantPrefix {
				t.Errorf("getShellFileConventions() prefix = %q, want %q", got.Prefix, tt.wantPrefix)
			}
			if got.Extension != tt.wantExtension {
				t.Errorf("getShellFileConventions() extension = %q, want %q", got.Extension, tt.wantExtension)
			}
		})
	}
}

func TestCompletionManager_SaveCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		shell       string
		programName string
		setup       func(*CompletionManager)
		checkFile   func(*testing.T, string)
		wantErr     bool
	}{
		{
			name:        "bash save",
			shell:       "bash",
			programName: "mytool",
			setup: func(cm *CompletionManager) {
				cm.script = "test script"
			},
			checkFile: func(t *testing.T, path string) {
				if !strings.HasSuffix(path, "mytool") {
					t.Error("Bash completion file should not have extension")
				}
			},
		},
		{
			name:        "zsh save",
			shell:       "zsh",
			programName: "mytool",
			setup: func(cm *CompletionManager) {
				cm.script = "test script"
			},
			checkFile: func(t *testing.T, path string) {
				if !strings.HasPrefix(filepath.Base(path), "_") {
					t.Error("ZSH completion file should start with _")
				}
			},
		},
		{
			name:        "fish save",
			shell:       "fish",
			programName: "mytool",
			setup: func(cm *CompletionManager) {
				cm.script = "test script"
			},
			checkFile: func(t *testing.T, path string) {
				if !strings.HasSuffix(path, ".fish") {
					t.Error("Fish completion file should have .fish extension")
				}
			},
		},
		{
			name:        "powershell save",
			shell:       "powershell",
			programName: "mytool",
			setup: func(cm *CompletionManager) {
				cm.script = "test script"
			},
			checkFile: func(t *testing.T, path string) {
				if !strings.HasSuffix(path, ".ps1") {
					t.Error("PowerShell completion file should have .ps1 extension")
				}
			},
		},
		{
			name:        "no script",
			shell:       "bash",
			programName: "mytool",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewCompletionManager(tt.shell, tt.programName)
			if err != nil {
				t.Fatal(err)
			}

			// Override paths for testing
			manager.Paths.Primary = filepath.Join(tmpDir, tt.name)
			manager.Paths.Fallback = filepath.Join(tmpDir, tt.name+"_fallback")

			if tt.setup != nil {
				tt.setup(manager)
			}

			err = manager.SaveCompletion()
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveCompletion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFile != nil {
				path := manager.getCompletionFilePath()
				tt.checkFile(t, path)
			}
		})
	}
}
