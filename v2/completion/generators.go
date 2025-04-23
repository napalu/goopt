package completion

type Generator interface {
	Generate(programName string, data CompletionData) string
}

func GetGenerator(shell string) Generator {
	switch shell {
	case "zsh":
		return &ZshGenerator{}
	case "fish":
		return &FishGenerator{}
	case "powershell":
		return &PowerShellGenerator{}
	case "bash":
		fallthrough
	default:
		return &BashGenerator{}
	}
}
