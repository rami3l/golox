package vm

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/josharian/intern"
	"github.com/rami3l/golox/debug"
	e "github.com/rami3l/golox/errors"
	"github.com/sirupsen/logrus"
)

type Parser struct {
	*Scanner
	*Compiler
	prev, curr Token

	loopStart    *int
	loopEndHoles []int

	errors *multierror.Error
	// Whether the parser is trying to sync, i.e. in the error recovery process.
	panicMode bool
}

func NewParser() *Parser { return &Parser{} }

type Compiler struct {
	enclosing *Compiler
	fun       VFun
	funType   FunType
	locals    []Local
	depth     int
}

type FunType int

//go:generate stringer -type=FunType
const (
	FFun FunType = iota
	FScript
)

func NewCompiler(enclosing *Compiler, funType FunType) *Compiler {
	res := Compiler{
		enclosing: enclosing,
		fun:       NewVFun(),
		funType:   funType,
		// Reserve the locals slot 0 to indicate the function being called.
		locals: []Local{{}},
	}
	return &res
}

// wrapCompiler replaces the Compiler with a new one enclosing the current one.
func (p *Parser) wrapCompiler(funType FunType) {
	res := NewCompiler(p.Compiler, funType)
	if funType != FScript {
		funName := intern.String(p.prev.String())
		res.fun.name = &funName
	}
	p.Compiler = res
}

const Uninit = -1

func (c *Compiler) addLocal(name Token) {
	if len(c.locals) >= math.MaxUint8+1 {
		logrus.Panicln("too many variables in function")
	}
	c.locals = append(c.locals, Local{name, Uninit})
}

type Local struct {
	name  Token
	depth int
}

/* Single-pass compilation */

func (p *Parser) emitConst(val Value) { p.emitBytes(byte(OpConst), p.mkConst(val)) }

func (p *Parser) mkConst(val Value) byte {
	const_ := p.currChunk().AddConst(val)
	if const_ > math.MaxUint8 {
		logrus.Panicln("too many consts in one chunk")
	}
	return byte(const_)
}

func (p *Parser) num(_canAssign bool) {
	val, err := strconv.ParseFloat(p.prev.String(), 64)
	p.errors = multierror.Append(p.errors, err)
	p.emitConst(VNum(val))
}

func (p *Parser) grouping(_canAssign bool) {
	p.expr()
	p.consume(TRParen, "expect ')' after expression")
}

func (p *Parser) lit(_canAssign bool) {
	switch p.prev.Type {
	case TFalse:
		p.emitBytes(byte(OpFalse))
	case TNil:
		p.emitBytes(byte(OpNil))
	case TTrue:
		p.emitBytes(byte(OpTrue))
	default:
		panic(e.Unreachable)
	}
}

func (p *Parser) str(_canAssign bool) {
	runes := p.prev.Runes
	// COPY the lexeme inside the quotes as a string.
	unquoted := string(runes[1 : len(runes)-1])
	p.emitConst(NewVStr(unquoted))
}

func (p *Parser) var_(canAssign bool) { p.namedVar(p.prev, canAssign) }

func (p *Parser) namedVar(name Token, canAssign bool) {
	slot := p.resolveLocal(name)
	if slot > math.MaxUint8 {
		logrus.Panicln("scope depth limit exceeded")
	}

	var (
		arg      byte
		get, set OpCode
	)
	if slot == Uninit {
		arg, get, set = p.identConst(&name), OpGetGlobal, OpSetGlobal
	} else {
		arg, get, set = byte(slot), OpGetLocal, OpSetLocal
	}

	switch {
	case canAssign && p.match(TEqual):
		p.expr()
		p.emitBytes(byte(set), arg)
	default:
		p.emitBytes(byte(get), arg)
	}
}

