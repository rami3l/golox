package vm

import (
	"bufio"
	"fmt"
	"os"

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

func (vm *VM) REPL() error {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(">> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if line == "" {
			return nil
		}
		if err := vm.Interpret(line); err != nil {
			return err
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
		return &e.RuntimeError{
			Line:   -1,
			Reason: "chunk uninitialized",
		}
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
		case OpAdd:
			rhs := vm.pop()
			vm.push(vm.pop() + rhs)
		case OpSub:
			rhs := vm.pop()
			vm.push(vm.pop() - rhs)
		case OpMul:
			rhs := vm.pop()
			vm.push(vm.pop() * rhs)
		case OpDiv:
			rhs := vm.pop()
			vm.push(vm.pop() / rhs)
		case OpNeg:
			vm.push(-vm.pop())
		case OpReturn:
			fmt.Printf("%s\n", vm.pop())
			return nil
		case OpConst:
			const_ := vm.chunk.consts[readByte()]
			vm.push(const_)
		default:
			return &e.RuntimeError{
				Line:   vm.chunk.lines[oldIP],
				Reason: fmt.Sprintf("unknown instruction '%d'", inst),
			}
		}
	}
}

func (vm *VM) stackTrace() string {
	res := "          "
	for _, slot := range vm.stack {
		res += fmt.Sprintf("[ %s ]", slot)
	}
	return res
}
