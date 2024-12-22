package completion

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

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

func getCompletionPaths(shell string) (CompletionPaths, error) {
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
