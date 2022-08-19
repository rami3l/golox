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
	t.Parallel()
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
	assert.Empty(t, errSubstr, "a successful test must have an empty errSubStr")
}

func TestCalculator(t *testing.T) {
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
	assertEval(t, "", []TestPair{
		{"var foo = 2;", "nil"},
		{"foo", "2"},
		{"foo + 3 == 1 + foo * foo", "true"},
		{"var bar;", "nil"},
		{"bar", "nil"},
		{"bar = foo = 2;", "nil"},
		{"foo", "2"},
		{"bar", "2"},
		{"{ foo = foo + 1; var bar; var foo1 = foo; foo1 = foo1 + 1; }", "nil"},
		{"foo", "3"},
	}...)
}

func TestVarOwnInit(t *testing.T) {
	assertEval(t, "can't read local variable in its own initializer",
		[]TestPair{
			{"var foo = 2;", "nil"},
			{"{ var foo = foo; }", ""},
		}...,
	)
}

func TestIfElse(t *testing.T) {
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
	assertEval(t, "", []TestPair{
		{"var i = 1; var product = 1;", "nil"},
		{"while (i <= 5) { product = product * i; i = i + 1; }", "nil"},
		{"product", "120"},
	}...)
}

func TestWhileJump(t *testing.T) {
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
					if (i > 6) { break; }
				}
			`),
			"nil",
		},
		{"product", "48"},
	}...)
}

func TestFor(t *testing.T) {
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

func TestBareBreak(t *testing.T) {
	assertEval(t, "expect 'break' in a loop", []TestPair{
		{"break;", ""},
	}...)
}

func TestBareContinue(t *testing.T) {
	assertEval(t, "expect 'continue' in a loop", []TestPair{
		{"continue;", ""},
	}...)
}

func TestBareReturn(t *testing.T) {
	assertEval(t, "can't return from top-level code", []TestPair{
		{"return true;", ""},
	}...)
}

func TestFunReturnInLoop(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				fun fact(n) {
					var i; var product;
					for (i = product = 1; ; i = i + 1) {
						product = product * i;
						if (i >= n) { return product; }
					}
				}
			`),
			"nil",
		},
		{"fact(5)", "120"},
	}...)
}

func TestFunArity(t *testing.T) {
	assertEval(t, "expected 2 arguments but got 1", []TestPair{
		{"fun f(j, k) { return (1 + j) * k; }", "nil"},
		{"f(2)", ""},
	}...)
}

func TestFunRecursive(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"fun fact(i) { if (i <= 0) { return 1; } return i * fact(i - 1); }", "nil"},
		{"fact(5)", "120"},
	}...)
}

func TestFunRefGlobal(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"var a = 3; fun f() { return a; } a = 4;", "nil"},
		{"f()", "4"},
	}...)
}

func TestFunLateInit(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"fun f() { return a; } var a = 4;", "nil"},
		{"f()", "4"},
	}...)
}

func TestFunLateInitFun(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"fun f() { return four(); } fun four() { return 4; }", "nil"},
		{"f()", "4"},
	}...)
}

func TestBareBreakInClos(t *testing.T) {
	assertEval(t, "expect 'break' in a loop", []TestPair{
		{"for (var i = 0; i < 10; i = i + 1) { fun g() { break; } }", ""},
	}...)
}

func TestBareContinueInClos(t *testing.T) {
	assertEval(t, "expect 'continue' in a loop", []TestPair{
		{"for (var i = 0; i < 10; i = i + 1) { fun g() { continue; } }", ""},
	}...)
}

func TestBareReturnInClos(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"var i;", "nil"},
		{"for (i = 0; i < 10; i = i + 1) { fun g() { return; } }", "nil"},
		{"i", "10"},
	}...)
}

