package vm_test

import (
	"fmt"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/rami3l/golox/vm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() { logrus.SetLevel(logrus.DebugLevel) }

type TestPair struct{ input, output string }

func assertEval(t *testing.T, errSubstr string, pairs ...TestPair) {
	t.Helper()
	vm_ := vm.NewVM()
	for _, pair := range pairs {
		val, err := vm_.Interpret(pair.input+"\n", true)
		switch {
		case errSubstr == "":
			assert.Nil(t, err)
		case err != nil:
			assert.ErrorContains(t, err, errSubstr)
			return
		}
		valStr := fmt.Sprintf("%s", val)
		assert.Equal(t, pair.output, valStr)
	}
}

func TestCalculator(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"2 +2", "4"},
		{"11.4 + 5.14 / 19198.10", "11.400267734827926"},
		{"-6 *(-4+ -3) == 6*4 + 2  *((((9))))", "true"},
		{
			heredoc.Doc(`
                4/1 - 4/3 + 4/5 - 4/7 + 4/9 - 4/11 
                    + 4/13 - 4/15 + 4/17 - 4/19 + 4/21 - 4/23
            `),
			"3.058402765927333",
		},
		{
			heredoc.Doc(`
                3
                    + 4/(2*3*4)
                    - 4/(4*5*6)
                    + 4/(6*7*8)
                    - 4/(8*9*10)
                    + 4/(10*11*12)
                    - 4/(12*13*14)
            `),
			"3.1408813408813407",
		},
	}...)
}

func TestVarsBlocks(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var foo = 2;", "nil"},
		{"foo", "2"},
		{"foo + 3 == 1 + foo * foo", "true"},
		{"var bar;", "nil"},
		{"bar", "nil"},
		{"bar = foo = 2;", "nil"},
		{"foo", "2"},
		{"bar", "2"},
		{
			"{ foo = foo + 1; var bar; var foo1 = foo; foo1 = foo1 + 1; }",
			"nil",
		},
		{"foo", "3"},
	}...)
}

func TestVarOwnInit(t *testing.T) {
	t.Parallel()
	assertEval(t, "can't read local variable in its own initializer",
		[]TestPair{
			{"var foo = 2;", "nil"},
			{"{ var foo = foo; }", ""},
		}...,
	)
}

func TestIfElse(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var foo = 2;", "nil"},
		{"if (foo == 2) foo = foo + 1; else { foo = 42; }", "nil"},
		{"foo", "3"},
		{"if (foo == 2) { foo = foo + 1; } else foo = nil;", "nil"},
		{"foo", "nil"},
		{"if (!foo) foo = 1;", "nil"},
		{"foo", "1"},
		{"if (foo) foo = 2;", "nil"},
		{"foo", "2"},
	}...)
}

func TestAndOr(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{`"trick" or __TREAT__`, `"trick"`},
		{"996 or 007", "996"},
		{`nil or "hi"`, `"hi"`},
		{"nil and what", "nil"},
		{`true and "then_what"`, `"then_what"`},
		{"var B = 66;", "nil"},
		{"2*B or !2*B", "132"},
	}...)
}

func TestIfAndOr(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var foo = 2;", "nil"},
		{
			"if (foo != 2 and whatever) foo = foo + 42; else { foo = 3; }",
			"nil",
		},
		{"foo", "3"},
		{
			"if (0 <= foo and foo <= 3) { foo = foo + 1; } else { foo = nil; }",
			"nil",
		},
		{"foo", "4"},
		{"if (!!!(2 + 2 != 5) or !!!!!!!!foo) foo = 1;", "nil"},
		{"foo", "1"},
		{"if (true or whatever) foo = 2;", "nil"},
		{"foo", "2"},
	}...)
}

func TestWhile(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var i = 1; var product = 1;", "nil"},
		{"while (i <= 5) { product = product * i; i = i + 1; }", "nil"},
		{"product", "120"},
	}...)
}

func TestWhileJump(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var i = 1; var product = 1;", "nil"},
		{
			heredoc.Doc(`
                while (true) {
                    if (i == 3 or i == 5) {
                        i = i + 1;
                        continue;
                    }
                    product = product * i;
                    i = i + 1;
                    if (i > 6) {
                        break;
                    }
                }
            `),
			"nil",
		},
		{"product", "48"},
	}...)
}

func TestFor(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var product = 1;", "nil"},
		{
			"for (var i = 1; i <= 5; i = i + 1) { product = product * i; }",
			"nil",
		},
		{"product", "120"},
	}...)
}

func TestForBreak(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var i = 1; var product = 1;", "nil"},
		{
			"for (; ; i = i + 1) { product = product * i; if (i == 5) break; }",
			"nil",
		},
		{"i", "5"},
		{"product", "120"},
	}...)
}

func TestForContinue(t *testing.T) {
	t.Parallel()
	assertEval(t, "", []TestPair{
		{"var i = 1; var product = 1;", "nil"},
		{
			"for (; ; i = i + 1) { product = product * i; if (i < 5) continue; break; }",
			"nil",
		},
		{"i", "5"},
		{"product", "120"},
	}...)
}
