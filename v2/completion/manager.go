package completion

import (
	"fmt"
	"os"
	"path/filepath"
)

// Manager installs a completion script for a given shell at the shell's conventional
// location. The script itself is produced by the parser (a runtime-completion stub);
// Manager owns only the file-system concerns — where it goes and writing it.
type Manager struct {
	Shell       string          // The shell to install completion for
	ProgramName string          // The name of the program
	Paths       CompletionPaths // The paths where the completion script will be saved
	script      string          // The completion script to install
}

// NewManager creates a completion manager for a given shell. It fails for shells with
// no known completion-path convention.
func NewManager(shell, programName string) (*Manager, error) {
	paths, err := getCompletionPaths(shell)
	if err != nil {
		return nil, fmt.Errorf("failed to get completion paths: %w", err)
	}

	return &Manager{
		Shell:       shell,
		ProgramName: filepath.Base(programName),
		Paths:       paths,
	}, nil
}

// Accept stores the completion script to be installed (e.g. a stub from
// Parser.GenerateCompletionStub).
func (cm *Manager) Accept(script string) {
	cm.script = script
}

// Save saves the previously generated completion script
func (cm *Manager) Save() (path string, err error) {
	if cm.script == "" {
		return path, fmt.Errorf("no completion script generated")
	}

	path = cm.Paths.Primary
	if err := cm.ensureCompletionPath(path); err != nil {
		if cm.Paths.Fallback != "" {
			if err := cm.ensureCompletionPath(cm.Paths.Fallback); err != nil {
				return cm.Paths.Fallback, err
			}
			path = cm.Paths.Fallback
		} else {
			return path, err
		}
	}

	completionPath := cm.getCompletionFilePath(path)
	if err := os.WriteFile(completionPath, []byte(cm.script), 0644); err != nil {
		err = fmt.Errorf("failed to write completion file: %w", err)
		return completionPath, err
	}

	err = ensurePermission(completionPath, 0644)
	if err != nil {
		return completionPath, err
	}

	return completionPath, nil
}

// IsShellSupported returns whether the current shell has completion generation support
func (cm *Manager) IsShellSupported() bool {
	switch cm.Shell {
	case "bash", "zsh", "fish", "powershell":
		return true
	default:
		return false
	}
}

// HasExistingCompletion checks if a completion script is already installed
// Returns the path to the existing completion file and whether it exists
func (cm *Manager) HasExistingCompletion() (string, bool) {
	return cm.checkCompletionFile(cm.Paths.Primary)
}

func (cm *Manager) checkCompletionFile(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	completionPath := cm.getCompletionFilePath(path)
	st, err := os.Stat(completionPath)
	if err == nil && !st.IsDir() && st.Size() > 0 {
		// Try to open the file to verify permissions
		f, err := os.Open(completionPath)
		if err == nil {
			f.Close()
			return completionPath, true
		}
	}

	// If primary path doesn't have a completion, try fallback
	if path == cm.Paths.Primary && cm.Paths.Fallback != "" {
		return cm.checkCompletionFile(cm.Paths.Fallback)
	}

	return "", false
}

func (cm *Manager) ensureCompletionPath(path string) error {
	perm := os.FileMode(0755)
	err := os.MkdirAll(path, perm)
	if err != nil {
		return fmt.Errorf("failed to create completion path directory %s: %w", path, err)
	}

	err = ensurePermission(path, perm)
	if err == nil {
		return nil
	}

	return fmt.Errorf("failed to create completion directories: %w", err)
}

func (cm *Manager) getShellFileConventions() CompletionFileInfo {
	switch cm.Shell {
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

func (cm *Manager) getCompletionFilePath(path string) string {
	conventions := cm.getShellFileConventions()
	filename := conventions.Prefix + cm.ProgramName + conventions.Extension
	return filepath.Join(path, filename)
}