func TestClosNoEscape(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				fun outer() {
					var x = "outside";
					fun inner() { return x; }
					return inner();
				}
			`),
			"nil",
		},
		{"outer()", `"outside"`},
	}...)
}

func TestClosEscape(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				fun outer() {
					var x = "outside";
					fun inner() { return x;} 
					return inner;
  				}
			`),
			"nil",
		},
		{"outer()()", `"outside"`},
	}...)
}

func TestClosCurrying(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				fun f(j) { 
					fun g(k) { return (1 + j) * k; }
					return g;
				}
			`),
			"nil",
		},
		{"f(2)(3)", "9"},
	}...)
}

func TestClosRecursive(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"var res;", "nil"},
		{
			heredoc.Doc(`
				{
					fun fact(i) { if (i <= 0) { return 1; } return i * fact(i - 1); }
					res = fact(5);
				}
			`),
			"nil",
		},
		{"res", "120"},
	}...)
}

func TestClosCounter(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				fun make_counter() {
					var i = 0;
					fun count() { i = i + 1; return i; }
					return count;
				}
				var counter = make_counter();
			`),
			"nil",
		},
		{"counter()", "1"},
		{"counter()", "2"},
	}...)
}

func TestClosShareRef(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				var globalSet; var globalGet;
				fun main() {
					var a = "initial";
					fun set() { a = "updated"; }
					fun get() { return a; }
					globalSet = set; globalGet = get;
				}
				main();
				globalSet();
			`),
			"nil",
		},
		{"globalGet()", `"updated"`},
	}...)
}

func TestClosParamShadow(t *testing.T) {
	assertEval(t, "already a variable with this name in this scope", []TestPair{
		{
			heredoc.Doc(`
				var g = "global";
				fun scope(l) {
					var l = "local";
					return l;
				}
				var l = scope(g);
			`),
			"",
		},
	}...)
}

func TestClosVarShadow(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				var a = "global";
				var a1; var a2;
				{
					fun get_a() { return a; }
					a1 = get_a();
					var a = "block";
					a2 = get_a();
				}
			`),
			"nil",
		},
		{"a1", `"global"`},
		{"a2", `"global"`},
	}...)
}

// http://www.rosettacode.org/wiki/Man_or_boy_test#Lox
var manOrBoy = heredoc.Doc(`
	fun A(k, xa, xb, xc, xd, xe) {
		fun B() {
			k = k - 1;
			return A(k, B, xa, xb, xc, xd);
		}
		if (k <= 0) { return xd() + xe(); }
		return B();
	}
	fun I0()  { return  0; }
	fun I1()  { return  1; }
	fun I_1() { return -1; }
`)

func TestClosManOrBoy4(t *testing.T) {
	assertEval(t, "", []TestPair{
		{manOrBoy, "nil"},
		{"A(4, I1, I_1, I_1, I1, I0)", "1"},
	}...)
}

func TestClosManOrBoy10(t *testing.T) {
	assertEval(t, "", []TestPair{
		{manOrBoy, "nil"},
		{"A(10, I1, I_1, I_1, I1, I0)", "-67"},
	}...)
}

func TestClassEmpty(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"class Foo {}", "nil"},
		{"Foo", "<class Foo>"},
	}...)
}

func TestClassGetSet(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"class Foo {}", "nil"},
		{"var foo = Foo();", "nil"},
		{"foo", "<instanceof Foo>"},
		{"foo.bar = 10086", "10086"},
		{"foo.bar", "10086"},
		{`foo.bar = "foobar"`, `"foobar"`},
		{`foo.bar`, `"foobar"`},
		{`foo.baz = foo.bar + "baz"`, `"foobarbaz"`},
		{`foo.baz`, `"foobarbaz"`},
	}...)
}

func TestClassGetUndefined(t *testing.T) {
	assertEval(t, "undefined property 'bar'", []TestPair{
		{"class Foo {}", "nil"},
		{"Foo().bar", ""},
	}...)
}

