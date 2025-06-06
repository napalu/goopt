package options

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/i18n"
)

// GenerateCmd command configuration
type GenerateCmd struct {
	Output  string `goopt:"short:o;desc:Output Go file;required:true;descKey:app.generate_cmd.output_desc"`
	Package string `goopt:"short:p;desc:Package name;default:messages;descKey:app.generate_cmd.package_desc"`
	Prefix  string `goopt:"desc:Optional prefix to strip from keys;descKey:app.generate_cmd.prefix_desc"`
	Exec    goopt.CommandFunc
}

// ValidateCmd command configuration
type ValidateCmd struct {
	Scan            []string `goopt:"short:s;desc:Go source files to scan for descKey references;descKey:app.validate_cmd.scan_desc"`
	Strict          bool     `goopt:"desc:Exit with error if validation fails;descKey:app.validate_cmd.strict_desc"`
	GenerateMissing bool     `goopt:"short:g;desc:Generate stub entries for missing translation keys;descKey:app.validate_cmd.generate_missing_desc"`
	Exec            goopt.CommandFunc
}

// AuditCmd command configuration
type AuditCmd struct {
	Files            []string `goopt:"desc:Go source files to scan (default: **/*.go);default:**/*.go;descKey:app.audit_cmd.files_desc"`
	GenerateDescKeys bool     `goopt:"short:d;desc:Generate descKey tags for fields that don't have them;descKey:app.audit_cmd.generate_desc_keys_desc"`
	GenerateMissing  bool     `goopt:"short:g;desc:Generate stub entries for missing translation keys;descKey:app.audit_cmd.generate_missing_desc"`
	KeyPrefix        string   `goopt:"desc:Prefix for generated descKeys;default:app;descKey:app.audit_cmd.key_prefix_desc"`
	AutoUpdate       bool     `goopt:"short:u;desc:Automatically update source files with generated descKeys;descKey:app.audit_cmd.auto_update_desc"`
	BackupDir        string   `goopt:"desc:Directory for backup files;default:.goopt-i18n-backup;descKey:app.audit_cmd.backup_dir_desc"`
	Exec             goopt.CommandFunc
}

// InitCmd command configuration
type InitCmd struct {
	Force bool `goopt:"short:f;desc:Force overwrite existing files;descKey:app.init_cmd.force_desc"`
	Exec  goopt.CommandFunc
}

// AddCmd command configuration
type AddCmd struct {
	Key      string `goopt:"short:k;desc:Single key to add;descKey:app.add_cmd.key_desc"`
	Value    string `goopt:"short:V;desc:Value for the key;descKey:app.add_cmd.value_desc"`
	FromFile string `goopt:"short:F;desc:JSON file containing keys to add;descKey:app.add_cmd.from_file_desc"`
	Mode     string `goopt:"short:m;desc:How to handle existing keys (skip, replace, error);default:skip;descKey:app.add_cmd.mode_desc"`
	DryRun   bool   `goopt:"short:n;desc:Show what would be added without modifying files;descKey:app.add_cmd.dry_run_desc"`
	Exec     goopt.CommandFunc
}

