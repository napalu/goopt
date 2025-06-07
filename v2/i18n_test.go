package goopt

import (
	"testing"

	"github.com/napalu/goopt/v2/i18n"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

// TestReplaceDefaultBundle tests the ReplaceDefaultBundle function
func TestReplaceDefaultBundle(t *testing.T) {
	t.Run("replace with valid bundle", func(t *testing.T) {
		p := NewParser()
		
		// Create a new bundle with custom translations
		bundle := i18n.NewEmptyBundle()
		bundle.AddLanguage(language.English, map[string]string{
			"test.key": "Test Value",
		})
		
		err := p.ReplaceDefaultBundle(bundle)
		assert.NoError(t, err)
		
		// Verify the bundle was replaced
		assert.Equal(t, bundle, p.GetSystemBundle())
	})
	
	t.Run("replace with nil bundle returns error", func(t *testing.T) {
		p := NewParser()
		
		err := p.ReplaceDefaultBundle(nil)
		assert.Error(t, err)
		// The error will contain the i18n key since translations aren't loaded
		assert.Contains(t, err.Error(), "nil_pointer")
	})
}

// TestWithLanguage tests the WithLanguage configuration function
func TestWithLanguage(t *testing.T) {
	t.Run("set language to German", func(t *testing.T) {
		p, err := NewParserWith(WithLanguage(language.German))
		assert.NoError(t, err)
		
		// The parser should have German as the default language
		bundle := p.GetSystemBundle()
		assert.Equal(t, language.German, bundle.GetDefaultLanguage())
	})
	
	t.Run("set language to French", func(t *testing.T) {
		p, err := NewParserWith(WithLanguage(language.French))
		assert.NoError(t, err)
		
		bundle := p.GetSystemBundle()
		assert.Equal(t, language.French, bundle.GetDefaultLanguage())
	})
}

// TestWithUserBundle tests the WithUserBundle configuration function
func TestWithUserBundle(t *testing.T) {
	t.Run("set valid user bundle", func(t *testing.T) {
		userBundle := i18n.NewEmptyBundle()
		userBundle.AddLanguage(language.English, map[string]string{
			"custom.key": "Custom Value",
		})
		
		p, err := NewParserWith(WithUserBundle(userBundle))
		assert.NoError(t, err)
		
		// Verify the user bundle was set
		assert.Equal(t, userBundle, p.GetUserBundle())
	})
	
	t.Run("set nil user bundle", func(t *testing.T) {
		// WithUserBundle with nil should return an error
		p, err := NewParserWith(WithUserBundle(nil))
		assert.Error(t, err)
		assert.Nil(t, p)
	})
}

// TestWithReplaceBundle tests the WithReplaceBundle configuration function
func TestWithReplaceBundle(t *testing.T) {
	t.Run("replace bundle during parser creation", func(t *testing.T) {
		customBundle := i18n.NewEmptyBundle()
		customBundle.AddLanguage(language.English, map[string]string{
			"replaced.key": "Replaced Value",
		})
		
		p, err := NewParserWith(WithReplaceBundle(customBundle))
		assert.NoError(t, err)
		
		// Verify the bundle was replaced
		assert.Equal(t, customBundle, p.GetSystemBundle())
	})
	
	t.Run("replace with nil bundle returns error", func(t *testing.T) {
		// WithReplaceBundle should handle the error internally
		// The parser creation might fail or use default bundle
		p, err := NewParserWith(WithReplaceBundle(nil))
		assert.Error(t, err)
		
		// Parser should not be created
		assert.Nil(t, p)
	})
}

// TestWithExecOnParseComplete tests the WithExecOnParseComplete configuration function
func TestWithExecOnParseComplete(t *testing.T) {
	t.Run("execute callbacks after parse complete", func(t *testing.T) {
		executed := false
		
		cmd := NewCommand(
			WithName("test"),
			WithCallback(func(p *Parser, c *Command) error {
				executed = true
				return nil
			}),
		)
		
		p, err := NewParserWith(
			WithCommand(cmd),
			WithExecOnParseComplete(true),
		)
		assert.NoError(t, err)
		
		// WithExecOnParseComplete automatically executes commands after successful parse
		success := p.Parse([]string{"test"})
		assert.True(t, success)
		
		// Callback should have been executed automatically
		assert.True(t, executed)
	})
	
	t.Run("interaction with WithExecOnParse", func(t *testing.T) {
		executed := false
		
		cmd := NewCommand(
			WithName("test"),
			WithCallback(func(p *Parser, c *Command) error {
				executed = true
				return nil
			}),
		)
		
		// WithExecOnParse takes precedence
		p, err := NewParserWith(
			WithCommand(cmd),
			WithExecOnParse(true),
			WithExecOnParseComplete(true), // This should have no effect
		)
		assert.NoError(t, err)
		
		success := p.Parse([]string{"test"})
		assert.True(t, success)
		
		// Callback should have executed during parse due to WithExecOnParse
		assert.True(t, executed)
	})
}