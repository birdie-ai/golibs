package dql

import (
	"fmt"
	"strings"
)

var clauseMap = map[string]QueryNode{
	"$and": AND,
	"$or":  OR,
	"$not": NOT,
}

var binOpMap = map[string]Predicate{
	"$eq":    Eq,
	"$match": Match,
}

var unaryOpMap = map[string]Predicate{
	"$exists": Exists,
}

var opBoundMap = map[string]Predicate{
	"$gte": Gte,
	"$gt":  Gt,
	"$lte": Lte,
	"$lt":  Lt,
}

func parseLegacyQuery(l *lexer) (query *QueryExpr, err error) {
	l.Eat(1) // {
	query = &QueryExpr{
		Type: OR,
	}
	next, err := l.Next()
	if err != nil {
		return nil, err
	}
	switch next.Type {
	default:
		return nil, errUnexpectedToken(next, "`}` | STRING")
	case rbrackToken:
		return query, nil
	case stringToken:
		typ, ok := clauseMap[next.Value]
		if !ok {
			return nil, errUnexpectedToken(next, `"$and" | "$or" | "$not"`)
		}
		query.Type = typ
		next, err := l.Next()
		if err != nil {
			return nil, err
		}
		if next.Type != colonToken {
			return nil, errUnexpectedToken(next, `":"`)
		}
		next, err = l.Peek()
		if err != nil {
			return nil, err
		}
		if next.Type != lbrackToken {
			return nil, errUnexpectedToken(next, `"["`)
		}
		qlist, err := parseLegacyPredicateList(l)
		if err != nil {
			return nil, err
		}
		query.Children = qlist
		// TODO(i4k): check if legacy supports more than one logical operator in the same level.
		next, err = l.Next()
		if err != nil {
			return nil, err
		}
		if next.Type != rbraceToken {
			return nil, errUnexpectedToken(next, `"}"`)
		}
		return query, nil
	}
}

func parseLegacyPredicateList(l *lexer) (qlist []*QueryExpr, err error) {
	l.Eat(1) // [
	for {
		next, err := l.Peek()
		if err != nil {
			return nil, err
		}
		if next.Type != lbraceToken {
			return nil, errUnexpectedToken(next, `"{"`)
		}
		pred, err := parseLegacyPredicate(l)
		if err != nil {
			return nil, err
		}
		qlist = append(qlist, pred)
		next, err = l.Next()
		if err != nil {
			return nil, err
		}
		if next.Type == rbrackToken {
			return qlist, nil
		}
		if next.Type != commaToken {
			return nil, errUnexpectedToken(next, `"," | "]"`)
		}
	}
}

func parseLegacyPredicate(l *lexer) (*QueryExpr, error) {
	next, err := l.Peek()
	if err != nil {
		return nil, err
	}
	if next.Type != lbraceToken {
		return nil, errUnexpectedToken(next, `{`)
	}
	next, err = l.PeekNext()
	if err != nil {
		return nil, err
	}
	if next.Type != stringToken {
		return nil, errUnexpectedToken(next, `STRING`)
	}
	if _, ok := clauseMap[next.Value]; ok {
		return parseLegacyQuery(l)
	}
	l.Eat(2)

	lhs := next.Value
	q := &QueryExpr{
		LHS: strings.Split(lhs, "."),
	}

	next, err = l.Next()
	if err != nil {
		return nil, err
	}
	if next.Type != colonToken {
		return nil, errUnexpectedToken(next, `:`)
	}
	next, err = l.Peek()
	if err != nil {
		return nil, err
	}
	if op, ok := unaryOpMap[lhs]; ok {
		if next.Type != stringToken {
			return nil, errUnexpectedToken(next, `STRING`)
		}
		q.LHS = strings.Split(next.Value, ".")
		q.OP = op
		l.Eat(1)

		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		if tok.Type != rbraceToken {
			return nil, errUnexpectedToken(tok, `}`)
		}
		return q, nil
	} else if next.Type == lbraceToken {
		l.Eat(1)
		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		if tok.Type != stringToken {
			return nil, errUnexpectedToken(tok, `STRING`)
		}
		op1, ok := binOpMap[tok.Value]
		if !ok {
			op1, ok = opBoundMap[tok.Value]
			if !ok {
				return nil, errUnexpectedToken(tok, `$eq|$gte|$gt|$lte|$lt`)
			}
		}
		tok, err = l.Next()
		if err != nil {
			return nil, err
		}
		if tok.Type != colonToken {
			return nil, errUnexpectedToken(tok, `:`)
		}
		rhs1, err := parsePredicateRHS(l)
		if err != nil {
			return nil, err
		}
		tok, err = l.Next()
		if err != nil {
			return nil, err
		}
		if tok.Type == commaToken {
			tok, err = l.Next()
			if err != nil {
				return nil, err
			}
			if tok.Type != stringToken {
				return nil, errUnexpectedToken(tok, `STRING`)
			}
			op2, ok := opBoundMap[tok.Value]
			if !ok {
				return nil, errUnexpectedToken(tok, `$gte|$gt|$lte|$lt`)
			}
			tok, err = l.Next()
			if err != nil {
				return nil, err
			}
			if tok.Type != colonToken {
				return nil, errUnexpectedToken(tok, `:`)
			}
			rhs2, err := parsePredicateRHS(l)
			if err != nil {
				return nil, err
			}
			if !op1.IsRange() {
				return nil, fmt.Errorf("%w: %s used with range predicate", ErrSyntax, op1)
			}
			q.OP = Range
			setBound(q, op1, rhs1)
			setBound(q, op2, rhs2)
			tok, err = l.Next()
			if err != nil {
				return nil, err
			}
		} else {
			if op1.IsRange() {
				setBound(q, op1, rhs1)
			} else {
				q.OP = op1
				q.RHS = rhs1
			}
		}
		if tok.Type != rbraceToken {
			return nil, errUnexpectedToken(tok, "}")
		}
		tok, err = l.Next()
		if err != nil {
			return nil, err
		}
		if tok.Type != rbraceToken {
			return nil, errUnexpectedToken(tok, `"}"`)
		}
		return q, nil
	}
	q.RHS, err = parsePredicateRHS(l)
	if err != nil {
		return nil, err
	}
	q.OP = Eq
	tok, err := l.Next()
	if err != nil {
		return nil, err
	}
	if tok.Type != rbraceToken {
		return nil, errUnexpectedToken(tok, `}`)
	}
	return q, nil
}

func setBound(q *QueryExpr, op Predicate, val Expr) {
	q.OP = Range
	if op == Gte || op == Gt {
		q.Lower = Bound{
			Set: true,
			OP:  op,
			Val: val,
		}
	} else {
		q.Upper = Bound{
			Set: true,
			OP:  op,
			Val: val,
		}
	}
}

func parsePredicateRHS(l *lexer) (Expr, error) {
	tok, err := l.Peek()
	if err != nil {
		return nil, err
	}
	switch tok.Type {
	default:
		return nil, errUnexpectedToken(tok, `"[" | STRING | NUMBER | true | false`)
	case stringToken:
		return parseStringExpr(l)
	case numberToken:
		return parseNumberExpr(l)
	case keywordToken:
		if tok.Value != "true" && tok.Value != "false" {
			return nil, errUnexpectedToken(tok, `"{" | "[" | STRING | NUMBER | true | false`)
		}
		l.Eat(1)
		return NewBoolExpr(tok.Value == "true"), nil
	}
}
