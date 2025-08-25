package completion

import (
	"strings"
	"testing"
)

func getTestI18nCompletionData() CompletionData {
	return CompletionData{
		Commands: []string{"server", "deploy", "server start", "server stop", "deploy production"},
		CommandDescriptions: map[string]string{
			"server":            "Server management",
			"deploy":            "Deploy application",
			"server start":      "Start the server",
			"server stop":       "Stop the server",
			"deploy production": "Deploy to production",
		},
		Flags: []FlagPair{
			{Long: "verbose", Short: "v", Description: "Verbose output"},
			{Long: "config", Short: "c", Description: "Config file", Type: FlagTypeFile},
			{Long: "level", Short: "l", Description: "Log level"},
		},
		CommandFlags: map[string][]FlagPair{
			"server": {
				{Long: "port", Short: "p", Description: "Server port"},
				{Long: "host", Short: "h", Description: "Server host"},
			},
			"deploy": {
				{Long: "environment", Short: "e", Description: "Target environment"},
				{Long: "force", Short: "f", Description: "Force deployment"},
			},
		},
		FlagValues: map[string][]CompletionValue{
			"level": {
				{Pattern: "debug", Description: "Debug logging"},
				{Pattern: "info", Description: "Info logging"},
				{Pattern: "error", Description: "Error logging"},
			},
			"environment": {
				{Pattern: "staging", Description: "Staging environment"},
				{Pattern: "production", Description: "Production environment"},
			},
		},
		// I18n translations
		TranslatedCommands: map[string]string{
			"server":            "serveur",
			"deploy":            "déployer",
			"server start":      "serveur démarrer",
			"server stop":       "serveur arrêter",
			"deploy production": "déployer production",
		},
		TranslatedFlags: map[string]string{
			"verbose":     "verbeux",
			"config":      "configuration",
			"level":       "niveau",
			"port":        "port",
			"host":        "hôte",
			"environment": "environnement",
			"force":       "forcer",
		},
		// French descriptions
		TranslatedCommandDescriptions: map[string]string{
			"server":            "Gestion du serveur",
			"deploy":            "Déployer l'application",
			"server start":      "Démarrer le serveur",
			"server stop":       "Arrêter le serveur",
			"deploy production": "Déployer en production",
		},
		TranslatedFlagDescriptions: map[string]string{
			"verbose":     "Sortie détaillée",
			"config":      "Fichier de configuration",
			"level":       "Niveau de journalisation",
			"port":        "Port du serveur",
			"host":        "Hôte du serveur",
			"environment": "Environnement cible",
			"force":       "Forcer le déploiement",
		},
	}
}

// prepareI18nTestData simulates what goopt does - it puts translated descriptions
// in the main description fields when translations are available
func prepareI18nTestData(data CompletionData) CompletionData {
	// Update command descriptions with translations
	if data.TranslatedCommandDescriptions != nil {
		for cmd, translatedDesc := range data.TranslatedCommandDescriptions {
			if translatedDesc != "" {
				data.CommandDescriptions[cmd] = translatedDesc
			}
		}
	}

	// Update flag descriptions with translations
	if data.TranslatedFlagDescriptions != nil {
		// Update global flags
		for i, flag := range data.Flags {
			if translatedDesc, ok := data.TranslatedFlagDescriptions[flag.Long]; ok && translatedDesc != "" {
				data.Flags[i].Description = translatedDesc
			}
		}

		// Update command-specific flags
		for cmd, flags := range data.CommandFlags {
			for i, flag := range flags {
				if translatedDesc, ok := data.TranslatedFlagDescriptions[flag.Long]; ok && translatedDesc != "" {
					data.CommandFlags[cmd][i].Description = translatedDesc
				}
			}
		}
	}

	return data
}

