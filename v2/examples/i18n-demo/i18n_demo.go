// i18n-demo demonstrates how to extend goopt with support for additional languages
// beyond the built-in English, German, and French.
//
// This example shows:
// 1. How to create system message translations for new languages
// 2. How to use WithExtendBundle() to add these languages to goopt
// 3. How to create application-specific translations with goopt-i18n-gen
//
// While this demo uses Spanish and Japanese (which already have packages available),
// the same technique can be used to add ANY language like Italian, Russian, Polish,
// Vietnamese, etc. - languages that don't yet have official goopt support.
//
// To add a new language to your application:
// 1. Create a JSON file with goopt system message translations (see system-locales/)
// 2. Create your app's translations (see locales/)
// 3. Use WithExtendBundle() to register your system translations
// 4. Use WithUserBundle() for your app-specific translations

package main

// Build the tool first: cd ../../cmd/goopt-i18n-gen && go build
//go:generate ../../cmd/goopt-i18n-gen/goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages

import (
	"embed"
	"fmt"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/examples/i18n-demo/messages"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
	"os"
)

//go:embed locales/*.json
var userLocales embed.FS

//go:embed system-locales/*.json
var systemLocales embed.FS

type Config struct {
	Verbose bool `goopt:"short:v;descKey:i18n.demo.verbose_desc"`
	User    struct {
		List struct {
			ShowAll bool   `goopt:"short:a;name:all;descKey:i18n.demo.user.list.all_desc"`
			Format  string `goopt:"short:f;descKey:i18n.demo.user.list.format_desc;default:table"`
			Exec    goopt.CommandFunc
		} `goopt:"kind:command;name:list;descKey:i18n.demo.user.list_desc"`

		Create struct {
			Username string `goopt:"short:u;descKey:i18n.demo.user.create.username_desc;required:true"`
			Email    string `goopt:"short:e;descKey:i18n.demo.user.create.email_desc;required:true"`
			Admin    bool   `goopt:"short:a;descKey:i18n.demo.user.create.admin_desc"`
			Exec     goopt.CommandFunc
		} `goopt:"kind:command;name:create;descKey:i18n.demo.user.create_desc"`

		Delete struct {
			Username string `goopt:"short:u;descKey:i18n.demo.user.delete.username_desc;required:true"`
			Force    bool   `goopt:"short:f;descKey:i18n.demo.user.delete.force_desc"`
			Exec     goopt.CommandFunc
		} `goopt:"kind:command;name:delete;descKey:i18n.demo.user.delete_desc"`
	} `goopt:"kind:command;name:user;descKey:i18n.demo.user_desc"`

	Database struct {
		Backup struct {
			Output   string `goopt:"short:o;descKey:i18n.demo.db.backup.output_desc;required:true"`
			Compress bool   `goopt:"short:c;descKey:i18n.demo.db.backup.compress_desc"`
			Exec     goopt.CommandFunc
		} `goopt:"kind:command;name:backup;descKey:i18n.demo.db.backup_desc"`

		Restore struct {
			Input     string `goopt:"short:i;descKey:i18n.demo.db.restore.input_desc;required:true"`
			DropFirst bool   `goopt:"short:d;descKey:i18n.demo.db.restore.drop_desc"`
			Exec      goopt.CommandFunc
		} `goopt:"kind:command;name:restore;descKey:i18n.demo.db.restore_desc"`
	} `goopt:"kind:command;name:database;descKey:i18n.demo.db_desc"`
	TR i18n.Translator `ignore:"true"` // tell goopt to skip this field
}

