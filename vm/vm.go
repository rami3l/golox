package vm

import (
	"fmt"
	"io"
	"time"

	"github.com/chzyer/readline"
	"github.com/rami3l/golox/debug"
	e "github.com/rami3l/golox/errors"
	"github.com/rami3l/golox/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type VM struct {
	stack []Value
	// The call stack.
	frames  []CallFrame
	globals map[VStr]Value

	// The head pointer of an intrusive linked list of open VUpvals, required for escape analysis.
	openUpvals *VUpval
}

func NewVM() *VM {
	// * Note: This deviates from the original implementation because no manual GC is required.
	return &VM{globals: map[VStr]Value{
		// Native functions.
		*NewVStr("clock"): NewVNativeFun(func(_ ...Value) (Value, error) {
			return VNum(time.Now().UnixNano()) / VNum(time.Second), nil
		}),
	}}
}

func (vm *VM) Recover() {
	vm.stack = []Value{}
	vm.frames = []CallFrame{}
}

func (vm *VM) frame() *CallFrame {
	if len(vm.frames) == 0 {
		return nil
	}
	return &vm.frames[len(vm.frames)-1]
}

func (vm *VM) slotIdxAt(idx int) (stackIdx int) {
	frame := vm.frame()
	if frame == nil {
		return Uninit
	}
	return frame.base + idx
}

func (vm *VM) slotAt(idx int) *Value {
	idx1 := vm.slotIdxAt(idx)
	if idx1 == Uninit || idx1 >= len(vm.stack) {
		return nil
	}
	return &vm.stack[idx1]
}

func (vm *VM) chunk() *Chunk {
	frame := vm.frame()
	if frame == nil || frame.clos == nil {
		return nil
	}
	return vm.frame().clos.chunk
}

func (vm *VM) ip() *int {
	frame := vm.frame()
	if frame == nil {
		return nil
	}
	return &frame.ip
}

type CallFrame struct {
	clos *VClos
	ip   int
	// base is the leftmost index of slots.
	// Slots conceptually represent a top-justified slice view of the stack,
	// in which `fun` and all of `fun`'s variables live.
	// Thus, base is also the index at which `fun` is found in the stack.
	base int
}

func (vm *VM) peek(distance int) Value { return vm.stack[len(vm.stack)-1-distance] }

func (vm *VM) push(val Value) (last *Value) {
	vm.stack = append(vm.stack, val)
	return &vm.stack[len(vm.stack)-1]
}

func (vm *VM) pop() (last Value) {
	len_ := len(vm.stack)
	vm.stack, last = vm.stack[:len_-1], vm.stack[len_-1]
	return
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
	defer func() {
		if err != nil {
			vm.Recover()
		}
	}()

	parser := NewParser()
	fun, err := parser.Compile(src, isREPL)
	clos := NewVClos(fun)
	if err != nil {
		return VNil{}, err
	}
	// Push the current function to slack slot 0.
	vm.push(clos)
	// Set up the call frame for the top-level code.
	if err = vm.call(clos, 0); err != nil {
		return
	}
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

	readStr := func() *VStr { return readConst().(*VStr) }

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
			// Close every remaining open upval owned by the returning function.
			vm.closeUpvals(frame.base)
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
			// Chop off the frame slots from the current stack,
			// and put the return value back to the stack top.
			vm.stack = append(vm.stack[:frame.base], res)
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
			name := *readStr()
			val, ok := vm.globals[name]
			if !ok {
				return VNil{}, vm.MkErrorf("undefined variable '%s'", name.Inner())
			}
			vm.push(val)
		case OpDefGlobal:
			name := *readStr()
			vm.globals[name] = vm.pop()
		case OpSetGlobal:
			name := *readStr()
			if _, ok := vm.globals[name]; !ok {
				return VNil{}, vm.MkErrorf("undefined variable '%s'", name.Inner())
			}
			vm.globals[name] = vm.peek(0)
			// Don't pop, since the set operation has the RHS as its return value.
		case OpGetUpval:
			slot := int(readByte())
			vm.push(*vm.frame().clos.upvals[slot].val)
		case OpSetUpval:
			slot := int(readByte())
			upval := vm.frame().clos.upvals[slot]
			upval.val = utils.Ref(vm.peek(0))
			upval.idx = utils.Ref(len(vm.stack) - 1)
			// Don't pop, since the set operation has the RHS as its return value.
		case OpGetProp:
			this, ok := vm.peek(0).(*VInstance)
			if !ok {
				return VNil{}, vm.MkError("only instances have properties")
			}
			name := *readStr()
			res, ok := this.fields[name]
			if !ok {
				// Fall back to method resolution.
				method, ok := this.methods[name]
				if !ok {
					return VNil{}, vm.MkErrorf("undefined property '%s'", name.Inner())
				}
				res = NewVBoundMethod(vm.peek(0), method.(*VClos))
			}
			vm.stack[len(vm.stack)-1] = res // Replace the instance with the result.
		case OpSetProp:
			this, ok := vm.peek(1).(*VInstance)
			if !ok {
				return VNil{}, vm.MkError("only instances have fields")
			}
			name := *readStr()
			this.fields[name] = vm.peek(0) // The RHS.
			// Pop off the instance, keep the RHS as its return value.
			vm.stack = slices.Delete(vm.stack, len(vm.stack)-2, len(vm.stack)-1)
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
			callee := vm.peek(argCount)
			if err := vm.call(callee, argCount); err != nil {
				return VNil{}, err
			}
		case OpInvoke:
			name := *readStr()
			argCount := int(readByte())
			this, ok := vm.peek(argCount).(*VInstance)
			if !ok {
				return VNil{}, vm.MkError("only instances have methods")
			}
			// What if `method` in `this.method()` is not a method but a regular closure?
			if field, ok := this.fields[name]; ok {
				base := len(vm.stack) - argCount - 1
				vm.stack[base] = field
				if err := vm.call(field, argCount); err != nil {
					return VNil{}, err
				}
				break
			}
			if err := vm.invokeFromClass(this.VClass, name, argCount); err != nil {
				return VNil{}, err
			}
		case OpClos:
			fun := readConst().(*VFun)
			clos := NewVClos(fun)
			upvals := clos.upvals
			vm.push(clos)
			for i := range upvals { // ! Here we use the index only.
				isLocal := utils.IntToBool(readByte())
				idx := int(readByte())
				if isLocal {
					upvals[i] = vm.captureUpval(vm.slotIdxAt(idx))
				} else {
					upvals[i] = vm.frame().clos.upvals[idx]
				}
			}
		case OpCloseUpval:
			vm.closeUpvals(len(vm.stack) - 1) // Hoist the upval.
			vm.pop()                          // Pop the hoisted upval off the stack.
		case OpClass:
			vm.push(NewVClass(readStr()))
		case OpMethod:
			name := *readStr()
			method := vm.pop()
			class := vm.peek(0).(*VClass)
			class.methods[name] = method
		default:
			return VNil{}, &e.RuntimeError{
				Line:   vm.chunk().lines[oldIP],
				Reason: fmt.Sprintf("unknown instruction '%d'", inst),
			}
		}
	}
}