func (p *Parser) unary(_canAssign bool) {
	op := p.prev.Type

	// Compile the RHS.
	p.parsePrec(PrecUnary)

	// Emit the operator instruction.
	switch op {
	case TBang:
		p.emitBytes(byte(OpNot))
	case TMinus:
		p.emitBytes(byte(OpNeg))
	default:
		panic(e.Unreachable)
	}
}

func (p *Parser) binary(_canAssign bool) {
	op := p.prev.Type
	rule := parseRules[op]

	// Compile the RHS.
	p.parsePrec(rule.Prec + 1)

	// Emit the operator instruction.
	switch op {
	case TBangEqual:
		p.emitBytes(byte(OpEqual), byte(OpNot))
	case TEqualEqual:
		p.emitBytes(byte(OpEqual))
	case TGreater:
		p.emitBytes(byte(OpGreater))
	case TGreaterEqual:
		p.emitBytes(byte(OpLess), byte(OpNot))
	case TLess:
		p.emitBytes(byte(OpLess))
	case TLessEqual:
		p.emitBytes(byte(OpGreater), byte(OpNot))
	case TPlus:
		p.emitBytes(byte(OpAdd))
	case TMinus:
		p.emitBytes(byte(OpSub))
	case TStar:
		p.emitBytes(byte(OpMul))
	case TSlash:
		p.emitBytes(byte(OpDiv))
	default:
		panic(e.Unreachable)
	}
}

func (p *Parser) and(_canAssign bool) {
	// If the LHS is falsey, then `LHS and RHS == false`.
	// So we skip the RHS and leave the LHS as the result.
	endJump := p.emitJump(OpJumpUnless)
	// If the LHS is truthy, then `LHS and RHS == RHS`.
	// So we pop out the LHS.
	p.emitBytes(byte(OpPop))
	p.parsePrec(PrecAnd)
	p.patchJump(endJump)
}

func (p *Parser) or(_canAssign bool) {
	// If the LHS is truthy, then `LHS or RHS == true`.
	// So we skip the RHS and leave the LHS as the result.
	elseJump := p.emitJump(OpJumpUnless) // <-- else
	endJump := p.emitJump(OpJump)        // <-- then
	// If the LHS is falsey, then `LHS or RHS == RHS`.
	// So we pop out the LHS.
	p.patchJump(elseJump) // --> else
	p.emitBytes(byte(OpPop))
	p.parsePrec(PrecOr)
	p.patchJump(endJump) // --> then
}

func (p *Parser) call(_canAssign bool) {
	argCount := p.argList()
	p.emitBytes(byte(OpCall), byte(argCount))
}

func (p *Parser) argList() (argCount int) {
	if !p.check(TRParen) {
		for {
			p.expr()
			if argCount++; argCount >= math.MaxUint8 {
				p.Error("too many arguments")
			}
			if !p.match(TComma) {
				break
			}
		}
	}
	p.consume(TRParen, "expect ')' after arguments")
	return
}

func (p *Parser) expr() { p.parsePrec(PrecAssign) }

func (p *Parser) exprStmt() {
	p.expr()
	p.consume(TSemi, "expect ';' after value")
	p.emitBytes(byte(OpPop))
}

func (p *Parser) printStmt() {
	p.expr()
	p.consume(TSemi, "expect ';' after value")
	p.emitBytes(byte(OpPrint))
}

func (p *Parser) block() {
	for !p.check(TRBrace) && !p.check(TEOF) {
		p.decl()
	}
	p.consume(TRBrace, "expect '}' after block")
}

func (p *Parser) ifStmt() {
	p.consume(TLParen, "expect '(' after 'if'")
	p.expr()
	p.consume(TRParen, "expect ')' after condition")

	thenJump := p.emitJump(OpJumpUnless) // <-- `else` branch stops.
	p.emitBytes(byte(OpPop))             // Drop the predicate before the `then` statement.
	p.stmt()

	elseJump := p.emitJump(OpJump) // <-- `then` branch stops.
	p.patchJump(thenJump)          // --> `else` branch continues.

	p.emitBytes(byte(OpPop)) // Drop the predicate before the `else` statement.
	if p.match(TElse) {
		p.stmt()
	}
	p.patchJump(elseJump) // --> `then` branch continues.
}

