package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/napalu/goopt"
	"github.com/napalu/goopt/migration"
)

type Config struct {
	Dir     string `goopt:"name:dir;short:d;desc:Directory to process recursively"`
	File    string `goopt:"name:file;short:f;desc:Single file to process"`
	DryRun  bool   `goopt:"name:dry-run;desc:Show what would be changed without making changes"`
	Verbose bool   `goopt:"name:verbose;short:v;desc:Show detailed progress"`
	Help    bool   `goopt:"name:help;short:h;desc:Show help"`
}

func main() {
	cfg := &Config{}
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !parser.Parse(os.Args) {
		for _, err := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	if cfg.Help {
		parser.PrintUsageWithGroups(os.Stdout)
		os.Exit(0)
	}

	if cfg.File == "" && cfg.Dir == "" {
		fmt.Fprintln(os.Stderr, "Error: either --file or --dir must be specified")
		os.Exit(1)
	}

	if cfg.File != "" && cfg.Dir != "" {
		fmt.Fprintln(os.Stderr, "Error: cannot specify both --file and --dir")
		os.Exit(1)
	}

	if cfg.File != "" {
		if cfg.Verbose {
			fmt.Printf("Processing file: %s\n", cfg.File)
		}
		baseDir := filepath.Dir(cfg.File)
		if cfg.DryRun {
			changes, err := migration.PreviewChanges(cfg.File)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(changes)
			return
		}
		err = migration.ConvertSingleFile(cfg.File, baseDir)
	} else {
		if cfg.Verbose {
			fmt.Printf("Processing directory: %s\n", cfg.Dir)
		}
		err = processDir(cfg.Dir, cfg.DryRun, cfg.Verbose, cfg.Dir)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func processDir(dir string, dryRun bool, verbose bool, baseDir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		if verbose {
			if dryRun {
				fmt.Printf("Would process: %s\n", path)
			} else {
				fmt.Printf("Processing: %s\n", path)
			}
		}

		if dryRun {
			changes, err := migration.PreviewChanges(path)
			if err != nil {
				return fmt.Errorf("preview %s: %w", path, err)
			}
			if changes != "" {
				fmt.Println(changes)
			}
			return nil
		}

		if err := migration.ConvertSingleFile(path, baseDir); err != nil {
			return fmt.Errorf("converting %s: %w", path, err)
		}

		return nil
	})
}
