package translations

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findGoModPath finds the nearest go.mod file starting from the given directory
func findGoModPath(startDir string) (string, error) {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// getModuleName extracts the module name from go.mod
func getModuleName(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			// Extract module name
			moduleName := strings.TrimPrefix(line, "module ")
			moduleName = strings.TrimSpace(moduleName)
			// Remove any comments
			if idx := strings.Index(moduleName, "//"); idx >= 0 {
				moduleName = strings.TrimSpace(moduleName[:idx])
			}
			return moduleName, nil
		}
	}

	return "", fmt.Errorf("module directive not found in go.mod")
}

// resolvePackagePath resolves the package path based on the module context
func resolvePackagePath(packageName string, workingDir string) (string, error) {
	// If it's already a full import path (contains dots), use as-is
	if strings.Contains(packageName, ".") || strings.Contains(packageName, "/") {
		return packageName, nil
	}

	// Find go.mod
	goModPath, err := findGoModPath(workingDir)
	if err != nil {
		// No go.mod found, can't resolve
		return packageName, nil
	}

	// Get module name
	moduleName, err := getModuleName(goModPath)
	if err != nil {
		return packageName, err
	}

	// Calculate relative path from go.mod to working directory
	goModDir := filepath.Dir(goModPath)
	relPath, err := filepath.Rel(goModDir, workingDir)
	if err != nil {
		return packageName, err
	}

	// Build the full import path
	var parts []string
	parts = append(parts, moduleName)

	if relPath != "." && relPath != "" {
		// Convert file path separators to forward slashes for import paths
		relPath = filepath.ToSlash(relPath)
		parts = append(parts, relPath)
	}

	parts = append(parts, packageName)

	return strings.Join(parts, "/"), nil
}
