package vm

import (
	"fmt"
	"io"

	"github.com/chzyer/readline"
	"github.com/rami3l/golox/debug"
	e "github.com/rami3l/golox/errors"
	"github.com/sirupsen/logrus"
)

type VM struct {
	chunk   *Chunk
	ip      int
	stack   []Value
	globals map[VStr]Value
}

func NewVM() *VM { return &VM{globals: map[VStr]Value{}} }

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

		val, err := vm.Interpret(line, true)
		if err != nil {
			logrus.Error(err)
		}
		switch val := val.(type) {
		case VNil:
			// Noop.
		default:
			fmt.Printf("<< %s\n", val)
		}
	}
}

func (vm *VM) Interpret(src string, isREPL bool) (Value, error) {
	parser := NewParser()
	chunk, err := parser.Compile(src, isREPL)
	if err != nil {
		return VNil{}, err
	}
	vm.chunk = chunk
	vm.ip = 0
	return vm.run()
}

func (vm *VM) run() (Value, error) {
	if vm.chunk == nil {
		return VNil{}, vm.MkError("chunk uninitialized")
	}

	readByte := func() (res byte) {
		res = vm.chunk.code[vm.ip]
		vm.ip++
		return
	}

	readShort := func() (res uint16) {
		res = uint16(readByte()) << 8
		res |= uint16(readByte())
		return
	}

	readConst := func() Value { return vm.chunk.consts[readByte()] }

	for {
		if debug.DEBUG {
			logrus.Debugln(vm.stackTrace())
		}
		oldIP := vm.ip
		if debug.DEBUG {
			instDump, _ := vm.chunk.DisassembleInst(oldIP)
			logrus.Debugln(instDump)
		}
		switch inst := OpCode(readByte()); inst {
		case OpReturn:
			switch len := len(vm.stack); len {
			case 0:
				return VNil{}, nil
			case 1:
				res := vm.stack[0]
				vm.stack = vm.stack[:0]
				return res, nil
			default:
				return VNil{}, vm.MkErrorf("too many values in the stack: '%v'", vm.stack)
			}
		case OpConst:
			vm.push(readConst())
		case OpNil:
			vm.push(VNil{})
		case OpTrue:
			vm.push(VBool(true))
		case OpFalse:
			vm.push(VBool(false))
		case OpPop:
			vm.pop()
		case OpGetLocal:
			slot := readByte()
			vm.push(vm.stack[slot])
		case OpSetLocal:
			slot := readByte()
			vm.stack[slot] = vm.peek(0)
			// Don't pop, since the set operation has the RHS as its return value.
		case OpGetGlobal:
			name := readConst().(VStr)
			val, ok := vm.globals[name]
			if !ok {
				return VNil{}, vm.MkErrorf("undefined variable '%v'", name)
			}
			vm.push(val)
		case OpDefGlobal:
			name := readConst().(VStr)
			vm.globals[name] = vm.peek(0)
			vm.pop()
		case OpSetGlobal:
			name := readConst().(VStr)
			if _, ok := vm.globals[name]; !ok {
				return VNil{}, vm.MkErrorf("undefined variable '%v'", name)
			}
			vm.globals[name] = vm.peek(0)
			// Don't pop, since the set operation has the RHS as its return value.
		case OpEqual:
			rhs := vm.pop()
			vm.push(VEq(vm.pop(), rhs))
		case OpGreater:
			rhs := vm.pop()
			res, ok := VGreater(vm.pop(), rhs)
			if !ok {
				return VNil{}, vm.MkError("operands must be numbers")
			}
			vm.push(res)
		case OpLess:
			rhs := vm.pop()
			res, ok := VLess(vm.pop(), rhs)
			if !ok {
				return VNil{}, vm.MkError("operands must be numbers")
			}
			vm.push(res)
		case OpNot:
			vm.push(!VTruthy(vm.pop()))
		case OpNeg:
			res, ok := VNeg(vm.pop())
			if !ok {
				return VNil{}, vm.MkError("operand must be a number")
			}
			vm.push(res)
		case OpAdd:
			rhs := vm.pop()
			res, ok := VAdd(vm.pop(), rhs)
			if !ok {
				return VNil{}, vm.MkError("operands must be all numbers or all strings")
			}
			vm.push(res)
		case OpSub:
			rhs := vm.pop()
			res, ok := VSub(vm.pop(), rhs)
			if !ok {
				return VNil{}, vm.MkError("operands must be numbers")
			}
			vm.push(res)
		case OpMul:
			rhs := vm.pop()
			res, ok := VMul(vm.pop(), rhs)
			if !ok {
				return VNil{}, vm.MkError("operands must be numbers")
			}
			vm.push(res)
		case OpDiv:
			rhs := vm.pop()
			res, ok := VDiv(vm.pop(), rhs)
			if !ok {
				return VNil{}, vm.MkError("operands must be numbers")
			}
			vm.push(res)
		case OpPrint:
			fmt.Printf("%s\n", vm.pop())
		case OpJump:
			offset := readShort()
			vm.ip += int(offset)
		case OpJumpUnless:
			offset := readShort()
			if !VTruthy(vm.peek(0)) {
				vm.ip += int(offset)
			}
		case OpLoop:
			offset := readShort()
			vm.ip -= int(offset)
		default:
			return VNil{}, &e.RuntimeError{
				Line:   vm.chunk.lines[oldIP],
				Reason: fmt.Sprintf("unknown instruction '%d'", inst),
			}
		}
	}
}

func (vm *VM) MkError(reason string) *e.RuntimeError {
	err := &e.RuntimeError{Reason: reason}
	if vm.chunk != nil {
		err.Line = vm.chunk.lines[vm.ip]
	}
	return err
}

func (vm *VM) MkErrorf(format string, a ...any) *e.RuntimeError {
	return vm.MkError(fmt.Sprintf(format, a...))
}

func (vm *VM) stackTrace() string {
	res := "          "
	if len(vm.stack) == 0 {
		res += "[   ]"
		return res
	}
	for _, slot := range vm.stack {
		res += fmt.Sprintf("[ %s ]", slot)
	}
	return res
}
