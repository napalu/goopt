package completion

import (
	"fmt"
	"strings"
)

type ZshGenerator struct{}

func (g *ZshGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	script.WriteString(fmt.Sprintf(`#compdef %s

function _%s() {
    local -a commands flags

    # Define commands with descriptions
    commands=(`, programName, programName))

	// Add commands and their descriptions
	for _, cmd := range data.Commands {
		if !strings.Contains(cmd, " ") {
			desc := data.CommandDescriptions[cmd]
			script.WriteString(fmt.Sprintf(`
        "%s:%s"`, cmd, escapeZsh(desc)))
		}
	}

	script.WriteString(`
    )

    # Define subcommands for each command
    local -A subcmds=(`)

	// Group subcommands by their parent command
	commandGroups := make(map[string][]string)
	for _, cmd := range data.Commands {
		parts := strings.Split(cmd, " ")
		if len(parts) > 1 {
			parent := parts[0]
			sub := parts[1]
			desc := data.CommandDescriptions[cmd]
			if _, ok := commandGroups[parent]; !ok {
				commandGroups[parent] = make([]string, 0)
			}
			commandGroups[parent] = append(commandGroups[parent], fmt.Sprintf("%s:%s", sub, desc))
		}
	}

	// Add subcommands
	for parent, subs := range commandGroups {
		script.WriteString(fmt.Sprintf(`
        "%s:(%s)"`, parent, strings.Join(subs, " ")))
	}

	script.WriteString(`
    )

    # Define flags with descriptions
    flags=(`)

	// Add global flags
	for _, flag := range data.Flags {
		if flag.Short != "" {
			// Flag with both short and long forms - short form first
			script.WriteString(fmt.Sprintf(`
        "(-%s --%s)"{-%s,--%s}"[%s]"`,
				flag.Short, flag.Long,
				flag.Short, flag.Long,
				escapeZsh(flag.Description)))

			// Add value completion if available
			if values, ok := data.FlagValues[flag.Long]; ok {
				var valueStrs []string
				for _, v := range values {
					valueStrs = append(valueStrs, fmt.Sprintf("%s\\:\"%s\"", v.Pattern, escapeZsh(v.Description)))
				}
				script.WriteString(fmt.Sprintf(`:(%s)`, strings.Join(valueStrs, " ")))
			} else if flag.Type == FlagTypeFile {
				script.WriteString(":_files")
			}
		} else {
			// Flag with only long form
			script.WriteString(fmt.Sprintf(`
        "--%s[%s]"`, flag.Long, escapeZsh(flag.Description)))

			// Add value completion if available
			if values, ok := data.FlagValues[flag.Long]; ok {
				var valueStrs []string
				for _, v := range values {
					valueStrs = append(valueStrs, fmt.Sprintf("%s\\:\"%s\"", v.Pattern, escapeZsh(v.Description)))
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

	// Add command-specific flags
	for cmd, flags := range data.CommandFlags {
		script.WriteString(fmt.Sprintf(`
                %s)
                    local -a cmd_flags=(`, cmd))
		for _, flag := range flags {
			if flag.Short != "" {
				script.WriteString(fmt.Sprintf(`
                        "(-%s --%s)"{-%s,--%s}"[%s]"`,
					flag.Short, flag.Long,
					flag.Short, flag.Long,
					escapeZsh(flag.Description)))
			} else {
				script.WriteString(fmt.Sprintf(`
                        "--%s[%s]"`, flag.Long, escapeZsh(flag.Description)))
			}
		}
		script.WriteString(`
                    )
                    _arguments $cmd_flags
                    ;;`)
	}

	script.WriteString(fmt.Sprintf(`
            esac
            ;;
    esac
}

_%s "$@"`, programName))

	return script.String()
}
