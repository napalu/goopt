package completion

import (
	"fmt"
	"strings"
)

type FishGenerator struct{}

func (g *FishGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	// Global flags
	for _, flag := range data.Flags {
		// Start with base command, add -f unless it's a file type flag
		cmd := fmt.Sprintf("complete -c %s", programName)
		if flag.Type != FlagTypeFile {
			cmd = fmt.Sprintf("%s -f", cmd)
		}

		// Combined short and long flags
		if flag.Short != "" && flag.Long != "" {
			cmd = fmt.Sprintf("%s -l %s -s %s", cmd, flag.Long, flag.Short)
		} else if flag.Short != "" {
			cmd = fmt.Sprintf("%s -s %s", cmd, flag.Short)
		} else {
			cmd = fmt.Sprintf("%s -l %s", cmd, flag.Long)
		}
		cmd = fmt.Sprintf("%s -d '%s'", cmd, escapeFish(flag.Description))
		script.WriteString(cmd + "\n")

		// Add flag values if any
		if values, ok := data.FlagValues[flag.Long]; ok {
			for _, val := range values {
				valueCmd := fmt.Sprintf("complete -c %s -f", programName)
				if flag.Short != "" && flag.Long != "" {
					valueCmd = fmt.Sprintf("%s -l %s -s %s -n '__fish_seen_argument -l %s -s %s'",
						valueCmd, flag.Long, flag.Short, flag.Long, flag.Short)
				} else if flag.Short != "" {
					valueCmd = fmt.Sprintf("%s -s %s -n '__fish_seen_argument -s %s'",
						valueCmd, flag.Short, flag.Short)
				} else {
					valueCmd = fmt.Sprintf("%s -l %s -n '__fish_seen_argument -l %s'",
						valueCmd, flag.Long, flag.Long)
				}
				valueCmd = fmt.Sprintf("%s -a '%s' -d '%s'",
					valueCmd, val.Pattern, escapeFish(val.Description))
				script.WriteString(valueCmd + "\n")
			}
		}
	}

	// Commands (always disable file completion for commands)
	for _, cmd := range data.Commands {
		if !strings.Contains(cmd, " ") {
			desc := data.CommandDescriptions[cmd]
			script.WriteString(fmt.Sprintf(
				"complete -c %s -f -n '__fish_use_subcommand' -a '%s' -d '%s'\n",
				programName, cmd, escapeFish(desc)))
		}
	}

	// Subcommands (always disable file completion for subcommands)
	for _, cmd := range data.Commands {
		if strings.Contains(cmd, " ") {
			parts := strings.SplitN(cmd, " ", 2)
			mainCmd, subCmd := parts[0], parts[1]
			desc := data.CommandDescriptions[cmd]
			script.WriteString(fmt.Sprintf(
				"complete -c %s -f -n '__fish_seen_subcommand_from %s' -a '%s' -d '%s'\n",
				programName, mainCmd, subCmd, escapeFish(desc)))
		}
	}

	// Command-specific flags
	for cmd, flags := range data.CommandFlags {
		for _, flag := range flags {
			// Always start with -f unless it's a file type flag
			cmdFlag := fmt.Sprintf("complete -c %s", programName)
			if flag.Type != FlagTypeFile {
				cmdFlag = fmt.Sprintf("%s -f", cmdFlag)
			}

			// Add command context
			cmdFlag = fmt.Sprintf("%s -n '__fish_seen_subcommand_from %s'", cmdFlag, cmd)

			// Add flag options
			if flag.Short != "" && flag.Long != "" {
				cmdFlag = fmt.Sprintf("%s -l %s -s %s", cmdFlag, flag.Long, flag.Short)
			} else if flag.Short != "" {
				cmdFlag = fmt.Sprintf("%s -s %s", cmdFlag, flag.Short)
			} else {
				cmdFlag = fmt.Sprintf("%s -l %s", cmdFlag, flag.Long)
			}
			cmdFlag = fmt.Sprintf("%s -d '%s'", cmdFlag, escapeFish(flag.Description))
			script.WriteString(cmdFlag + "\n")

			// Add command-specific flag values if any
			if values, ok := data.FlagValues[flag.Long]; ok {
				for _, val := range values {
					valueCmd := fmt.Sprintf("complete -c %s -f -n '__fish_seen_subcommand_from %s'",
						programName, cmd)
					if flag.Short != "" && flag.Long != "" {
						valueCmd = fmt.Sprintf("%s -l %s -s %s -n '__fish_seen_argument -l %s -s %s'",
							valueCmd, flag.Long, flag.Short, flag.Long, flag.Short)
					} else if flag.Short != "" {
						valueCmd = fmt.Sprintf("%s -s %s -n '__fish_seen_argument -s %s'",
							valueCmd, flag.Short, flag.Short)
					} else {
						valueCmd = fmt.Sprintf("%s -l %s -n '__fish_seen_argument -l %s'",
							valueCmd, flag.Long, flag.Long)
					}
					valueCmd = fmt.Sprintf("%s -a '%s' -d '%s'",
						valueCmd, val.Pattern, escapeFish(val.Description))
					script.WriteString(valueCmd + "\n")
				}
			}
		}
	}

	return script.String()
}
