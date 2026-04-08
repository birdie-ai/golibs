package dql

import (
	"errors"
	"fmt"
)

type Error struct {
	err    error
	Line   int
	Column int
}

func (e Error) Error() string {
	return fmt.Sprintf(`%d:%d %s`, e.Line, e.Column, e.err.Error())
}

func (e Error) Is(target error) bool {
	return errors.Is(e.err, target)
}
