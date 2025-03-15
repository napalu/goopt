package parse

import (
	"errors"

	"github.com/napalu/goopt/util"
)

// State represents the current state of the argument parser
type State interface {
	Pos() int                                // Get the current position
	SetPos(pos int)                          // Set the current position
	Skip()                                   // Skip the current argument
	Args() []string                          // Get the entire argument list
	InsertArgsAt(pos int, newArgs ...string) // Insert new arguments at a specific position
	ReplaceArgs(newArgs ...string)           // Replace the entire argument list
	CurrentArg() string                      // Get the current argument
	ArgAt(pos int) (string, error)           // Get the argument at a specific position
	Peek() string                            // Peek at the next argument
	Advance() bool                           // New method for advancing to the next argument
	Len() int                                // Gets the length of the argument list
}

// ErrInvalidPosition is an error that occurs when an invalid position is accessed
var ErrInvalidPosition = errors.New("invalid position")

// DefaultState is the default implementation of the State interface
type DefaultState struct {
	pos  int
	args []string
}

// NewState creates a new State instance with the given argument list
func NewState(args []string) State {
	return &DefaultState{
		pos:  -1,
		args: args,
	}
}

// Pos returns the current position in the argument list
func (s *DefaultState) Pos() int {
	return s.pos
}

// SetPos sets the current position in the argument list
func (s *DefaultState) SetPos(pos int) {
	s.pos = pos
}

// Skip advances the current position to the next argument
func (s *DefaultState) Skip() {
	s.pos++
}

// Args returns the entire argument list
func (s *DefaultState) Args() []string {
	return s.args
}

// CurrentArg returns the current argument
func (s *DefaultState) CurrentArg() string {
	if s.pos < 0 || s.pos >= len(s.args) {
		return ""
	}
	return s.args[s.pos]
}

// InsertArgsAt inserts new arguments at a specific position
func (s *DefaultState) InsertArgsAt(pos int, newArgs ...string) {
	s.args = util.InsertSlice(s.args, pos, newArgs...)
}

// ReplaceArgs replaces the entire argument list with new arguments
func (s *DefaultState) ReplaceArgs(newArgs ...string) {
	s.args = newArgs
}

// Advance advances to the next argument, returning true if successful
func (s *DefaultState) Advance() bool {
	if s.pos+1 < len(s.args) {
		s.pos++
		return true
	}
	return false
}

// Peek returns the next argument without advancing the current position
func (s *DefaultState) Peek() string {
	if s.pos+1 < len(s.args) {
		return s.args[s.pos+1]
	}

	return ""
}

// ArgAt returns the argument at a specific position
func (s *DefaultState) ArgAt(pos int) (string, error) {
	if pos < 0 || pos >= len(s.args) {
		return "", ErrInvalidPosition
	}

	return s.args[pos], nil
}

// Len returns the length of the argument list
func (s *DefaultState) Len() int {
	return len(s.args)
}
