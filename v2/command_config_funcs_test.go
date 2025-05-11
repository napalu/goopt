package goopt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand_WithOverwriteSubcommands(t *testing.T) {
	cmd := NewCommand(
		WithName("parent"),
		WithSubcommands(
			NewCommand(WithName("old")),
		),
	)

	WithOverwriteSubcommands(
		NewCommand(WithName("new")),
	)(cmd)

	assert.Equal(t, 1, len(cmd.Subcommands), "should have one subcommand")
	assert.Equal(t, "new", cmd.Subcommands[0].Name, "should have overwritten subcommand")
}

func TestCommand_WithExecuteOnParse(t *testing.T) {
	wasExecuted := false
	cmd := NewCommand(
		WithName("exec"),
		WithCallback(func(cmdLine *Parser, command *Command) error {
			wasExecuted = true
			return nil
		}),
		WithExecuteOnParse(true),
	)
	p := NewParser()
	_ = p.AddCommand(cmd)
	p.ParseString("exec")
	assert.True(t, wasExecuted)
}