func (p *Parser) whileStmt() {
	p.beginLoop()
	p.consume(TLParen, "expect '(' after 'while'")
	p.expr()
	p.consume(TRParen, "expect ')' after condition")

	exitJump := p.emitJump(OpJumpUnless)
	p.emitBytes(byte(OpPop)) // Pop the condition.
	p.stmt()
	p.emitLoop(*p.loopStart)
	p.endLoop()

	p.patchJump(exitJump) // Pop the condition.
	p.emitBytes(byte(OpPop))
}

func (p *Parser) forStmt() {
	// for (init; cond; incr) body
	p.beginScope()
	defer p.endScope()

	// init
	p.consume(TLParen, "expect '(' after 'for'")
	switch {
	case p.match(TSemi):
		// Noop.
	case p.match(TVar):
		p.varDecl()
	default:
		p.exprStmt()
	}

	// cond
	start := p.beginLoop()
	exitJump := (*int)(nil)
	if !p.match(TSemi) {
		p.expr()
		p.consume(TSemi, "expect ';' after loop condition")
		exitJump1 := p.emitJump(OpJumpUnless) // <-- !!cond == false
		exitJump = &exitJump1
		p.emitBytes(byte(OpPop)) // Pop the condition.
	}

	// incr
	if !p.match(TRParen) {
		bodyJump := p.emitJump(OpJump) // <-- body
		p.beginLoop()                  // <-- incr
		// Parse an exprStmt sans the trailing ';'.
		p.expr()
		p.emitBytes(byte(OpPop)) // Pure side effect.

		p.consume(TRParen, "expect ')' after for clauses")

		p.emitLoop(start)     // --> incr, towards the next iteration
		p.patchJump(bodyJump) // --> body
	}

	// body
	p.stmt()
	p.emitLoop(*p.loopStart) // --> towards incr (if exists, otherwise next iteration)

	if exitJump != nil {
		p.patchJump(*exitJump)   // --> !!cond == false
		p.emitBytes(byte(OpPop)) // Pop the condition.
	}
	p.endLoop()
}

func (p *Parser) breakStmt() {
	p.consume(TSemi, "expect ';' after 'break'")
	hole := p.emitJump(OpJump)
	p.loopEndHoles = append(p.loopEndHoles, hole)
}

func (p *Parser) continueStmt() {
	p.consume(TSemi, "expect ';' after 'continue'")
	p.emitLoop(*p.loopStart)
}

func (p *Parser) returnStmt() {
	if p.match(TSemi) {
		p.emitReturn()
		return
	}
	p.expr()
	p.consume(TSemi, "expect ';' after return value")
	p.emitBytes(byte(OpReturn))
}

func (p *Parser) stmt() {
	switch {
	case p.match(TBreak):
		if !p.isInLoop() {
			p.Error("expect 'break' in a loop")
			return
		}
		p.breakStmt()
	case p.match(TContinue):
		if p.isInLoop() {
			p.Error("expect 'continue' in a loop")
			return
		}
		p.continueStmt()
	case p.match(TPrint):
		p.printStmt()
	case p.match(TFor):
		p.forStmt()
	case p.match(TIf):
		p.ifStmt()
	case p.match(TReturn):
		if p.funType == FScript {
			p.Error("can't return from top-level code")
			return
		}
		p.returnStmt()
	case p.match(TWhile):
		p.whileStmt()
	case p.match(TLBrace):
		p.beginScope()
		p.block()
		p.endScope()
	default:
		p.exprStmt()
	}
}

