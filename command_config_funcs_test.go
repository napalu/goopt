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