func main() {
	cfg := &Config{}

	// Assign command functions
	cfg.User.List.Exec = executeUserList
	cfg.User.Create.Exec = executeUserCreate
	cfg.User.Delete.Exec = executeUserDelete
	cfg.Database.Backup.Exec = executeDatabaseBackup
	cfg.Database.Restore.Exec = executeDatabaseRestore

	bundle, err := i18n.NewBundleWithFS(userLocales, "locales")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bundle: %v\n", err)
		os.Exit(1)
	}

	// DEMONSTRATION: How to add support for languages not built into goopt
	//
	// While goopt includes English, German, and French by default, and has
	// optional packages for Spanish, Japanese, Arabic, Hebrew, etc., you may
	// need to support additional languages like Italian, Russian, Polish, etc.
	//
	// This example shows how to extend goopt's system messages with new languages
	// by creating your own translation files. We're using Spanish and Japanese
	// as examples, but the same technique works for ANY language.
	//
	// Step 1: Create a bundle with your system message translations
	// The system-locales/ directory contains translations for goopt's built-in
	// messages (errors, help text, etc.) in your target languages. Specify one of the
	// languages which is specified in your locale files (otherwise language defaults to English,
	// which will cause errors in English, is not in your bundle).
	//
	// ⚠️ Language Selection Behavior:
	//
	// goopt uses the language specified via '--lang', environment variables, or
	// programmatically via goopt.WithLanguage(...).
	//
	// If none is specified, it defaults to English for CLI UI messages.
	//
	// The default language passed to i18n.NewBundleWithFS(..., defaultLang) affects
	// which language is used as a fallback for validation and system messages.
	//
	// 👉 All translation files in a bundle must define the same keys.
	//    Missing keys may silently fall back to the bundle's defaultLang, so keep
	//    translations complete to ensure consistent behavior.
	systemBundle, err := i18n.NewBundleWithFS(systemLocales, "system-locales", language.Spanish)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add system-locales to bundle: %v\n", err)
		os.Exit(1)
	}

	// Assign the user bundle to the config. This will be used to translate
	// your application-specific messages (command descriptions, etc.)
	cfg.TR = bundle

	// Step 2: Create parser with both bundles
	// - WithUserBundle: Your application's translations
	// - WithExtendBundle: Extends goopt's system messages with new languages
	parser, err := goopt.NewParserFromStruct(cfg,
		goopt.WithUserBundle(bundle),
		goopt.WithExtendBundle(systemBundle),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create parser: %v\n", err)
		os.Exit(1)
	}

	success := parser.Parse(os.Args)

	// Handle errors
	if !success {
		for _, e := range parser.GetErrors() {
			fmt.Fprintf(os.Stderr, "%v\n", e)
		}
		parser.PrintUsageWithGroups(os.Stderr)
		os.Exit(1)
	}

	// Execute commands
	errCount := parser.ExecuteCommands()
	if errCount > 0 {
		for _, cmdErr := range parser.GetCommandExecutionErrors() {
			fmt.Fprintf(os.Stderr, "Command %s failed: %v\n", cmdErr.Key, cmdErr.Value)
		}
		os.Exit(1)
	}
}

func executeUserList(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T(messages.Keys.I18nDemoUserList.Executing))

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoUserList.Options, cfg.User.List.ShowAll, cfg.User.List.Format))
	}

	// Simulate listing users
	users := []struct {
		username string
		email    string
		admin    bool
	}{
		{"alice", "alice@example.com", true},
		{"bob", "bob@example.com", false},
		{"charlie", "charlie@example.com", false},
	}

	if cfg.User.List.Format == "table" {
		fmt.Println("\n" + cfg.TR.T(messages.Keys.I18nDemoUserList.Header))
		fmt.Println("----------------------------------------")
		for _, user := range users {
			if cfg.User.List.ShowAll || user.admin {
				adminStr := cfg.TR.T(messages.Keys.I18nDemo.No)
				if user.admin {
					adminStr = cfg.TR.T(messages.Keys.I18nDemo.Yes)
				}
				fmt.Printf("%-10s %-25s %s\n", user.username, user.email, adminStr)
			}
		}
	} else {
		// JSON format
		fmt.Println("[")
		for i, user := range users {
			if cfg.User.List.ShowAll || user.admin {
				fmt.Printf(`  {"username": "%s", "email": "%s", "admin": %v}`,
					user.username, user.email, user.admin)
				if i < len(users)-1 {
					fmt.Println(",")
				} else {
					fmt.Println()
				}
			}
		}
		fmt.Println("]")
	}

	return nil
}

func executeUserCreate(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T(messages.Keys.I18nDemoUserCreate.Executing))

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoUserCreate.Creating,
			cfg.User.Create.Username, cfg.User.Create.Email))
		if cfg.User.Create.Admin {
			fmt.Println(cfg.TR.T(messages.Keys.I18nDemoUserCreate.WithAdmin))
		}
	}

	// Simulate user creation
	fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoUserCreate.Success, cfg.User.Create.Username))

	return nil
}

func executeUserDelete(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T(messages.Keys.I18nDemoUserDelete.Executing))

	if !cfg.User.Delete.Force {
		fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoUserDelete.Confirm, cfg.User.Delete.Username))
		return nil
	}

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoUserDelete.Deleting, cfg.User.Delete.Username))
	}

	// Simulate user deletion
	fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoUserDelete.Success, cfg.User.Delete.Username))

	return nil
}

func executeDatabaseBackup(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T(messages.Keys.I18nDemoDbBackup.Executing))

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoDbBackup.BackingUp, cfg.Database.Backup.Output))
		if cfg.Database.Backup.Compress {
			fmt.Println(cfg.TR.T(messages.Keys.I18nDemoDbBackup.WithCompression))
		}
	}

	// Simulate backup
	fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoDbBackup.Success, cfg.Database.Backup.Output))

	return nil
}

func executeDatabaseRestore(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T(messages.Keys.I18nDemoDbRestore.Executing))

	if cfg.Database.Restore.DropFirst {
		fmt.Println(cfg.TR.T(messages.Keys.I18nDemoDbRestore.Dropping))
	}

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoDbRestore.Restoring, cfg.Database.Restore.Input))
	}

	// Simulate restore
	fmt.Printf(cfg.TR.T(messages.Keys.I18nDemoDbRestore.Success, cfg.Database.Restore.Input))

	return nil
}
