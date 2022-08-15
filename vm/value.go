package vm

import (
	"fmt"

	"github.com/josharian/intern"
	"github.com/rami3l/golox/utils"
)

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

type VStr struct{ _inner string }

func NewVStr(s string) *VStr  { return &VStr{intern.String(s)} }
func (v *VStr) Inner() string { return v._inner }

func (_ *VStr) isValue()      {}
func (_ *VStr) isObj()        {}
func (v VStr) String() string { return fmt.Sprintf(`"%s"`, v.Inner()) }

type VFun struct {
	name       *string
	arity      int
	upvalCount int
	chunk      *Chunk
}

func NewVFun() *VFun { return &VFun{chunk: NewChunk()} }

func (v *VFun) Name() string {
	if v.name == nil {
		return "?"
	}
	return *v.name
}

func (_ *VFun) isValue()      {}
func (_ *VFun) isObj()        {}
func (v VFun) String() string { return fmt.Sprintf("<fun %s>", v.Name()) }

type VUpval struct{ Value }

func NewVUpval(val Value) *VUpval { return &VUpval{Value: val} }

func (v VUpval) String() string { return fmt.Sprintf("upvalue(%s)", v.Value) }

// VClos is a Lox closure.
type VClos struct {
	fun    *VFun
	upvals []*VUpval // A list of borrowed VUpval.
}

func NewVClos(fun *VFun) *VClos { return &VClos{fun: fun, upvals: make([]*VUpval, fun.upvalCount)} }

func (_ *VClos) isValue()      {}
func (_ *VClos) isObj()        {}
func (v VClos) String() string { return v.fun.String() }

type (
	VNativeFun NativeFun
	NativeFun  = func(args ...Value) (res Value, ok bool)
)

func NewVNativeFun(fun NativeFun) *VNativeFun { return utils.Ref(VNativeFun(fun)) }

func (_ *VNativeFun) isValue()      {}
func (_ *VNativeFun) isObj()        {}
func (v VNativeFun) String() string { return fmt.Sprintf("<native fun>") }

/* Value operations */

func VAdd(v, w Value) (res Value, ok bool) {
	res = NewValue()
	switch v := v.(type) {
	case VNum:
		switch w := w.(type) {
		case VNum:
			return v + w, true
		}
	case *VStr:
		switch w := w.(type) {
		case *VStr:
			return NewVStr(v.Inner() + w.Inner()), true
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
