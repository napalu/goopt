// completion/powershell.go
package completion

import (
	"fmt"
	"strings"
)

type PowerShellGenerator struct{}

func (g *PowerShellGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder
	
	// Add i18n comment if translations are present
	hasTranslations := len(data.TranslatedCommands) > 0 || len(data.TranslatedFlags) > 0
	if hasTranslations {
		script.WriteString(fmt.Sprintf(`# PowerShell completion for %s with i18n support

`, programName))
	}

	script.WriteString(fmt.Sprintf(`Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    
    $tokens = $commandAst.CommandElements
    $currentToken = $tokens | Where-Object { $_.Extent.StartOffset -le $cursorPosition } | Select-Object -Last 1`, programName))
	
	// Add helper function for i18n if translations are present
	if hasTranslations {
		script.WriteString(`
    
    # Helper function to create completion with i18n info
    function New-I18nCompletion {
        param($Value, $Description, $CanonicalForm, $Type = 'ParameterValue')
        
        if ($CanonicalForm -and $Value -ne $CanonicalForm) {
            if ($Description) {
                $Description = "$Description (canonical: $CanonicalForm)"
            } else {
                $Description = "(canonical: $CanonicalForm)"
            }
        }
        
        [CompletionResult]::new($Value, $Value, [CompletionResultType]::$Type, $Description)
    }`)
	}

	script.WriteString(`

    # Handle parameter value completion
    if ($currentToken -is [System.Management.Automation.Language.CommandParameterAst]) {
        switch ($currentToken.ParameterName) {`)

	// Add flag value completions with i18n support
	for flagName, values := range data.FlagValues {
		// Get all forms of this flag
		allForms := getAllFlagForms(data, flagName)
		
		// Find the corresponding flag to get its short form
		var shortForm string
		for _, flag := range data.Flags {
			if flag.Long == flagName {
				shortForm = flag.Short
				break
			}
		}
		
		// Add patterns for all long forms
		patterns := []string{}
		for _, form := range allForms {
			patterns = append(patterns, fmt.Sprintf("'%s'", form))
		}
		if shortForm != "" {
			patterns = append(patterns, fmt.Sprintf("'%s'", shortForm))
		}
		
		script.WriteString(fmt.Sprintf(`
            {$_ -in %s} {`, strings.Join(patterns, ", ")))
		
		for _, val := range values {
			script.WriteString(fmt.Sprintf(`
                [CompletionResult]::new('%s', '%s', [CompletionResultType]::ParameterValue, '%s')`,
				escapePatternPowershell(val.Pattern), escapePatternPowershell(val.Pattern), escapePowerShell(val.Description)))
		}
		script.WriteString(`
            }`)
	}

	script.WriteString(`
        }
        return
    }

    # Handle command completion
    if ($tokens.Count -eq 1) {
        @(`)

	// Add top-level commands with i18n support
	processedCommands := make(map[string]bool)
	for _, cmd := range data.Commands {
		if !strings.Contains(cmd, " ") && !processedCommands[cmd] {
			processedCommands[cmd] = true
			
			preferredForm := getPreferredCommandForm(data, cmd)
			desc := data.CommandDescriptions[cmd]
			
			if hasTranslations {
				canonicalNote := ""
				if preferredForm != cmd {
					canonicalNote = cmd
				}
				script.WriteString(fmt.Sprintf(`
            New-I18nCompletion '%s' '%s' '%s' 'Command'`,
					preferredForm, escapePowerShell(desc), canonicalNote))
				
				// Add canonical form if different
				if preferredForm != cmd {
					canonicalDesc := fmt.Sprintf("(canonical form) %s", desc)
					script.WriteString(fmt.Sprintf(`
            [CompletionResult]::new('%s', '%s', [CompletionResultType]::Command, '%s')`,
						cmd, cmd, escapePowerShell(canonicalDesc)))
				}
			} else {
				script.WriteString(fmt.Sprintf(`
            [CompletionResult]::new('%s', '%s', [CompletionResultType]::Command, '%s')`,
					cmd, cmd, escapePowerShell(desc)))
			}
		}
	}

	script.WriteString(`
        )
        return
    }

    # Handle subcommand completion
    if ($tokens.Count -gt 1) {
        switch ($tokens[1].Value) {`)

	// Group subcommands by their parent command
	commandGroups := make(map[string][]string)
	for _, cmd := range data.Commands {
		parts := strings.Split(cmd, " ")
		if len(parts) > 1 {
			parent := parts[0]
			sub := parts[1]
			commandGroups[parent] = append(commandGroups[parent], sub)
		}
	}

	// Add subcommand completions with i18n support
	processedParents := make(map[string]bool)
	for parentCmd, subCmds := range commandGroups {
		if !processedParents[parentCmd] {
			processedParents[parentCmd] = true
			
			// Get all forms of the parent command
			allParentForms := getAllCommandForms(data, parentCmd)
			for _, parentForm := range allParentForms {
				script.WriteString(fmt.Sprintf(`
            '%s' {
                @(`, parentForm))
				
				// Process each subcommand
				processedSubs := make(map[string]bool)
				for _, subCmd := range subCmds {
					fullCmd := parentCmd + " " + subCmd
					if !processedSubs[fullCmd] {
						processedSubs[fullCmd] = true
						
						// Get preferred form of subcommand
						preferredFullForm := getPreferredCommandForm(data, fullCmd)
						preferredSubCmd := subCmd
						// Try to extract from translated form
						// First get the preferred parent form
						preferredParent := getPreferredCommandForm(data, parentCmd)
						if strings.HasPrefix(preferredFullForm, preferredParent+" ") {
							preferredSubCmd = strings.TrimPrefix(preferredFullForm, preferredParent+" ")
						} else if strings.HasPrefix(preferredFullForm, parentCmd+" ") {
							preferredSubCmd = strings.TrimPrefix(preferredFullForm, parentCmd+" ")
						}
						
						desc := data.CommandDescriptions[fullCmd]
						
						if hasTranslations {
							canonicalNote := ""
							if preferredSubCmd != subCmd {
								canonicalNote = subCmd
							}
							script.WriteString(fmt.Sprintf(`
                    New-I18nCompletion '%s' '%s' '%s' 'Command'`,
								preferredSubCmd, escapePowerShell(desc), canonicalNote))
							
							// Add canonical form if different
							if preferredSubCmd != subCmd {
								canonicalDesc := fmt.Sprintf("(canonical form) %s", desc)
								script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('%s', '%s', [CompletionResultType]::Command, '%s')`,
									subCmd, subCmd, escapePowerShell(canonicalDesc)))
							}
						} else {
							script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('%s', '%s', [CompletionResultType]::Command, '%s')`,
								subCmd, subCmd, escapePowerShell(desc)))
						}
					}
				}
				
				script.WriteString(`
                )
                return
            }`)
			}
		}
	}

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
            %s) {
                @(`, patterns))
			
			// Add command-specific flags with i18n
			for _, flag := range flags {
				preferredLong := getPreferredFlagForm(data, flag.Long)
				desc := flag.Description
				
				if hasTranslations {
					canonicalNote := ""
					if preferredLong != flag.Long {
						canonicalNote = flag.Long
					}
					script.WriteString(fmt.Sprintf(`
                    New-I18nCompletion '--%s' '%s' '--%s' 'ParameterName'`,
						preferredLong, escapePowerShell(desc), canonicalNote))
					
					// Add canonical form if different
					if preferredLong != flag.Long {
						canonicalDesc := fmt.Sprintf("(canonical form) %s", desc)
						script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
							flag.Long, flag.Long, escapePowerShell(canonicalDesc)))
					}
				} else {
					script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
						flag.Long, flag.Long, escapePowerShell(flag.Description)))
				}
				
				if flag.Short != "" {
					script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('-%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
						flag.Short, flag.Short, escapePowerShell(desc)))
				}
			}
			
			// Also include global flags
			for _, flag := range data.Flags {
				preferredLong := getPreferredFlagForm(data, flag.Long)
				desc := flag.Description
				
				if hasTranslations {
					canonicalNote := ""
					if preferredLong != flag.Long {
						canonicalNote = flag.Long
					}
					script.WriteString(fmt.Sprintf(`
                    New-I18nCompletion '--%s' '%s' '--%s' 'ParameterName'`,
						preferredLong, escapePowerShell(desc), canonicalNote))
					
					if preferredLong != flag.Long {
						canonicalDesc := fmt.Sprintf("(canonical form) %s", desc)
						script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
							flag.Long, flag.Long, escapePowerShell(canonicalDesc)))
					}
				} else {
					script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
						flag.Long, flag.Long, escapePowerShell(flag.Description)))
				}
				
				if flag.Short != "" {
					script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('-%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
						flag.Short, flag.Short, escapePowerShell(desc)))
				}
			}
			
			script.WriteString(`
                )
                return
            }`)
		}
	}

	script.WriteString(`
        }
    }

    # Handle global flags
    @(`)

	// Add global flags with i18n support
	for _, flag := range data.Flags {
		preferredLong := getPreferredFlagForm(data, flag.Long)
		desc := flag.Description
		
		if hasTranslations {
			canonicalNote := ""
			if preferredLong != flag.Long {
				canonicalNote = flag.Long
			}
			script.WriteString(fmt.Sprintf(`
        New-I18nCompletion '--%s' '%s' '--%s' 'ParameterName'`,
				preferredLong, escapePowerShell(desc), canonicalNote))
			
			// Add canonical form if different
			if preferredLong != flag.Long {
				canonicalDesc := fmt.Sprintf("(canonical form) %s", desc)
				script.WriteString(fmt.Sprintf(`
        [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
					flag.Long, flag.Long, escapePowerShell(canonicalDesc)))
			}
		} else {
			script.WriteString(fmt.Sprintf(`
        [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
				flag.Long, flag.Long, escapePowerShell(flag.Description)))
		}
		
		if flag.Short != "" {
			script.WriteString(fmt.Sprintf(`
        [CompletionResult]::new('-%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
				flag.Short, flag.Short, escapePowerShell(desc)))
		}
	}

	script.WriteString(`
    )
}`)

	return script.String()
}
