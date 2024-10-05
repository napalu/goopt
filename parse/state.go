package parse

import (
	"errors"
	"github.com/napalu/goopt/util"
)

type State interface {
	CurrentPos() int
	SetPos(pos int)
	SkipCurrent()
	Args() []string
	InsertArgsAt(pos int, newArgs ...string)
	ReplaceArgs(newArgs ...string)
	CurrentArg() string
	Peek() string
	Advance() bool // New method for advancing to the next argument
	Len() int      // Gets the length of the argument list
}

var InvalidPositionError = errors.New("invalid position")

type DefaultState struct {
	pos  int
	args []string
	init bool
}

func NewState(args []string) *DefaultState {
	return &DefaultState{
		pos:  -1,
		args: args,
	}
}

func (s *DefaultState) CurrentPos() int {
	return s.pos
}

func (s *DefaultState) SetPos(pos int) {
	s.pos = pos
}

func (s *DefaultState) SkipCurrent() {
	s.pos++
}

func (s *DefaultState) Args() []string {
	return s.args
}

func (s *DefaultState) CurrentArg() string {
	return s.args[s.pos]
}

func (s *DefaultState) InsertArgsAt(pos int, newArgs ...string) {
	s.args = util.InsertSlice(s.args, pos, newArgs...)
}

func (s *DefaultState) ReplaceArgs(newArgs ...string) {
	s.args = newArgs
}

func (s *DefaultState) Advance() bool {
	if s.pos+1 < len(s.args) {
		s.pos++
		return true
	}
	return false
}

func (s *DefaultState) Peek() string {
	if s.pos+1 < len(s.args) {
		return s.args[s.pos+1]
	}

	return ""
}

func (s *DefaultState) ArgAt(pos int) (string, error) {
	if pos < 0 || pos >= len(s.args) {
		return "", InvalidPositionError
	}

	return s.args[pos], nil
}

func (s *DefaultState) Len() int {
	return len(s.args)
}