func (vm *VM) call(callee Value, argCount int) error {
	base := len(vm.stack) - argCount - 1
	switch callee := callee.(type) {
	case *VClass:
		// Replace the called class with a new instance.
		vm.stack[base] = NewVInstance(callee)
		// Execute `init` if exists and is a closure.
		if init, ok := callee.methods[*NewVStr("init")]; ok {
			if init, ok := init.(*VClos); ok {
				// The `init` closure assumes the slot 0 to be `this`, just like regular methods.
				return vm.callClos(init, argCount)
			}
		} else if argCount != 0 {
			return vm.MkErrorf("expected 0 arguments but got %d.",
				argCount)
		}
	case *VBoundMethod:
		// Replace the called method with `this`.
		vm.stack[base] = callee.this
		return vm.callClos(callee.VClos, argCount)
	case *VClos:
		return vm.callClos(callee, argCount)
	case *VNativeFun:
		res, err := (*callee)(vm.stack[base:]...)
		if err != nil {
			return err
		}
		// Chop off the frame slots and the function slot from the current stack.
		vm.stack = append(vm.stack[:base], res)
	default:
		return vm.MkError("can only call functions and classes")
	}
	return nil
}

func (vm *VM) callClos(clos *VClos, argCount int) error {
	base := len(vm.stack) - argCount - 1
	if argCount != clos.arity {
		return vm.MkErrorf("expected %d arguments but got %d",
			clos.arity, argCount)
	}
	// * NOTE: We could also add a stack overflow check here.
	vm.frames = append(vm.frames, CallFrame{clos: clos, base: base})
	return nil
}

func (vm *VM) invokeFromClass(class *VClass, methodName VStr, argCount int) error {
	method, ok := class.methods[methodName]
	if !ok {
		return vm.MkErrorf("undefined property '%s'", methodName.Inner())
	}
	return vm.call(method, argCount)
}

func (vm *VM) closeUpvals(minStackIdx int) {
	for curr := &vm.openUpvals; *curr != nil && *(*curr).idx >= minStackIdx; *curr = (*curr).next {
		// Thanks to Go's garbage collection, there's no need to actually save the VUpval to a "closed" field.
		// Here, we set idx to nil to indicate that the Upval is closed.
		(*curr).idx = nil
	}
}

func (vm *VM) captureUpval(stackIdx int) (res *VUpval) {
	prev, curr := (*VUpval)(nil), vm.openUpvals
	for curr != nil && *curr.idx > stackIdx {
		prev, curr = curr, curr.next
	}
	if curr != nil && *curr.idx == stackIdx {
		// An existing VUpval can be reused.
		return curr
	}

	res = NewVUpval(&vm.stack[stackIdx], stackIdx)
	res.next = curr
	if prev == nil {
		// The iteration didn't start: vm.openUpVals had too low idx or was empty.
		// Insert res to the beginning of the vm.openUpvals linked list.
		vm.openUpvals = res
	} else {
		// res needs to be inserted between prev and curr.
		prev.next = res
	}
	return
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
		clos := frame.clos
		res += fmt.Sprintf(
			"\n          [L%d] in %s()",
			// The - 1 is because the IP is already sitting on the next instruction to be executed,
			// but we want the stack trace to point to the previous failed instruction.
			clos.chunk.lines[frame.ip-1],
			clos.Name(),
		)
	}
	return
}
