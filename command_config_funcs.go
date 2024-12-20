package goopt

// NewCommand creates and returns a new Command object. This function takes variadic `ConfigureCommandFunc` functions to customize the created command.
func NewCommand(configs ...ConfigureCommandFunc) *Command {
	cmd := &Command{
		Name:        "",
		Subcommands: nil,
		Callback:    nil,
		Description: "",
		Required:    false,
		Path:        "",
	}

	for _, config := range configs {
		config(cmd)
	}

	return cmd
}

// Set is a helper config function that allows setting multiple configuration functions on a command.
func (c *Command) Set(configs ...ConfigureCommandFunc) {
	for _, config := range configs {
		config(c)
	}
}

// WithName sets the name for the command. The name is used to identify the command and invoke it from the command line.
func WithName(name string) ConfigureCommandFunc {
	return func(command *Command) {
		command.Name = name
	}
}

// WithCallback sets the callback function for the command. This function is run when the command gets executed.
func WithCallback(callback CommandFunc) ConfigureCommandFunc {
	return func(command *Command) {
		command.Callback = callback
	}
}

// WithCommandDescription sets the description for the command. This description helps users to understand what the command does.
func WithCommandDescription(description string) ConfigureCommandFunc {
	return func(command *Command) {
		command.Description = description
	}
}

// SetCommandRequired function is used to set the command as required or optional.
// If a command is set as required and not provided by the user, an error is generated.
func SetCommandRequired(required bool) ConfigureCommandFunc {
	return func(command *Command) {
		command.Required = required
	}
}

// WithSubcommands function takes a list of subcommands and associates them with a command.
func WithSubcommands(subcommands ...*Command) ConfigureCommandFunc {
	return func(command *Command) {
		for i := 0; i < len(subcommands); i++ {
			command.Subcommands = append(command.Subcommands, *subcommands[i])
		}
	}
}

// WithOverwriteSubcommands allows replacing a Command's subcommands.
func WithOverwriteSubcommands(subcommands ...*Command) ConfigureCommandFunc {
	return func(command *Command) {
		command.Subcommands = command.Subcommands[:0]
		for i := 0; i < len(subcommands); i++ {
			command.Subcommands = append(command.Subcommands, *subcommands[i])
		}
	}
}
