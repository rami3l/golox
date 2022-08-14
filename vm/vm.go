package vm

import (
	"fmt"
	"io"
	"time"

	"github.com/chzyer/readline"
	"github.com/rami3l/golox/debug"
	e "github.com/rami3l/golox/errors"
	"github.com/sirupsen/logrus"
)

type VM struct {
	stack   []Value     // Contract: put only Vxxx types and not *Vxxx types.
	frames  []CallFrame // The call stack.
	globals map[VStr]Value
}

func NewVM() *VM {
	// * Note: This deviates from the original implementation because no manual GC is required.
	return &VM{globals: map[VStr]Value{
		// Native functions.
		NewVStr("clock"): VNativeFun(func(_ ...Value) (Value, bool) {
			return VNum(time.Now().UnixMicro()) * 1e-6, true
		}),
	}}
}

func (vm *VM) frame() *CallFrame     { return &vm.frames[len(vm.frames)-1] }
func (vm *VM) slotAt(idx int) *Value { return &vm.stack[vm.frame().base+idx] }
func (vm *VM) chunk() *Chunk         { return vm.frame().fun.chunk }
func (vm *VM) ip() *int              { return &vm.frame().ip }

type CallFrame struct {
	fun *VFun
	ip  int
	// base is the leftmost index of slots.
	// Slots conceptually represent a top-justified slice view of the stack,
	// in which `fun` and all of `fun`'s variables live.
	// Thus, base is also the index at which `fun` is found in the stack.
	base int
}

func NewCallFrame() *CallFrame {
	fun := NewVFun()
	return &CallFrame{fun: &fun}
}

// Contract: push only Vxxx types and not *Vxxx types.
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
			logrus.Errorln(err)
			logrus.Errorln(vm.callTrace())
		}
		fmt.Printf("<< %s\n", val)
	}
}

func (vm *VM) Interpret(src string, isREPL bool) (res Value, err error) {
	parser := NewParser()
	fun, err := parser.Compile(src, isREPL)
	if err != nil {
		return VNil{}, err
	}
	vm.push(fun)    // Push the current function to slack slot 0.
	vm.call(fun, 0) // Set up the call frame for the top-level code.
	return vm.run()
}

func (vm *VM) run() (Value, error) {
	if vm.chunk() == nil {
		return nil, vm.MkError("chunk uninitialized")
	}

	readByte := func() (res byte) {
		res = vm.chunk().code[*vm.ip()]
		*vm.ip()++
		return
	}

	readShort := func() (res uint16) {
		res = uint16(readByte()) << 8
		res |= uint16(readByte())
		return
	}

	readConst := func() (res Value) {
		res = vm.chunk().consts[readByte()]
		if debug.DEBUG {
			logrus.Debugf("          readConst %11s", res)
		}
		return
	}

	for {
		if debug.DEBUG {
			logrus.Debugln(vm.stackTrace())
		}
		oldIP := *vm.ip()
		if debug.DEBUG {
			instDump, _ := vm.chunk().DisassembleInst(oldIP)
			logrus.Debugln(instDump)
		}
		switch inst := OpCode(readByte()); inst {
		case OpReturn:
			res := vm.pop()
			frame := vm.frames[len(vm.frames)-1]
			if vm.frames = vm.frames[:len(vm.frames)-1]; len(vm.frames) == 0 {
				// Special case for the top-most function.
				switch len := len(vm.stack); len {
				case 1:
					vm.pop() // Pop off the top-most function.
					return res, nil
				case 2:
					res = vm.pop()
					vm.pop() // Pop off the top-most function.
					return res, nil
				default:
					return VNil{}, vm.MkErrorf("unexpected number of values in the stack: '%v'", vm.stack)
				}
			}
			// Chop off the frame slots from the current stack.
			vm.stack = vm.stack[:frame.base]
			// Put the return value back to the stack top.
			vm.push(res)
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
			slot := int(readByte())
			vm.push(*vm.slotAt(slot))
		case OpSetLocal:
			slot := int(readByte())
			*vm.slotAt(slot) = vm.peek(0)
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
			*vm.ip() += int(offset)
		case OpJumpUnless:
			offset := readShort()
			if !VTruthy(vm.peek(0)) {
				*vm.ip() += int(offset)
			}
		case OpLoop:
			offset := readShort()
			*vm.ip() -= int(offset)
		case OpCall:
			argCount := int(readByte())
			fun := vm.peek(argCount)
			if ok := vm.call(fun, argCount); !ok {
				return VNil{}, vm.MkError("can only call functions and classes")
			}
		default:
			return VNil{}, &e.RuntimeError{
				Line:   vm.chunk().lines[oldIP],
				Reason: fmt.Sprintf("unknown instruction '%d'", inst),
			}
		}
	}
}

func (vm *VM) call(callee Value, argCount int) (ok bool) {
	base := len(vm.stack) - argCount - 1
	switch callee := callee.(type) {
	case VFun:
		if argCount != callee.arity {
			vm.MkErrorf("expected %d arguments but got %d",
				callee.arity, argCount)
			return false
		}
		// * NOTE: We could also add a stack overflow check here.
		vm.frames = append(vm.frames, CallFrame{fun: &callee, base: base})
		return true
	case VNativeFun:
		res, ok := callee(vm.stack[base:]...)
		// Chop off the frame slots and the function slot from the current stack.
		vm.stack = vm.stack[:base]
		vm.push(res)
		return ok
	default:
		return false
	}
}

func (vm *VM) MkError(reason string) *e.RuntimeError {
	err := &e.RuntimeError{Reason: reason}
	if vm.chunk() != nil {
		err.Line = vm.chunk().lines[*vm.ip()]
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

func (vm *VM) callTrace() (res string) {
	res = "call trace:"
	for i := len(vm.frames) - 1; i >= 0; i-- {
		frame := &vm.frames[i]
		fun := frame.fun
		res += fmt.Sprintf(
			"\n          [L%d] in %s()",
			// The - 1 is because the IP is already sitting on the next instruction to be executed,
			// but we want the stack trace to point to the previous failed instruction.
			fun.chunk.lines[frame.ip-1],
			fun.Name(),
		)
	}
	return
}
