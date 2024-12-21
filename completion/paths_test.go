// completion/paths_test.go
package completion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetCompletionPaths(t *testing.T) {
	tests := []struct {
		name       string
		shell      string
		wantErr    bool
		checkPaths func(t *testing.T, paths CompletionPaths)
	}{
		{
			name:  "bash paths",
			shell: "bash",
			checkPaths: func(t *testing.T, paths CompletionPaths) {
				if !filepath.IsAbs(paths.Primary) {
					t.Error("Primary path should be absolute")
				}
				if !strings.Contains(paths.Primary, filepath.Join("bash-completion", "completions")) {
					t.Error("Expected bash completion path")
				}
			},
		},
		{
			name:    "invalid shell",
			shell:   "invalid",
			wantErr: true,
		},
		{
			name:    "powershell",
			shell:   "powershell",
			wantErr: false,
			checkPaths: func(t *testing.T, paths CompletionPaths) {
				if !filepath.IsAbs(paths.Primary) {
					t.Error("Primary path should be absolute")
				}

				switch runtime.GOOS {
				case "windows":
					if !strings.Contains(paths.Primary, "Documents\\PowerShell") {
						t.Error("Expected Windows PowerShell path in Documents")
					}
				case "darwin":
					if !strings.Contains(paths.Primary, "Library/PowerShell") {
						t.Error("Expected macOS PowerShell path in Library")
					}
				default: // linux and others
					if !strings.Contains(paths.Primary, ".config/powershell") {
						t.Error("Expected Linux PowerShell path in .config")
					}
				}

				if paths.Extension != ".ps1" {
					t.Error("Expected .ps1 extension for PowerShell")
				}
			},
		},
		{
			name:    "fish",
			shell:   "fish",
			wantErr: false,
			checkPaths: func(t *testing.T, paths CompletionPaths) {
				if !filepath.IsAbs(paths.Primary) {
					t.Error("Primary path should be absolute")
				}
				if !strings.Contains(paths.Primary, "fish") {
					t.Error("Expected fish completion path")
				}
				if paths.Extension != ".fish" {
					t.Error("Expected .fish extension")
				}
			},
		},
		{
			name:    "zsh",
			shell:   "zsh",
			wantErr: false,
			checkPaths: func(t *testing.T, paths CompletionPaths) {
				if !filepath.IsAbs(paths.Primary) {
					t.Error("Primary path should be absolute")
				}
				if !strings.Contains(paths.Primary, "zsh") {
					t.Error("Expected zsh completion path")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := GetCompletionPaths(tt.shell)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCompletionPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkPaths != nil {
				tt.checkPaths(t, paths)
			}
		})
	}
}

func TestEnsureCompletionPath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		paths   CompletionPaths
		setup   func() error
		wantErr bool
	}{
		{
			name: "create new directory",
			paths: CompletionPaths{
				Primary: filepath.Join(tmpDir, "new_dir"),
			},
			wantErr: false,
		},
		{
			name: "use existing directory",
			paths: CompletionPaths{
				Primary: filepath.Join(tmpDir, "existing_dir"),
			},
			setup: func() error {
				return os.MkdirAll(filepath.Join(tmpDir, "existing_dir"), 0755)
			},
			wantErr: false,
		},
		{
			name: "fallback on permission error",
			paths: CompletionPaths{
				Primary:  filepath.Join(tmpDir, "noperm"),
				Fallback: filepath.Join(tmpDir, "fallback"),
			},
			setup: func() error {
				// Create directory with no write permission
				if runtime.GOOS == "windows" {
					return nil
				}
				if err := os.MkdirAll(filepath.Join(tmpDir, "noperm"), 0555); err != nil {
					return err
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatal(err)
				}
			}

			err := EnsureCompletionPath(tt.paths)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureCompletionPath() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check directory exists and has correct permissions
			if !tt.wantErr {
				path := tt.paths.Primary
				if _, err := os.Stat(path); os.IsNotExist(err) {
					path = tt.paths.Fallback
				}

				info, err := os.Stat(path)
				if err != nil {
					t.Errorf("Failed to stat path: %v", err)
					return
				}

				if runtime.GOOS != "windows" {
					if perm := info.Mode().Perm(); perm != 0755 {
						t.Errorf("Wrong permissions: got %o, want %o", perm, 0755)
					}
				}
			}
		})
	}
}

