package goopt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgument_String(t *testing.T) {
	arg := NewArg(
		WithDescription("test desc"),
		WithShortFlag("t"),
		WithDefaultValue("default"),
		SetRequired(true),
	)

	str := arg.String()
	assert.Contains(t, str, "test desc", "string representation should contain description")
	assert.Contains(t, str, "-t", "string representation should contain short flag")
	assert.Contains(t, str, "default", "string representation should contain default value")
	assert.Contains(t, str, "required", "string representation should indicate required status")
}
