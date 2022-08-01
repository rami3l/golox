package main

import (
	"fmt"

	"github.com/rami3l/golox/vm"
)

func main() {
	fmt.Println("Hello from Lox!")

	c := vm.NewChunk()

	n1 := c.AddConst(vm.Value(1.2))
	c.Write(uint8(vm.OpConst), 123)
	// HACK: Truncating from int to uint8.
	c.Write(uint8(n1), 123)

	c.Write(uint8(vm.OpReturn), 123)

	fmt.Println(c.Disassemble("test"))
}
