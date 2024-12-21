// completion/zsh.go
package completion

import (
	"fmt"
	"strings"
)

type ZshGenerator struct{}

func (g *ZshGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	script.WriteString(fmt.Sprintf(`#compdef %[1]s

__%[1]s_completion() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \`, programName))

	// Global flags with descriptions
	for _, flag := range data.Flags {
		desc := escapeBash(data.Descriptions[flag])
		script.WriteString(fmt.Sprintf(`
        '*%s[%s]' \`, flag, desc))
	}

	// Add flag value completions
	for flag, values := range data.FlagValues {
		valuesList := make([]string, 0, len(values))
		for _, v := range values {
			if v.Description != "" {
				desc := strings.Replace(v.Description, " ", "\\ ", -1)
				valuesList = append(valuesList, fmt.Sprintf("%s\\:%s",
					v.Pattern, escapeBash(desc)))
			} else {
				valuesList = append(valuesList, v.Pattern)
			}
		}
		script.WriteString(fmt.Sprintf(`
        '*%s:value:(%s)' \`, flag, strings.Join(valuesList, " ")))
	}

	// Commands with descriptions
	script.WriteString(`
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            _values 'commands' \`)

	for cmd, desc := range data.CommandDescriptions {
		// Escape spaces in command names
		cmdName := strings.Replace(cmd, " ", "\\ ", -1)
		script.WriteString(fmt.Sprintf(`
                '%s[%s]' \`, cmdName, escapeBash(desc)))
	}

	// Command-specific flags
	script.WriteString(`
            ;;
        args)
            case $words[1] in`)

	for cmd, flags := range data.CommandFlags {
		script.WriteString(fmt.Sprintf(`
                %s)
                    _arguments \`, cmd))
		for _, flag := range flags {
			desc := escapeBash(data.Descriptions[cmd+"@"+flag])
			script.WriteString(fmt.Sprintf(`
                        '*%s[%s]' \`, flag, desc))

			// Add command-specific flag values if any
			if values, ok := data.FlagValues[cmd+"@"+flag]; ok {
				valuesList := make([]string, 0, len(values))
				for _, v := range values {
					if v.Description != "" {
						desc := strings.Replace(v.Description, " ", "\\ ", -1)
						valuesList = append(valuesList, fmt.Sprintf("%s\\:%s",
							v.Pattern, escapeBash(desc)))
					} else {
						valuesList = append(valuesList, v.Pattern)
					}
				}
				script.WriteString(fmt.Sprintf(`':value:(%s)' \`, strings.Join(valuesList, " ")))
			}
		}
		script.WriteString(`
                    ;;`)
	}

	script.WriteString(fmt.Sprintf(`
            esac
            ;;
    esac
}

__%[1]s_completion "$@"`, programName))

	return script.String()
}