func TestBashI18nCompletion(t *testing.T) {
	data := prepareI18nTestData(getTestI18nCompletionData())
	gen := &BashGenerator{}
	result := gen.Generate("myapp", data)

	tests := []struct {
		name     string
		expected []string
	}{
		{
			name: "i18n comment",
			expected: []string{
				"# Shell completion for myapp with i18n support",
			},
		},
		{
			name: "translated commands",
			expected: []string{
				`commands="$commands serveur"`,
				`commands="$commands server"`,
				`commands="$commands déployer"`,
				`commands="$commands deploy"`,
			},
		},
		{
			name: "translated flags",
			expected: []string{
				`flags="$flags --verbeux"`,
				`flags="$flags --verbose"`,
				`flags="$flags --configuration"`,
				`flags="$flags --config"`,
			},
		},
		{
			name: "translated flag values",
			expected: []string{
				`--niveau|--level|-l)`,
				`COMPREPLY=( $(compgen -W "debug info error" -- "$cur") )`,
			},
		},
		{
			name: "translated subcommands",
			expected: []string{
				`serveur)`,
				`COMPREPLY=( $(compgen -W "démarrer start arrêter stop" -- "$sub_cmd") )`,
			},
		},
		{
			name: "command-specific translated flags",
			expected: []string{
				`server)`,
				`serveur)`,
				`cmd_flags="$cmd_flags --port"`,
				`cmd_flags="$cmd_flags --hôte"`,
				`cmd_flags="$cmd_flags --host"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected completion to contain %q", exp)
					t.Logf("Generated script:\n%s", result)
				}
			}
		})
	}
}

func TestZshI18nCompletion(t *testing.T) {
	data := prepareI18nTestData(getTestI18nCompletionData())
	gen := &ZshGenerator{}
	result := gen.Generate("myapp", data)

	tests := []struct {
		name     string
		expected []string
	}{
		{
			name: "i18n comment",
			expected: []string{
				"# Zsh completion for myapp with i18n support",
			},
		},
		{
			name: "translated commands with canonical note",
			expected: []string{
				`"serveur:Gestion du serveur (canonical: server)"`,
				`"server:Gestion du serveur"`,
				`"déployer:Déployer l'application (canonical: deploy)"`,
				`"deploy:Déployer l'application"`,
			},
		},
		{
			name: "translated flags with canonical note",
			expected: []string{
				`"(-v --verbeux)"{-v,--verbeux}"[Sortie détaillée (canonical: --verbose)]"`,
				`"--{}verbose[Sortie détaillée]"`,
			},
		},
		{
			name: "translated subcommands",
			expected: []string{
				`"serveur:(démarrer:Démarrer le serveur (canonical: start) start:Démarrer le serveur arrêter:Arrêter le serveur (canonical: stop) stop:Arrêter le serveur)"`,
				`"server:(démarrer:Démarrer le serveur (canonical: start) start:Démarrer le serveur arrêter:Arrêter le serveur (canonical: stop) stop:Arrêter le serveur)"`,
			},
		},
		{
			name: "command-specific patterns with translations",
			expected: []string{
				`serveur|server)`,
				`"(-p --port)"{-p,--port}"[Port du serveur]"`,
				`"(-h --hôte)"{-h,--hôte}"[Hôte du serveur (canonical: --host)]"`,
				`"--{}host[Hôte du serveur]"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected completion to contain %q", exp)
					t.Logf("Generated script:\n%s", result)
				}
			}
		})
	}
}

func TestFishI18nCompletion(t *testing.T) {
	data := prepareI18nTestData(getTestI18nCompletionData())
	gen := &FishGenerator{}
	result := gen.Generate("myapp", data)

	tests := []struct {
		name     string
		expected []string
	}{
		{
			name: "i18n comment",
			expected: []string{
				"# Fish completion for myapp with i18n support",
			},
		},
		{
			name: "translated flags with canonical note",
			expected: []string{
				`complete -c myapp -f -l verbeux -s v -d 'Sortie détaillée (canonical: --verbose)'`,
				`complete -c myapp -f -l verbose -d '(canonical form) Sortie détaillée'`,
			},
		},
		{
			name: "translated commands",
			expected: []string{
				`complete -c myapp -f -n '__fish_use_subcommand' -a 'serveur' -d 'Gestion du serveur (canonical: server)'`,
				`complete -c myapp -f -n '__fish_use_subcommand' -a 'server' -d '(canonical form) Gestion du serveur'`,
			},
		},
		{
			name: "translated subcommands",
			expected: []string{
				`complete -c myapp -f -n '__fish_seen_subcommand_from serveur server' -a 'démarrer' -d 'Démarrer le serveur (canonical: start)'`,
				`complete -c myapp -f -n '__fish_seen_subcommand_from serveur server' -a 'start' -d '(canonical form) Démarrer le serveur'`,
			},
		},
		{
			name: "command-specific translated flags",
			expected: []string{
				`complete -c myapp -f -n '__fish_seen_subcommand_from serveur server' -l hôte -s h -d 'Hôte du serveur (canonical: --host)'`,
				`complete -c myapp -f -n '__fish_seen_subcommand_from serveur server' -l host -d '(canonical form) Hôte du serveur'`,
			},
		},
		{
			name: "flag values with translations",
			expected: []string{
				`complete -c myapp -f -l niveau -s l -n '__fish_seen_argument -l niveau -s l' -a 'debug' -d 'Debug logging'`,
				`complete -c myapp -f -l level -n '__fish_seen_argument -l level' -a 'debug' -d 'Debug logging'`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected completion to contain %q", exp)
					t.Logf("Generated script:\n%s", result)
				}
			}
		})
	}
}

