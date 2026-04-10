package dql

import (
	"errors"
	"strconv"
)

// parser errors
var (
	ErrSyntax = errors.New("syntax error")
)

func Parse(in string) (Program, error) {
	l := newlexer(in)

	prog := Program{}

	for {
		tok, err := l.Peek()
		if err != nil {
			return Program{}, err
		}

		switch tok.Type {
		default:
			return Program{}, errUnexpectedToken(tok, "Keyword(AS)|Keyword(SEARCH)|Keyword(RETURN)")
		case eofToken:
			return prog, nil
		case keywordToken:
			switch tok.Value {
			case "AS", "SEARCH":
				stmt, err := parseStmt(l)
				if err != nil {
					return Program{}, err
				}
				prog.Stmts = append(prog.Stmts, stmt)
			case "RETURN":
				// parse return
				panic(1)
			default:
				return Program{}, errUnexpectedToken(tok, "Keyword(AS)|Keyword(SEARCH)|Keyword(RETURN)")
			}
		}
	}
}

func parseStmt(l *lexer) (Stmt, error) {
	stmt := Stmt{}
	tok, err := l.Next()
	if err != nil {
		return Stmt{}, err
	}
	if tok.Value == "AS" {
		nametok, err := l.Next()
		if err != nil {
			return Stmt{}, err
		}
		if nametok.Type != identToken {
			return Stmt{}, errUnexpectedToken(tok, "IDENT(varName)")
		}
		stmt.Name = nametok.Value
		tok, err = l.Next()
		if err != nil {
			return Stmt{}, err
		}
		if tok.Type != keywordToken || tok.Value != "SEARCH" {
			return Stmt{}, errUnexpectedToken(tok, "IDENT(SEARCH)")
		}
		// fall below
	}
	tok, err = l.Next()
	if err != nil {
		return Stmt{}, err
	}
	if tok.Type != identToken {
		return Stmt{}, errUnexpectedToken(tok, "IDENT(entityName)")
	}
	stmt.Entity = tok.Value

	tok, err = l.Peek()
	if err != nil {
		return Stmt{}, err
	}

	if tok.Type == semicolonToken {
		l.Eat(1)
		return stmt, nil
	}

	if tok.Type != keywordToken {
		stmt.Fields, err = parseExprList(l)
		if err != nil {
			return Stmt{}, err
		}
	}

	tok, err = l.Peek()
	if tok.Type == semicolonToken {
		l.Eat(1)
		return stmt, nil
	}

	if tok.Type == keywordToken {
		switch tok.Value {
		default:
			return Stmt{}, errUnexpectedToken(tok, "WHERE | ORDER BY | LIMIT | AGGS | WITH CURSOR | AFTER | ;")
		case "WHERE":
			l.Eat(1)
			stmt.Where, err = parseWhere(l)
			if err != nil {
				return Stmt{}, err
			}
		}
	}

	// TODO(i4k): rest

	tok, err = l.Next()
	if err != nil {
		return Stmt{}, err
	}
	if tok.Type != semicolonToken {
		return Stmt{}, errUnexpectedToken(tok, ";")
	}
	return stmt, nil
}

func parseExprList(l *lexer) (exprs []Expr, err error) {
	expr, err := parseExpr(l)
	if err != nil {
		return nil, err
	}
	exprs = append(exprs, expr)
	next, err := l.Peek()
	if err != nil {
		return nil, err
	}
	for next.Type == commaToken {
		l.Eat(1)
		expr, err = parseExpr(l)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
		next, err = l.Peek()
		if err != nil {
			return nil, err
		}
	}
	return exprs, nil
}

func parseExpr(l *lexer) (expr Expr, err error) {
	tok, err := l.Peek()
	if err != nil {
		return nil, err
	}
	switch tok.Type {
	default:
		return nil, errUnexpectedToken(tok, "IDENT(expr)|NUMBER|STRING|{|[")
	case numberToken:
		return parseNumberExpr(l)
	case stringToken:
		return parseStringExpr(l)
	case identToken:
		next, err := l.PeekNext()
		if err != nil {
			return nil, err
		}
		if next.Type == lparenToken {
			return parseFncallExpr(l)
		}
		l.Eat(1)
		// TODO(i4k): handle path traversals, indexing, etc
		return NewVarExpr(tok.Value), nil
	}
}

func parseFncallExpr(l *lexer) (fn FncallExpr, err error) {
	nametok, err := l.Next()
	if err != nil {
		return FncallExpr{}, err
	}
	fn.Name = nametok.Value
	l.Eat(1) // skip `(`
	fn.Args, err = parseExprList(l)
	if err != nil {
		return FncallExpr{}, err
	}
	next, err := l.Next()
	if err != nil {
		return FncallExpr{}, err
	}
	if next.Type != rparenToken {
		return FncallExpr{}, errUnexpectedToken(next, `")"`)
	}
	return fn, nil
}

func parseNumberExpr(l *lexer) (NumberExpr, error) {
	tok, err := l.Next()
	if err != nil {
		return NumberExpr{}, err
	}
	// TODO(i4k): parse number as float64
	val, err := strconv.Atoi(tok.Value)
	if err != nil {
		return NumberExpr{}, err
	}
	return NewNumberExpr(float64(val)), nil
}

func parseStringExpr(l *lexer) (StringExpr, error) {
	tok, err := l.Next()
	if err != nil {
		return StringExpr{}, err
	}
	return NewStringExpr(tok.Value), nil
}

func parseWhere(l *lexer) (where *Query, err error) {
	tok, err := l.Peek()
	if err != nil {
		return nil, err
	}
	if tok.Type == lbraceToken {
		return parseLegacyQuery(l)
	}
	if tok.Type != identToken {
		return nil, errUnexpectedToken(tok, "IDENT | {")
	}
	where = &Query{
		Type: OR,
	}

	left, err := parsePredicate(l)
	if err != nil {
		return nil, err
	}
	next, err := l.Peek()
	if err != nil {
		return nil, err
	}
	if next.Type != keywordToken {
		where.Children = append(where.Children, left)
		return where, nil
	}
	switch next.Value {
	default:
		where.Children = append(where.Children, left)
	case "AND":
		l.Next()
		qand := &Query{
			Type: AND,
		}
		right, err := parsePredicate(l)
		if err != nil {
			return nil, err
		}
		qand.Children = append(qand.Children, left, right)
		where.Children = append(where.Children, qand)
	case "OR":
		l.Next()
		right, err := parsePredicate(l)
		if err != nil {
			return nil, err
		}
		where.Children = append(where.Children, left, right)
	}
	return where, nil
}

func parsePredicate(l *lexer) (*Query, error) {
	tok, err := l.Next()
	if err != nil {
		return nil, err
	}
	if tok.Type != identToken {
		return nil, errUnexpectedToken(tok, "IDENT(field)")
	}
	op, err := l.Next()
	if err != nil {
		return nil, err
	}
	if op.Type != equalToken {
		return nil, errUnexpectedToken(op, "=")
	}
	predicate := Eq
	valexpr, err := parseExpr(l)
	if err != nil {
		return nil, err
	}
	return &Query{
		LHS: tok.Value,
		RHS: valexpr,
		OP:  predicate,
	}, nil
}

func parseLegacyQuery(_ *lexer) (query *Query, err error) {
	panic("not yet")
}

func errUnexpectedToken(tok tokval, expected string) error {
	return tokerr(tok, "%w: unexpected %s (expected %s)", ErrSyntax, tok, expected)
}
