package debug

import "fmt"

func Assertf(b bool, format string, a ...any) {
	if DEBUG && !b {
		panic(fmt.Sprintf(format, a...))
	}
}

func AssertEq[T comparable](expected, got T) { Assertf(expected == got, "%v != %v", expected, got) }
