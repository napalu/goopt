package completion

import (
	"fmt"
	"strings"
)

type BashGenerator struct{}

func (g *BashGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	// Add i18n comment if translations are present
	hasTranslations := len(data.TranslatedCommands) > 0 || len(data.TranslatedFlags) > 0
	if hasTranslations {
		script.WriteString(fmt.Sprintf(`#!/bin/bash
# Shell completion for %s with i18n support

`, programName))
	} else {
		script.WriteString(`#!/bin/bash

`)
	}

	script.WriteString(fmt.Sprintf(`function __%[1]s_completion() {
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

	// Add flag value completions (including translated forms)
	for _, flag := range data.Flags {
		allLongForms := getAllFlagForms(data, flag.Long)
		
		if flag.Type == FlagTypeFile {
			// Handle file completion
			patterns := []string{}
			for _, form := range allLongForms {
				patterns = append(patterns, "--"+form)
			}
			if flag.Short != "" {
				patterns = append(patterns, "-"+flag.Short)
			}
			
			script.WriteString(fmt.Sprintf(`
        %s)
            _filedir
            return
            ;;`, strings.Join(patterns, "|")))
		} else if values, ok := data.FlagValues[flag.Long]; ok {
			// Handle value completion
			patterns := []string{}
			for _, form := range allLongForms {
				patterns = append(patterns, "--"+form)
			}
			if flag.Short != "" {
				patterns = append(patterns, "-"+flag.Short)
			}
			
			script.WriteString(fmt.Sprintf(`
        %s)
            COMPREPLY=( $(compgen -W "%s" -- "$cur") )
            return
            ;;`, strings.Join(patterns, "|"), strings.Join(getValueStrings(values), " ")))
		}
	}
	
	// Add command-specific flag value completions
	for _, cmdFlags := range data.CommandFlags {
		for _, flag := range cmdFlags {
			allLongForms := getAllFlagForms(data, flag.Long)
			
			if flag.Type == FlagTypeFile {
				patterns := []string{}
				for _, form := range allLongForms {
					patterns = append(patterns, "--"+form)
				}
				if flag.Short != "" {
					patterns = append(patterns, "-"+flag.Short)
				}
				
				script.WriteString(fmt.Sprintf(`
        %s)
            _filedir
            return
            ;;`, strings.Join(patterns, "|")))
			} else if values, ok := data.FlagValues[flag.Long]; ok {
				patterns := []string{}
				for _, form := range allLongForms {
					patterns = append(patterns, "--"+form)
				}
				if flag.Short != "" {
					patterns = append(patterns, "-"+flag.Short)
				}
				
				script.WriteString(fmt.Sprintf(`
        %s)
            COMPREPLY=( $(compgen -W "%s" -- "$cur") )
            return
            ;;`, strings.Join(patterns, "|"), strings.Join(getValueStrings(values), " ")))
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

	// Add nested command completions with i18n support
	seenBaseCommands := make(map[string][]string)
	canonicalToBase := make(map[string]string)
	
	for _, cmd := range data.Commands {
		parts := strings.Split(cmd, " ")
		if len(parts) > 1 {
			baseCanonical := parts[0]
			// Get all forms of the base command
			baseForms := getAllCommandForms(data, baseCanonical)
			for _, baseForm := range baseForms {
				seenBaseCommands[baseForm] = append(seenBaseCommands[baseForm], strings.Join(parts[1:], " "))
				canonicalToBase[baseForm] = baseCanonical
			}
		}
	}

	for baseCmd, subCmds := range seenBaseCommands {
		// Get all subcommand forms
		allSubCmds := []string{}
		for _, subCmd := range subCmds {
			fullCmd := canonicalToBase[baseCmd] + " " + subCmd
			// Get translated subcommand
			preferredSubCmd := extractTranslatedSubcommand(data, fullCmd, canonicalToBase[baseCmd], subCmd)
			allSubCmds = append(allSubCmds, preferredSubCmd)
			// Add canonical form if different
			if preferredSubCmd != subCmd {
				allSubCmds = append(allSubCmds, subCmd)
			}
		}
		
		script.WriteString(fmt.Sprintf(`
            %s)
                COMPREPLY=( $(compgen -W "%s" -- "$sub_cmd") )
                return
                ;;`, baseCmd, strings.Join(allSubCmds, " ")))
	}

	script.WriteString(`
        esac
    fi

    # If we're completing a flag
    if [[ "$cur" == -* ]]; then
        local flags=""

        # Global flags`)

	// Add global flags (including translated forms)
	for _, flag := range data.Flags {
		for _, form := range getAllFlagForms(data, flag.Long) {
			script.WriteString(fmt.Sprintf(`
        flags="$flags --%s"`, form))
		}
		if flag.Short != "" {
			script.WriteString(fmt.Sprintf(`
        flags="$flags -%s"`, flag.Short))
		}
	}

	// Add command-specific flags (without descriptions)
	script.WriteString(`

        # Command-specific flags
        case "$cmd" in`)

	for cmdName, flags := range data.CommandFlags {
		if len(flags) > 0 {
			// Get all forms of the command
			allCmdForms := getAllCommandForms(data, cmdName)
			
			for _, cmdForm := range allCmdForms {
				script.WriteString(fmt.Sprintf(`
            %s)
                local cmd_flags=""`, cmdForm))
				for _, flag := range flags {
					for _, form := range getAllFlagForms(data, flag.Long) {
						script.WriteString(fmt.Sprintf(`
                cmd_flags="$cmd_flags --%s"`, form))
					}
					if flag.Short != "" {
						script.WriteString(fmt.Sprintf(`
                cmd_flags="$cmd_flags -%s"`, flag.Short))
					}
				}
				script.WriteString(`
                flags="$flags$cmd_flags"
                ;;`)
			}
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

	// Add all command forms (canonical and translated)
	processedCommands := make(map[string]bool)
	for _, cmd := range data.Commands {
		if !processedCommands[cmd] {
			processedCommands[cmd] = true
			forms := getAllCommandForms(data, cmd)
			for _, form := range forms {
				// Escape spaces in command names for bash
				escapedForm := strings.ReplaceAll(form, " ", "\\ ")
				script.WriteString(fmt.Sprintf(`
        commands="$commands %s"`, escapedForm))
			}
		}
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
