package errors

import (
	"errors"
	"fmt"
)

type CompilationError struct {
	Line   int
	Reason string
}

func (e *CompilationError) Error() string {
	return fmt.Sprintf("compilation error [L%d]: %s", e.Line, e.Reason)
}

type RuntimeError struct {
	Line   int
	Reason string
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("runtime error [L%d]: %s", e.Line, e.Reason)
}

var UnreachableError = errors.New("internal error: entered unreachable code")
