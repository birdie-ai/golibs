package dql

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

type lexer struct {
	in   []byte
	pos  int
	line int // start at zero
	col  int
}

// lexer errors
var (
	errUnfinishedEscape = errors.New(`unfinished escape (\)`)
	errUnknownEscapeSeq = errors.New(`unknown escape sequence`)
)

func newlexer(in string) *lexer {
	return &lexer{
		in:  []byte(in),
		col: 1,
	}
}

func (l *lexer) Next() (tokval, error) {
	l.skipblank()
	curpos := Pos{Byte: l.pos, Line: l.line, Column: l.col}
	r := l.next()
	if r == eof {
		return l.newtok(eofToken, ""), nil
	}
	switch r {
	case '(':
		return newtokat(lparenToken, "(", curpos), nil
	case ')':
		return newtokat(rparenToken, ")", curpos), nil
	case '[':
		return newtokat(lbrackToken, "[", curpos), nil
	case ']':
		return newtokat(rbrackToken, "]", curpos), nil
	case '{':
		return newtokat(lbraceToken, "{", curpos), nil
	case '}':
		return newtokat(rbraceToken, "}", curpos), nil
	case '.':
		return newtokat(dotToken, ".", curpos), nil
	case ':':
		return newtokat(colonToken, ":", curpos), nil
	case ',':
		return newtokat(commaToken, ",", curpos), nil
	case ';':
		return newtokat(semicolonToken, ";", curpos), nil
	case '=':
		return newtokat(equalToken, "=", curpos), nil
	case '"':
		return l.lexString(curpos)
	default:
		if unicode.IsLetter(r) {
			return l.lexIdent(r, curpos), nil
		}
		if unicode.IsDigit(r) {
			return l.lexNumber(r, curpos), nil
		}
		panic(fmt.Sprintf("%c", r))
	}
}

func (l *lexer) lexIdent(r rune, pos Pos) tokval {
	// keywords are ident but lexically different!
	ident := make([]rune, 1, 6)
	ident[0] = r
	for {
		r, width := l.peek()
		if r == eof {
			break
		}
		if !unicode.IsLetter(r) && r != '_' && !unicode.IsDigit(r) {
			break
		}
		l.eat(r, width)
		ident = append(ident, r)
	}
	str := string(ident)
	if _, ok := keywords[str]; ok {
		return newtokat(keywordToken, str, pos)
	}
	return newtokat(identToken, str, pos)
}

// TODO(i4k): handle float. Maybe there should be benefits if we distinguish integer and float
// at the lexer level... still thinking about this.
func (l *lexer) lexNumber(r rune, pos Pos) tokval {
	number := make([]rune, 1, 10)
	number[0] = r
	for {
		r, width := l.peek()
		if r == eof {
			break
		}
		if !unicode.IsDigit(r) {
			break
		}
		l.eat(r, width)
		number = append(number, r)
	}
	return newtokat(numberToken, string(number), pos)
}

func (l *lexer) lexString(pos Pos) (tokval, error) {
	str := make([]rune, 0, 64)
	var escaped bool
	for {
		r, width := l.peek()
		if r == eof {
			break
		}
		if !escaped && r == '"' {
			l.eat(r, width)
			break
		}
		if escaped {
			switch r {
			case '\\':
			default:
				return tokval{}, Error{
					err:    fmt.Errorf("%w: \\%c", errUnknownEscapeSeq, r),
					Line:   l.line,
					Column: l.col,
				}
			}
		}
		escaped = !escaped && r == '\\'
		if escaped {
			l.eat(r, width)
			continue
		}
		l.eat(r, width)
		str = append(str, r)
	}
	if escaped {
		return tokval{}, Error{
			err:    errUnfinishedEscape,
			Line:   l.line,
			Column: l.col,
		}
	}
	return tokval{
		Type:  stringToken,
		Value: string(str),
		Pos:   pos,
	}, nil
}

func (l *lexer) peek() (rune, int) {
	if l.pos >= len(l.in) {
		return eof, 0
	}
	r, width := utf8.DecodeRune(l.in[l.pos:])
	return r, width
}

func (l *lexer) next() (r rune) {
	r, width := l.peek()
	if r == eof {
		return r
	}
	l.eat(r, width)
	return r
}

func (l *lexer) skipblank() {
	for {
		r, width := l.peek()
		if r == eof {
			return
		}
		if !unicode.IsSpace(r) {
			return
		}
		l.eat(r, width)
	}
}

func (l *lexer) eat(r rune, width int) {
	l.pos += width
	if r == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *lexer) newtok(t toktype, val string) tokval {
	return newtokat(t, val, Pos{Byte: l.pos, Line: l.line, Column: l.col})
}

func newtokat(t toktype, val string, pos Pos) tokval {
	return tokval{
		Type:  t,
		Value: val,
		Pos:   pos,
	}
}

const eof = -1
