package completion

import "strings"

// getAllCommandForms returns both translated and canonical forms of a command
// (translated form first if available)
func getAllCommandForms(data CompletionData, canonicalCmd string) []string {
	var forms []string
	
	if data.TranslatedCommands != nil {
		if translated, ok := data.TranslatedCommands[canonicalCmd]; ok && translated != canonicalCmd && translated != "" {
			forms = append(forms, translated)
		}
	}
	
	forms = append(forms, canonicalCmd)
	return forms
}

// getAllFlagForms returns both translated and canonical forms of a flag
// (translated form first if available)
func getAllFlagForms(data CompletionData, canonicalFlag string) []string {
	var forms []string
	
	if data.TranslatedFlags != nil {
		if translated, ok := data.TranslatedFlags[canonicalFlag]; ok && translated != canonicalFlag && translated != "" {
			forms = append(forms, translated)
		}
	}
	
	forms = append(forms, canonicalFlag)
	return forms
}

// getPreferredCommandForm returns the translated form if available, otherwise canonical
func getPreferredCommandForm(data CompletionData, canonicalCmd string) string {
	if data.TranslatedCommands != nil {
		if translated, ok := data.TranslatedCommands[canonicalCmd]; ok && translated != "" {
			return translated
		}
	}
	
	return canonicalCmd
}

// getPreferredFlagForm returns the translated form if available, otherwise canonical
func getPreferredFlagForm(data CompletionData, canonicalFlag string) string {
	if data.TranslatedFlags != nil {
		if translated, ok := data.TranslatedFlags[canonicalFlag]; ok && translated != "" {
			return translated
		}
	}
	
	return canonicalFlag
}

// extractTranslatedSubcommand extracts the subcommand part from a full translated command path
// For example: "server start" -> "serveur démarrer" extracts "démarrer"
func extractTranslatedSubcommand(data CompletionData, fullCmd string, parentCmd string, defaultSubCmd string) string {
	preferredFullForm := getPreferredCommandForm(data, fullCmd)
	
	// First try with the translated parent form
	preferredParent := getPreferredCommandForm(data, parentCmd)
	if strings.HasPrefix(preferredFullForm, preferredParent+" ") {
		return strings.TrimPrefix(preferredFullForm, preferredParent+" ")
	}
	
	// Then try with the canonical parent form
	if strings.HasPrefix(preferredFullForm, parentCmd+" ") {
		return strings.TrimPrefix(preferredFullForm, parentCmd+" ")
	}
	
	// Default to the original subcommand
	return defaultSubCmd
}