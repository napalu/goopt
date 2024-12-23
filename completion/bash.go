package completion

import (
	"fmt"
	"strings"
)

type BashGenerator struct{}

func (g *BashGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	script.WriteString(fmt.Sprintf(`#!/bin/bash

function __%[1]s_completion() {
    local cur prev words cword cmd subcmd
    _init_completion || return

    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    cmd=""

    # Find the main command
    for ((i=1; i < COMP_CWORD; i++)); do
        if [[ "${COMP_WORDS[i]}" != -* ]]; then
            cmd="${COMP_WORDS[i]}"
            break
        fi
    done

    # Handle flag values
    case "${prev}" in`, programName))

	// Add flag value completions
	for _, flag := range data.Flags {
		if flag.Type == FlagTypeFile {
			// Handle file completion
			if flag.Short != "" {
				script.WriteString(fmt.Sprintf(`
        -%s|--%s)
            _filedir
            return
            ;;`, flag.Short, flag.Long))
			} else {
				script.WriteString(fmt.Sprintf(`
        --%s)
            _filedir
            return
            ;;`, flag.Long))
			}
		} else if values, ok := data.FlagValues[flag.Long]; ok {
			// Handle value completion
			if flag.Short != "" {
				script.WriteString(fmt.Sprintf(`
        -%s|--%s)
            COMPREPLY=( $(compgen -W "%s" -- "$cur") )
            return
            ;;`, flag.Short, flag.Long, strings.Join(getValueStrings(values), " ")))
			} else {
				script.WriteString(fmt.Sprintf(`
        --%s)
            COMPREPLY=( $(compgen -W "%s" -- "$cur") )
            return
            ;;`, flag.Long, strings.Join(getValueStrings(values), " ")))
			}
		}
	}

	script.WriteString(`
    esac

    # Handle nested commands
    if [[ "$cmd" == *" "* ]]; then
        local base_cmd="${cmd%% *}"
        local sub_cmd="${cmd#* }"
        case "$base_cmd" in`)

	// Add nested command completions
	seenBaseCommands := make(map[string][]string)
	for _, cmd := range data.Commands {
		parts := strings.Split(cmd, " ")
		if len(parts) > 1 {
			seenBaseCommands[parts[0]] = append(seenBaseCommands[parts[0]], strings.Join(parts[1:], " "))
		}
	}

	for baseCmd, subCmds := range seenBaseCommands {
		script.WriteString(fmt.Sprintf(`
            %s)
                COMPREPLY=( $(compgen -W "%s" -- "$sub_cmd") )
                return
                ;;`, baseCmd, strings.Join(subCmds, " ")))
	}

	script.WriteString(`
        esac
    fi

    # If we're completing a flag
    if [[ "$cur" == -* ]]; then
        local flags=""

        # Global flags`)

	// Add global flags (without descriptions)
	for _, flag := range data.Flags {
		if flag.Short != "" {
			script.WriteString(fmt.Sprintf(`
        flags="$flags -%s"`, flag.Short))
		}
		script.WriteString(fmt.Sprintf(`
        flags="$flags --%s"`, flag.Long))
	}

	// Add command-specific flags (without descriptions)
	script.WriteString(`

        # Command-specific flags
        case "$cmd" in`)

	for cmdName, flags := range data.CommandFlags {
		if len(flags) > 0 {
			script.WriteString(fmt.Sprintf(`
            %s)
                local cmd_flags=""`, cmdName))
			for _, flag := range flags {
				if flag.Short != "" {
					script.WriteString(fmt.Sprintf(`
                cmd_flags="$cmd_flags -%s"`, flag.Short))
				}
				script.WriteString(fmt.Sprintf(`
                cmd_flags="$cmd_flags --%s"`, flag.Long))
			}
			script.WriteString(`
                flags="$flags$cmd_flags"
                ;;`)
		}
	}

	script.WriteString(`
        esac

        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return
    fi

    # Complete commands if no command is present yet
    if [[ -z "$cmd" ]]; then
        local commands=""`)

	// Add command completions (without descriptions)
	for _, cmd := range data.Commands {
		escapedCmd := strings.ReplaceAll(cmd, " ", "\\ ")
		script.WriteString(fmt.Sprintf(`
        commands="$commands %s"`, escapedCmd))
	}

	script.WriteString(fmt.Sprintf(`
        COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
    fi
}

complete -o default -F __%s_completion %s`, programName, programName))

	return script.String()
}

func getValueStrings(values []CompletionValue) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = escapePatternBash(v.Pattern)
	}
	return result
}
