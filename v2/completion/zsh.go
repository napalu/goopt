package completion

import (
	"fmt"
	"strings"
)

type ZshGenerator struct{}

func (g *ZshGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	// Add i18n comment if translations are present
	hasTranslations := len(data.TranslatedCommands) > 0 || len(data.TranslatedFlags) > 0
	if hasTranslations {
		script.WriteString(fmt.Sprintf(`#compdef %s
# Zsh completion for %[1]s with i18n support

`, programName))
	} else {
		script.WriteString(fmt.Sprintf(`#compdef %s

`, programName))
	}

	script.WriteString(fmt.Sprintf(`function _%s() {
    local -a commands flags

    # Define commands with descriptions
    commands=(`, programName))

	// Add commands and their descriptions (showing translated names when available)
	processedCommands := make(map[string]bool)
	for _, cmd := range data.Commands {
		if !strings.Contains(cmd, " ") && !processedCommands[cmd] {
			processedCommands[cmd] = true

			// Get preferred form and description
			preferredForm := getPreferredCommandForm(data, cmd)
			desc := data.CommandDescriptions[cmd]

			// If translated form differs, show canonical in description
			if preferredForm != cmd && hasTranslations {
				if desc != "" {
					desc = fmt.Sprintf("%s (canonical: %s)", desc, cmd)
				} else {
					desc = fmt.Sprintf("(canonical: %s)", cmd)
				}
			}

			script.WriteString(fmt.Sprintf(`
        "%s:%s"`, preferredForm, escapeZsh(desc)))

			// Add canonical form as hidden alternative if different
			if preferredForm != cmd {
				script.WriteString(fmt.Sprintf(`
        "%s:%s"`, cmd, escapeZsh(data.CommandDescriptions[cmd])))
			}
		}
	}

	script.WriteString(`
    )

    # Define subcommands for each command
    local -A subcmds=(`)

	// Group subcommands by their parent command with i18n support
	commandGroups := make(map[string][]string)
	for _, cmd := range data.Commands {
		parts := strings.Split(cmd, " ")
		if len(parts) > 1 {
			parent := parts[0]
			sub := strings.Join(parts[1:], " ")

			// Get preferred forms
			fullCmd := cmd
			preferredFullForm := getPreferredCommandForm(data, fullCmd)

			// Extract just the subcommand part
			preferredSub := sub
			// Try to extract from translated form
			// First get the preferred parent form
			preferredParent := getPreferredCommandForm(data, parent)
			if strings.HasPrefix(preferredFullForm, preferredParent+" ") {
				preferredSub = strings.TrimPrefix(preferredFullForm, preferredParent+" ")
			} else if strings.HasPrefix(preferredFullForm, parent+" ") {
				preferredSub = strings.TrimPrefix(preferredFullForm, parent+" ")
			}

			desc := data.CommandDescriptions[cmd]

			// Show canonical form in description if different
			if preferredSub != sub && hasTranslations {
				if desc != "" {
					desc = fmt.Sprintf("%s (canonical: %s)", desc, sub)
				} else {
					desc = fmt.Sprintf("(canonical: %s)", sub)
				}
			}

			if _, ok := commandGroups[parent]; !ok {
				commandGroups[parent] = make([]string, 0)
			}
			commandGroups[parent] = append(commandGroups[parent], fmt.Sprintf("%s:%s", preferredSub, desc))

			// Also add canonical form if different
			if preferredSub != sub {
				commandGroups[parent] = append(commandGroups[parent], fmt.Sprintf("%s:%s", sub, data.CommandDescriptions[cmd]))
			}
		}
	}

	// Add subcommands (including all forms of parent commands)
	processedParents := make(map[string]bool)
	for parent, subs := range commandGroups {
		if !processedParents[parent] {
			processedParents[parent] = true

			// Get all forms of the parent command
			allParentForms := getAllCommandForms(data, parent)
			for _, parentForm := range allParentForms {
				script.WriteString(fmt.Sprintf(`
        "%s:(%s)"`, parentForm, strings.Join(subs, " ")))
			}
		}
	}

	script.WriteString(`
    )

    # Define flags with descriptions
    flags=(`)

	// Add global flags with i18n support
	for _, flag := range data.Flags {
		// Get preferred form
		preferredLong := getPreferredFlagForm(data, flag.Long)

		// Build description that includes canonical form if different
		desc := flag.Description
		if preferredLong != flag.Long && hasTranslations {
			if desc != "" {
				desc = fmt.Sprintf("%s (canonical: --%s)", desc, flag.Long)
			} else {
				desc = fmt.Sprintf("(canonical: --%s)", flag.Long)
			}
		}

		if flag.Short != "" {
			// Flag with both short and long forms - short form first
			script.WriteString(fmt.Sprintf(`
        "(-%s --%s)"{-%s,--%s}"[%s]"`,
				flag.Short, preferredLong,
				flag.Short, preferredLong,
				escapeZsh(desc)))
		} else {
			// Flag with only long form
			script.WriteString(fmt.Sprintf(`
        "--%s[%s]"`, preferredLong, escapeZsh(desc)))
		}

		// Add value completion if available
		if values, ok := data.FlagValues[flag.Long]; ok {
			var valueStrs []string
			for _, v := range values {
				valueStrs = append(valueStrs, fmt.Sprintf("%s\\:\"%s\"", escapePatternZsh(v.Pattern), escapeZsh(v.Description)))
			}
			script.WriteString(fmt.Sprintf(`:(%s)`, strings.Join(valueStrs, " ")))
		} else if flag.Type == FlagTypeFile {
			script.WriteString(":_files")
		}

		// Add canonical form as hidden alternative if different
		if preferredLong != flag.Long {
			script.WriteString(fmt.Sprintf(`
        "--{}%s[%s]"`, flag.Long, escapeZsh(flag.Description)))

			// Add value completion for canonical form too
			if values, ok := data.FlagValues[flag.Long]; ok {
				var valueStrs []string
				for _, v := range values {
					valueStrs = append(valueStrs, fmt.Sprintf("%s\\:\"%s\"", escapePatternZsh(v.Pattern), escapeZsh(v.Description)))
				}
				script.WriteString(fmt.Sprintf(`:(%s)`, strings.Join(valueStrs, " ")))
			} else if flag.Type == FlagTypeFile {
				script.WriteString(":_files")
			}
		}
	}

	script.WriteString(`
    )

    _arguments -C \
        $flags \
        "1: :->command" \
        "*:: :->args"

    case $state in
        command)
            _describe "commands" commands
            ;;
        args)
            local cmd=$words[1]
            if (( CURRENT == 2 )); then
                if [[ -n "${subcmds[$cmd]}" ]]; then
                    local -a subcmd_list
                    subcmd_list=( ${(P)subcmds[$cmd]} )
                    _describe "subcommands" subcmd_list
                    return
                fi
            fi
            case $cmd in`)

	// Add command-specific flags with i18n support
	processedCmds := make(map[string]bool)
	for cmd, flags := range data.CommandFlags {
		if !processedCmds[cmd] {
			processedCmds[cmd] = true

			// Get all forms of the command
			allCmdForms := getAllCommandForms(data, cmd)

			// Build patterns for all forms
			patterns := strings.Join(allCmdForms, "|")

			script.WriteString(fmt.Sprintf(`
                %s)
                    local -a cmd_flags=(`, patterns))

			// Add command-specific flags with i18n
			for _, flag := range flags {
				preferredLong := getPreferredFlagForm(data, flag.Long)
				desc := flag.Description

				if preferredLong != flag.Long && hasTranslations {
					if desc != "" {
						desc = fmt.Sprintf("%s (canonical: --%s)", desc, flag.Long)
					} else {
						desc = fmt.Sprintf("(canonical: --%s)", flag.Long)
					}
				}

				if flag.Short != "" {
					script.WriteString(fmt.Sprintf(`
                        "(-%s --%s)"{-%s,--%s}"[%s]"`,
						flag.Short, preferredLong,
						flag.Short, preferredLong,
						escapeZsh(desc)))
				} else {
					script.WriteString(fmt.Sprintf(`
                        "--%s[%s]"`, preferredLong, escapeZsh(desc)))
				}

				// Add canonical as hidden alternative
				if preferredLong != flag.Long {
					script.WriteString(fmt.Sprintf(`
                        "--{}%s[%s]"`, flag.Long, escapeZsh(flag.Description)))
				}
			}

			// Also include global flags
			for _, flag := range data.Flags {
				preferredLong := getPreferredFlagForm(data, flag.Long)
				desc := flag.Description

				if preferredLong != flag.Long && hasTranslations {
					if desc != "" {
						desc = fmt.Sprintf("%s (canonical: --%s)", desc, flag.Long)
					} else {
						desc = fmt.Sprintf("(canonical: --%s)", flag.Long)
					}
				}

				if flag.Short != "" {
					script.WriteString(fmt.Sprintf(`
                        "(-%s --%s)"{-%s,--%s}"[%s]"`,
						flag.Short, preferredLong,
						flag.Short, preferredLong,
						escapeZsh(desc)))
				} else {
					script.WriteString(fmt.Sprintf(`
                        "--%s[%s]"`, preferredLong, escapeZsh(desc)))
				}

				if preferredLong != flag.Long {
					script.WriteString(fmt.Sprintf(`
                        "--{}%s[%s]"`, flag.Long, escapeZsh(flag.Description)))
				}
			}

			script.WriteString(`
                    )
                    _arguments $cmd_flags
                    ;;`)
		}
	}

	script.WriteString(fmt.Sprintf(`
            esac
            ;;
    esac
}

_%s "$@"`, programName))

	return script.String()
}
