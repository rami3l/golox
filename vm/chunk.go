package vm

import (
	"fmt"
)

//go:generate stringer -type=OpCode
type OpCode uint8

const (
	OpReturn OpCode = iota
	OpConst
)

type Chunk struct {
	code []uint8
	// Contract: len(lines) == len(code)
	lines  []int
	consts []Value
}

func NewChunk() *Chunk {
	return &Chunk{}
}

func (c *Chunk) Write(byte_ uint8, line int) {
	c.code = append(c.code, byte_)
	c.lines = append(c.lines, line)
}

func (c *Chunk) AddConst(const_ Value) (idx int) {
	idx = len(c.consts)
	c.consts = append(c.consts, const_)
	return
}

func (c *Chunk) Disassemble(name string) (res string) {
	printf := func(format string, a ...any) {
		res += fmt.Sprintf(format, a...)
	}

	printf("== %s ==\n", name)

	showInst := func(c *Chunk, offset int) int {
		printf("%04d ", offset)
		if offset > 0 && c.lines[offset] == c.lines[offset-1] {
			printf("   | ")
		} else {
			printf("%4d ", c.lines[offset])
		}

		switch inst := OpCode(c.code[offset]); inst {
		case OpReturn:
			printf("%s\n", inst)
			return offset + 1
		case OpConst:
			const_ := c.code[offset+1]
			printf("%-16s %4d '%g'\n", name, const_, c.consts[const_])
			return offset + 2
		default:
			printf("Unknown opcode %d\n", inst)
			return offset + 1
		}
	}

	for i := 0; i < len(c.code); i = showInst(c, i) {
	}
	return res
}
