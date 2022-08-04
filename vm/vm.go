package vm

import (
	"fmt"
	"io"

	"github.com/chzyer/readline"
	e "github.com/rami3l/golox/errors"
	"github.com/sirupsen/logrus"
)

type VM struct {
	chunk *Chunk
	ip    int
	stack []Value
}

func NewVM() *VM { return &VM{} }

func (vm *VM) push(val Value) {
	vm.stack = append(vm.stack, val)
}

func (vm *VM) pop() (last Value) {
	len_ := len(vm.stack)
	vm.stack, last = vm.stack[:len_-1], vm.stack[len_-1]
	return
}

func (vm *VM) peek(distance int) Value {
	return vm.stack[len(vm.stack)-1-distance]
}

func (vm *VM) REPL() error {
	reader, err := readline.New(">> ")
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		line, err := reader.Readline()
		switch err {
		case nil:
			if line == "" {
				return nil
			}
		case readline.ErrInterrupt: // ^C
			continue
		case io.EOF: // ^D
			return nil
		default:
			return err
		}

		if err := vm.Interpret(line); err != nil {
			logrus.Error(err)
		}
	}
}

func (vm *VM) Interpret(src string) error {
	parser := NewParser()
	chunk, err := parser.Compile(src)
	if err != nil {
		return err
	}
	vm.chunk = chunk
	vm.ip = 0
	return vm.run()
}

func (vm *VM) run() error {
	if vm.chunk == nil {
		return vm.Error("chunk uninitialized")
	}

	readByte := func() (res byte) {
		res = vm.chunk.code[vm.ip]
		vm.ip++
		return
	}

	for {
		logrus.Debugln(vm.stackTrace())
		oldIP := vm.ip
		instDump, _ := vm.chunk.DisassembleInst(oldIP)
		logrus.Debugln(instDump)
		switch inst := OpCode(readByte()); inst {
		case OpReturn:
			fmt.Printf("%s\n", vm.pop())
			return nil
		case OpConst:
			const_ := vm.chunk.consts[readByte()]
			vm.push(const_)
		case OpNil:
			vm.push(VNil{})
		case OpTrue:
			vm.push(VBool(true))
		case OpFalse:
			vm.push(VBool(false))
		case OpEqual:
			rhs := vm.pop()
			vm.push(VEq(vm.pop(), rhs))
		case OpNot:
			vm.push(!VTruthy(vm.pop()))
		case OpNeg:
			res, ok := VNeg(vm.pop())
			if !ok {
				return vm.Error("operand must be a number")
			}
			vm.push(res)
		case OpAdd:
			rhs := vm.pop()
			res, ok := VAdd(vm.pop(), rhs)
			if !ok {
				return vm.Error("operands must be all numbers or all strings")
			}
			vm.push(res)
		case OpSub:
			rhs := vm.pop()
			res, ok := VSub(vm.pop(), rhs)
			if !ok {
				return vm.Error("operands must be numbers")
			}
			vm.push(res)
		case OpMul:
			rhs := vm.pop()
			res, ok := VMul(vm.pop(), rhs)
			if !ok {
				return vm.Error("operands must be numbers")
			}
			vm.push(res)
		case OpDiv:
			rhs := vm.pop()
			res, ok := VDiv(vm.pop(), rhs)
			if !ok {
				return vm.Error("operands must be numbers")
			}
			vm.push(res)
		case OpGreater:
			rhs := vm.pop()
			res, ok := VGreater(vm.pop(), rhs)
			if !ok {
				return vm.Error("operands must be numbers")
			}
			vm.push(res)
		case OpLess:
			rhs := vm.pop()
			res, ok := VLess(vm.pop(), rhs)
			if !ok {
				return vm.Error("operands must be numbers")
			}
			vm.push(res)
		default:
			return &e.RuntimeError{
				Line:   vm.chunk.lines[oldIP],
				Reason: fmt.Sprintf("unknown instruction '%d'", inst),
			}
		}
	}
}

func (vm *VM) Error(reason string) *e.RuntimeError {
	err := &e.RuntimeError{Reason: reason}
	if vm.chunk != nil {
		err.Line = vm.chunk.lines[vm.ip]
	}
	return err
}

func (vm *VM) stackTrace() string {
	res := "          "
	for _, slot := range vm.stack {
		res += fmt.Sprintf("[ %s ]", slot)
	}
	return res
}
