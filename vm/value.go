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
	name       *VStr
	arity      int
	upvalCount int
	chunk      *Chunk
}

func NewVFun() *VFun { return &VFun{chunk: NewChunk()} }

func (v *VFun) Name() string {
	if v.name == nil {
		return "?"
	}
	return v.name.Inner()
}

func (_ *VFun) isValue()      {}
func (_ *VFun) isObj()        {}
func (v VFun) String() string { return fmt.Sprintf("<fun %s>", v.Name()) }

type VUpval struct {
	val *Value
	// The index at which val can be found in the stack if it is still open.
	// If it is closed, idx should be nil.
	idx *int
	// The next pointer of an intrusive linked list of open VUpvals, required for escape analysis.
	next *VUpval
}

func NewVUpval(val *Value, idx int) *VUpval { return &VUpval{val: val, idx: utils.Ref(idx)} }

func (_ *VUpval) isValue() {}
func (_ *VUpval) isObj()   {}

func (v VUpval) String() string {
	if v.val == nil {
		return "upvalue(nil)"
	}
	return fmt.Sprintf("upvalue(%s)", *v.val)
}

// VClos is a Lox closure.
type VClos struct {
	*VFun
	upvals []*VUpval // A list of borrowed VUpval.
}

func NewVClos(fun *VFun) *VClos { return &VClos{VFun: fun, upvals: make([]*VUpval, fun.upvalCount)} }

func (_ *VClos) isValue()      {}
func (_ *VClos) isObj()        {}
func (v VClos) String() string { return v.VFun.String() }

type (
	VNativeFun NativeFun
	NativeFun  = func(args ...Value) (res Value, err error)
)

func NewVNativeFun(fun NativeFun) *VNativeFun { return utils.Ref(VNativeFun(fun)) }

func (_ *VNativeFun) isValue()      {}
func (_ *VNativeFun) isObj()        {}
func (v VNativeFun) String() string { return "<native fun>" }

type VClass struct {
	name    *VStr
	methods map[VStr]Value
}

func NewVClass(name *VStr) *VClass { return &VClass{name: name, methods: map[VStr]Value{}} }

func (_ *VClass) isValue()      {}
func (_ *VClass) isObj()        {}
func (v VClass) String() string { return fmt.Sprintf("<class %s>", v.name.Inner()) }

type VInstance struct {
	*VClass
	fields map[VStr]Value
}

func NewVInstance(class *VClass) *VInstance {
	return &VInstance{VClass: class, fields: map[VStr]Value{}}
}

func (v VInstance) String() string { return fmt.Sprintf("<instanceof %s>", v.VClass.name.Inner()) }

type VBoundMethod struct {
	*VClos
	this Value
}

func NewVBoundMethod(this Value, clos *VClos) *VBoundMethod {
	return &VBoundMethod{VClos: clos, this: this}
}

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
