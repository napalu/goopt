package goopt

import (
	"sort"
	"strings"

	"github.com/napalu/goopt/v2/errs"
)

// ContractKind enumerates the cross-flag relational constraints a flag can
// declare under the `contract:` directive. Contracts are evaluated after parsing
// completes, when the full set of flags and their presence is known.
type ContractKind int

const (
	// ContractMutex is mutex(group): at most one flag in the named group may be set.
	ContractMutex ContractKind = iota
	// ContractConflicts is conflicts(a,b): this flag may not be set together with any of the named flags.
	ContractConflicts
	// ContractRequires is requires(a,b): when this flag is set, each named flag must also be set.
	ContractRequires
	// ContractRequiredOn is requiredOn(a,b): this flag is required whenever any named flag is set or command is invoked.
	ContractRequiredOn
	// ContractExactlyOne is exactlyone(group): exactly one flag in the named group must be set (mutex + required).
	ContractExactlyOne
)

// Contract is a single relational constraint declared on a flag. For mutex,
// Targets holds the group name; for conflicts, the list of conflicting flag names.
type Contract struct {
	Kind    ContractKind
	Targets []string
}

// parseContracts converts contract specifications (e.g. "mutex(source)",
// "conflicts(a,b)") into Contracts. Unlike validators, contracts do not nest —
// the spec language is deliberately flat.
func parseContracts(specs []string) ([]Contract, error) {
	var contracts []Contract
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		c, err := parseContract(spec)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, c)
	}
	return contracts, nil
}

func parseContract(spec string) (Contract, error) {
	open := strings.Index(spec, "(")
	if open <= 0 || !strings.HasSuffix(spec, ")") {
		return Contract{}, errs.ErrInvalidContract.WithArgs(spec)
	}
	name := strings.ToLower(strings.TrimSpace(spec[:open]))
	argsStr := spec[open+1 : len(spec)-1]

	var args []string
	for _, a := range strings.Split(argsStr, ",") {
		if a = strings.TrimSpace(a); a != "" {
			args = append(args, a)
		}
	}

	switch name {
	case "mutex":
		if len(args) != 1 {
			return Contract{}, errs.ErrContractArgs.WithArgs(name)
		}
		return Contract{Kind: ContractMutex, Targets: args}, nil
	case "exactlyone":
		if len(args) != 1 {
			return Contract{}, errs.ErrContractArgs.WithArgs(name)
		}
		return Contract{Kind: ContractExactlyOne, Targets: args}, nil
	case "conflicts":
		if len(args) == 0 {
			return Contract{}, errs.ErrContractArgs.WithArgs(name)
		}
		return Contract{Kind: ContractConflicts, Targets: args}, nil
	case "requires":
		if len(args) == 0 {
			return Contract{}, errs.ErrContractArgs.WithArgs(name)
		}
		return Contract{Kind: ContractRequires, Targets: args}, nil
	case "requiredon":
		if len(args) == 0 {
			return Contract{}, errs.ErrContractArgs.WithArgs(name)
		}
		return Contract{Kind: ContractRequiredOn, Targets: args}, nil
	default:
		return Contract{}, errs.ErrUnknownContract.WithArgs(name)
	}
}

// validateContractGroups runs the structural (build-time) checks on contract
// groups: a mutex/exactlyone group with fewer than two members is almost always a
// misspelled group name. It runs once; errors are added to the parser and the
// first is returned so NewParserFromStruct can fail construction — keeping this
// developer-facing error out of end-user runtime output.
func (p *Parser) validateContractGroups() error {
	if p.contractGroupsChecked {
		return nil
	}
	p.contractGroupsChecked = true

	// Group membership is scoped to the owning command: a mutex/exactlyone group in
	// `export` is independent of a same-named group in `sync`, so a singleton in one
	// command is still caught even if another command happens to reuse the label.
	type groupKey struct{ cmd, label string }
	counts := map[groupKey]int{}
	for _, flagInfo := range p.acceptedFlags.All() {
		for _, c := range flagInfo.Argument.Contracts {
			if (c.Kind == ContractMutex || c.Kind == ContractExactlyOne) && len(c.Targets) > 0 {
				counts[groupKey{flagInfo.CommandPath, c.Targets[0]}]++
			}
		}
	}

	keys := make([]groupKey, 0, len(counts))
	for g := range counts {
		keys = append(keys, g)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].cmd != keys[j].cmd {
			return keys[i].cmd < keys[j].cmd
		}
		return keys[i].label < keys[j].label
	})

	var firstErr error
	for _, g := range keys {
		if counts[g] < 2 {
			e := errs.ErrSingletonContractGroup.WithArgs(g.label)
			p.addError(e)
			if firstErr == nil {
				firstErr = e
			}
		}
	}
	return firstErr
}

