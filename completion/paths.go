package completion

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// GetCompletionPaths returns the appropriate completion paths for the given shell
func GetCompletionPaths(shell string) (CompletionPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return CompletionPaths{}, fmt.Errorf("couldn't get user home directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return getWindowsCompletionPaths(home, shell)
	case "darwin":
		return getDarwinCompletionPaths(home, shell)
	default:
		return getLinuxCompletionPaths(home, shell)
	}
}

// EnsureCompletionPath creates the completion directory if it doesn't exist
func EnsureCompletionPath(paths CompletionPaths) error {
	perm := os.FileMode(0755)
	err := os.MkdirAll(paths.Primary, perm)
	if err != nil {
		return fmt.Errorf("failed to create primary completion directory: %w", err)
	}

	err = ensurePermission(paths.Primary, perm)
	if err == nil {
		return nil
	}

	if paths.Fallback != "" {
		err = os.MkdirAll(paths.Fallback, perm)
		if err != nil {
			return fmt.Errorf("failed to create fallback completion directory: %w", err)
		}
		err = ensurePermission(paths.Fallback, perm)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to create completion directories: %w", err)
}

// GetCompletionFilePath returns the proper filename for the completion script
func GetCompletionFilePath(paths CompletionPaths, shell, programName string) string {
	// Always use base name
	baseName := filepath.Base(programName)

	conventions := GetShellFileConventions(shell)

	filename := conventions.Prefix + baseName + conventions.Extension

	return filepath.Join(paths.Primary, filename)
}

// SafeSaveCompletion ensures the completion directory exists and saves the completion script
func SafeSaveCompletion(shell, programName string, content []byte) error {
	// Extract base name for completion script
	programName = filepath.Base(programName)

	paths, err := GetCompletionPaths(shell)
	if err != nil {
		return err
	}

	err = EnsureCompletionPath(paths)
	if err != nil {
		return err
	}

	filepath := GetCompletionFilePath(paths, shell, programName)
	err = os.WriteFile(filepath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write completion file: %w", err)
	}

	err = ensurePermission(filepath, 0644)
	if err != nil {
		return fmt.Errorf("failed to ensure completion file permissions: %w", err)
	}

	return nil
}

// GetShellFileConventions returns the naming conventions for each shell
func GetShellFileConventions(shell string) CompletionFileInfo {
	switch shell {
	case "bash":
		return CompletionFileInfo{
			Prefix:    "", // No prefix needed
			Extension: "", // No extension needed
			Comment:   "Bash completion files are typically just the command name",
		}

	case "zsh":
		return CompletionFileInfo{
			Prefix:    "_", // zsh completions typically start with underscore
			Extension: "",  // No extension needed
			Comment:   "Zsh completion files should start with _ (e.g., _git)",
		}

	case "fish":
		return CompletionFileInfo{
			Prefix:    "",      // No prefix needed
			Extension: ".fish", // .fish extension required
			Comment:   "Fish completion files must end in .fish",
		}

	case "powershell":
		return CompletionFileInfo{
			Prefix:    "",     // No prefix needed
			Extension: ".ps1", // .ps1 extension required
			Comment:   "PowerShell completion files must end in .ps1",
		}

	default:
		return CompletionFileInfo{}
	}
}

func ensurePermission(path string, perm os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if runtime.GOOS == "windows" {
		return nil
	}

	actualPerm := info.Mode().Perm()
	if actualPerm != perm {
		if err := os.Chmod(path, perm); err != nil {
			return fmt.Errorf("failed to set permissions on %s from %o to %o: %w",
				path, actualPerm, perm, err)
		}
	}

	return nil
}

func isPowerShellCore() bool {
	_, err := exec.LookPath("pwsh")
	return err == nil
}

func getWindowsCompletionPaths(home, shell string) (CompletionPaths, error) {
	switch shell {
	case "powershell":
		if isPowerShellCore() {
			return CompletionPaths{
				Primary:   filepath.Join(home, "Documents", "PowerShell", "Completions"),
				Fallback:  filepath.Join(home, ".config", "powershell", "Completions"),
				Extension: ".ps1",
				Comment:   "PowerShell Core user completions directory",
			}, nil
		}
		return CompletionPaths{
			Primary:   filepath.Join(home, "Documents", "WindowsPowerShell", "Completions"),
			Fallback:  filepath.Join(home, ".config", "WindowsPowerShell", "Completions"),
			Extension: ".ps1",
			Comment:   "Windows PowerShell user completions directory",
		}, nil

	case "bash":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".local", "share", "bash-completion", "completions"),
			Fallback:  filepath.Join(home, ".bash_completion.d"),
			Extension: "",
			Comment:   "Git Bash user completions directory",
		}, nil

	case "zsh":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".zsh", "completion"),
			Fallback:  filepath.Join(home, ".zfunc"),
			Extension: "",
			Comment:   "Zsh user completions directory (WSL/Cygwin)",
		}, nil

	case "fish":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".config", "fish", "completions"),
			Fallback:  filepath.Join(home, ".local", "share", "fish", "completions"),
			Extension: ".fish",
			Comment:   "Fish user completions directory",
		}, nil

	default:
		return CompletionPaths{}, fmt.Errorf("unsupported shell: %s", shell)
	}
}

func getDarwinCompletionPaths(home, shell string) (CompletionPaths, error) {
	switch shell {
	case "bash":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".local", "share", "bash-completion", "completions"),
			Fallback:  filepath.Join(home, ".bash_completion.d"),
			Extension: "",
			Comment:   "User-local bash completions, compatible with bash-completion@2",
		}, nil

	case "zsh":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".zsh", "completion"),
			Fallback:  filepath.Join(home, ".zfunc"),
			Extension: "",
			Comment:   "User-local zsh completions directory",
		}, nil

	case "fish":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".config", "fish", "completions"),
			Fallback:  filepath.Join(home, ".local", "share", "fish", "completions"),
			Extension: ".fish",
			Comment:   "Fish user completions directory",
		}, nil

	case "powershell":
		return CompletionPaths{
			Primary:   filepath.Join(home, "Library", "PowerShell", "Completions"),
			Fallback:  filepath.Join(home, ".config", "powershell", "Completions"),
			Extension: ".ps1",
			Comment:   "PowerShell Core user completions directory",
		}, nil

	default:
		return CompletionPaths{}, fmt.Errorf("unsupported shell: %s", shell)
	}
}

func getLinuxCompletionPaths(home, shell string) (CompletionPaths, error) {
	switch shell {
	case "bash":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".local", "share", "bash-completion", "completions"),
			Fallback:  filepath.Join(home, ".bash_completion.d"),
			Extension: "",
			Comment:   "XDG-compatible user-local bash completions directory",
		}, nil

	case "zsh":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".zsh", "completion"),
			Fallback:  filepath.Join(home, ".zfunc"),
			Extension: "",
			Comment:   "User-local zsh completions directory",
		}, nil

	case "fish":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".config", "fish", "completions"),
			Fallback:  filepath.Join(home, ".local", "share", "fish", "completions"),
			Extension: ".fish",
			Comment:   "Fish user completions directory",
		}, nil

	case "powershell":
		return CompletionPaths{
			Primary:   filepath.Join(home, ".config", "powershell", "Completions"),
			Fallback:  filepath.Join(home, ".local", "share", "powershell", "Completions"),
			Extension: ".ps1",
			Comment:   "PowerShell Core user completions directory",
		}, nil

	default:
		return CompletionPaths{}, fmt.Errorf("unsupported shell: %s", shell)
	}
}
