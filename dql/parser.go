package dql

import "errors"

// parser errors
var (
	ErrSyntax = errors.New("syntax error")
)

func Parse(in string) (Stmts, Return, error) {
	l := newlexer(in)

	tok, err := l.Next()
	if err != nil {
		return nil, Return{}, err
	}

	_ = tok

	return Stmts{}, Return{}, nil
}
