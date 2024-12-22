package completion

import (
	"fmt"
	"os"
	"path/filepath"
)

type CompletionManager struct {
	Shell       string
	ProgramName string
	Paths       CompletionPaths
	generator   Generator
	script      string
}

// NewCompletionManager creates a completion manager which can be used to manage and save completion scripts for a given shell
func NewCompletionManager(shell, programName string) (*CompletionManager, error) {
	paths, err := getCompletionPaths(shell)
	if err != nil {
		return nil, fmt.Errorf("failed to get completion paths: %w", err)
	}

	return &CompletionManager{
		Shell:       shell,
		ProgramName: filepath.Base(programName),
		Paths:       paths,
		generator:   GetGenerator(shell),
	}, nil
}

// Accept generates and stores the completion script from the provided data
func (cm *CompletionManager) Accept(data CompletionData) {
	cm.script = cm.generator.Generate(cm.ProgramName, data)
}

// SaveCompletion saves the previously generated completion script
func (cm *CompletionManager) SaveCompletion() error {
	if cm.script == "" {
		return fmt.Errorf("no completion script generated")
	}

	if err := cm.ensureCompletionPath(); err != nil {
		return err
	}

	filepath := cm.getCompletionFilePath()
	if err := os.WriteFile(filepath, []byte(cm.script), 0644); err != nil {
		return fmt.Errorf("failed to write completion file: %w", err)
	}

	return ensurePermission(filepath, 0644)
}

func (cm *CompletionManager) ensureCompletionPath() error {
	perm := os.FileMode(0755)
	err := os.MkdirAll(cm.Paths.Primary, perm)
	if err != nil {
		return fmt.Errorf("failed to create primary completion directory: %w", err)
	}

	err = ensurePermission(cm.Paths.Primary, perm)
	if err == nil {
		return nil
	}

	if cm.Paths.Fallback != "" {
		err = os.MkdirAll(cm.Paths.Fallback, perm)
		if err != nil {
			return fmt.Errorf("failed to create fallback completion directory: %w", err)
		}
		return ensurePermission(cm.Paths.Fallback, perm)
	}

	return fmt.Errorf("failed to create completion directories: %w", err)
}

func (cm *CompletionManager) getShellFileConventions() CompletionFileInfo {
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

func (cm *CompletionManager) getCompletionFilePath() string {
	conventions := cm.getShellFileConventions()
	filename := conventions.Prefix + cm.ProgramName + conventions.Extension
	return filepath.Join(cm.Paths.Primary, filename)
}
