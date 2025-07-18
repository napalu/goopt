package main

import (
	"fmt"
	"golang.org/x/text/language"
	"os"
	"strings"

	"github.com/napalu/goopt/v2"
)

// ComplexApp demonstrates the improved help parser features
type ComplexApp struct {
	// Global flags
	Verbose bool   `goopt:"short:v;desc:Enable verbose output"`
	Config  string `goopt:"short:c;desc:Configuration file path"`
	Debug   bool   `goopt:"desc:Enable debug mode"`
	Demo    struct {
		Exec goopt.CommandFunc
	} `goopt:"desc:Demo mode;kind:command"`
	// Core settings
	Core struct {
		LDAP struct {
			Host     string `goopt:"short:H;desc:LDAP host;required:true"`
			Port     int    `goopt:"short:P;desc:LDAP port;default:389"`
			SSL      bool   `goopt:"short:s;desc:Use SSL"`
			BindUser string `goopt:"short:b;desc:Bind user DN;required:true"`
			BaseDN   string `goopt:"desc:Base DN for searches;default:dc=example,dc=com"`
		} `goopt:"name:ldap"`

		Database struct {
			Host     string `goopt:"desc:Database host;default:localhost"`
			Port     int    `goopt:"desc:Database port;default:5432"`
			Name     string `goopt:"desc:Database name;required:true"`
			User     string `goopt:"desc:Database user;required:true"`
			Password string `goopt:"desc:Database password;secure:true"`
		} `goopt:"name:db"`

		Vault struct {
			URL      string `goopt:"desc:Vault URL"`
			Token    string `goopt:"desc:Vault token;secure:true"`
			RoleID   string `goopt:"desc:Vault role ID"`
			SecretID string `goopt:"desc:Vault secret ID;secure:true"`
		} `goopt:"name:ault"`
	}

	// Commands
	Users struct {
		Create struct {
			Username string `goopt:"short:u;desc:Username;required:true"`
			Email    string `goopt:"short:e;desc:Email address;required:true"`
			Groups   string `goopt:"short:g;desc:Comma-separated groups"`
		} `goopt:"kind:command;desc:Create a new user"`

		Delete struct {
			Username string `goopt:"short:u;desc:Username to delete;required:true"`
			Force    bool   `goopt:"short:f;desc:Force deletion without confirmation"`
		} `goopt:"kind:command;desc:Delete a user"`

		List struct {
			Filter string `goopt:"short:f;desc:Filter expression"`
			Format string `goopt:"desc:Output format (json|table|csv);default:table"`
		} `goopt:"kind:command;desc:List users"`
	} `goopt:"kind:command;desc:User management commands"`

	Groups struct {
		Create struct {
			Name        string `goopt:"short:n;desc:Group name;required:true"`
			Description string `goopt:"short:d;desc:Group description"`
		} `goopt:"kind:command;desc:Create a new group"`

		AddMember struct {
			Group    string `goopt:"short:g;desc:Group name;required:true"`
			Username string `goopt:"short:u;desc:Username to add;required:true"`
		} `goopt:"kind:command;desc:Add user to group"`
	} `goopt:"kind:command;desc:Group management commands"`

	Azure struct {
		Sync struct {
			TenantID     string `goopt:"desc:Azure tenant ID;required:true"`
			ClientID     string `goopt:"desc:Azure client ID;required:true"`
			ClientSecret string `goopt:"desc:Azure client secret;secure:true"`
			DryRun       bool   `goopt:"desc:Perform dry run only"`
		} `goopt:"kind:command;desc:Sync with Azure AD"`
	} `goopt:"kind:command;desc:Azure integration commands"`
}

func main() {
	app := &ComplexApp{}
	// Demo: Show different help modes
	app.Demo.Exec = demonstrateHelpModes

	// Create parser with improved help
	parser, err := goopt.NewParserFromStruct(app,
		goopt.WithHelpStyle(goopt.HelpStyleHierarchical),
		goopt.WithHelpBehavior(goopt.HelpBehaviorSmart),
		// start executing commands as soon as parse is done
		goopt.WithExecOnParseComplete(true),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating parser: %v\n", err)
		os.Exit(1)
	}

	// Parse arguments - normally we'd check if parse was successful and display errors,
	// but since we control the demo and want to show the help modes, we ignore them
	_ = parser.ParseString(`demo --core.ldap.host localhost 
		--core.ldap.bindUser test 
		--core.db.name db_test 
		--core.db.user app_user`)

	// Normal execution
	fmt.Println("Application running...")
}

func demonstrateHelpModes(parser *goopt.Parser, command *goopt.Command) error {
	fmt.Println("\n=== Demonstrating Improved Help Features ===")

	languages := []string{"en", "de", "fr"}
	demos := []struct {
		name string
		args []string
	}{
		{
			name: "1. Show only global flags",
			args: []string{"--help", "globals"},
		},
		{
			name: "2. Show only commands",
			args: []string{"--help", "commands"},
		},
		{
			name: "3. Show help with defaults",
			args: []string{"--help", "--show-defaults"},
		},
		{
			name: "4. Filter flags by pattern",
			args: []string{"--help", "--filter", "core.ldap.*"},
		},
		{
			name: "5. Search help content",
			args: []string{"--help", "--search", "user"},
		},
		{
			name: "6. Command-specific help",
			args: []string{"users", "--help"},
		},
		{
			name: "7. Invalid command (error context)",
			args: []string{"invalid-cmd"},
		},
		{
			name: "8. Show examples",
			args: []string{"--help", "examples"},
		},
		{
			name: "9. Show help for a command",
			args: []string{"--help", "users", "create"},
		},
		{
			name: "10. Show help for help",
			args: []string{"--help", "--help"},
		},
	}

	for _, lang := range languages {
		// parser language sets help language
		_ = parser.SetLanguage(language.Make(lang))
		fmt.Printf("\n=== Language: %s ===\n", lang)
		for _, demo := range demos {
			fmt.Printf("\n--- %s ---\n", demo.name)
			fmt.Printf("Command: app %s\n\n", strings.Join(demo.args, " "))

			// Create help parser
			helpConfig := parser.GetHelpConfig()
			helpParser := goopt.NewHelpParser(parser, helpConfig)

			// Parse and show help
			err := helpParser.Parse(demo.args)
			if err != nil {
				fmt.Printf("(Error: %v)\n", err)
			}

			fmt.Println("\n" + strings.Repeat("-", 60))
		}
	}

	return nil
}
