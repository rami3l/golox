package vm

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/rami3l/golox/debug"
	e "github.com/rami3l/golox/errors"
	"github.com/sirupsen/logrus"
)

type Parser struct {
	*Scanner
	prev, curr     Token
	compilingChunk *Chunk

	errors *multierror.Error
	// Whether the parser is trying to sync, i.e. in the error recovery process.
	panicMode bool
}

func NewParser() *Parser { return &Parser{} }

/* Single-pass compilation */

func (p *Parser) emitConst(val Value) { p.emitBytes(byte(OpConst), p.makeConst(val)) }

func (p *Parser) makeConst(val Value) byte {
	const_ := p.currentChunk().AddConst(val)
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
	arg := p.identConst(&name)
	switch {
	case canAssign && p.match(TEqual):
		p.expr()
		p.emitBytes(byte(OpSetGlobal), arg)
	default:
		p.emitBytes(byte(OpGetGlobal), arg)
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

func (p *Parser) stmt() {
	switch {
	case p.match(TPrint):
		p.printStmt()
	default:
		p.exprStmt()
	}
}

func (p *Parser) identConst(name *Token) byte { return p.makeConst(NewVStr(name.String())) }
func (p *Parser) defVar(global byte)          { p.emitBytes(byte(OpDefGlobal), global) }

func (p *Parser) varDecl() {
	if target := p.consume(TIdent, "expect variable name"); target != nil {
		global := p.identConst(target)
		defer p.defVar(global)
	} else {
		p.advance()
	}
	switch {
	case p.match(TEqual):
		p.expr()
	default:
		p.emitBytes(byte(OpNil))
	}
	p.consume(TSemi, "expect ';' after variable declaration")
}

func (p *Parser) decl() {
	switch {
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
		TLParen:       {(*Parser).grouping, nil, PrecNone},
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
		TFalse:        {(*Parser).lit, nil, PrecNone},
		TNil:          {(*Parser).lit, nil, PrecNone},
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

func (p *Parser) Compile(src string) (*Chunk, error) {
	res := NewChunk()
	p.compilingChunk = res
	defer func() { p.compilingChunk = nil }()

	p.Scanner = NewScanner(src)
	p.advance()

	for !p.match(TEOF) {
		p.decl()
	}

	p.endCompiler()
	return res, p.errors.ErrorOrNil()
}

func (p *Parser) currentChunk() *Chunk { return p.compilingChunk }

func (p *Parser) emitBytes(bs ...byte) {
	for _, b := range bs {
		p.currentChunk().Write(b, p.prev.Line)
	}
}

func (p *Parser) endCompiler() {
	p.emitBytes(byte(OpReturn))
	if debug.DEBUG {
		logrus.Debugln(p.currentChunk().Disassemble("endCompiler"))
	}
}

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
		}
	}
	p.advance()
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
		logrus.Debugln(p.currentChunk().Disassemble("ErrorAt"))
		logrus.Debugln(err)
	}

	p.errors = multierror.Append(p.errors, err)
}

func (p *Parser) Error(reason string)       { p.ErrorAt(p.prev, reason) }
func (p *Parser) ErrorAtCurr(reason string) { p.ErrorAt(p.curr, reason) }
func (p *Parser) HadError() bool            { return p.errors != nil }
