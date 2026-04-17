package dql

func parseAggs(l *lexer) (Aggs, error) {
	tok, err := l.Peek()
	if err != nil {
		return nil, err
	}
	if tok.Type == keywordToken && tok.Value == "JSON" {
		return parseAggsJSON(l)
	}
	if tok.Type != lbraceToken {
		return nil, errUnexpectedToken(tok, `"{"`)
	}

	l.Eat(1)
	aggs := Aggs{}

	for {
		tok, err := l.Peek()
		if err != nil {
			return nil, err
		}
		if tok.Type == rbraceToken {
			break
		}
		agg, err := parseAgg(l)
		if err != nil {
			return nil, err
		}
		aggs[agg.Name] = agg

		tok, err = l.Peek()
		if err != nil {
			return nil, err
		}
		if tok.Type != commaToken {
			break
		}
		l.Eat(1)
	}

	tok, err = l.Next()
	if err != nil {
		return nil, err
	}
	if tok.Type != rbraceToken {
		return nil, errUnexpectedToken(tok, `"}"`)
	}
	return aggs, nil
}

func parseAgg(l *lexer) (Agg, error) {
	tok, err := l.Next()
	if err != nil {
		return Agg{}, err
	}
	if !anyof(tok.Type, identToken, stringToken) {
		return Agg{}, errUnexpectedToken(tok, `STRING | IDENT`)
	}
	name := tok.Value

	tok, err = l.Next()
	if err != nil {
		return Agg{}, err
	}
	if tok.Type != colonToken {
		return Agg{}, errUnexpectedToken(tok, `":"`)
	}
	tok, err = l.Peek()
	if err != nil {
		return Agg{}, err
	}
	if tok.Type != identToken {
		return Agg{}, errUnexpectedToken(tok, `IDENT`)
	}
	// TODO(i4k): parseFncallExpr() still expects lookakead=2
	next, err := l.PeekNext()
	if err != nil {
		return Agg{}, err
	}
	if next.Type != lparenToken {
		return Agg{}, errUnexpectedToken(tok, `"("`)
	}
	fn, err := parseFncallExpr(l)
	if err != nil {
		return Agg{}, err
	}
	agg := Agg{
		Name: name,
		Func: fn,
	}
	tok, err = l.Peek()
	if err != nil {
		return Agg{}, err
	}
	if tok.Type == lbraceToken {
		// nested agg
		aggs, err := parseAggs(l)
		if err != nil {
			return Agg{}, err
		}
		agg.Children = aggs
	}
	return agg, nil
}

func parseAggsJSON(l *lexer) (Aggs, error) {
	panic("not yet")
}
