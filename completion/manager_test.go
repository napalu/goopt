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
				Commands: []string{"test"},
				Flags:    []string{"-v", "--verbose", "-o", "--output"},
				Descriptions: map[string]string{
					"-v":        "Enable verbose output (short)",
					"--verbose": "Enable verbose output (long)",
					"-o":        "Output file (short)",
					"--output":  "Output file (long)",
				},
			},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "mytool") {
					t.Error("Script should contain program name")
				}
				if !strings.Contains(script, "-v") {
					t.Error("Script should contain short flags")
				}
				if !strings.Contains(script, "--verbose") {
					t.Error("Script should contain long flags")
				}
				if !strings.Contains(script, "Enable verbose output") {
					t.Error("Script should contain flag descriptions")
				}
			},
		},
		{
			name:        "zsh completion",
			shell:       "zsh",
			programName: "mytool",
			data: CompletionData{
				Commands: []string{"test"},
				Flags:    []string{"-v", "--verbose", "-o", "--output"},
				Descriptions: map[string]string{
					"-v":        "Enable verbose output (short)",
					"--verbose": "Enable verbose output (long)",
					"-o":        "Output file (short)",
					"--output":  "Output file (long)",
				},
			},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "#compdef mytool") {
					t.Error("ZSH script should start with #compdef")
				}
				if !strings.Contains(script, "-v") {
					t.Error("Script should contain short flags")
				}
				if !strings.Contains(script, "--verbose") {
					t.Error("Script should contain long flags")
				}
			},
		},
		{
			name:        "fish completion",
			shell:       "fish",
			programName: "mytool",
			data: CompletionData{
				Commands: []string{"test", "test sub"},
				Flags:    []string{"-v", "--verbose", "-o", "--output"},
				CommandFlags: map[string][]string{
					"test": {"-t", "--test-flag"},
				},
				Descriptions: map[string]string{
					"-v":               "Enable verbose output (short)",
					"--verbose":        "Enable verbose output (long)",
					"-o":               "Output file (short)",
					"--output":         "Output file (long)",
					"test@-t":          "Test flag (short)",
					"test@--test-flag": "Test flag (long)",
				},
				CommandDescriptions: map[string]string{
					"test":     "Test command",
					"test sub": "Test subcommand",
				},
				FlagValues: map[string][]CompletionValue{
					"--output": {
						{Pattern: "file.txt", Description: "Text file"},
					},
					"-o": {
						{Pattern: "file.txt", Description: "Text file"},
					},
				},
			},
			checkScript: func(t *testing.T, script string) {
				t.Logf("Generated fish script:\n%s", script)
				checks := []struct {
					name    string
					content string
				}{
					{"global short flag", "-s v -d 'Enable verbose output"},
					{"global long flag", "-l verbose -d 'Enable verbose output"},
					{"main command", "__fish_use_subcommand' -a 'test' -d 'Test command"},
					{"subcommand", "__fish_seen_subcommand_from test' -a 'sub' -d 'Test subcommand"},
					{"command short flag", "__fish_seen_subcommand_from test' -s t -d 'Test flag"},
					{"command long flag", "__fish_seen_subcommand_from test' -l test-flag -d 'Test flag"},
					{"short flag value", "__fish_seen_argument -s o' -a 'file.txt' -d 'Text file"},
					{"long flag value", "__fish_seen_argument -l output' -a 'file.txt' -d 'Text file"},
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
				Commands: []string{"test"},
				Flags:    []string{"-v", "--verbose", "-o", "--output"},
				Descriptions: map[string]string{
					"-v":        "Enable verbose output (short)",
					"--verbose": "Enable verbose output (long)",
					"-o":        "Output file (short)",
					"--output":  "Output file (long)",
				},
			},
			checkScript: func(t *testing.T, script string) {
				if !strings.Contains(script, "Register-ArgumentCompleter") {
					t.Error("PowerShell script should use Register-ArgumentCompleter")
				}
				if !strings.Contains(script, "-v") {
					t.Error("Script should contain short flags")
				}
				if !strings.Contains(script, "--verbose") {
					t.Error("Script should contain long flags")
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
			manager, err := NewCompletionManager(tt.shell, tt.programName)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}

			manager.Accept(tt.data)

			if manager.script == "" {
				t.Error("Accept() did not generate a script")
			}

			if tt.checkScript != nil {
				tt.checkScript(t, manager.script)
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
				cm.Accept(CompletionData{Commands: []string{"test"}})
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
				cm.Accept(CompletionData{Commands: []string{"test"}})
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
				cm.Accept(CompletionData{Commands: []string{"test"}})
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
				cm.Accept(CompletionData{Commands: []string{"test"}})
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