// validateContracts evaluates cross-flag contracts after parsing completes.
// Contracts are command-scoped: a flag owned by a command participates only when
// that command (or one of its subcommands) was invoked, and its target names
// resolve within the command's flag scope. Global flags always participate.
func (p *Parser) validateContracts() {
	p.validateContractGroups()

	invoked := p.GetCommands()
	isActive := func(cmdPath string) bool {
		if cmdPath == "" {
			return true // global flag — always in scope
		}
		for _, ip := range invoked {
			// The owning command is active when it, or a child of it, was invoked
			// (a parent's flags are inherited by its subcommands).
			if ip == cmdPath || strings.HasPrefix(ip, cmdPath+" ") {
				return true
			}
		}
		return false
	}

	type member struct {
		key     string
		present bool
	}
	groups := map[string][]member{}
	groupRequired := map[string]bool{}
	conflictsReported := map[string]bool{}

	for flagKey, flagInfo := range p.acceptedFlags.All() {
		cmdPath := flagInfo.CommandPath
		if !isActive(cmdPath) {
			continue // contracts on flags of an uninvoked command don't apply
		}
		present := p.HasFlag(flagKey)
		for _, c := range flagInfo.Argument.Contracts {
			switch c.Kind {
			case ContractMutex, ContractExactlyOne:
				if len(c.Targets) == 0 {
					continue
				}
				g := c.Targets[0]
				groups[g] = append(groups[g], member{flagKey, present})
				if c.Kind == ContractExactlyOne {
					groupRequired[g] = true
				}
			case ContractConflicts:
				if !present {
					continue
				}
				for _, other := range c.Targets {
					otherKey := p.flagOrShortFlag(other, cmdPath)
					if !p.HasFlag(otherKey) {
						continue
					}
					// Dedup symmetric reports (a conflicts b == b conflicts a).
					if conflictsReported[flagKey+"\x00"+otherKey] || conflictsReported[otherKey+"\x00"+flagKey] {
						continue
					}
					conflictsReported[flagKey+"\x00"+otherKey] = true
					p.addError(errs.ErrConflictingFlags.WithArgs(
						p.formatFlagForError(flagKey), p.formatFlagForError(otherKey)))
				}
			case ContractRequires:
				if !present {
					continue
				}
				for _, target := range c.Targets {
					targetKey := p.flagOrShortFlag(target, cmdPath)
					if !p.HasFlag(targetKey) {
						p.addError(errs.ErrFlagRequires.WithArgs(
							p.formatFlagForError(flagKey), p.formatFlagForError(targetKey)))
					}
				}
			case ContractRequiredOn:
				if present {
					continue
				}
				if trigger, ok := p.contractActiveTrigger(c.Targets, cmdPath); ok {
					p.addError(errs.ErrRequiredWhen.WithArgs(
						p.formatFlagForError(flagKey), trigger))
				}
			}
		}
	}

	// mutex: deterministic order, singleton-group guard, then at-most-one.
	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)
	for _, g := range names {
		members := groups[g]
		if len(members) < 2 {
			// Singleton groups are a config error raised once by
			// validateContractGroups (build-time); skip cardinality here.
			continue
		}
		var set, all []string
		for _, m := range members {
			all = append(all, p.formatFlagForError(m.key))
			if m.present {
				set = append(set, p.formatFlagForError(m.key))
			}
		}
		if len(set) > 1 {
			// User-facing: name the flags they typed, not the internal group name.
			p.addError(errs.ErrMutexViolation.WithArgs(strings.Join(set, ", ")))
		}
		if groupRequired[g] && len(set) == 0 {
			p.addError(errs.ErrExactlyOneRequired.WithArgs(strings.Join(all, ", ")))
		}
	}
}

// Mutex builds a mutex(group) contract: at most one flag carrying this group may
// be set. Use it with WithContracts or the Parser's AddFlagContracts/
// SetFlagContracts accessors; WithMutex is the equivalent construction-time shorthand.
func Mutex(group string) Contract {
	return Contract{Kind: ContractMutex, Targets: []string{group}}
}

// ExactlyOne builds an exactlyone(group) contract: exactly one flag in the group
// must be set (mutually exclusive and required).
func ExactlyOne(group string) Contract {
	return Contract{Kind: ContractExactlyOne, Targets: []string{group}}
}

