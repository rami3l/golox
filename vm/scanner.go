package vm

import e "github.com/rami3l/golox/errors"

type Scanner struct {
	start, curr, line int
	src               []rune
}

func NewScanner(src string) *Scanner {
	return &Scanner{src: []rune(src), line: 1}
}

func (s *Scanner) ScanToken() Token {
	s.skipWhitespace()
	s.start = s.curr
	if s.isAtEnd() {
		return s.makeToken(TEOF)
	}

	c := s.advance()
	switch {
	case isDigit(c): // Number literal.
		// Consume the integral part.
		for isDigit(s.peek()) {
			s.advance()
		}

		// Consume the fractional part if it exists.
		if s.peek() == '.' && isDigit(s.peekNext()) {
			s.advance()
			for isDigit(s.peek()) {
				s.advance()
			}
		}

		return s.makeToken(TNum)

	case isAlpha(c): // Identifier.
		for c1 := c; isAlpha(c1) || isDigit(c1); c1 = s.peek() {
			s.advance()
		}
		return s.makeToken(s.identType())
	}

	switch c {
	case '(':
		return s.makeToken(TLParen)
	case ')':
		return s.makeToken(TRParen)
	case '{':
		return s.makeToken(TLBrace)
	case '}':
		return s.makeToken(TRBrace)
	case ';':
		return s.makeToken(TSemi)
	case ',':
		return s.makeToken(TComma)
	case '.':
		return s.makeToken(TDot)
	case '-':
		return s.makeToken(TMinus)
	case '+':
		return s.makeToken(TPlus)
	case '/':
		return s.makeToken(TSlash)
	case '*':
		return s.makeToken(TStar)

	case '!':
		if s.match('=') {
			return s.makeToken(TBangEqual)
		}
		return s.makeToken(TBang)

	case '=':
		if s.match('=') {
			return s.makeToken(TEqualEqual)
		}
		return s.makeToken(TEqual)

	case '<':
		if s.match('=') {
			return s.makeToken(TLessEqual)
		}
		return s.makeToken(TLess)

	case '>':
		if s.match('=') {
			return s.makeToken(TGreaterEqual)
		}
		return s.makeToken(TGreater)

	case '"': // String literal.
		for {
			switch s.peek() {
			case '\n':
				s.line++
			case '"':
				// Consume the closing quote.
				s.advance()
				return s.makeToken(TStr)
			default:
				if s.isAtEnd() {
					return s.errorToken("unterminated string")
				}
				s.advance()
			}
		}
	}

	return s.errorToken("unexpected character")
}

// skipWhitespace makes the Scanner skip consecutive whitespaces and comments.
func (s *Scanner) skipWhitespace() {
	for {
		switch s.peek() {
		case '\n':
			s.line++
			fallthrough

		case ' ', '\r', '\t':
			s.advance()

		case '/': // Skip comments.
			if s.peekNext() != '/' {
				return
			}
			// Skip until the end of the line.
			for s.peek() != '\n' && !s.isAtEnd() {
				s.advance()
			}

		default:
			return
		}
	}
}

func (s *Scanner) advance() (res rune) {
	res = s.src[s.curr]
	s.curr++
	return
}

func (s *Scanner) peek() (res rune) {
	if s.isAtEnd() {
		return
	}
	return s.src[s.curr]
}

func (s *Scanner) peekNext() (res rune) {
	if s.isAtEnd() || s.curr+1 >= len(s.src) {
		return
	}
	return s.src[s.curr+1]
}

func (s *Scanner) match(expected rune) bool {
	if c := s.peek(); c == 0 /* isAtEnd */ || c != expected {
		return false
	}
	s.curr++
	return true
}

func (s *Scanner) Error(reason string) *e.CompilationError {
	return &e.CompilationError{Line: s.line, Reason: reason}
}

type Token struct {
	Type             TokenType
	Start, Len, Line int

	// Error message for TErr.
	Error *string
}

//go:generate stringer -type=TokenType
type TokenType int

const (
	TLParen TokenType = iota
	TRParen
	TLBrace
	TRBrace
	TComma
	TDot
	TMinus
	TPlus
	TSemi
	TSlash
	TStar
	TBang
	TBangEqual
	TEqual
	TEqualEqual
	TGreater
	TGreaterEqual
	TLess
	TLessEqual
	TIdent
	TStr
	TNum
	TAnd
	TBreak
	TClass
	TContinue
	TElse
	TFalse
	TFor
	TFun
	TIf
	TNil
	TOr
	TPrint
	TReturn
	TSuper
	TThis
	TTrue
	TVar
	TWhile
	TErr
	TEOF
)
