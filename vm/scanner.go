package vm

import (
	e "github.com/rami3l/golox/errors"
	"golang.org/x/exp/slices"
)

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
		for p := s.peek(); isAlpha(p) || isDigit(p); p = s.peek() {
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

func (s *Scanner) identType() TokenType {
	checkKeyword := func(
		start int, rest string, ty TokenType,
	) TokenType {
		absStart := s.start + start
		if s.curr >= absStart && slices.Equal(s.src[absStart:s.curr], []rune(rest)) {
			return ty
		}
		return TIdent
	}

	switch s.src[s.start] {
	case 'a':
		return checkKeyword(1, "nd", TAnd)
	case 'b':
		return checkKeyword(1, "reak", TBreak)
	case 'c':
		if s.curr-s.start > 1 {
			switch s.src[s.start+1] {
			case 'l':
				return checkKeyword(2, "ass", TClass)
			case 'o':
				return checkKeyword(2, "ntinue", TContinue)
			}
		}
	case 'e':
		return checkKeyword(1, "lse", TElse)
	case 'f':
		if s.curr-s.start > 1 {
			switch s.src[s.start+1] {
			case 'a':
				return checkKeyword(2, "lse", TFalse)
			case 'o':
				return checkKeyword(2, "r", TFor)
			case 'u':
				return checkKeyword(2, "n", TFun)
			}
		}
	case 'i':
		return checkKeyword(1, "f", TIf)
	case 'n':
		return checkKeyword(1, "il", TNil)
	case 'o':
		return checkKeyword(1, "r", TOr)
	case 'p':
		return checkKeyword(1, "rint", TPrint)
	case 'r':
		return checkKeyword(1, "eturn", TReturn)
	case 's':
		return checkKeyword(1, "uper", TSuper)
	case 't':
		if s.curr-s.start > 1 {
			switch s.src[s.start+1] {
			case 'h':
				return checkKeyword(2, "is", TThis)
			case 'r':
				return checkKeyword(2, "ue", TTrue)
			}
		}
	case 'v':
		return checkKeyword(1, "ar", TVar)
	case 'w':
		return checkKeyword(1, "hile", TWhile)
	}
	return TIdent
}

func (s *Scanner) makeToken(ty TokenType) Token {
	return Token{
		Type:  ty,
		Line:  s.line,
		Runes: s.src[s.start:s.curr],
	}
}

func (s *Scanner) errorToken(reason string) (res Token) {
	res = s.makeToken(TErr)
	res.Runes = []rune(reason)
	return
}

func (s *Scanner) isAtEnd() bool { return s.curr >= len(s.src) }

func isAlpha(c rune) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' }
func isDigit(c rune) bool { return c >= '0' && c <= '9' }

type Token struct {
	Type TokenType
	Line int
	// The corresponding lexeme of this token, or the error message if Type is TErr.
	Runes []rune
}

func (t Token) String() string  { return string(t.Runes) }
func (t Token) Eq(u Token) bool { return t.Type == u.Type && slices.Equal(t.Runes, u.Runes) }

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
