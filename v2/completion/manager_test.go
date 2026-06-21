package completion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