func (p *Parser) fun_() {
	p.wrapCompiler(FFun)
	p.beginScope()

	p.consume(TLParen, "expect '(' after function name")
	if !p.check(TRParen) {
		for {
			if p.fun.arity++; p.fun.arity > math.MaxUint8 {
				p.ErrorAtCurr("too many parameters")
			}
			param := p.parseVar("expect parameter name")
			p.defVar(param)
			if !p.match(TComma) {
				break
			}
		}
	}
	p.consume(TRParen, "expect ')' after parameters")
	p.consume(TLBrace, "expect '{' before function body")
	p.block()

	// Because we end Compiler completely when we reach the end of the function body,
	// there’s no need to close the lingering outermost scope
	fun := p.endCompiler()
	p.emitBytes(byte(OpConst), p.mkConst(fun))
}

func (p *Parser) funDecl() {
	global := p.parseVar("expect function name")
	validName := p.checkPrev(TIdent)
	p.fun_()

	// Global functions are immediately initialized and defined.
	if validName {
		p.markInit()
		p.defVar(global)
	}
}

func (p *Parser) varDecl() {
	global := p.parseVar("expect variable name")
	validName := p.checkPrev(TIdent)
	switch {
	case p.match(TEqual):
		p.expr()
	default:
		p.emitBytes(byte(OpNil))
	}
	p.consume(TSemi, "expect ';' after variable declaration")
	if validName {
		p.defVar(global)
	}
}

func (p *Parser) decl() {
	switch {
	case p.match(TFun):
		p.funDecl()
	case p.match(TVar):
		p.varDecl()
	default:
		p.stmt()
	}
	if p.panicMode {
		p.sync()
	}
}

type ParseFn = func(p *Parser, canAssign bool)

type ParseRule struct {
	Prefix, Infix ParseFn
	Prec
}

var parseRules []ParseRule

func init() {
	parseRules = []ParseRule{
		TLParen:       {(*Parser).grouping, (*Parser).call, PrecCall},
		TMinus:        {(*Parser).unary, (*Parser).binary, PrecTerm},
		TPlus:         {nil, (*Parser).binary, PrecTerm},
		TSlash:        {nil, (*Parser).binary, PrecFactor},
		TStar:         {nil, (*Parser).binary, PrecFactor},
		TBang:         {(*Parser).unary, nil, PrecNone},
		TBangEqual:    {nil, (*Parser).binary, PrecEqual},
		TEqualEqual:   {nil, (*Parser).binary, PrecEqual},
		TGreater:      {nil, (*Parser).binary, PrecComp},
		TGreaterEqual: {nil, (*Parser).binary, PrecComp},
		TLess:         {nil, (*Parser).binary, PrecComp},
		TLessEqual:    {nil, (*Parser).binary, PrecComp},
		TIdent:        {(*Parser).var_, nil, PrecNone},
		TStr:          {(*Parser).str, nil, PrecNone},
		TNum:          {(*Parser).num, nil, PrecNone},
		TAnd:          {nil, (*Parser).and, PrecAnd},
		TFalse:        {(*Parser).lit, nil, PrecNone},
		TNil:          {(*Parser).lit, nil, PrecNone},
		TOr:           {nil, (*Parser).or, PrecOr},
		TTrue:         {(*Parser).lit, nil, PrecNone},
		TEOF:          {},
	}
}

func (p *Parser) parsePrec(prec Prec) {
	p.advance()

	// Parse LHS.
	prefix := parseRules[p.prev.Type].Prefix
	if prefix == nil {
		p.Error("expect expression")
		return
	}
	canAssign := prec <= PrecAssign
	prefix(p, canAssign)

	// Parse RHS if there's one maintaining rule.Prec >= prec.
	for {
		rule := parseRules[p.curr.Type]
		if rule.Prec < prec {
			break
		}
		p.advance()
		if rule.Infix == nil {
			panic(e.Unreachable)
		}
		rule.Infix(p, canAssign)
	}

	if canAssign && p.match(TEqual) {
		p.Error("invalid assignment target")
		p.advance()
	}
}

/* Parsing helpers */

