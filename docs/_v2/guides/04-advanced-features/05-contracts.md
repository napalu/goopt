---
layout: default
title: Contracts
parent: Advanced Features
nav_order: 5
version: v2
---

# Contracts Guide

Validators answer the question *"is this **one** value correct?"*. **Contracts** answer a
different question: *"is this **combination** of flags allowed?"*. They express relational
constraints **between** flags — mutual exclusion, co-requirement, conditional requirement —
declaratively, without hand-written `RequiredIf` callbacks.

Contracts are evaluated **after parsing completes**, when the full set of flags and their
presence is known. A violation is reported as a regular, translatable parse error.

## The Five Contracts

| Contract | Struct Tag | Option Func / Value | Meaning |
|---|---|---|---|
| **mutex** | `contract:mutex(group)` | `WithMutex("group")` / `Mutex("group")` | At most **one** flag in the named group may be set. |
| **exactlyone** | `contract:exactlyone(group)` | `WithExactlyOne("group")` / `ExactlyOne("group")` | **Exactly one** flag in the group must be set (mutex *and* required). |
| **conflicts** | `contract:conflicts(a,b)` | `WithConflicts("a","b")` / `Conflicts("a","b")` | This flag may **not** be set together with any named flag. |
| **requires** | `contract:requires(a,b)` | `WithRequires("a","b")` / `Requires("a","b")` | When this flag is set, **each** named flag must also be set. |
| **requiredOn** | `contract:requiredOn(a,b)` | `WithRequiredOn("a","b")` / `RequiredOn("a","b")` | This flag becomes **required** whenever any named flag is set or named command is invoked. |

> **Group vs. list semantics.** `mutex` and `exactlyone` take a single **group name** — every
> flag tagged with the same group name is a member. `conflicts`, `requires`, and `requiredOn`
> take a **list of flag (or command) names** that this particular flag points at.

### Contract Syntax

Contracts use the same parenthesis-based syntax as validators:

```go
`goopt:"name:json;contract:mutex(format)"`
```

**Key Rules:**
1.  The argument list goes **inside parentheses**: `mutex(format)`, `conflicts(a,b)`.
2.  Multiple contracts on one flag are **comma-separated**: `contract:requires(token),conflicts(anonymous)`.
3.  Contract names are case-insensitive; `mutex`, `exactlyone`, `conflicts`, `requires`, `requiredOn` are the only recognized names.

## Using Contracts

### In Struct Tags

```go
type Config struct {
    // Mutually exclusive output modes: --json XOR --yaml XOR --table (at most one).
    JSON  bool `goopt:"name:json;contract:mutex(format)"`
    YAML  bool `goopt:"name:yaml;contract:mutex(format)"`
    Table bool `goopt:"name:table;contract:mutex(format)"`

    // Exactly one source must be chosen.
    FromFile bool `goopt:"name:from-file;contract:exactlyone(source)"`
    FromURL  bool `goopt:"name:from-url;contract:exactlyone(source)"`

    // --token is required whenever --remote is used.
    Remote bool   `goopt:"name:remote"`
    Token  string `goopt:"name:token;contract:requiredOn(remote)"`

    // --verbose conflicts with --quiet.
    Verbose bool `goopt:"name:verbose;contract:conflicts(quiet)"`
    Quiet   bool `goopt:"name:quiet"`
}
```

### Programmatically

Build contract values with the constructor functions and apply them with `WithContracts`,
or add them after the fact with the parser accessors — mirroring the validators API.

```go
// During parser creation
parser, err := goopt.NewParserWith(
    goopt.WithFlag("json", goopt.NewArg(
        goopt.WithType(types.Standalone),
        goopt.WithMutex("format"),                 // shorthand
    )),
    goopt.WithFlag("yaml", goopt.NewArg(
        goopt.WithType(types.Standalone),
        goopt.WithContracts(goopt.Mutex("format")), // explicit value
    )),
)

// Or add/replace/clear contracts after the fact
parser.AddFlagContracts("token", goopt.RequiredOn("remote"))
parser.SetFlagContracts("verbose", goopt.Conflicts("quiet"))
parser.ClearFlagContracts("legacy")

// Read them back (returns a copy)
contracts, _ := parser.GetFlagContracts("token")
```

