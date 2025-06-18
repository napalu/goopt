package goopt

import (
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

// TestCommandName tests the CommandName renderer function
func TestCommandName(t *testing.T) {
	t.Run("command with name key", func(t *testing.T) {
		// Create a bundle with translations
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.English, map[string]string{
			"cmd.test": "Translated Test Command",
		})

		p := NewParser()
		err := p.SetUserBundle(bundle)
		assert.NoError(t, err)
		cmd := &Command{
			Name:    "test",
			NameKey: "cmd.test",
		}

		renderer := p.renderer
		name := renderer.CommandName(cmd)
		assert.Equal(t, "Translated Test Command", name)
	})

	t.Run("command without name key", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name: "test",
		}

		renderer := p.renderer
		name := renderer.CommandName(cmd)
		assert.Equal(t, "test", name)
	})

	t.Run("command with invalid name key", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:    "test",
			NameKey: "nonexistent.key",
		}

		renderer := p.renderer
		name := renderer.CommandName(cmd)
		// When key is not found, i18n returns the key itself
		assert.Equal(t, "nonexistent.key", name)
	})
}

// TestCommandUsage tests the CommandUsage renderer function
func TestCommandUsage(t *testing.T) {
	t.Run("command with description", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name:        "test",
			Description: "Test command description",
		}

		renderer := p.renderer
		usage := renderer.CommandUsage(cmd)
		assert.Equal(t, `test "Test command description"`, usage)
	})

	t.Run("command with translated name and description", func(t *testing.T) {
		// Create a new bundle instead of modifying the global default
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.English, map[string]string{
			"cmd.test":      "translated-test",
			"cmd.test.desc": "Translated description",
		})

		p := NewParser()
		// Set the user bundle instead of modifying system bundle
		err := p.SetUserBundle(bundle)
		assert.NoError(t, err)

		cmd := &Command{
			Name:           "test",
			NameKey:        "cmd.test",
			Description:    "Default description",
			DescriptionKey: "cmd.test.desc",
		}

		renderer := p.renderer
		usage := renderer.CommandUsage(cmd)
		assert.Equal(t, `translated-test "Translated description"`, usage)
	})

	t.Run("command without description", func(t *testing.T) {
		p := NewParser()

		cmd := &Command{
			Name: "test",
		}

		renderer := p.renderer
		usage := renderer.CommandUsage(cmd)
		assert.Equal(t, `test`, usage)
	})
}

// TestGetArgumentInfoByID tests the internal getArgumentInfoByID function
func TestGetArgumentInfoByID(t *testing.T) {
	t.Run("valid argument ID", func(t *testing.T) {
		p := NewParser()

		// Add a flag with ID
		arg := NewArg(WithDescription("Test flag"))
		err := p.AddFlag("test-flag", arg)
		assert.NoError(t, err)

		// The UUID is set internally by ensureInit()
		// Test getArgumentInfoByID
		info := p.getArgumentInfoByID(arg.uniqueID)
		assert.NotNil(t, info)
		assert.Equal(t, "Test flag", info.Argument.Description)
	})

	t.Run("invalid argument ID", func(t *testing.T) {
		p := NewParser()

		info := p.getArgumentInfoByID("nonexistent-id")
		assert.Nil(t, info)
	})

	t.Run("ID in lookup but not in acceptedFlags", func(t *testing.T) {
		p := NewParser()

		// Add ID to lookup but not to acceptedFlags
		p.lookup["orphan-id"] = "nonexistent-flag"

		info := p.getArgumentInfoByID("orphan-id")
		assert.Nil(t, info)
	})

	t.Run("empty ID", func(t *testing.T) {
		p := NewParser()

		info := p.getArgumentInfoByID("")
		assert.Nil(t, info)
	})
}
