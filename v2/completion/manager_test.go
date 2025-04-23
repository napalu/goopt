package completion

import (
	"os"
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
			manager, err := NewManager(tt.shell, "mytool")
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
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
		setup       func(*Manager)
		checkFile   func(*testing.T, string)
		wantErr     bool
	}{
		{
			name:        "bash save",
			shell:       "bash",
			programName: "mytool",
			setup: func(cm *Manager) {
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
			setup: func(cm *Manager) {
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
			setup: func(cm *Manager) {
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
			setup: func(cm *Manager) {
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
			manager, err := NewManager(tt.shell, tt.programName)
			if err != nil {
				t.Fatal(err)
			}

			manager.Paths.Primary = filepath.Join(tmpDir, tt.name)
			manager.Paths.Fallback = filepath.Join(tmpDir, tt.name+"_fallback")

			if tt.setup != nil {
				tt.setup(manager)
			}

			path, err := manager.Save()
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveCompletion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFile != nil {
				tt.checkFile(t, path)
			}
		})
	}
}

func TestCompletionManager_IsShellSupported(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		programName string
		want        bool
	}{
		{
			name:        "bash is supported",
			shell:       "bash",
			programName: "mytool",
			want:        true,
		},
		{
			name:        "zsh is supported",
			shell:       "zsh",
			programName: "mytool",
			want:        true,
		},
		{
			name:        "powershell is supported",
			shell:       "powershell",
			programName: "mytool",
			want:        true,
		},
		{
			name:        "unsupported shell",
			shell:       "invalid-shell",
			programName: "mytool",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := NewManager(tt.shell, tt.programName)
			if err != nil && tt.want {
				t.Fatalf("NewManager() error = %v", err)
			}
			if err == nil && !tt.want {
				t.Fatal("NewManager() expected error for unsupported shell")
			}
			if cm != nil && cm.IsShellSupported() != tt.want {
				t.Errorf("IsShellSupported() = %v, want %v", cm.IsShellSupported(), tt.want)
			}
		})
	}
}

func TestCompletionManager_HasExistingCompletion(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		shell          string
		programName    string
		setupFunc      func(t *testing.T, cm *Manager)
		wantExists     bool
		wantPathSuffix string
	}{
		{
			name:        "no completion file exists",
			shell:       "bash",
			programName: "mytool",
			setupFunc: func(t *testing.T, cm *Manager) {
				cm.Paths.Primary = filepath.Join(tempDir, "nonexistent")
			},
			wantExists: false,
		},
		{
			name:        "completion file exists in primary path",
			shell:       "bash",
			programName: "mytool",
			setupFunc: func(t *testing.T, cm *Manager) {
				cm.Paths.Primary = filepath.Join(tempDir, "primary")
				completionPath := cm.getCompletionFilePath(cm.Paths.Primary)
				if err := os.MkdirAll(filepath.Dir(completionPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(completionPath, []byte("test completion"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantExists:     true,
			wantPathSuffix: "mytool",
		},
		{
			name:        "empty completion file",
			shell:       "bash",
			programName: "mytool",
			setupFunc: func(t *testing.T, cm *Manager) {
				cm.Paths.Primary = filepath.Join(tempDir, "empty")
				completionPath := cm.getCompletionFilePath(cm.Paths.Primary)
				if err := os.MkdirAll(filepath.Dir(completionPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(completionPath, []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantExists: false,
		},
		{
			name:        "fallback path has completion",
			shell:       "bash",
			programName: "mytool",
			setupFunc: func(t *testing.T, cm *Manager) {
				cm.Paths.Primary = filepath.Join(tempDir, "primary")
				cm.Paths.Fallback = filepath.Join(tempDir, "fallback")
				completionPath := cm.getCompletionFilePath(cm.Paths.Fallback)
				if err := os.MkdirAll(filepath.Dir(completionPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(completionPath, []byte("test completion"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantExists:     true,
			wantPathSuffix: "mytool",
		},
		{
			name:        "primary path empty but fallback has completion",
			shell:       "bash",
			programName: "mytool",
			setupFunc: func(t *testing.T, cm *Manager) {
				cm.Paths.Primary = ""
				cm.Paths.Fallback = filepath.Join(tempDir, "fallback")
				if err := os.MkdirAll(cm.Paths.Fallback, 0755); err != nil {
					t.Fatal(err)
				}
				completionPath := cm.getCompletionFilePath(cm.Paths.Fallback)
				if err := os.WriteFile(completionPath, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantExists:     false,
			wantPathSuffix: "",
		},
		{
			name:        "both paths empty",
			shell:       "bash",
			programName: "mytool",
			setupFunc: func(t *testing.T, cm *Manager) {
				cm.Paths.Primary = ""
				cm.Paths.Fallback = ""
			},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := NewManager(tt.shell, tt.programName)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			tt.setupFunc(t, cm)

			gotPath, gotExists := cm.HasExistingCompletion()
			if gotExists != tt.wantExists {
				t.Errorf("HasExistingCompletion() exists '%s' = %v, want %v", tt.name, gotExists, tt.wantExists)
			}
			if tt.wantExists && !strings.HasSuffix(gotPath, tt.wantPathSuffix) {
				t.Errorf("HasExistingCompletion() path '%s' = %v, want suffix %v", tt.name, gotPath, tt.wantPathSuffix)
			}
		})
	}
}
