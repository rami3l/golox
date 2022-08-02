package vm

import (
	"fmt"
)

//go:generate stringer -type=OpCode
type OpCode uint8

const (
	OpReturn OpCode = iota
	OpConst
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpNeg
)

type Chunk struct {
	code []uint8
	// Contract: len(lines) == len(code)
	lines  []int
	consts []Value
}

func NewChunk() *Chunk { return &Chunk{} }

func (c *Chunk) Write(byte_ uint8, line int) {
	c.code = append(c.code, byte_)
	c.lines = append(c.lines, line)
}

func (c *Chunk) AddConst(const_ Value) (idx int) {
	idx = len(c.consts)
	c.consts = append(c.consts, const_)
	return
}

func (c *Chunk) DisassembleInst(offset int) (res string, newOffset int) {
	sprintf := func(format string, a ...any) { res += fmt.Sprintf(format, a...) }

	sprintf("%04d ", offset)
	if offset > 0 && c.lines[offset] == c.lines[offset-1] {
		sprintf("   | ")
	} else {
		sprintf("%4d ", c.lines[offset])
	}

	switch inst := OpCode(c.code[offset]); inst {
	// Nullary operators.
	case OpReturn, OpNeg, OpAdd, OpSub, OpMul, OpDiv:
		sprintf("%s", inst)
		return res, offset + 1
	// Unary operators.
	case OpConst:
		const_ := c.code[offset+1]
		sprintf("%-16s %4d '%s'", inst, const_, c.consts[const_])
		return res, offset + 2
	default:
		sprintf("Unknown opcode %d", inst)
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
