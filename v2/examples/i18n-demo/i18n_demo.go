package main

import (
	"embed"
	"fmt"
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
	"golang.org/x/text/language"
	"os"
	"strings"
)

//go:embed locales/*.json
var userLocales embed.FS

//go:embed system-locales/*.json
var systemLocales embed.FS

type Config struct {
	Language string `goopt:"short:l;name:lang;descKey:i18n.demo.lang_desc;default:en"`
	Verbose  bool   `goopt:"short:v;descKey:i18n.demo.verbose_desc"`
	Help     bool   `goopt:"short:h;descKey:i18n.demo.help_desc"`
	User     struct {
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

	// i18n.Default() is the default system bundle that is used by the parser
	systemBundle := i18n.Default()
	// add the missing Japanese and Spanish system languages to the parser
	err = systemBundle.LoadFromFS(systemLocales, "system-locales")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add system-locales to bundle: %v\n", err)
		os.Exit(1)
	}

	// Assign the user bundle to the config. This will be to translate messages.
	cfg.TR = bundle

	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithUserBundle(bundle))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create parser: %v\n", err)
		os.Exit(1)
	}

	success := parser.Parse(os.Args)
	if cfg.Language != "" && cfg.Language != bundle.GetDefaultLanguage().String() {
		lang := parseLanguage(cfg.Language)
		if lang != language.Und {
			bundle.SetDefaultLanguage(lang)
			systemBundle.SetDefaultLanguage(lang)
		}
	}

	// Handle help
	if cfg.Help {
		parser.PrintUsageWithGroups(os.Stderr)
		os.Exit(0)
	}

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

func parseLanguage(lang string) language.Tag {
	switch strings.ToLower(lang) {
	case "en":
		return language.English
	case "es":
		return language.Spanish
	case "ja":
		return language.Japanese
	case "fr":
		return language.French
	case "de":
		return language.German
	default:
		return language.Und
	}
}

func executeUserList(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T("i18n.demo.user.list.executing"))

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T("i18n.demo.user.list.options", cfg.User.List.ShowAll, cfg.User.List.Format))
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
		fmt.Println("\n" + cfg.TR.T("i18n.demo.user.list.header"))
		fmt.Println("----------------------------------------")
		for _, user := range users {
			if cfg.User.List.ShowAll || user.admin {
				adminStr := cfg.TR.T("i18n.demo.no")
				if user.admin {
					adminStr = cfg.TR.T("i18n.demo.yes")
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

	fmt.Println(cfg.TR.T("i18n.demo.user.create.executing"))

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T("i18n.demo.user.create.creating",
			cfg.User.Create.Username, cfg.User.Create.Email))
		if cfg.User.Create.Admin {
			fmt.Println(cfg.TR.T("i18n.demo.user.create.with_admin"))
		}
	}

	// Simulate user creation
	fmt.Printf(cfg.TR.T("i18n.demo.user.create.success", cfg.User.Create.Username))

	return nil
}

func executeUserDelete(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T("i18n.demo.user.delete.executing"))

	if !cfg.User.Delete.Force {
		fmt.Printf(cfg.TR.T("i18n.demo.user.delete.confirm", cfg.User.Delete.Username))
		return nil
	}

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T("i18n.demo.user.delete.deleting", cfg.User.Delete.Username))
	}

	// Simulate user deletion
	fmt.Printf(cfg.TR.T("i18n.demo.user.delete.success", cfg.User.Delete.Username))

	return nil
}

func executeDatabaseBackup(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T("i18n.demo.db.backup.executing"))

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T("i18n.demo.db.backup.backing_up", cfg.Database.Backup.Output))
		if cfg.Database.Backup.Compress {
			fmt.Println(cfg.TR.T("i18n.demo.db.backup.with_compression"))
		}
	}

	// Simulate backup
	fmt.Printf(cfg.TR.T("i18n.demo.db.backup.success", cfg.Database.Backup.Output))

	return nil
}

func executeDatabaseRestore(parser *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*Config](parser)
	if !ok {
		return fmt.Errorf("failed to get config from parser")
	}

	fmt.Println(cfg.TR.T("i18n.demo.db.restore.executing"))

	if cfg.Database.Restore.DropFirst {
		fmt.Println(cfg.TR.T("i18n.demo.db.restore.dropping"))
	}

	if cfg.Verbose {
		fmt.Printf(cfg.TR.T("i18n.demo.db.restore.restoring", cfg.Database.Restore.Input))
	}

	// Simulate restore
	fmt.Printf(cfg.TR.T("i18n.demo.db.restore.success", cfg.Database.Restore.Input))

	return nil
}
