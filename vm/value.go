package vm

import "fmt"

type Value interface{ isValue() }

func NewValue() Value { return VNil{} }

type VBool bool

func (_ VBool) isValue()       {}
func (v VBool) String() string { return fmt.Sprintf("%t", v) }

type VNil struct{}

func (_ VNil) isValue()       {}
func (v VNil) String() string { return "nil" }

type VNum float64

func (_ VNum) isValue()       {}
func (v VNum) String() string { return fmt.Sprintf("%g", v) }

type VObj interface {
	Value
	isObj()
}

type VStr string // TODO: Support string interning?

func (_ VStr) isValue()       {}
func (_ VStr) isObj()         {}
func (v VStr) String() string { return fmt.Sprintf(`"%s"`, string(v)) }

func VAdd(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return v + w, true
		}
	case VStr:
		switch w := w.(type) {
		case VStr:
			return v + w, true
		}
	}
	return
}

func VSub(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return v - w, true
		}
	}
	return
}

func VMul(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return v * w, true
		}
	}
	return
}

func VDiv(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return v / w, true
		}
	}
	return
}

func VGreater(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return VBool(v > w), true
		}
	}
	return
}

func VLess(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return VBool(v < w), true
		}
	}
	return
}

func VNeg(v Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		return -v, true
	}
	return
}

func VTruthy(v Value) VBool {
	switch v := v.(type) {
	case VBool:
		return v
	case VNil:
		return false
	default:
		return true
	}
}

func VEq(v, w Value) VBool { return v == w }
