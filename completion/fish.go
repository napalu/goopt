package completion

import (
	"fmt"
	"strings"
)

type FishGenerator struct{}

func (g *FishGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	// Process global flags in order
	for _, flag := range data.Flags {
		desc := data.Descriptions[flag]
		if strings.HasPrefix(flag, "--") {
			script.WriteString(fmt.Sprintf("complete -c %s -l %s -d '%s'\n",
				programName,
				strings.TrimPrefix(flag, "--"),
				escapeFish(desc)))
		} else if strings.HasPrefix(flag, "-") {
			script.WriteString(fmt.Sprintf("complete -c %s -s %s -d '%s'\n",
				programName,
				strings.TrimPrefix(flag, "-"),
				escapeFish(desc)))
		}
	}

	// Process main commands in order
	for _, cmd := range data.Commands {
		desc := data.CommandDescriptions[cmd]
		if !strings.Contains(cmd, " ") {
			script.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_use_subcommand' -a '%s' -d '%s'\n",
				programName,
				cmd,
				escapeFish(desc)))
		}
	}

	// Process subcommands in order
	for _, cmd := range data.Commands {
		if strings.Contains(cmd, " ") {
			parts := strings.SplitN(cmd, " ", 2)
			mainCmd, subCmd := parts[0], parts[1]
			desc := data.CommandDescriptions[cmd]
			script.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_seen_subcommand_from %s' -a '%s' -d '%s'\n",
				programName,
				mainCmd,
				subCmd,
				escapeFish(desc)))
		}
	}

	// Process flag values in order
	for _, flag := range data.Flags {
		if values, ok := data.FlagValues[flag]; ok {
			flagName := strings.TrimPrefix(flag, "--")
			if strings.HasPrefix(flag, "-") && !strings.HasPrefix(flag, "--") {
				// Short flag
				flagName = strings.TrimPrefix(flag, "-")
				for _, v := range values {
					script.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_seen_argument -s %s' -a '%s' -d '%s'\n",
						programName,
						flagName,
						v.Pattern,
						escapeFish(v.Description)))
				}
			} else {
				// Long flag
				for _, v := range values {
					script.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_seen_argument -l %s' -a '%s' -d '%s'\n",
						programName,
						flagName,
						v.Pattern,
						escapeFish(v.Description)))
				}
			}
		}
	}

	// Process command flags in order
	for _, cmd := range data.Commands {
		if flags, ok := data.CommandFlags[cmd]; ok {
			for _, flag := range flags {
				desc := data.Descriptions[cmd+"@"+flag]
				if strings.HasPrefix(flag, "--") {
					script.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_seen_subcommand_from %s' -l %s -d '%s'\n",
						programName,
						cmd,
						strings.TrimPrefix(flag, "--"),
						escapeFish(desc)))
				} else if strings.HasPrefix(flag, "-") {
					script.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_seen_subcommand_from %s' -s %s -d '%s'\n",
						programName,
						cmd,
						strings.TrimPrefix(flag, "-"),
						escapeFish(desc)))
				}
			}
		}
	}

	return script.String()
}
