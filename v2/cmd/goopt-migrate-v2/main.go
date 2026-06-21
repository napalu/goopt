// Command goopt-migrate rewrites source for goopt/v2 API changes — currently, wrapping
// bare func(string) error validators in validation.ValidatorFunc after the
// validator-interface change. It dogfoods goopt to parse its own flags.
package main

import (
	"fmt"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/migration"
)

type config struct {
	File      string `goopt:"name:file;short:f;desc:convert a single .go file"`
	Dir       string `goopt:"name:dir;short:d;desc:convert a directory"`
	Recursive bool   `goopt:"name:recursive;short:r;desc:recurse into subdirectories (with --dir)"`
	DryRun    bool   `goopt:"name:dry-run;short:n;desc:show what would change without writing"`
	Backup    bool   `goopt:"name:backup;short:b;desc:write a .bak next to each changed file"`
}

func main() {
	cfg := &config{}
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if !parser.Parse(os.Args) {
		for _, e := range parser.GetErrors() {
			fmt.Fprintln(os.Stderr, e)
		}
		parser.PrintHelp(os.Stderr)
		os.Exit(2)
	}
	if cfg.File == "" && cfg.Dir == "" {
		fmt.Fprintln(os.Stderr, "specify --file or --dir")
		parser.PrintHelp(os.Stderr)
		os.Exit(2)
	}

	if cfg.File != "" {
		if err := runFile(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if cfg.Dir != "" {
		if err := runDir(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func runFile(cfg *config) error {
	if cfg.DryRun {
		out, changed, err := migration.PreviewFile(cfg.File)
		if err != nil {
			return err
		}
		if changed {
			fmt.Printf("--- would change %s ---\n%s", cfg.File, out)
		} else {
			fmt.Printf("%s: no changes\n", cfg.File)
		}
		return nil
	}
	changed, err := migration.ConvertFile(cfg.File, cfg.Backup)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("converted %s\n", cfg.File)
	} else {
		fmt.Printf("%s: no changes\n", cfg.File)
	}
	return nil
}

func runDir(cfg *config) error {
	if cfg.DryRun {
		// Walk read-only via PreviewFile by reusing ConvertDir's traversal would write;
		// instead just report per-file previews is overkill — for dry-run on a dir we
		// only list files that would change.
		return previewDir(cfg)
	}
	changed, err := migration.ConvertDir(cfg.Dir, cfg.Recursive, cfg.Backup)
	if err != nil {
		return err
	}
	for _, f := range changed {
		fmt.Printf("converted %s\n", f)
	}
	fmt.Printf("%d file(s) changed\n", len(changed))
	return nil
}

func previewDir(cfg *config) error {
	// Reuse ConvertDir semantics for traversal but in preview mode: ConvertFile isn't
	// called; we walk and PreviewFile each .go file.
	return migration.WalkPreview(cfg.Dir, cfg.Recursive, func(path string) {
		fmt.Printf("would change %s\n", path)
	})
}
