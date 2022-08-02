package vm

import (
	"fmt"

	e "github.com/rami3l/golox/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Compiler struct {
	scanner *Scanner
}

func NewCompiler() *Compiler { return &Compiler{} }

func (c *Compiler) Compile(src string) (*Chunk, error) {
	c.scanner = NewScanner(src)

	// TODO: IMPLEMENT THIS METHOD
	for line := -1; ; {
		token := c.scanner.ScanToken()

		header := "   | "
		if token.Line != line {
			header = fmt.Sprintf("%4d ", token.Line)
			line = token.Line
		}
		logrus.Debugf(
			"%s%8s '%s'", header, token.Type,
			string(c.scanner.src[token.Start:token.Start+token.Len]),
		)
		if token.Type == TEOF {
			break
		}
	}

	// TODO: FIX ME!
	return nil, &e.CompilationError{Reason: "Unimplemented"}
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
	case 'c':
		return checkKeyword(1, "lass", TClass)
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
				return checkKeyword(2, "r", TFun)
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
		Start: s.start,
		Len:   s.curr - s.start,
		Line:  s.line,
	}
}

func (s *Scanner) errorToken(reason string) (res Token) {
	res = s.makeToken(TErr)
	res.Error = &reason
	return
}

func (s *Scanner) isAtEnd() bool { return s.curr >= len(s.src) }

func isAlpha(c rune) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'z') || c == '_' }
func isDigit(c rune) bool { return c >= '0' && c <= '9' }
