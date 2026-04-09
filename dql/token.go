package dql

import "fmt"

type (
	toktype int
	tokval  struct {
		Type  toktype
		Value string
		Pos   Pos
	}
	Pos struct {
		Byte   int
		Line   int
		Column int
	}
	tokvals []tokval
)

const (
	eofToken toktype = -1
	invalid  toktype = iota
	identToken
	stringToken
	numberToken
	lparenToken    // (
	rparenToken    // )
	lbrackToken    // [
	rbrackToken    // ]
	lbraceToken    // {
	rbraceToken    // }
	dotToken       // .
	colonToken     // :
	commaToken     // ,
	semicolonToken // ;
	equalToken     // =
)

func (t tokval) String() string {
	switch t.Type {
	case identToken, stringToken, numberToken:
		return fmt.Sprintf(`%s(%s)`, t.Type.String(), t.Value)
	default:
		return t.Type.String()
	}
}

func (tt toktype) String() string {
	switch tt {
	case eofToken:
		return "EOF"
	case identToken:
		return "Ident"
	case numberToken:
		return "Number"
	case lparenToken:
		return "LParen"
	case rparenToken:
		return "RParen"
	case lbrackToken:
		return "LBracket"
	case rbrackToken:
		return "RBracket"
	case lbraceToken:
		return "LBrace"
	case rbraceToken:
		return "RBrace"
	case dotToken:
		return "."
	case colonToken:
		return ":"
	case commaToken:
		return ","
	case semicolonToken:
		return ";"
	case equalToken:
		return "="
	case stringToken:
		return "String"
	default:
		panic("unreachable")
	}
}

func tokerr(t tokval, format string, args ...any) error {
	return Error{
		err:    fmt.Errorf(format, args...),
		Line:   t.Pos.Line,
		Column: t.Pos.Column,
	}
}
