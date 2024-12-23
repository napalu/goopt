// completion/powershell.go
package completion

import (
	"fmt"
	"strings"
)

type PowerShellGenerator struct{}

func (g *PowerShellGenerator) Generate(programName string, data CompletionData) string {
	var script strings.Builder

	script.WriteString(fmt.Sprintf(`
Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    
    $tokens = $commandAst.CommandElements
    $currentToken = $tokens | Where-Object { $_.Extent.StartOffset -le $cursorPosition } | Select-Object -Last 1

    # Handle parameter value completion
    if ($currentToken -is [System.Management.Automation.Language.CommandParameterAst]) {
        switch ($currentToken.ParameterName) {`, programName))

	// Add flag value completions
	for flagName, values := range data.FlagValues {
		// Find the corresponding flag to get its short form
		var shortForm string
		for _, flag := range data.Flags {
			if flag.Long == flagName {
				shortForm = flag.Short
				break
			}
		}

		// Handle both long and short forms
		if shortForm != "" {
			script.WriteString(fmt.Sprintf(`
            '%s' { # --%s or -%s`, flagName, flagName, shortForm))
		} else {
			script.WriteString(fmt.Sprintf(`
            '%s' { # --%s`, flagName, flagName))
		}
		for _, val := range values {
			script.WriteString(fmt.Sprintf(`
                [CompletionResult]::new('%s', '%s', [CompletionResultType]::ParameterValue, '%s')`,
				escapePatternPowershell(val.Pattern), escapePatternPowershell(val.Pattern), escapePowerShell(val.Description)))
		}
		script.WriteString(`
            }`)

		if shortForm != "" {
			script.WriteString(fmt.Sprintf(`
            '%s' { # Short form`, shortForm))
			for _, val := range values {
				script.WriteString(fmt.Sprintf(`
                [CompletionResult]::new('%s', '%s', [CompletionResultType]::ParameterValue, '%s')`,
					escapePatternPowershell(val.Pattern), escapePatternPowershell(val.Pattern), escapePowerShell(val.Description)))
			}
			script.WriteString(`
            }`)
		}
	}

	script.WriteString(`
        }
        return
    }

    # Handle command completion
    if ($tokens.Count -eq 1) {
        @(`)

	// Add top-level commands
	for _, cmd := range data.Commands {
		if !strings.Contains(cmd, " ") {
			desc := data.CommandDescriptions[cmd]
			script.WriteString(fmt.Sprintf(`
            [CompletionResult]::new('%s', '%s', [CompletionResultType]::Command, '%s')`,
				cmd, cmd, escapePowerShell(desc)))
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

	// Add subcommand completions
	for parentCmd, subCmds := range commandGroups {
		script.WriteString(fmt.Sprintf(`
            '%s' {
                @(`, parentCmd))
		for _, subCmd := range subCmds {
			fullCmd := parentCmd + " " + subCmd
			desc := data.CommandDescriptions[fullCmd]
			script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('%s', '%s', [CompletionResultType]::Command, '%s')`,
				subCmd, subCmd, escapePowerShell(desc)))
		}
		script.WriteString(`
                )
                return
            }`)
	}

	// Add command-specific flags
	for cmd, flags := range data.CommandFlags {
		script.WriteString(fmt.Sprintf(`
            '%s' {
                @(`, cmd))
		for _, flag := range flags {
			script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
				flag.Long, flag.Long, escapePowerShell(flag.Description)))
			if flag.Short != "" {
				script.WriteString(fmt.Sprintf(`
                    [CompletionResult]::new('-%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
					flag.Short, flag.Short, escapePowerShell(flag.Description)))
			}
		}
		script.WriteString(`
                )
                return
            }`)
	}

	script.WriteString(`
        }
    }

    # Handle global flags
    @(`)

	// Add global flags
	for _, flag := range data.Flags {
		script.WriteString(fmt.Sprintf(`
        [CompletionResult]::new('--%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
			flag.Long, flag.Long, escapePowerShell(flag.Description)))
		if flag.Short != "" {
			script.WriteString(fmt.Sprintf(`
        [CompletionResult]::new('-%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
				flag.Short, flag.Short, escapePowerShell(flag.Description)))
		}
	}

	script.WriteString(`
    )
}`)

	return script.String()
}
