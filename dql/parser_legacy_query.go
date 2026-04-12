package dql

import "strings"

var clauseMap = map[string]QueryNode{
	"$and": AND,
	"$or":  OR,
	"$not": NOT,
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
		return nil, err
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
	q := &QueryExpr{
		LHS: Path(strings.Split(next.Value, ".")...),
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
	switch next.Type {
	default:
		return nil, errUnexpectedToken(next, `"{" | "[" | STRING | NUMBER | true | false`)
	case stringToken:
		val, err := parseStringExpr(l)
		if err != nil {
			return nil, err
		}
		q.RHS = val
		q.OP = Eq
	case numberToken:
		val, err := parseNumberExpr(l)
		if err != nil {
			return nil, err
		}
		q.RHS = val
		q.OP = Eq
	case keywordToken:
		if next.Value != "true" && next.Value != "false" {
			return nil, errUnexpectedToken(next, `"{" | "[" | STRING | NUMBER | true | false`)
		}
		q.RHS = NewBoolExpr(next.Value == "true")
		q.OP = Eq
	}
	next, err = l.Next()
	if err != nil {
		return nil, err
	}
	if next.Type != rbraceToken {
		return nil, errUnexpectedToken(next, `"}"`)
	}
	return q, nil
}
