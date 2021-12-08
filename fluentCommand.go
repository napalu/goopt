package goopt

func NewCommand(configs ...ConfigureCommandFunc) *Command {
	cmd := &Command{
		Name:         "",
		Subcommands:  nil,
		Callback:     nil,
		Description:  "",
		DefaultValue: "",
		Required:     false,
		path:         "",
	}

	for _, config := range configs {
		config(cmd)
	}

	return cmd
}

func WithName(name string) ConfigureCommandFunc {
	return func(command *Command) {
		command.Name = name
	}
}

func WithCallback(callback CommandFunc) ConfigureCommandFunc {
	return func(command *Command) {
		command.Callback = callback
	}
}

func WithCommandDescription(description string) ConfigureCommandFunc {
	return func(command *Command) {
		command.Description = description
	}
}

func SetCommandRequired(required bool) ConfigureCommandFunc {
	return func(command *Command) {
		command.Required = required
	}
}

func WithCommandDefault(defaultValue string) ConfigureCommandFunc {
	return func(command *Command) {
		command.DefaultValue = defaultValue
	}
}

func WithSubcommands(subcommands ...*Command) ConfigureCommandFunc {
	return func(command *Command) {
		for i := 0; i < len(subcommands); i++ {
			command.Subcommands = append(command.Subcommands, *subcommands[i])
		}
	}
}