// ExtractCmd handles the extract command configuration
type ExtractCmd struct {
	Files          string `goopt:"short:s;desc:Go files to scan;default:**/*.go;descKey:app.extract_cmd.files_desc"`
	MatchOnly      string `goopt:"short:M;desc:Regex to match strings for inclusion;descKey:app.extract_cmd.match_only_desc"`
	SkipMatch      string `goopt:"short:S;desc:Regex to match strings for exclusion;descKey:app.extract_cmd.skip_match_desc"`
	KeyPrefix      string `goopt:"short:P;desc:Prefix for generated keys;default:app.extracted;descKey:app.extract_cmd.key_prefix_desc"`
	MinLength      int    `goopt:"short:L;desc:Minimum string length;default:2;descKey:app.extract_cmd.min_length_desc"`
	DryRun         bool   `goopt:"short:n;desc:Preview what would be extracted;descKey:app.extract_cmd.dry_run_desc"`
	AutoUpdate     bool   `goopt:"short:u;desc:Update source files (add comments or replace strings);descKey:app.extract_cmd.auto_update_desc"`
	TrPattern      string `goopt:"desc:Translator pattern for replacements;default:tr.T;descKey:app.extract_cmd.tr_pattern_desc"`
	Package        string `goopt:"short:p;desc:Package name for imports;default:messages;descKey:app.extract_cmd.package_desc"`
	KeepComments   bool   `goopt:"desc:Keep i18n comments after replacement;descKey:app.extract_cmd.keep_comments_desc"`
	CleanComments  bool   `goopt:"desc:Remove all i18n-* comments;descKey:app.extract_cmd.clean_comments_desc"`
	BackupDir      string `goopt:"desc:Directory for backup files;default:.goopt-i18n-backup;descKey:app.extract_cmd.backup_dir_desc"`
	TransformMode        string   `goopt:"desc:What strings to transform: user-facing, with-comments, all-marked, all;default:user-facing;descKey:app.extract_cmd.transform_mode_desc"`
	UserFacingRegex      []string `goopt:"desc:Regex patterns to identify user-facing functions;descKey:app.extract_cmd.user_facing_regex_desc"`
	FormatFunctionRegex  []string `goopt:"desc:Regex pattern and format arg index (pattern:index);descKey:app.extract_cmd.format_function_regex_desc"`
	Exec                 goopt.CommandFunc
}

// SyncCmd command configuration
type SyncCmd struct {
	Target       []string `goopt:"short:t;desc:Target JSON files to sync against reference files (-i flag);descKey:app.sync_cmd.target_desc"`
	RemoveExtra  bool     `goopt:"short:r;desc:Remove keys that don't exist in reference;descKey:app.sync_cmd.remove_extra_desc"`
	TodoPrefix   string   `goopt:"desc:Prefix for new non-English translations;default:[TODO];descKey:app.sync_cmd.todo_prefix_desc"`
	DryRun       bool     `goopt:"short:n;desc:Preview what would be changed;descKey:app.sync_cmd.dry_run_desc"`
	Exec         goopt.CommandFunc
}

// AppConfig main application configuration
type AppConfig struct {
	Input    []string        `goopt:"short:i;desc:Input JSON files (supports wildcards);required:true;descKey:app.app_config.input_desc"`
	Verbose  bool            `goopt:"short:v;desc:Enable verbose output;descKey:app.app_config.verbose_desc"`
	Language string          `goopt:"short:l;desc:Language for output (en, de, fr);descKey:app.app_config.language_desc"`
	Help     bool            `goopt:"short:h;desc:Show help;descKey:app.app_config.help_desc"`
	Generate GenerateCmd     `goopt:"kind:command;name:generate;desc:Generate Go constants from JSON;descKey:app.app_config.generate_desc"`
	Validate ValidateCmd     `goopt:"kind:command;name:validate;desc:Check that all descKey references have translations;descKey:app.app_config.validate_desc"`
	Audit    AuditCmd        `goopt:"kind:command;name:audit;desc:Audit goopt fields for missing descKey tags;descKey:app.app_config.audit_desc"`
	Init     InitCmd         `goopt:"kind:command;name:init;desc:Initialize a new i18n setup;descKey:app.app_config.init_desc"`
	Add      AddCmd          `goopt:"kind:command;name:add;desc:Add new translation keys to locale files;descKey:app.app_config.add_desc"`
	Extract  ExtractCmd      `goopt:"kind:command;name:extract;desc:Extract strings from Go source files;descKey:app.app_config.extract_desc"`
	Sync     SyncCmd         `goopt:"kind:command;name:sync;desc:Synchronize keys across locale files;descKey:app.app_config.sync_desc"`
	TR       i18n.Translator `ignore:"true"` // Translator for messages
}
