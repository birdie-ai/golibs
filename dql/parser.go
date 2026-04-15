package dql

import (
	"errors"
	"slices"
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
		case "AGGS":
			l.Eat(1)
			stmt.Aggs, err = parseAggs(l)
			if err != nil {
				return Stmt{}, err
			}
		case "LIMIT":
			l.Eat(1)
			tok, err := l.Next()
			if err != nil {
				return Stmt{}, err
			}
			if tok.Type != numberToken {
				return Stmt{}, errUnexpectedToken(tok, `NUMBER`)
			}
			stmt.Limit, err = strconv.Atoi(tok.Value)
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
	next, err := l.Peek()
	if err != nil {
		return nil, err
	}
	exprs = append(exprs, expr)
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
		expr, err = parseNumberExpr(l)
	case stringToken:
		expr, err = parseStringExpr(l)
	case identToken:
		next, err := l.PeekNext()
		if err != nil {
			return nil, err
		}
		if next.Type == lparenToken {
			expr, err = parseFncallExpr(l)
		} else {
			l.Eat(1)
			// TODO(i4k): handle path traversals, indexing, etc
			expr = NewVarExpr(tok.Value)
		}
	}
	if err != nil {
		return nil, err
	}

	// we have to check if the expr is succeded by DOT because if so it's a PathExpr.
	next, err := l.Peek()
	if err != nil {
		return nil, err
	}
	if next.Type != dotToken {
		return expr, nil
	}

	// PathExpr
	return parsePathExpr(l, expr)
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

func parsePathExpr(l *lexer, base Expr) (path PathExpr, err error) {
	path.Base = base

	for {
		next, err := l.Peek()
		if err != nil {
			return PathExpr{}, err
		}
		// in the first iteration it's always DOT
		if next.Type != dotToken {
			break
		}
		l.Eat(1)
		next, err = l.Peek()
		if err != nil {
			return PathExpr{}, err
		}
		switch next.Type {
		default:
			return PathExpr{}, errUnexpectedToken(next, "IDENT(field) | `[`")
		case identToken:
			l.Eat(1)
			path.Steps = append(path.Steps, NewFieldStep(next.Value))
		case lbrackToken:
			l.Eat(1)
			expr, err := parseExpr(l)
			if err != nil {
				return PathExpr{}, err
			}
			path.Steps = append(path.Steps, NewIndexStep(expr))
			next, err := l.Next()
			if err != nil {
				return PathExpr{}, err
			}
			if next.Type != rbrackToken {
				return PathExpr{}, errUnexpectedToken(next, "`]`")
			}
		}
	}
	return path, nil
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

func parseWhere(l *lexer) (where *QueryExpr, err error) {
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
	left, err := parsePredicate(l)
	if err != nil {
		return nil, err
	}
	next, err := l.Peek()
	if err != nil {
		return nil, err
	}
	if next.Type != keywordToken || (next.Value != "AND" && next.Value != "OR") {
		return left, nil
	}
	l.Next()
	right, err := parsePredicate(l)
	if err != nil {
		return nil, err
	}
	var logicalop QueryNode
	if next.Value == "AND" {
		logicalop = AND
	} else {
		logicalop = OR
	}
	return &QueryExpr{
		Type:     logicalop,
		Children: []*QueryExpr{left, right},
	}, nil
}

func parsePredicate(l *lexer) (*QueryExpr, error) {
	tok, err := l.Next()
	if err != nil {
		return nil, err
	}
	if tok.Type != identToken {
		return nil, errUnexpectedToken(tok, "IDENT(field)")
	}
	lhs := StaticPath{tok.Value}
	for {
		next, err := l.Peek()
		if err != nil {
			return nil, err
		}
		if next.Type != dotToken {
			break
		}
		l.Eat(1)
		next, err = l.Peek()
		if err != nil {
			return nil, err
		}
		if next.Type != identToken {
			return nil, errUnexpectedToken(next, "IDENT(path)")
		}
		lhs = append(lhs, next.Value)
		l.Eat(1)
	}
	op, err := l.Next()
	if err != nil {
		return nil, err
	}
	if op.Type != equalToken {
		return nil, errUnexpectedToken(op, `"="`)
	}
	predicate := Eq
	valexpr, err := parseExpr(l)
	if err != nil {
		return nil, err
	}
	return &QueryExpr{
		LHS: lhs,
		RHS: valexpr,
		OP:  predicate,
	}, nil
}

func anyof(typ toktype, targets ...toktype) bool {
	return slices.Contains(targets, typ)
}

func errUnexpectedToken(tok tokval, expected string) error {
	return tokerr(tok, "%w: unexpected %s (expected %s)", ErrSyntax, tok, expected)
}