func TestClassGetInvalid(t *testing.T) {
	assertEval(t, "only instances have properties", []TestPair{
		{"true.story", ""},
	}...)
}

func TestClassSetInvalid(t *testing.T) {
	assertEval(t, "only instances have fields", []TestPair{
		{"true.story = 42", ""},
	}...)
}

func TestClassMethodUnbound(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				class Scone {
					topping(first, second) {
						return "scone with " + first + " and " + second;
					}
				}
			`),
			"nil",
		},
		{"var scone = Scone();", "nil"},
		{`scone.topping("berries", "cream")`, `"scone with berries and cream"`},
	}...)
}

func TestClassMethodBound(t *testing.T) {
	assertEval(t, "", []TestPair{
		{`class Egotist { speak() { return "Just " + this.name; } }`, "nil"},
		{`var jimmy = Egotist(); jimmy.name = "Jimmy";`, "nil"},
		{"jimmy.speak()", `"Just Jimmy"`},
	}...)
}

func TestClassMethodBoundRef(t *testing.T) {
	assertEval(t, "", []TestPair{
		{`class Egotist { speak() { return "Just " + this.name; } }`, "nil"},
		{`var jimmy = Egotist(); jimmy.name = "Jimmy";`, "nil"},
		{"var s = jimmy.speak;", "nil"},
		{"s()", `"Just Jimmy"`},
	}...)
}

func TestClassMethodBoundNested(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				class Nested {
					method() { 
						fun f() { return this; }
						return f();
					}
				}
			`),
			"nil",
		},
		{"Nested().method()", "<instanceof Nested>"},
	}...)
}

func TestBareThis(t *testing.T) {
	assertEval(t, "can't use 'this' outside of a class", []TestPair{
		{"this", ""},
	}...)
}

func TestBareThisFun(t *testing.T) {
	assertEval(t, "can't use 'this' outside of a class", []TestPair{
		{"fun foo() { return this; }", ""},
	}...)
}

func TestClassInit(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				class CoffeeMaker {
					init(coffee) { this.coffee = coffee; }
					brew() {
						var res = "Enjoy your cup of " + this.coffee;
						// No reusing the grounds!
						this.coffee = nil;
						return res;
					}
				}
			`),
			"nil",
		},
		{`var maker = CoffeeMaker("coffee and chicory");`, "nil"},
		{"maker.brew()", `"Enjoy your cup of coffee and chicory"`},
	}...)
}

func TestClassInitReturn(t *testing.T) {
	assertEval(t, "", []TestPair{
		{"class Foo { init(name) { return; } }", "nil"},
	}...)
}

func TestClassInitReturnVal(t *testing.T) {
	assertEval(t, "can't return a value from an initializer", []TestPair{
		{"class Bar { init(name) { return name; } }", ""},
	}...)
}

func TestClassInitArity0(t *testing.T) {
	assertEval(t, "expected 1 arguments but got 0", []TestPair{
		{"class Bar { init(name) {} }", "nil"},
		{"Bar()", "nil"},
	}...)
}

func TestClassInitArityN(t *testing.T) {
	assertEval(t, "expected 1 arguments but got 3", []TestPair{
		{"class Bar { init(name) {} }", "nil"},
		{"Bar(0, 1, 2)", "nil"},
	}...)
}

func TestClassNoInitArity(t *testing.T) {
	assertEval(t, "expected 0 arguments but got 3", []TestPair{
		{"class Bar {}", "nil"},
		{"Bar(0, 1, 2)", ""},
	}...)
}

func TestClassInvokeOptim(t *testing.T) {
	assertEval(t, "", []TestPair{
		{
			heredoc.Doc(`
				class Oops {
					init() {
						fun f() { this.foo = "bar"; }
						this.field = f;
					}
				}
			`),
			"nil",
		},
		{"var oops = Oops();", "nil"},
		{"oops.field();", "nil"},
		{"oops.foo", `"bar"`},
	}...)
}