| Accessor | Behavior |
|---|---|
| `AddFlagContracts(flag, …Contract)` | Appends contracts to an existing flag. |
| `SetFlagContracts(flag, …Contract)` | Replaces all contracts on the flag. |
| `ClearFlagContracts(flag)` | Removes all contracts from the flag. |
| `GetFlagContracts(flag)` | Returns a deep copy of the flag's contracts. |

## Contract Reference

### `mutex(group)` — at most one

Every flag tagged with the same group name forms a mutually-exclusive set. Setting two or
more is an error; setting zero or one is fine.

```
$ app --json --yaml
error: only one of 'json', 'yaml' may be used at a time
```

### `exactlyone(group)` — exactly one

Like `mutex`, but the group is also **required**: the user must pick exactly one.

```
$ app                         # none chosen
error: one of 'from-file', 'from-url' must be set

$ app --from-file --from-url  # too many
error: only one of 'from-file', 'from-url' may be used at a time
```

### `conflicts(a,b,…)` — not together

When this flag is set, none of the named flags may also be set. Reports are de-duplicated,
so `a conflicts b` and `b conflicts a` produce a single error.

```
$ app --verbose --quiet
error: 'verbose' and 'quiet' cannot be used together
```

### `requires(a,b,…)` — co-requirement

When this flag is set, each named flag must also be set. This is **error-level** — distinct
from the warning-level [`DependsOn`]({{ site.baseurl }}/v2/guides/04-advanced-features/04-flag-inheritance/)
relationship. If the flag is absent, the requirement is not enforced.

```
$ app --deploy            # --deploy requires --target
error: 'deploy' requires 'target'
```

### `requiredOn(a,b,…)` — conditional requirement

The inverse of `requires`: this flag becomes required whenever any **trigger** is active. A
trigger may be another flag **or an invoked command** — flag and command names do not collide,
so a name resolves unambiguously.

```
$ app --remote            # --remote makes --token required
error: 'token' is required when 'remote' is used
```

> **`requiredOn` vs. `RequiredIf`.** `requiredOn` is the declarative, common case ("required
> when these other flags/commands are present"). `RequiredIf` remains the fully-flexible escape
> hatch for arbitrary, value-dependent logic — see [below](#when-to-use-what).

## Build-time vs. Runtime Errors

Contracts distinguish **developer mistakes** from **user mistakes**:

- **User-facing** violations (the cases above) are reported at parse time in the user's
  language, naming the flags they actually typed — never the internal group name.
- **Developer-facing** structural errors are caught earlier. A `mutex` or `exactlyone` group
  with **fewer than two members** is almost always a misspelled group name, so it is reported
  as a configuration error:

  ```go
  type CLI struct {
      Solo bool `goopt:"name:solo;contract:mutex(typo)"`  // only member of "typo"
  }
  _, err := goopt.NewParserFromStruct(&CLI{})
  // err: contract group "typo" has fewer than two members — likely a misspelled group name
  ```

  With the struct-first API this fails **construction** (`NewParserFromStruct` returns the
  error), keeping it out of end-user output. When you add contracts programmatically after the
  parser exists, the same guard runs on the next `Parse`.

## Internationalization

All contract messages are fully translatable through the standard i18n system. The user-facing
keys are `goopt.error.mutex_violation`, `conflicting_flags`, `flag_requires`, `required_when`,
and `exactly_one_required`; the developer-facing key is `goopt.error.singleton_contract_group`.
goopt ships translations for all built-in locales (and applies RTL bidi isolation around flag
names for right-to-left languages). See
[Internationalization]({{ site.baseurl }}/v2/guides/06-internationalization/index/).

## When to Use What

`goopt` offers several relational tools; pick the most specific one that fits:

| You want to… | Use |
|---|---|
| Validate a **single** value's format/range | A [Validator]({{ site.baseurl }}/v2/guides/04-advanced-features/01-validation/) |
| Allow at most one of a set | `mutex` |
| Force exactly one of a set | `exactlyone` |
| Forbid a specific combination | `conflicts` |
| Require companions when a flag is used | `requires` |
| Require a flag when others/commands are present | `requiredOn` |
| Express a value-conditional dependency (`--format=json` ⇒ `--pretty`) | [`DependsOn`]({{ site.baseurl }}/v2/guides/04-advanced-features/04-flag-inheritance/) (warning-level) |
| Arbitrary, hand-rolled "is this required?" logic | `RequiredIf` |

Contracts cover the common relational shapes declaratively; reach for `RequiredIf` only when
your rule is genuinely custom.