func (p *Parser) check(ty TokenType) bool     { return p.curr.Type == ty }
func (p *Parser) checkPrev(ty TokenType) bool { return p.prev.Type == ty }

func (p *Parser) advance() {
	p.prev = p.curr
	for {
		// Skip until the first non-TErr token.
		if p.curr = p.ScanToken(); !p.check(TErr) {
			break
		}
		p.Error(p.curr.String())
	}
}

func (p *Parser) match(ty TokenType) (matched bool) {
	if !p.check(ty) {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) consume(ty TokenType, errorMsg string) *Token {
	if !p.check(ty) {
		p.ErrorAtCurr(errorMsg)
		return nil
	}
	p.advance()
	return &p.prev
}

/* Compiling helpers */

func (p *Parser) Compile(src string, isREPL bool) (res VFun, err error) {
	res, err = p.compileWithRule(src, func(p *Parser) {
		for !p.match(TEOF) {
			p.decl()
		}
	})
	if isREPL && err != nil {
		declsErr := err
		p.errors = nil
		res, err = p.compileWithRule(src, (*Parser).expr)
		if err != nil {
			err = fmt.Errorf("%w\ncaused by:\n%s", declsErr, err)
		}
	}
	return
}

func (p *Parser) compileWithRule(src string, rule func(*Parser)) (res VFun, err error) {
	p.wrapCompiler(FScript)
	p.Scanner = NewScanner(src)

	p.advance()
	rule(p)
	res = p.endCompiler()
	err = p.errors.ErrorOrNil()
	return
}

func (p *Parser) currChunk() *Chunk { return p.fun.chunk }

func (p *Parser) emitBytes(bs ...byte) {
	for _, b := range bs {
		p.currChunk().Write(b, p.prev.Line)
	}
}

func (p *Parser) emitReturn() { p.emitBytes(byte(OpNil), byte(OpReturn)) }

func (p *Parser) endCompiler() (res VFun) {
	p.emitReturn()
	res = p.fun
	if debug.DEBUG {
		logrus.Debugln(p.currChunk().Disassemble(res.Name()))
	}
	p.Compiler = p.Compiler.enclosing
	return
}

func (p *Parser) identConst(name *Token) byte { return p.mkConst(NewVStr(name.String())) }

func (p *Parser) markInit() {
	if p.depth == 0 {
		return
	}
	p.locals[len(p.locals)-1].depth = p.depth
}

func (p *Parser) defVar(global *byte) {
	if global == nil || p.depth > 0 {
		// Local vars. Mark it as initialized.
		p.markInit()
		return
	}
	p.emitBytes(byte(OpDefGlobal), *global)
}

func (p *Parser) parseVar(errorMsg string) *byte {
	target := p.consume(TIdent, errorMsg)
	if target == nil {
		p.advance()
		return nil // Early return if the assignee is not valid.
	}
	p.declVar()
	if p.depth > 0 {
		return nil // Local vars are not resolved using `identConst`, but stay on the stack.
	}
	res := p.identConst(target)
	return &res
}

func (p *Parser) declVar() {
	if p.depth == 0 {
		return
	}
	name := p.prev
	// Search for the latest variable declaration of the same name.
	for i := len(p.locals) - 1; i >= 0; i-- {
		local := p.locals[i]
		if local.depth != Uninit && local.depth < p.depth {
			break // Variable shadowing in a deeper scope is allowed.
		}
		if name.Eq(local.name) {
			p.Error("already a variable with this name in this scope")
		}
	}
	p.addLocal(name)
}

func (p *Parser) beginLoop() (start int) {
	start = len(p.currChunk().code)
	p.loopStart = &start
	return
}

func (p *Parser) endLoop() {
	for _, hole := range p.loopEndHoles {
		p.patchJump(hole)
	}

	p.loopStart = nil
	p.loopEndHoles = p.loopEndHoles[:0]
	return
}

func (p *Parser) isInLoop() bool { return p.loopStart != nil }
func (p *Parser) beginScope()    { p.depth++ }

func (p *Parser) endScope() {
	p.depth--
	for len(p.locals) > 0 && p.locals[len(p.locals)-1].depth > p.depth {
		p.emitBytes(byte(OpPop)) // Pop off the local on the stack.
		p.locals = p.locals[0 : len(p.locals)-1]
	}
}

func (p *Parser) resolveLocal(name Token) (slot int) {
	// Search for the latest variable declaration of the same name.
	for i := len(p.locals) - 1; i >= 0; i-- {
		local := p.locals[i]
		if name.Eq(local.name) {
			if local.depth == Uninit {
				p.Error("can't read local variable in its own initializer")
			}
			return i
		}
	}
	return Uninit // Global variable.
}

func (p *Parser) emitJump(inst OpCode) (offset int) {
	p.emitBytes(byte(inst), 0xff, 0xff)
	return len(p.currChunk().code) - 2
}

func (p *Parser) patchJump(offset int) {
	code := p.currChunk().code
	// A jump uses 2 bytes to encode the offset, so
	// -2 to adjust for the bytecode for the jump offset itself:
	// [OpJump] [0xff@offset] [0xff@(offset+1)] [GOAL@(offset+2)] ... [CURR@(len-1)]
	jump := len(code) - (offset + 2) // The bytes to jump over.
	if jump > math.MaxUint16 {
		logrus.Panicln("too much code to jump over")
	}
	code[offset], code[offset+1] = byte(jump>>8&0xff), byte(jump&0xff)
}

func (p *Parser) emitLoop(start int) {
	p.emitBytes(byte(OpLoop))
	code := p.currChunk().code
	// [start] ... [OpLoop@(len-1)] [backJump] [backJump] [CURR@(len+2)]
	backJump := len(code) + 2 - start // The bytes to jump backwards over.
	if backJump > math.MaxUint16 {
		logrus.Panicln("loop body too large")
	}
	p.emitBytes(byte(backJump>>8&0xff), byte(backJump&0xff))
}

/* Precedence */

//go:generate stringer -type=Prec
type Prec int

const (
	PrecNone   Prec = iota
	PrecAssign      // =
	PrecOr          // or
	PrecAnd         // and
	PrecEqual       // == !=
	PrecComp        // < > <= >=
	PrecTerm        // + -
	PrecFactor      // * /
	PrecUnary       // ! -
	PrecCall        // . ()
	PrecPrimary
)

/* Error handling */

func (p *Parser) sync() {
	p.panicMode = false
	for !p.check(TEOF) && !p.checkPrev(TSemi) {
		switch p.curr.Type {
		case TClass, TFun, TVar, TFor, TIf, TWhile, TPrint, TReturn:
			return
		default:
			p.advance()
		}
	}
}

func (p *Parser) ErrorAt(tk Token, reason string) {
	// Don't collect error when we're syncing.
	if p.panicMode {
		return
	}
	p.panicMode = true

	var tkStr string
	switch tk.Type {
	case TEOF:
		tkStr = "EOF"
	case TIdent:
		tkStr = fmt.Sprintf("identifier `%v`", tk)
	default:
		tkStr = fmt.Sprintf("`%v`", tk)
	}
	reason1 := fmt.Sprintf("at %s, %s", tkStr, reason)
	err := &e.CompilationError{Line: tk.Line, Reason: reason1}

	if debug.DEBUG {
		logrus.Debugln(p.currChunk().Disassemble("ErrorAt"))
		logrus.Debugln(err)
	}

	p.errors = multierror.Append(p.errors, err)
}

func (p *Parser) Error(reason string)       { p.ErrorAt(p.prev, reason) }
func (p *Parser) ErrorAtCurr(reason string) { p.ErrorAt(p.curr, reason) }
func (p *Parser) HadError() bool            { return p.errors != nil }
