package goopt

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWithHelpConfig(t *testing.T) {
	t.Run("WithHelpConfig sets configuration", func(t *testing.T) {
		config := HelpConfig{
			Style:            HelpStyleCompact,
			ShowDefaults:     false,
			ShowShortFlags:   false,
			ShowRequired:     false,
			ShowDescription:  false,
			MaxGlobals:       5,
			GroupSharedFlags: false,
			CompactThreshold: 10,
		}

		parser, err := NewParserWith(
			WithHelpConfig(config),
		)
		assert.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify config was set
		actualConfig := parser.GetHelpConfig()
		assert.Equal(t, HelpStyleCompact, actualConfig.Style)
		assert.False(t, actualConfig.ShowDefaults)
		assert.False(t, actualConfig.ShowShortFlags)
		assert.False(t, actualConfig.ShowRequired)
		assert.False(t, actualConfig.ShowDescription)
		assert.Equal(t, 5, actualConfig.MaxGlobals)
		assert.False(t, actualConfig.GroupSharedFlags)
		assert.Equal(t, 10, actualConfig.CompactThreshold)
	})
}
