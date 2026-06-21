---
layout: default
title: What's New in v2
nav_order: 2
version: v2
---

{% include version-selector.html %}

# What's New in goopt v2

`goopt` v2 is a major update focused on providing powerful, out-of-the-box solutions for building professional, robust, and user-friendly command-line applications.

## New Major Features

### Runtime Shell Completion (Replaces the static generator)
Shell completion is now **computed at runtime by the live parser** instead of being
baked into a large static script. goopt installs a tiny stub that forwards each `<TAB>`
back to your program, and the parser answers from its one true model.
- **Cannot drift:** completion offers exactly what the parser accepts — inherited flags
  on subcommands included. There is no second command-tree model to keep in sync.
- **Dynamic values:** compute a flag's candidates at completion time (git branches, files,
  service data) with `WithCompleter` — impossible with a static script.
- **Smaller surface:** one forwarding stub + one formatter per shell, instead of a full
  completion script generator per shell.
- **Breaking change:** the static `GenerateCompletion` / `GetCompletionData` API has been
  removed. Migration is small (add one `HandleCompletion` line, swap to
  `GenerateCompletionStub`, move `AcceptedValues` → `WithCompleter`).
- **➡️ [Read the Shell Completion Guide]({{ site.baseurl }}/v2/guides/05-built-in-features/03-shell-completion/) · [Migrating from static completion]({{ site.baseurl }}/v2/guides/05-built-in-features/03-shell-completion/#migrating-from-static-completion)**

### Validators Drive Completion (and are now an interface)
A validator can now supply its own completion candidates, so a value-restricting
validator *also* powers shell completion — one declaration, no chance of the validated
set and the completed set drifting apart.
- **`validation.IsOneOf` both validates and completes:** `WithValidators(validation.IsOneOf("a","b"))`
  rejects anything outside the set *and* offers it on `<TAB>`. Any validator implementing
  `Enumerable` (`Candidates() []string`) does the same.
- **Breaking change:** `validation.Validator` is now an interface (`Validate(string) error`),
  and the validator setters (`WithValidator(s)`, `SetValidators`, `AddFlagValidators`,
  `SetFlagValidators`, `WithFlagValidators`) take it. Built-in validators and
  `validation.Custom(...)` are unchanged; a **bare inline `func(string) error`** must be
  wrapped in `validation.Custom(...)`, and `[]validation.ValidatorFunc` becomes
  `[]validation.Validator`.
- **Automated migration:** the **`goopt-migrate-v2`** codegen tool does the wrapping and
  slice-rename for you:
  ```bash
  go install github.com/napalu/goopt/v2/cmd/goopt-migrate-v2@latest
  goopt-migrate-v2 -d . -r --dry-run   # preview; drop --dry-run to apply
  ```
  (Distinct binary from the v1→v2 `goopt-migrate` tool, so both can be installed.)
- **➡️ [Migration tool details]({{ site.baseurl }}/v2/guides/05-built-in-features/03-shell-completion/) · see the `v2/migration` package README**

### A Powerful Validation Engine (Replaces `accepted`)
The old `accepted` tag has been **deprecated** in favor of a completely new validation engine that is more powerful, composable, and easier to use.

- **Directly in Struct Tags:** Define complex validation rules right where you define your flag.
- **Composable Logic:** Chain built-in validators (`email`, `port`, `range`) or combine them with logical operators (`oneof`, `all`, `not`).
- **Custom Validators:** Easily write and integrate your own domain-specific validation logic.
- **Clearer Syntax:** The new `validators` tag uses a more intuitive parenthesis-based syntax (e.g., `validators:"minlength(5)"`).
- **➡️ [Read the Validation Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/)**

### Flag Contracts (Relational Constraints)
Where validators check a single value, **contracts** declare constraints *between* flags —
mutual exclusion, co-requirement, and conditional requirement — declaratively, replacing
hand-written `RequiredIf` callbacks for the common cases.
- **Five primitives:** `mutex` (at most one), `exactlyone` (exactly one), `conflicts`, `requires`, and `requiredOn` (required when another flag/command is present).
- **All three paradigms:** struct tags (`contract:"mutex(format)"`), option funcs (`WithMutex`), or programmatic accessors (`AddFlagContracts`/`SetFlagContracts`/`GetFlagContracts`).
- **Friendly, translated errors:** user-facing messages name the flags actually typed and ship in every built-in locale; misspelled group names are caught as developer-facing construction errors.
- **➡️ [Read the Contracts Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/05-contracts/)**

### Command Execution Hooks
Manage the entire lifecycle of your commands with pre- and post-execution hooks. This is perfect for handling cross-cutting concerns without cluttering your command logic.
- **Use cases:** Authentication checks, database connection management, logging, metrics, and resource cleanup.
- **Flexible scope:** Apply hooks globally to all commands or target specific commands.
- **➡️ [Read the Execution Hooks Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/02-execution-hooks/)**

### Advanced Help & Version Systems
The help and version systems are now fully automatic and highly configurable.
- **Auto-Help:** `--help` and `-h` flags are now provided by default, with an adaptive display style that suits your CLI's complexity.
- **Interactive Help:** Users can now query the help system with commands like `myapp --help --search "database"`.
- **Intelligent Suggestions:** Automatic "did you mean?" suggestions for mistyped commands and flags with customizable distance thresholds.
- **Auto-Version:** Enable a `--version` flag with a single line of configuration, with support for dynamic build-time variables.
- **➡️ [See the Help System Guide]({{ site.baseurl }}/v2/guides/05-built-in-features/01-help-system/) and [Version Support Guide]({{ site.baseurl }}/v2/guides/05-built-in-features/02-version-support/)**

## Architectural Improvements

### Enhanced Internationalization (i18n)
The i18n system has been significantly enhanced with new features for true global application support.
- **Layered Bundles:** A clearer separation between the default system bundle and your application's user bundle.
- **Runtime-Switchable Names:** New `nameKey` tag allows translating flag and command names, not just descriptions. Users can interact with your CLI using commands and flags in their native language.
- **Context-Aware Suggestions:** The "did you mean?" system intelligently detects whether users are typing in the canonical or translated language and shows suggestions accordingly.
- **JIT Translation Registry:** Just-In-Time building of translation mappings optimizes startup time by constructing name-to-flag/command mappings only when needed.
- **Auto-Language Detection:** Sophisticated language detection from command-line flags, environment variables, and (optionally) system locale settings.
- **RTL Language Support:** Automatic right-to-left layout detection and formatting for languages like Arabic and Hebrew.
- **Locale-Aware Formatting:** Numbers and dates are automatically formatted according to the user's locale (e.g., 1,000 vs 1.000 vs 1'000).
- **Regional Variants:** Full support for regional language variants like Swiss German (de-CH) or Canadian French (fr-CA).
- **Smart Error Formatting:** Validation errors intelligently format numbers based on format specifiers (%d vs %s).
- **Naming Consistency Warnings:** New warning system helps maintain consistent naming conventions across translations and canonical names.
- **Extended Language Support:** Easy addition of new languages through locale packages or runtime loading.
- **Improved Tooling:** The `goopt-i18n-gen` tool is more powerful than ever, with a "360° workflow" to automate adding i18n to existing projects.
- **➡️ [Read the i18n Guide]({{ site.baseurl }}/v2/guides/06-internationalization/index/)**

### Hierarchical Flag Inheritance
Flag handling is now fully hierarchical and more predictable.
- **Parent-child flag resolution:** Flags defined on parent commands are available to all children.
- **Clear precedence rules:** Command-specific flags correctly override inherited flags.
- **➡️ [See the Flag Inheritance Guide]({{ site.baseurl }}/v2/guides/04-advanced-features/04-flag-inheritance/)**

### API Cleanup
The public API has been modernized and simplified.
- Deprecated methods from v1 have been removed.
- Naming has been made more consistent throughout the library.
- Error handling at initialization is more robust with the introduction of `NewArgE`.

## Breaking Changes

For a complete list of breaking changes and instructions on how to update your code from v1, please see the **[Migration Guide]({{ site.baseurl }}/v2/migration/)**.