package i18n

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
	"testing"
)

func TestNewSystemLocale(t *testing.T) {
	tag := language.Spanish
	translations := `{"key": "value"}`

	locale := NewLocale(tag, translations)

	assert.Equal(t, tag, locale.Tag)
	assert.Equal(t, translations, locale.Translations)
}
