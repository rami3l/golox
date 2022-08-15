package utils

import "golang.org/x/exp/constraints"

func Ref[T any](t T) *T                         { return &t }
func IntToBool[I constraints.Integer](i I) bool { return i != 0 }

func BoolToInt[I constraints.Integer](b bool) I {
	if b {
		return 1
	}
	return 0
}
