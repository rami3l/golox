package cmd

import (
	"fmt"
	"os"

	"github.com/rami3l/golox/vm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	easy "github.com/t-tomalak/logrus-easy-formatter"
)

func App() (app *cobra.Command) {
	app = &cobra.Command{
		Use:   "golox",
		Short: "Launch the `golox` interpreter",
	}

	app.Flags().SortFlags = true
	defaultVerbosityStr := "INFO"
	verbosity := app.Flags().StringP("verbosity", "v", defaultVerbosityStr, "Logging verbosity")

	app.Run = func(_ *cobra.Command, args []string) {
		verbosityLvl, err := logrus.ParseLevel(*verbosity)
		if err != nil {
			verbosityLvl, _ = logrus.ParseLevel(defaultVerbosityStr)
		}
		logrus.SetLevel(verbosityLvl)
		logrus.SetFormatter(&easy.Formatter{LogFormat: "//DBG// %msg%\n"})

		if err := appMain(); err != nil {
			logrus.Fatal(err)
			os.Exit(1)
		}
	}
	return
}

func appMain() error {
	fmt.Println("Hello from Lox!")

	c := vm.NewChunk()

	n1 := c.AddConst(vm.Value(1.2))
	c.Write(uint8(vm.OpConst), 123)
	// HACK: Truncating from int to uint8.
	c.Write(uint8(n1), 123)

	n2 := c.AddConst(vm.Value(3.4))
	c.Write(uint8(vm.OpConst), 123)
	// HACK: Truncating from int to uint8.
	c.Write(uint8(n2), 123)

	// 1.2 3.4 +
	c.Write(uint8(vm.OpAdd), 123)

	n3 := c.AddConst(vm.Value(5.6))
	c.Write(uint8(vm.OpConst), 123)
	// HACK: Truncating from int to uint8.
	c.Write(uint8(n3), 123)

	// 1.2 3.4 + 5.6 / -
	c.Write(uint8(vm.OpDiv), 123)
	c.Write(uint8(vm.OpNeg), 123)

	c.Write(uint8(vm.OpReturn), 123)

	fmt.Println(c.Disassemble("test"))

	vm_ := vm.NewVM()
	vm_.Interpret(c)

	return nil
}
