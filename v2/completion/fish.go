package completion

import (
	"fmt"
	"strings"
)

type FishGenerator struct{}

func (g *FishGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder
	
	// Add i18n comment if translations are present
	hasTranslations := len(data.TranslatedCommands) > 0 || len(data.TranslatedFlags) > 0
	if hasTranslations {
		script.WriteString(fmt.Sprintf(`# Fish completion for %s with i18n support

`, programName))
	}

	// Global flags with i18n support
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
		
		// Preferred form completion
		cmd := fmt.Sprintf("complete -c %s", programName)
		if flag.Type != FlagTypeFile {
			cmd = fmt.Sprintf("%s -f", cmd)
		}

		// Combined short and long flags
		switch {
		case flag.Short != "" && preferredLong != "":
			cmd = fmt.Sprintf("%s -l %s -s %s", cmd, preferredLong, flag.Short)
		case flag.Short != "":
			cmd = fmt.Sprintf("%s -s %s", cmd, flag.Short)
		default:
			cmd = fmt.Sprintf("%s -l %s", cmd, preferredLong)
		}
		cmd = fmt.Sprintf("%s -d '%s'", cmd, escapeFish(desc))
		script.WriteString(cmd + "\n")
		
		// Add canonical form as separate completion if different
		if preferredLong != flag.Long {
			canonicalDesc := fmt.Sprintf("(canonical form) %s", escapeFish(flag.Description))
			cmd = fmt.Sprintf("complete -c %s", programName)
			if flag.Type != FlagTypeFile {
				cmd = fmt.Sprintf("%s -f", cmd)
			}
			cmd = fmt.Sprintf("%s -l %s -d '%s'", cmd, flag.Long, canonicalDesc)
			script.WriteString(cmd + "\n")
		}

		// Add flag values if any
		if values, ok := data.FlagValues[flag.Long]; ok {
			for _, val := range values {
				// For preferred form
				valueCmd := fmt.Sprintf("complete -c %s -f", programName)
				switch {
				case flag.Short != "" && preferredLong != "":
					valueCmd = fmt.Sprintf("%s -l %s -s %s -n '__fish_seen_argument -l %s -s %s'",
						valueCmd, preferredLong, flag.Short, preferredLong, flag.Short)
				case flag.Short != "":
					valueCmd = fmt.Sprintf("%s -s %s -n '__fish_seen_argument -s %s'",
						valueCmd, flag.Short, flag.Short)
				default:
					valueCmd = fmt.Sprintf("%s -l %s -n '__fish_seen_argument -l %s'",
						valueCmd, preferredLong, preferredLong)
				}
				valueCmd = fmt.Sprintf("%s -a '%s' -d '%s'",
					valueCmd, val.Pattern, escapeFish(val.Description))
				script.WriteString(valueCmd + "\n")
				
				// For canonical form if different
				if preferredLong != flag.Long {
					valueCmd = fmt.Sprintf("complete -c %s -f -l %s -n '__fish_seen_argument -l %s' -a '%s' -d '%s'",
						programName, flag.Long, flag.Long, val.Pattern, escapeFish(val.Description))
					script.WriteString(valueCmd + "\n")
				}
			}
		}
	}

	// Commands with i18n support (always disable file completion for commands)
	processedCommands := make(map[string]bool)
	for _, cmd := range data.Commands {
		if !strings.Contains(cmd, " ") && !processedCommands[cmd] {
			processedCommands[cmd] = true
			
			// Get preferred form and description
			preferredForm := getPreferredCommandForm(data, cmd)
			desc := data.CommandDescriptions[cmd]
			
			// Add note about canonical form if different
			if preferredForm != cmd && hasTranslations {
				if desc != "" {
					desc = fmt.Sprintf("%s (canonical: %s)", desc, cmd)
				} else {
					desc = fmt.Sprintf("(canonical: %s)", cmd)
				}
			}
			
			script.WriteString(fmt.Sprintf(
				"complete -c %s -f -n '__fish_use_subcommand' -a '%s' -d '%s'\n",
				programName, preferredForm, escapeFish(desc)))
			
			// Add canonical form if different
			if preferredForm != cmd {
				canonicalDesc := fmt.Sprintf("(canonical form) %s", data.CommandDescriptions[cmd])
				script.WriteString(fmt.Sprintf(
					"complete -c %s -f -n '__fish_use_subcommand' -a '%s' -d '%s'\n",
					programName, cmd, escapeFish(canonicalDesc)))
			}
		}
	}

	// Subcommands with i18n support (always disable file completion for subcommands)
	for _, cmd := range data.Commands {
		if strings.Contains(cmd, " ") {
			parts := strings.SplitN(cmd, " ", 2)
			mainCmd, subCmd := parts[0], parts[1]
			
			// Get preferred forms
			preferredFullForm := getPreferredCommandForm(data, cmd)
			preferredSubCmd := subCmd
			// Try to extract from translated form
			// First get the preferred parent form
			preferredParent := getPreferredCommandForm(data, mainCmd)
			if strings.HasPrefix(preferredFullForm, preferredParent+" ") {
				preferredSubCmd = strings.TrimPrefix(preferredFullForm, preferredParent+" ")
			} else if strings.HasPrefix(preferredFullForm, mainCmd+" ") {
				preferredSubCmd = strings.TrimPrefix(preferredFullForm, mainCmd+" ")
			}
			
			desc := data.CommandDescriptions[cmd]
			
			// Add note about canonical form if different
			if preferredSubCmd != subCmd && hasTranslations {
				if desc != "" {
					desc = fmt.Sprintf("%s (canonical: %s)", desc, subCmd)
				} else {
					desc = fmt.Sprintf("(canonical: %s)", subCmd)
				}
			}
			
			// Get all forms of the main command for the condition
			allMainForms := getAllCommandForms(data, mainCmd)
			mainFormsStr := strings.Join(allMainForms, " ")
			
			script.WriteString(fmt.Sprintf(
				"complete -c %s -f -n '__fish_seen_subcommand_from %s' -a '%s' -d '%s'\n",
				programName, mainFormsStr, preferredSubCmd, escapeFish(desc)))
			
			// Add canonical form if different
			if preferredSubCmd != subCmd {
				canonicalDesc := fmt.Sprintf("(canonical form) %s", data.CommandDescriptions[cmd])
				script.WriteString(fmt.Sprintf(
					"complete -c %s -f -n '__fish_seen_subcommand_from %s' -a '%s' -d '%s'\n",
					programName, mainFormsStr, subCmd, escapeFish(canonicalDesc)))
			}
		}
	}

	// Command-specific flags with i18n support
	for cmd, flags := range data.CommandFlags {
		// Get all forms of the command
		allCmdForms := getAllCommandForms(data, cmd)
		cmdFormsStr := strings.Join(allCmdForms, " ")
		
		for _, flag := range flags {
			// Get preferred form
			preferredLong := getPreferredFlagForm(data, flag.Long)
			
			// Build description
			desc := flag.Description
			if preferredLong != flag.Long && hasTranslations {
				if desc != "" {
					desc = fmt.Sprintf("%s (canonical: --%s)", desc, flag.Long)
				} else {
					desc = fmt.Sprintf("(canonical: --%s)", flag.Long)
				}
			}
			
			// Preferred form
			cmdFlag := fmt.Sprintf("complete -c %s", programName)
			if flag.Type != FlagTypeFile {
				cmdFlag = fmt.Sprintf("%s -f", cmdFlag)
			}

			// Add command context with all forms
			cmdFlag = fmt.Sprintf("%s -n '__fish_seen_subcommand_from %s'", cmdFlag, cmdFormsStr)

			// Add flag options
			switch {
			case flag.Short != "" && preferredLong != "":
				cmdFlag = fmt.Sprintf("%s -l %s -s %s", cmdFlag, preferredLong, flag.Short)
			case flag.Short != "":
				cmdFlag = fmt.Sprintf("%s -s %s", cmdFlag, flag.Short)
			default:
				cmdFlag = fmt.Sprintf("%s -l %s", cmdFlag, preferredLong)
			}
			cmdFlag = fmt.Sprintf("%s -d '%s'", cmdFlag, escapeFish(desc))
			script.WriteString(cmdFlag + "\n")
			
			// Canonical form if different
			if preferredLong != flag.Long {
				canonicalDesc := fmt.Sprintf("(canonical form) %s", escapeFish(flag.Description))
				cmdFlag = fmt.Sprintf("complete -c %s", programName)
				if flag.Type != FlagTypeFile {
					cmdFlag = fmt.Sprintf("%s -f", cmdFlag)
				}
				cmdFlag = fmt.Sprintf("%s -n '__fish_seen_subcommand_from %s' -l %s -d '%s'",
					cmdFlag, cmdFormsStr, flag.Long, canonicalDesc)
				script.WriteString(cmdFlag + "\n")
			}

			// Add command-specific flag values if any
			if values, ok := data.FlagValues[flag.Long]; ok {
				for _, val := range values {
					// For preferred form
					valueCmd := fmt.Sprintf("complete -c %s -f -n '__fish_seen_subcommand_from %s'",
						programName, cmdFormsStr)
					switch {
					case flag.Short != "" && preferredLong != "":
						valueCmd = fmt.Sprintf("%s -l %s -s %s -n '__fish_seen_argument -l %s -s %s'",
							valueCmd, preferredLong, flag.Short, preferredLong, flag.Short)
					case flag.Short != "":
						valueCmd = fmt.Sprintf("%s -s %s -n '__fish_seen_argument -s %s'",
							valueCmd, flag.Short, flag.Short)
					default:
						valueCmd = fmt.Sprintf("%s -l %s -n '__fish_seen_argument -l %s'",
							valueCmd, preferredLong, preferredLong)
					}
					valueCmd = fmt.Sprintf("%s -a '%s' -d '%s'",
						valueCmd, val.Pattern, escapeFish(val.Description))
					script.WriteString(valueCmd + "\n")
					
					// For canonical form if different
					if preferredLong != flag.Long {
						valueCmd = fmt.Sprintf("complete -c %s -f -n '__fish_seen_subcommand_from %s' -l %s -n '__fish_seen_argument -l %s' -a '%s' -d '%s'",
							programName, cmdFormsStr, flag.Long, flag.Long, val.Pattern, escapeFish(val.Description))
						script.WriteString(valueCmd + "\n")
					}
				}
			}
		}
	}

	return script.String()
}
