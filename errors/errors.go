package errors

import (
	"fmt"
)

type CompilationError struct {
	Reason string
	Line   int
}

func (e *CompilationError) Error() string {
	return fmt.Sprintf("compilation error [L%d]: %s", e.Line, e.Reason)
}

type RuntimeError struct {
	Reason string
	Line   int
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("runtime error [L%d]: %s", e.Line, e.Reason)
}

const Unreachable = "internal error: entered unreachable code"