func TestPowerShellI18nCompletion(t *testing.T) {
	data := prepareI18nTestData(getTestI18nCompletionData())
	gen := &PowerShellGenerator{}
	result := gen.Generate("myapp", data)

	tests := []struct {
		name     string
		expected []string
	}{
		{
			name: "i18n comment and helper",
			expected: []string{
				"# PowerShell completion for myapp with i18n support",
				"function New-I18nCompletion {",
				"$Description = \"$Description (canonical: $CanonicalForm)\"",
			},
		},
		{
			name: "translated commands",
			expected: []string{
				`New-I18nCompletion 'serveur' 'Gestion du serveur' 'server' 'Command'`,
				`[CompletionResult]::new('server', 'server', [CompletionResultType]::Command, '(canonical form) Gestion du serveur')`,
			},
		},
		{
			name: "translated flag values",
			expected: []string{
				`{$_ -in 'niveau', 'level', 'l'} {`,
				`[CompletionResult]::new('debug', 'debug', [CompletionResultType]::ParameterValue, 'Debug logging')`,
			},
		},
		{
			name: "translated subcommands",
			expected: []string{
				`'serveur' {`,
				`New-I18nCompletion 'démarrer' 'Démarrer le serveur' 'start' 'Command'`,
				`[CompletionResult]::new('start', 'start', [CompletionResultType]::Command, '(canonical form) Démarrer le serveur')`,
			},
		},
		{
			name: "command-specific translated flags",
			expected: []string{
				`serveur|server) {`,
				`New-I18nCompletion '--hôte' 'Hôte du serveur' '--host' 'ParameterName'`,
				`[CompletionResult]::new('--host', 'host', [CompletionResultType]::ParameterName, '(canonical form) Hôte du serveur')`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected completion to contain %q", exp)
					t.Logf("Generated script:\n%s", result)
				}
			}
		})
	}
}

func TestI18nEdgeCases(t *testing.T) {
	t.Run("empty translations", func(t *testing.T) {
		data := CompletionData{
			Commands: []string{"test"},
			Flags: []FlagPair{
				{Long: "verbose", Short: "v", Description: "Verbose"},
			},
			// No translations
		}

		generators := []struct {
			name string
			gen  Generator
		}{
			{"bash", &BashGenerator{}},
			{"zsh", &ZshGenerator{}},
			{"fish", &FishGenerator{}},
			{"powershell", &PowerShellGenerator{}},
		}

		for _, g := range generators {
			t.Run(g.name, func(t *testing.T) {
				result := g.gen.Generate("myapp", data)
				// Should not contain i18n comment
				if strings.Contains(result, "with i18n support") {
					t.Errorf("%s generator included i18n comment when no translations present", g.name)
				}
			})
		}
	})

	t.Run("partial translations", func(t *testing.T) {
		data := CompletionData{
			Commands: []string{"server", "deploy", "test"},
			Flags: []FlagPair{
				{Long: "verbose", Short: "v", Description: "Verbose"},
				{Long: "config", Short: "c", Description: "Config"},
			},
			TranslatedCommands: map[string]string{
				"server": "serveur",
				// deploy and test not translated
			},
			TranslatedFlags: map[string]string{
				"verbose": "verbeux",
				// config not translated
			},
		}

		gen := &BashGenerator{}
		result := gen.Generate("myapp", data)

		// Should have translated forms
		if !strings.Contains(result, "serveur") {
			t.Error("Missing translated command 'serveur'")
		}
		if !strings.Contains(result, "--verbeux") {
			t.Error("Missing translated flag '--verbeux'")
		}

		// Should also have untranslated forms
		if !strings.Contains(result, "deploy") {
			t.Error("Missing untranslated command 'deploy'")
		}
		if !strings.Contains(result, "--config") {
			t.Error("Missing untranslated flag '--config'")
		}
	})

	t.Run("same canonical and translated", func(t *testing.T) {
		data := CompletionData{
			Commands: []string{"test"},
			TranslatedCommands: map[string]string{
				"test": "test", // Same as canonical
			},
		}

		gen := &FishGenerator{}
		result := gen.Generate("myapp", data)

		// Should not duplicate or show canonical notes
		if strings.Contains(result, "(canonical:") {
			t.Error("Should not show canonical note when translation is same as canonical")
		}
	})
}