// Conflicts builds a conflicts(flags...) contract: this flag may not be set
// together with any of the named flags.
func Conflicts(flags ...string) Contract {
	return Contract{Kind: ContractConflicts, Targets: flags}
}

// Requires builds a requires(flags...) contract: when this flag is set, each
// named flag must also be set (a hard requirement, error-level).
func Requires(flags ...string) Contract {
	return Contract{Kind: ContractRequires, Targets: flags}
}

// RequiredOn builds a requiredOn(targets...) contract: this flag is required
// whenever any named flag is set or named command is invoked.
func RequiredOn(targets ...string) Contract {
	return Contract{Kind: ContractRequiredOn, Targets: targets}
}

// WithMutex declares this flag a member of a mutually-exclusive group: at most
// one flag carrying mutex(group) may be set.
func WithMutex(group string) ConfigureArgumentFunc {
	return WithContracts(Mutex(group))
}

// WithExactlyOne declares this flag a member of a group from which exactly one
// flag must be set (mutually exclusive and required).
func WithExactlyOne(group string) ConfigureArgumentFunc {
	return WithContracts(ExactlyOne(group))
}

// WithConflicts declares that this flag may not be set together with any of the
// named flags.
func WithConflicts(flags ...string) ConfigureArgumentFunc {
	return WithContracts(Conflicts(flags...))
}

// WithContracts adds pre-built contracts to the argument.
func WithContracts(contracts ...Contract) ConfigureArgumentFunc {
	return func(a *Argument, err *error) {
		a.Contracts = append(a.Contracts, contracts...)
	}
}

// WithRequires declares that when this flag is set, each named flag must also be
// set (a hard requirement, error-level).
func WithRequires(flags ...string) ConfigureArgumentFunc {
	return WithContracts(Requires(flags...))
}

// WithRequiredOn declares this flag required whenever any of the named flags is
// set or commands is invoked.
func WithRequiredOn(targets ...string) ConfigureArgumentFunc {
	return WithContracts(RequiredOn(targets...))
}

// AddFlagContracts appends relational contracts to an existing flag. The flag
// must already be registered. Mirrors AddFlagValidators. Adding a contract clears
// the cached structural check so the singleton-group guard re-runs on the next Parse.
func (p *Parser) AddFlagContracts(flag string, contracts ...Contract) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return errs.ErrFlagDoesNotExist.WithArgs(flag)
	}
	flagInfo.Argument.Contracts = append(flagInfo.Argument.Contracts, contracts...)
	p.contractGroupsChecked = false
	return nil
}

// SetFlagContracts replaces all relational contracts on an existing flag.
// Mirrors SetFlagValidators.
func (p *Parser) SetFlagContracts(flag string, contracts ...Contract) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return errs.ErrFlagDoesNotExist.WithArgs(flag)
	}
	flagInfo.Argument.Contracts = contracts
	p.contractGroupsChecked = false
	return nil
}

// ClearFlagContracts removes all relational contracts from an existing flag.
// Mirrors ClearFlagValidators.
func (p *Parser) ClearFlagContracts(flag string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return errs.ErrFlagDoesNotExist.WithArgs(flag)
	}
	flagInfo.Argument.Contracts = nil
	p.contractGroupsChecked = false
	return nil
}

// GetFlagContracts returns the relational contracts declared on a flag (a copy,
// so the caller cannot mutate parser state). Returns an error if the flag is unknown.
func (p *Parser) GetFlagContracts(flag string) ([]Contract, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	flagInfo, found := p.acceptedFlags.Get(flag)
	if !found {
		return nil, errs.ErrFlagDoesNotExist.WithArgs(flag)
	}
	out := make([]Contract, len(flagInfo.Argument.Contracts))
	for i, c := range flagInfo.Argument.Contracts {
		targets := make([]string, len(c.Targets))
		copy(targets, c.Targets)
		out[i] = Contract{Kind: c.Kind, Targets: targets}
	}
	return out, nil
}

// contractActiveTrigger returns a formatted description of the first active
// target (a set flag or an invoked command) and whether one was found. Flag and
// command names do not collide, so a name resolves unambiguously.
func (p *Parser) contractActiveTrigger(targets []string, cmdPath string) (string, bool) {
	for _, t := range targets {
		key := p.flagOrShortFlag(t, cmdPath)
		if p.HasFlag(key) {
			return p.formatFlagForError(key), true
		}
		for _, cmd := range p.GetCommands() {
			if cmd == t {
				return "'" + t + "'", true
			}
		}
	}
	return "", false
}
