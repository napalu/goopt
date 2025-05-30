package translations

import (
	"encoding/json"
	"fmt"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/options"
	"os"
	"path/filepath"
	"strings"
)

func Init(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*options.AppConfig](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}
	if len(cfg.Input) == 0 {
		// Default to locales/en.json
		cfg.Input = []string{"locales/en.json"}
	}

	// Initialize each specified file
	for _, inputFile := range cfg.Input {
		// Check if file exists
		if _, err := os.Stat(inputFile); err == nil && !cfg.Init.Force {
			fmt.Println(cfg.TR.T(messages.Keys.AppInit.FileExists, inputFile))
			continue
		}

		// Create directory if needed
		dir := filepath.Dir(inputFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToCreateDir), dir, err)
		}

		// Create initial JSON with some example keys
		initialData := map[string]string{
			"app.name":        "My Application",
			"app.description": "Application description",
			"app.version":     "Version",
		}

		data, err := json.MarshalIndent(initialData, "", "  ")
		if err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToMarshal), err)
		}

		if err := os.WriteFile(inputFile, data, 0644); err != nil {
			return fmt.Errorf(cfg.TR.T(messages.Keys.AppError.FailedToCreateFile), inputFile, err)
		}

		fmt.Println(cfg.TR.T(messages.Keys.AppInit.CreatedFile, inputFile))
	}

	if len(cfg.Input) > 0 {
		fmt.Println()
		fmt.Println(cfg.TR.T(messages.Keys.AppInit.NextSteps))
		fmt.Printf("1. %s\n", cfg.TR.T(messages.Keys.AppInit.Step1, strings.Join(cfg.Input, ", ")))
		fmt.Printf("2. %s\n", cfg.TR.T(messages.Keys.AppInit.Step2, strings.Join(cfg.Input, ",")))
		fmt.Println("3. " + cfg.TR.T(messages.Keys.AppInit.Step3))
	}

	return nil
}
