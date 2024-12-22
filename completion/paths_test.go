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
			name:  "powershell paths",
			shell: "powershell",
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
			},
		},
		{
			name:  "fish paths",
			shell: "fish",
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
			name:  "zsh paths",
			shell: "zsh",
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
			paths, err := getCompletionPaths(tt.shell)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCompletionPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkPaths != nil {
				tt.checkPaths(t, paths)
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
