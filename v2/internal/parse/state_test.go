package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultState(t *testing.T) {
	t.Run("NewState", func(t *testing.T) {
		args := []string{"arg1", "arg2"}
		state := NewState(args)
		assert.Equal(t, -1, state.Pos())
		assert.Equal(t, args, state.Args())
	})

	t.Run("CurrentPos and SetPos", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		assert.Equal(t, -1, state.Pos())

		state.SetPos(1)
		assert.Equal(t, 1, state.Pos())
	})

	t.Run("SkipCurrent", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		state.SetPos(0)
		state.Skip()
		assert.Equal(t, 1, state.Pos())
	})

	t.Run("Args and ReplaceArgs", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		assert.Equal(t, []string{"arg1", "arg2"}, state.Args())

		newArgs := []string{"new1", "new2"}
		state.ReplaceArgs(newArgs...)
		assert.Equal(t, newArgs, state.Args())
	})

	t.Run("InsertArgsAt", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		state.InsertArgsAt(1, "new1", "new2")
		assert.Equal(t, []string{"arg1", "new1", "new2", "arg2"}, state.Args())
	})

	t.Run("CurrentArg", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		state.SetPos(0)
		assert.Equal(t, "arg1", state.CurrentArg())
	})

	t.Run("Peek", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		state.SetPos(0)
		assert.Equal(t, "arg2", state.Peek())

		// Test peek at end
		state.SetPos(1)
		assert.Equal(t, "", state.Peek())
	})

	t.Run("Advance", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		state.SetPos(0)

		assert.True(t, state.Advance())
		assert.Equal(t, 1, state.Pos())

		// Test advance at end
		assert.False(t, state.Advance())
		assert.Equal(t, 1, state.Pos())
	})

	t.Run("Len", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})
		assert.Equal(t, 2, state.Len())
	})

	t.Run("ArgAt", func(t *testing.T) {
		state := NewState([]string{"arg1", "arg2"})

		arg, err := state.ArgAt(1)
		assert.NoError(t, err)
		assert.Equal(t, "arg2", arg)

		// Test invalid position
		_, err = state.ArgAt(-1)
		assert.ErrorIs(t, err, ErrInvalidPosition)

		_, err = state.ArgAt(2)
		assert.ErrorIs(t, err, ErrInvalidPosition)
	})
}
