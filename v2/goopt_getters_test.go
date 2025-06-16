package goopt

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParserGetters(t *testing.T) {
	t.Run("GetAutoHelp", func(t *testing.T) {
		parser := NewParser()

		// Default should be true
		assert.True(t, parser.GetAutoHelp())

		// Test setting to false
		parser.SetAutoHelp(false)
		assert.False(t, parser.GetAutoHelp())

		// Test setting back to true
		parser.SetAutoHelp(true)
		assert.True(t, parser.GetAutoHelp())
	})

	t.Run("GetHelpFlags", func(t *testing.T) {
		parser := NewParser()

		// Default should be ["help", "h"]
		flags := parser.GetHelpFlags()
		assert.Equal(t, []string{"help", "h"}, flags)

		// Test custom flags
		parser.SetHelpFlags([]string{"ayuda", "a"})
		flags = parser.GetHelpFlags()
		assert.Equal(t, []string{"ayuda", "a"}, flags)
	})

	t.Run("GetAutoVersion", func(t *testing.T) {
		parser := NewParser()

		// Default should be true
		assert.True(t, parser.GetAutoVersion())

		// Test setting to false
		parser.SetAutoVersion(false)
		assert.False(t, parser.GetAutoVersion())
	})

	t.Run("GetVersionFlags", func(t *testing.T) {
		parser := NewParser()

		// Default should be ["version", "v"]
		flags := parser.GetVersionFlags()
		assert.Equal(t, []string{"version", "v"}, flags)

		// Test custom flags
		parser.SetVersionFlags([]string{"ver", "V"})
		flags = parser.GetVersionFlags()
		assert.Equal(t, []string{"ver", "V"}, flags)
	})
}
