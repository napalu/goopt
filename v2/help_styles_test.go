package goopt

import (
	"bytes"
	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestPrintGroupedHelp(t *testing.T) {
	parser := NewParser()

	// Add global flags
	parser.AddFlag("verbose", &Argument{
		Short:       "v",
		Description: "Enable verbose output",
		TypeOf:      types.Standalone,
	})

	// Add commands with flags
	parser.AddCommand(&Command{
		Name:        "serve",
		Description: "Start the server",
	})

	parser.AddFlag("port", &Argument{
		Short:        "p",
		Description:  "Server port",
		DefaultValue: "8080",
		TypeOf:       types.Single,
	}, "serve")

	// Force grouped style
	parser.SetHelpStyle(HelpStyleGrouped)

	var buf bytes.Buffer
	parser.printGroupedHelp(&buf)
	output := buf.String()

	// Check that output contains expected sections
	assert.Contains(t, output, "Global Flags:")
	assert.Contains(t, output, "--verbose")
	assert.Contains(t, output, "Commands:")
	assert.Contains(t, output, "serve")
	assert.Contains(t, output, "--port")
}

func TestPrintHierarchicalHelp(t *testing.T) {
	parser := NewParser()

	// Add multiple flags and commands to trigger hierarchical
	for i := 0; i < 10; i++ {
		parser.AddFlag(strings.ToLower(string(rune('a'+i)))+"flag", &Argument{
			Description: "Test flag",
			TypeOf:      types.Single,
		})
	}

	// Add nested commands
	parser.AddCommand(&Command{
		Name:        "cluster",
		Description: "Manage clusters",
		Subcommands: []Command{
			{Name: "create", Description: "Create a cluster"},
			{Name: "delete", Description: "Delete a cluster"},
		},
	})

	// Force hierarchical style
	parser.SetHelpStyle(HelpStyleHierarchical)

	var buf bytes.Buffer
	parser.printHierarchicalHelp(&buf)
	output := buf.String()

	// Check hierarchical output
	assert.Contains(t, output, "Global Flags:")
	assert.Contains(t, output, "Command Structure:")
	assert.Contains(t, output, "cluster")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "delete")
	assert.Contains(t, output, "Create a cluster")
	assert.Contains(t, output, "Delete a cluster")
}

func TestCountCommandFlags(t *testing.T) {
	parser := NewParser()

	// Add command
	parser.AddCommand(&Command{
		Name: "test",
	})

	// Add flags to the command
	parser.AddFlag("flag1", &Argument{TypeOf: types.Single}, "test")
	parser.AddFlag("flag2", &Argument{TypeOf: types.Single}, "test")
	parser.AddFlag("flag3", &Argument{TypeOf: types.Single}, "test")

	// Count flags for the command
	count := parser.countCommandFlags("test")
	assert.Equal(t, 3, count)

	// Count for non-existent command
	count = parser.countCommandFlags("nonexistent")
	assert.Equal(t, 0, count)
}
