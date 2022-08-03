package vm

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-multierror"
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

func (p *Parser) emitReturn() { p.emitBytes(byte(OpReturn)) }

func (p *Parser) emitConst(val Value) { p.emitBytes(byte(OpConst), p.makeConst(val)) }

func (p *Parser) makeConst(val Value) byte {
	const_ := p.currentChunk().AddConst(val)
	if const_ > math.MaxUint8 {
		logrus.Panicln("too many consts in one chunk")
	}
	return byte(const_)
}

func (p *Parser) number() {
	val, err := strconv.ParseFloat(string(p.prev.Runes), 64)
	p.errors = multierror.Append(p.errors, err)
	p.emitConst(Value(val))
}

func (p *Parser) grouping() {
	p.expression()
	p.consume(TRParen, "expect ')' after expression")
}

func (p *Parser) expression() { p.parsePrec(PrecAssign) }

func (p *Parser) unary() {
	op := p.prev.Type

	// Compile the RHS.
	p.parsePrec(PrecUnary)

	// Emit the operator instruction.
	switch op {
	case TMinus:
		p.emitBytes(byte(OpNeg))
	default:
		panic(e.Unreachable)
	}
}

func (p *Parser) binary() {
	op := p.prev.Type
	rule := parseRules[op]

	// Compile the RHS.
	p.parsePrec(rule.Prec + 1)

	// Emit the operator instruction.
	switch op {
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

type ParseFn = func(*Parser)

type ParseRule struct {
	Prefix, Infix ParseFn
	Prec
}

var parseRules []ParseRule

func init() {
	parseRules = []ParseRule{
		TLParen: {(*Parser).grouping, nil, PrecNone},
		TMinus:  {(*Parser).unary, (*Parser).binary, PrecTerm},
		TPlus:   {nil, (*Parser).binary, PrecTerm},
		TSlash:  {nil, (*Parser).binary, PrecFactor},
		TStar:   {nil, (*Parser).binary, PrecFactor},
		TNum:    {(*Parser).number, nil, PrecNone},
		TEOF:    {},
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
	prefix(p)

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
		rule.Infix(p)
	}
}

/* Parsing helpers */

func (p *Parser) advance() {
	p.prev = p.curr
	for {
		// Skip until the first non-TErr token.
		if p.curr = p.ScanToken(); p.curr.Type != TErr {
			break
		}
		p.Error(string(p.curr.Runes))
	}
}

func (p *Parser) consume(ty TokenType, errorMsg string) {
	if p.curr.Type != ty {
		p.ErrorAtCurr(errorMsg)
		return
	}
	p.advance()
}

/* Compiling helpers */

func (p *Parser) Compile(src string) (*Chunk, error) {
	res := NewChunk()
	p.compilingChunk = res
	defer func() { p.compilingChunk = nil }()

	p.Scanner = NewScanner(src)
	p.advance()

	p.expression()
	p.consume(TEOF, "expect end of expression")

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
	p.emitReturn()
	logrus.Debugln(p.currentChunk().Disassemble("code"))
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
		tkStr = fmt.Sprintf("identifier `%s`", string(tk.Runes))
	default:
		tkStr = fmt.Sprintf("`%s`", string(tk.Runes))
	}
	reason1 := fmt.Sprintf("at %s, %s", tkStr, reason)
	err := &e.CompilationError{Line: tk.Line, Reason: reason1}
	p.errors = multierror.Append(p.errors, err)
}

func (p *Parser) Error(reason string)       { p.ErrorAt(p.prev, reason) }
func (p *Parser) ErrorAtCurr(reason string) { p.ErrorAt(p.curr, reason) }
func (p *Parser) HadError() bool            { return p.errors != nil }
