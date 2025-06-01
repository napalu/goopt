package options

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
)

// GenerateCmd command configuration
type GenerateCmd struct {
	Output  string `goopt:"short:o;desc:Output Go file;required:true;descKey:app.cmd.generate.output_desc"`
	Package string `goopt:"short:p;desc:Package name;default:messages;descKey:app.cmd.generate.package_desc"`
	Prefix  string `goopt:"desc:Optional prefix to strip from keys;descKey:app.cmd.generate.prefix_desc"`
	Exec    goopt.CommandFunc
}

// ValidateCmd command configuration
type ValidateCmd struct {
	Scan            []string `goopt:"short:s;desc:Go source files to scan for descKey references;descKey:app.cmd.validate.scan_desc"`
	Strict          bool     `goopt:"desc:Exit with error if validation fails;descKey:app.cmd.validate.strict_desc"`
	GenerateMissing bool     `goopt:"short:g;desc:Generate stub entries for missing translation keys;descKey:app.cmd.validate.generate_missing"`
	Exec            goopt.CommandFunc
}

// AuditCmd command configuration
type AuditCmd struct {
	Files            []string `goopt:"desc:Go source files to scan (default: *.go);descKey:app.cmd.audit.files_desc"`
	GenerateDescKeys bool     `goopt:"short:d;desc:Generate descKey tags for fields that don't have them;descKey:app.cmd.audit.generate_desc_keys"`
	GenerateMissing  bool     `goopt:"short:g;desc:Generate stub entries for missing translation keys;descKey:app.cmd.audit.generate_missing"`
	KeyPrefix        string   `goopt:"desc:Prefix for generated descKeys;default:app;descKey:app.cmd.audit.key_prefix_desc"`
	AutoUpdate       bool     `goopt:"short:u;desc:Automatically update source files with generated descKeys;descKey:app.cmd.audit.auto_update_desc"`
	BackupDir        string   `goopt:"desc:Directory for backup files;default:.goopt-i18n-backup;descKey:app.cmd.audit.backup_dir_desc"`
	Exec             goopt.CommandFunc
}

// InitCmd command configuration
type InitCmd struct {
	Force bool `goopt:"short:f;desc:Force overwrite existing files;descKey:app.cmd.init.force_desc"`
	Exec  goopt.CommandFunc
}

// AddCmd command configuration
type AddCmd struct {
	Key      string `goopt:"short:k;desc:Single key to add;descKey:app.cmd.add.key_desc"`
	Value    string `goopt:"short:V;desc:Value for the key;descKey:app.cmd.add.value_desc"`
	FromFile string `goopt:"short:F;desc:JSON file containing keys to add;descKey:app.cmd.add.from_file_desc"`
	Mode     string `goopt:"short:m;desc:How to handle existing keys (skip, replace, error);default:skip;descKey:app.cmd.add.mode_desc"`
	DryRun   bool   `goopt:"short:n;desc:Show what would be added without modifying files;descKey:app.cmd.add.dry_run_desc"`
	Exec     goopt.CommandFunc
}

// ExtractCmd handles the extract command configuration
type ExtractCmd struct {
	Files         string `goopt:"short:s;desc:Go files to scan;default:**/*.go;descKey:app.cmd.extract.files_desc"`
	MatchOnly     string `goopt:"short:M;desc:Regex to match strings for inclusion;descKey:app.cmd.extract.match_only_desc"`
	SkipMatch     string `goopt:"short:S;desc:Regex to match strings for exclusion;descKey:app.cmd.extract.skip_match_desc"`
	KeyPrefix     string `goopt:"short:P;desc:Prefix for generated keys;default:app.extracted;descKey:app.cmd.extract.key_prefix_desc"`
	MinLength     int    `goopt:"short:l;desc:Minimum string length;default:2;descKey:app.cmd.extract.min_length_desc"`
	DryRun        bool   `goopt:"short:n;desc:Preview what would be extracted;descKey:app.cmd.extract.dry_run_desc"`
	AutoUpdate    bool   `goopt:"short:u;desc:Update source files (add comments or replace strings);descKey:app.cmd.extract.auto_update_desc"`
	TrPattern     string `goopt:"desc:Translator pattern for replacements (e.g. tr.T);descKey:app.cmd.extract.tr_pattern_desc"`
	Package       string `goopt:"short:p;desc:Package name for imports;default:messages;descKey:app.cmd.extract.package_desc"`
	KeepComments  bool   `goopt:"desc:Keep i18n comments after replacement;descKey:app.cmd.extract.keep_comments_desc"`
	CleanComments bool   `goopt:"desc:Remove all i18n-* comments;descKey:app.cmd.extract.clean_comments_desc"`
	BackupDir     string `goopt:"desc:Directory for backup files;default:.goopt-i18n-backup;descKey:app.cmd.extract.backup_dir_desc"`
	Exec          goopt.CommandFunc
}

// AppConfig main application configuration
type AppConfig struct {
	Input    []string        `goopt:"short:i;desc:Input JSON files (supports wildcards);required:true;descKey:app.global.input_desc"`
	Verbose  bool            `goopt:"short:v;desc:Enable verbose output;descKey:app.global.verbose_desc"`
	Language string          `goopt:"short:l;desc:Language for output (en, de, fr);descKey:app.global.language_desc"`
	Help     bool            `goopt:"short:h;desc:Show help;descKey:app.global.help_desc"`
	Generate GenerateCmd     `goopt:"kind:command;name:generate;desc:Generate Go constants from JSON;descKey:app.cmd.generate_desc"`
	Validate ValidateCmd     `goopt:"kind:command;name:validate;desc:Check that all descKey references have translations;descKey:app.cmd.validate_desc"`
	Audit    AuditCmd        `goopt:"kind:command;name:audit;desc:Audit goopt fields for missing descKey tags;descKey:app.cmd.audit_desc"`
	Init     InitCmd         `goopt:"kind:command;name:init;desc:Initialize a new i18n setup;descKey:app.cmd.init_desc"`
	Add      AddCmd          `goopt:"kind:command;name:add;desc:Add new translation keys to locale files;descKey:app.cmd.add_desc"`
	Extract  ExtractCmd      `goopt:"kind:command;name:extract;desc:Extract strings from Go source files;descKey:app.cmd.extract_desc"`
	TR       i18n.Translator `ignore:"true"` // Translator for messages
}
