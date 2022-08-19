package vm

import (
	"fmt"

	"github.com/rami3l/golox/utils"
)

//go:generate stringer -type=OpCode
type OpCode byte

/* Stack effects are shown below using the Forth convention: ( before -- after ). */
const (
	// OpReturn() ends the current call frame, reclaims its slots,
	// and puts the result back at the top of the stack (or returns it if top-level).
	OpReturn OpCode = iota
	// OpConst(idx) pushes the constant at `idx`.
	// ( -- const )
	OpConst
	// OpNil() pushes a nil.
	// ( -- nil )
	OpNil
	// OpTrue() pushes a true.
	// ( -- true )
	OpTrue
	// OpFalse() pushes a false.
	// ( -- false )
	OpFalse
	// OpPop() pops a value.
	// ( val -- )
	OpPop
	// OpGetLocal(slot) pushes the local at the given `slot`.
	// ( -- local )
	OpGetLocal
	// OpSetLocal(slot) sets the local at the given `slot` to `val`.
	// ( val -- val )
	OpSetLocal
	// OpGetGlobal(name) pushes the global with the given `name`.
	// ( -- global )
	OpGetGlobal
	// OpDefGlobal(name) defines a new global named `name` with value `val`.
	// ( val -- )
	OpDefGlobal
	// OpSetGlobal(name) sets the global with the given `name` to `val`.
	// The global must be defined with `OpDefGlobal` first.
	// ( val -- val )
	OpSetGlobal
	// OpGetUpval(slot) pushes the upval at the given `slot`.
	// ( -- upval )
	OpGetUpval
	// OpSetUpval(slot) sets the upval at the given `slot` to point at `val`.
	// ( val -- val )
	OpSetUpval
	// OpGetProp(name) pushes the property (field/method) of `this` with the given `name`.
	// ( this -- prop )
	OpGetProp
	// OpSetProp(name) sets the field of `this` to `val`.
	// ( this val -- val )
	OpSetProp
	// OpEqual() tests equality.
	// ( x y -- xEqY )
	OpEqual
	// OpGreater() tests "greater than".
	// ( x y -- xGtY )
	OpGreater
	// OpLess() tests "less than".
	// ( x y -- xLtY )
	OpLess
	// OpNot() logically negates a value.
	// ( x -- notX )
	OpNot
	// OpNeg() arithmetically negates a value.
	// ( x -- negX )
	OpNeg
	// OpAdd() adds 2 values.
	// ( x y -- xAddY )
	OpAdd
	// OpSub() subtracts 2 values.
	// ( x y -- xSubY )
	OpSub
	// OpMul() multiplies 2 values.
	// ( x y -- xMulY )
	OpMul
	// OpDiv() divides 2 values.
	// ( x y -- xDivY )
	OpDiv
	// OpPrint() pops and prints a value.
	// ( val -- )
	OpPrint
	// OpJump(hi, lo) increments the IP by (hi<<8|lo).
	// ( -- )
	OpJump
	// OpJumpUnless(hi, lo) increments the IP by (hi<<8|lo) if `val` is falsey.
	// ( val -- val )
	OpJumpUnless
	// OpLoop(hi, lo) decrements the IP by (hi<<8|lo).
	// ( -- )
	OpLoop
	// OpCall(argCount) calls `callee` with a argument list of length `argCount`.
	// ( callee args...[argCount] -- res )
	OpCall
	// OpInvoke(name, argCount) calls the `name` method of `this` with a argument list of length `argCount`.
	// This is a superinstruction for OpGetProp(name) + OpCall(argCount).
	// ( this args...[argCount] -- res )
	OpInvoke
	// OpClos(fun, (isLocal, idx)...[fun.upvalCount]) makes a new closure
	// out of `fun` and given `upval` (isLocal, idx) pairs.
	// ( -- clos )
	OpClos
	// OpCloseUpval() closes and pops `openUpval`.
	// ( openUpval -- )
	OpCloseUpval
	// OpClass(name) pushes a new class named `name`.
	// ( -- class )
	OpClass
	// OpMethod(name) registers a new `method` under `class` using the given `name`.
	// ( class method -- class )
	OpMethod
)

type Chunk struct {
	code []byte
	// Contract: len(lines) == len(code)
	lines  []int
	consts []Value
}

func NewChunk() *Chunk { return &Chunk{} }

func (c *Chunk) Write(b byte, line int) {
	c.code = append(c.code, b)
	c.lines = append(c.lines, line)
}

func (c *Chunk) AddConst(const_ Value) (idx int) {
	idx = len(c.consts)
	c.consts = append(c.consts, const_)
	return
}

func (c *Chunk) DisassembleInst(offset int) (res string, newOffset int) {
	appendf := func(format string, a ...any) { res += fmt.Sprintf(format, a...) }

	appendf("%04d ", offset)
	if offset > 0 && c.lines[offset] == c.lines[offset-1] {
		appendf("   | ")
	} else {
		appendf("%4d ", c.lines[offset])
	}

	switch inst := OpCode(c.code[offset]); inst {
	case OpClos:
		const_ := c.code[offset+1]
		offset += 2
		fun := c.consts[const_].(*VFun)
		appendf("%-16s %4d %s", inst, const_, fun)
		for i := 0; i < fun.upvalCount; i++ {
			isLocal, idx := utils.IntToBool(c.code[offset]), c.code[offset+1]
			isLocalStr := "upvalue"
			if isLocal {
				isLocalStr = "local"
			}
			appendf(
				"\n%04d    |                     %s %d",
				offset, isLocalStr, idx,
			)
			offset += 2
		}
		return res, offset
	// Jump operators.
	case OpJump, OpJumpUnless, OpLoop: // `jumpInstruction`
		jump := int(c.code[offset+1])<<8 | int(c.code[offset+2])
		if inst == OpLoop {
			jump = -jump
		}
		appendf("%-16s %4d -> %d", inst, offset,
			offset+3+jump)
		return res, offset + 3
	// Binary operators.
	case OpInvoke:
		const_, argCount := c.code[offset+1], c.code[offset+2]
		appendf(
			"%-16s (%d args) %4d '%s'",
			inst, argCount, const_, c.consts[const_],
		)
		return res, offset + 3
	// Unary operators.
	case OpConst, OpGetGlobal, OpDefGlobal, OpSetGlobal, OpGetProp, OpSetProp, OpClass, OpMethod: // `constantInstruction`
		const_ := c.code[offset+1]
		appendf("%-16s %4d '%s'", inst, const_, c.consts[const_])
		return res, offset + 2
	case OpGetLocal, OpSetLocal, OpCall,
		OpGetUpval, OpSetUpval: // `byteInstruction`
		slot := c.code[offset+1]
		appendf("%-16s %4d", inst, slot)
		return res, offset + 2
	// Nullary operators.
	default: // `simpleInstruction`
		appendf("%s", inst)
		return res, offset + 1
	}
}

func (c *Chunk) Disassemble(name string) (res string) {
	res = fmt.Sprintf("== %s ==\n", name)
	for i := 0; i < len(c.code); {
		var delta string
		delta, i = c.DisassembleInst(i)
		res += delta + "\n"
	}
	return res
}