func TestSafeSaveCompletion(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		programName string
		content     []byte
		setup       func() error
		wantErr     bool
		checkFile   func(t *testing.T, path string)
	}{
		{
			name:        "basic save",
			shell:       "bash",
			programName: "mytool",
			content:     []byte("# completion script"),
			checkFile: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(content) != "# completion script" {
					t.Error("Wrong content")
				}

				if runtime.GOOS != "windows" {
					info, err := os.Stat(path)
					if err != nil {
						t.Errorf("Failed to stat file: %v", err)
						return
					}
					if perm := info.Mode().Perm(); perm != 0644 {
						t.Errorf("Wrong permissions: got %o, want %o", perm, 0644)
					}
				}
			},
		},
		{
			name:        "full path gets base name",
			shell:       "bash",
			programName: "/usr/local/bin/mytool",
			content:     []byte("# completion script"),
			checkFile: func(t *testing.T, path string) {
				// Should use just "mytool" as the completion script name
				if !strings.HasSuffix(path, "mytool") {
					t.Errorf("Expected completion script named 'mytool', got path: %s", path)
				}
			},
		},
		{
			name:    "invalid shell",
			shell:   "invalid",
			content: []byte("# completion script"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatal(err)
				}
			}

			err := SafeSaveCompletion(tt.shell, tt.programName, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeSaveCompletion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFile != nil {
				paths, _ := GetCompletionPaths(tt.shell)
				path := GetCompletionFilePath(paths, tt.shell, tt.programName)
				tt.checkFile(t, path)
			}
		})
	}
}

func TestGetCompletionFilePath(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		programName string
		want        string
		wantSuffix  string
	}{
		{
			name:        "bash basic",
			shell:       "bash",
			programName: "mytool",
			wantSuffix:  "mytool", // Just the name
		},
		{
			name:        "bash with path",
			shell:       "bash",
			programName: "/usr/local/bin/mytool",
			wantSuffix:  "mytool", // Just the base name
		},
		{
			name:        "zsh basic",
			shell:       "zsh",
			programName: "mytool",
			wantSuffix:  "_mytool", // With underscore prefix
		},
		{
			name:        "fish basic",
			shell:       "fish",
			programName: "mytool",
			wantSuffix:  "mytool.fish", // With .fish extension
		},
		{
			name:        "powershell basic",
			shell:       "powershell",
			programName: "mytool",
			wantSuffix:  "mytool.ps1", // With .ps1 extension
		},
		{
			name:        "zsh with path",
			shell:       "zsh",
			programName: "/usr/local/bin/mytool",
			wantSuffix:  "_mytool", // Should still get underscore prefix
		},
		{
			name:        "fish with dots",
			shell:       "fish",
			programName: "my.tool",
			wantSuffix:  "my.tool.fish", // Should handle dots correctly
		},
		{
			name:        "powershell with spaces",
			shell:       "powershell",
			programName: "My Tool",
			wantSuffix:  "My Tool.ps1", // Should preserve spaces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := GetCompletionPaths(tt.shell)
			if err != nil {
				t.Fatalf("GetCompletionPaths failed: %v", err)
			}

			got := GetCompletionFilePath(paths, tt.shell, tt.programName)

			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("GetCompletionFilePath() got = %v, want suffix %v", got, tt.wantSuffix)
			}

			// Check that the path is absolute
			if !filepath.IsAbs(got) {
				t.Error("Expected absolute path")
			}
		})
	}
}

func TestEnsurePermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests not applicable on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test")

	tests := []struct {
		name    string
		setup   func() error
		perm    os.FileMode
		wantErr bool
	}{
		{
			name: "fix restrictive permissions",
			setup: func() error {
				return os.WriteFile(testFile, []byte("test"), 0600)
			},
			perm:    0644,
			wantErr: false,
		},
		{
			name: "maintain correct permissions",
			setup: func() error {
				return os.WriteFile(testFile, []byte("test"), 0644)
			},
			perm:    0644,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(); err != nil {
				t.Fatal(err)
			}

			err := ensurePermission(testFile, tt.perm)
			if (err != nil) != tt.wantErr {
				t.Errorf("ensurePermission() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				info, err := os.Stat(testFile)
				if err != nil {
					t.Fatal(err)
				}
				if perm := info.Mode().Perm(); perm != tt.perm {
					t.Errorf("Wrong permissions: got %o, want %o", perm, tt.perm)
				}
			}
		})
	}
}
