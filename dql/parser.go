package dql

import (
	"errors"
)

// parser errors
var (
	ErrSyntax = errors.New("syntax error")
)

func Parse(in string) (Program, error) {
	l := newlexer(in)

	prog := Program{}

	for {
		tok, err := l.Next()
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
				stmt, err := parseStmt(l, tok)
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

func parseStmt(l *lexer, tok tokval) (Stmt, error) {
	stmt := Stmt{}
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
	tok, err := l.Next()
	if err != nil {
		return Stmt{}, err
	}
	if tok.Type != identToken {
		return Stmt{}, errUnexpectedToken(tok, "IDENT(entityName)")
	}
	stmt.Entity = tok.Value

	tok, err = l.Next()
	if err != nil {
		return Stmt{}, err
	}

	if tok.Type == semicolonToken {
		return stmt, nil
	}

	next := tok
	if next.Type != keywordToken {
		stmt.Fields, next, err = parseExprList(l, next)
		if err != nil {
			return Stmt{}, err
		}
	}

	if next.Type == semicolonToken {
		return stmt, nil
	}

	if next.Type == keywordToken {
		switch next.Value {
		default:
			return Stmt{}, errUnexpectedToken(next, "WHERE | ORDER BY | LIMIT | AGGS | WITH CURSOR | AFTER | ;")
		case "WHERE":
			stmt.Where, next, err = parseWhere(l, next)
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

func parseExprList(l *lexer, first tokval) (exprs []Expr, next tokval, err error) {
	expr, next, err := parseExpr(l, first)
	if err != nil {
		return nil, tokval{}, err
	}
	exprs = append(exprs, expr)
	for next.Type == commaToken {
		tok, err := l.Next()
		if err != nil {
			return nil, tokval{}, err
		}
		expr, next, err = parseExpr(l, tok)
		if err != nil {
			return nil, tokval{}, err
		}
		exprs = append(exprs, expr)
	}
	return exprs, next, nil
}

func parseExpr(l *lexer, first tokval) (expr Expr, next tokval, err error) {
	switch first.Type {
	default:
		return nil, tokval{}, errUnexpectedToken(first, "IDENT(expr)|NUMBER|STRING|{|[")
	case identToken:
		next, err = l.Next()
		if err != nil {
			return nil, tokval{}, err
		}
		if next.Type == lparenToken {
			return parseFncallExpr(l, first.Value)
		}
		// TODO(i4k): handle path traversals, indexing, etc
		return NewVarExpr(first.Value), next, nil
	}
}

func parseFncallExpr(l *lexer, name string) (fn FncallExpr, next tokval, err error) {
	fn.Name = name
	next, err = l.Next()
	if err != nil {
		return FncallExpr{}, tokval{}, err
	}
	fn.Args, next, err = parseExprList(l, next)
	if err != nil {
		return FncallExpr{}, tokval{}, err
	}
	if next.Type != rparenToken {
		return FncallExpr{}, tokval{}, errUnexpected(next, `")"`)
	}
	next, err = l.Next()
	if err != nil {
		return FncallExpr{}, tokval{}, err
	}
	return fn, next, nil
}

func parseWhere(l *lexer) (where Expr, next tokval, err error) {
	where = map[string]Expr{}
	next, err = l.Next()
	if err != nil {
		return nil, tokval{}, err
	}
	if next.Type == lbraceToken {
		return parseObjectExpr(l)
	}
	if next.Type != identToken {
		return nil, tokval{}, errUnexpectedToken(next, "IDENT | {")
	}
	var isFlatClauses bool
	var obj ObjectExpr
	for {
		ident := next
		next, err = l.Next()
		if err != nil {
			return nil, tokval{}, err
		}
		if !isFlatClauses && next.Type == lparenToken {
			return parseFncallExpr(l, ident.Value)
		}
		isFlatClauses = true
		if next.Type != equalToken {
			return nil, tokval{}, errUnexpectedToken(next, "EXPR")
		}
		next, err = l.Next()
		if err != nil {
			return nil, tokval{}, err
		}
	}
}

func errUnexpectedToken(tok tokval, expected string) error {
	return tokerr(tok, "%w: unexpected %s (expected %s)", ErrSyntax, tok, expected)
}
