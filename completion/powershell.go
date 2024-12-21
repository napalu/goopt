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
Register-ArgumentCompleter -Native -CommandName %[1]s -ScriptBlock {
    param($commandName, $wordToComplete, $cursorPosition)
    $commandElements = $wordToComplete -split "\s+"
    
    # Handle empty word completion
    if ($wordToComplete -eq '') {
        @(`, programName))

	// Add command completions
	for cmd, desc := range data.CommandDescriptions {
		script.WriteString(fmt.Sprintf(`
            [CompletionResult]::new('%[1]s', '%[2]s', [CompletionResultType]::Command, '%[3]s')`,
			cmd, strings.TrimPrefix(cmd, "--"), escapePowerShell(desc)))
	}

	script.WriteString(`
        )
        return
    }

    # Handle flag values`)

	// Add flag value completions
	for flag, values := range data.FlagValues {
		script.WriteString(fmt.Sprintf(`

    if ($wordToComplete -eq '%s') {
        @(`, flag))
		for _, v := range values {
			script.WriteString(fmt.Sprintf(`
            [CompletionResult]::new('%s', '%s', [CompletionResultType]::ParameterValue, '%s')`,
				v.Pattern, v.Pattern, escapePowerShell(v.Description)))
		}
		script.WriteString(`
        )
        return
    }`)
	}

	script.WriteString(`

    # Get current command
    $cmd = ""
    for ($i = 1; $i -lt $commandElements.Count; $i++) {
        if (!$commandElements[$i].StartsWith('-')) {
            $cmd = $commandElements[$i]
            break
        }
    }

    # Handle flags
    if ($wordToComplete.StartsWith('-')) {
        @(`)

	// Add global flags
	for _, flag := range data.Flags {
		desc := data.Descriptions[flag]
		script.WriteString(fmt.Sprintf(`
            [CompletionResult]::new('%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
			flag, strings.TrimPrefix(flag, "--"), escapePowerShell(desc)))
	}

	// Add command-specific flags
	if len(data.CommandFlags) > 0 {
		script.WriteString(`

        # Add command-specific flags
        switch ($cmd) {`)

		for cmd, flags := range data.CommandFlags {
			if len(flags) > 0 {
				script.WriteString(fmt.Sprintf(`
            '%s' {`, cmd))
				for _, flag := range flags {
					desc := data.Descriptions[cmd+"@"+flag]
					script.WriteString(fmt.Sprintf(`
                [CompletionResult]::new('%s', '%s', [CompletionResultType]::ParameterName, '%s')`,
						flag, strings.TrimPrefix(flag, "--"), escapePowerShell(desc)))
				}
				script.WriteString(`
            }`)
			}
		}

		script.WriteString(`
        }`)
	}

	script.WriteString(`
        )
        return
    }
}`)

	return script.String()
}
