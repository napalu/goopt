// completion/bash.go
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

	// Add flag value completions in order
	for _, flag := range data.Flags {
		if values, ok := data.FlagValues[flag]; ok {
			valStrs := make([]string, len(values))
			for i, v := range values {
				valStrs[i] = fmt.Sprintf("%s[%s]", v.Pattern, escapeBash(v.Description))
			}
			script.WriteString(fmt.Sprintf(`
        %s)
            local vals=(%s)
            COMPREPLY=( $(compgen -W "${vals[*]%%%%[*}" -- "$cur") )
            return
            ;;`, flag, strings.Join(valStrs, " ")))
		}
	}

	script.WriteString(`
    esac

    # Handle nested commands
    for c in "${COMP_WORDS[@]}"; do
        if [[ "${c}" == *" "* ]]; then
            cmd="${c%%[*}"
            subcmd="${c##* }"
            case "${cmd}" in`)

	// Process subcommands maintaining parent order
	mainCmds := make(map[string][]string)
	for _, cmd := range data.Commands {
		if strings.Contains(cmd, " ") {
			parts := strings.SplitN(cmd, " ", 2)
			mainCmd := parts[0]
			mainCmds[mainCmd] = append(mainCmds[mainCmd], cmd)
		}
	}

	// Output subcommands in original order
	for _, mainCmd := range data.Commands {
		if subCmds, ok := mainCmds[mainCmd]; ok {
			quotedCmds := make([]string, len(subCmds))
			for i, cmd := range subCmds {
				quotedCmds[i] = fmt.Sprintf(`"%s"`, cmd)
			}
			script.WriteString(fmt.Sprintf(`
                %s)
                    COMPREPLY+=( $(compgen -W %s -- "$subcmd") )
                    ;;`, mainCmd, strings.Join(quotedCmds, " ")))
		}
	}

	script.WriteString(`
            esac
        fi
    done

    # If we're completing a flag
    if [[ "$cur" == -* ]]; then
        local flags=()

        # Global flags`)

	// Add global flags in order
	for _, flag := range data.Flags {
		desc := data.Descriptions[flag]
		script.WriteString(fmt.Sprintf(`
        flags+=(%s[%s])`, flag, escapeBash(desc)))
	}

	// Add command-specific flags maintaining command order
	script.WriteString(`

        # Command-specific flags
        case "${cmd}" in`)

	for _, cmd := range data.Commands {
		if flags, ok := data.CommandFlags[cmd]; ok && len(flags) > 0 {
			flagStrs := make([]string, len(flags))
			for i, flag := range flags {
				desc := data.Descriptions[cmd+"@"+flag]
				flagStrs[i] = fmt.Sprintf("%s[%s]", flag, escapeBash(desc))
			}
			script.WriteString(fmt.Sprintf(`
            %s)
                local cmd_flags=(%s)
                flags+=("${cmd_flags[@]}")
                ;;`, cmd, strings.Join(flagStrs, " ")))
		}
	}

	script.WriteString(`
        esac

        COMPREPLY=( $(compgen -W "${flags[*]%%%%[*}" -- "$cur") )
        return
    fi

    # Complete commands if no command is present yet
    if [[ -z "$cmd" ]]; then
        local commands=(`)

	// Add command completions in original order
	cmdStrs := make([]string, 0, len(data.Commands))
	for _, cmd := range data.Commands {
		desc := data.CommandDescriptions[cmd]
		cmdStrs = append(cmdStrs, fmt.Sprintf("%s[%s]", cmd, escapeBash(desc)))
	}
	script.WriteString(strings.Join(cmdStrs, " "))

	script.WriteString(fmt.Sprintf(`)
        COMPREPLY=( $(compgen -W "${commands[*]%%%%[*}" -- "$cur") )
    fi
}

complete -F __%[1]s_completion %[1]s`, programName))

	return script.String()
}
