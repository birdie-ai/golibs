package dml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"unique"
)

// parser errors.
var (
	ErrSyntax = errors.New("syntax error")
)

// Parse the textual input and return a list of statements.
func Parse(in []byte) (Stmts, error) {
	var stmts Stmts
	for {
		// NOTE(i4k): len(rest) > 0 *if and only if* there's non-blank data
		// still to be processed.
		stmt, rest, err := parseStmt(in)
		if err == errEOF {
			break
		}
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
		if len(rest) == 0 {
			break
		}
		in = rest
	}
	return stmts, nil
}

func parseStmt(in []byte) (Stmt, []byte, error) {
	in = skipblank(in)
	if len(in) == 0 {
		return Stmt{}, nil, errEOF
	}
	ident, in, err := lexIdent(in)
	if err != nil {
		return Stmt{}, nil, err
	}
	var stmt Stmt
	switch strings.ToLower(ident) {
	case "set":
		stmt.Op = SET
	case "delete":
		stmt.Op = DELETE
	default:
		return Stmt{}, nil, fmt.Errorf("%w: %s", ErrInvalidOperation, ident)
	}

	in = skipblank(in)
	if len(in) == 0 {
		return Stmt{}, nil, errUnexpectedEOF()
	}
	entity, in, err := lexIdent(in)
	if err != nil {
		return Stmt{}, nil, err
	}
	stmt.Entity = unique.Make(entity)

	in = skipblank(in)
	if len(in) == 0 {
		return Stmt{}, nil, errUnexpectedEOF()
	}

	if stmt.Op == SET {
		stmt.Assign = Assign{}
		for len(in) > 0 {
			var (
				key string
				val any
				err error
			)
			key, val, in, err = parseAssign(in)
			if err != nil {
				return Stmt{}, nil, err
			}

			stmt.Assign[key] = val
			in = skipblank(in)
			if len(in) == 0 {
				return Stmt{}, nil, errUnexpectedEOF()
			}
			if in[0] == ',' {
				// only one "." assign
				if _, ok := stmt.Assign["."]; ok {
					return Stmt{}, nil, fmt.Errorf("%w: only one '.' assignment is permitted. Unexpected ','", ErrSyntax)
				}
				in = skipblank(in[1:])
				if len(in) == 0 {
					return Stmt{}, nil, errUnexpectedEOF()
				}
				continue
			}
			break
		}
	}
	in = skipblank(in)
	if len(in) == 0 {
		return Stmt{}, nil, errUnexpectedEOF()
	}
	ident, in, err = lexIdent(in)
	if err != nil {
		return Stmt{}, nil, err
	}
	if !strings.EqualFold(ident, "WHERE") {
		return Stmt{}, nil, fmt.Errorf("%w: expected WHERE token", ErrSyntax)
	}
	in = skipblank(in)
	if len(in) == 0 {
		return Stmt{}, nil, errUnexpectedEOF()
	}
	stmt.Where, in, err = parseWhere(in)
	if err != nil {
		return Stmt{}, nil, err
	}
	in = skipblank(in)
	if len(in) == 0 {
		return Stmt{}, nil, errUnexpectedEOF()
	}
	if in[0] != ';' {
		return Stmt{}, nil, ErrSyntax
	}
	in = in[1:]
	return stmt, in, nil
}

func parseAssign(in []byte) (string, any, []byte, error) {
	if in[0] == '.' {
		in = in[1:]
		in = skipblank(in)
		if len(in) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		if in[0] != '=' {
			return "", nil, nil, fmt.Errorf("%w: expected '=' token", ErrSyntax)
		}
		in = in[1:]
		in = skipblank(in)
		if len(in) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		var (
			val map[string]any
			err error
		)
		in, err = parseJSON(in, &val)
		if err != nil {
			return "", nil, nil, fmt.Errorf("%w: failed to parse value as JSON object: %v", ErrSyntax, err)
		}
		for _, k := range slices.Sorted(maps.Keys(val)) {
			if !isIdent(k) {
				return "", nil, nil, fmt.Errorf("%w: expect root assign (.) to an object "+
					"with identifier keys but found %q", ErrSyntax, k)
			}
		}
		return ".", val, in, nil
	}
	var (
		dotident []byte
		ident    string
		err      error
	)

	ident, in, err = lexIdent(in)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: %v", ErrSyntax, err)
	}

	dotident = append(dotident, []byte(ident)...)
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}

	for len(in) > 0 && in[0] == '.' {
		dotident = append(dotident, '.')
		in = skipblank(in[1:])
		if len(in) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		if in[0] == '"' {
			// parse the string as a JSON string.
			// This means we support all of its escape sequences!
			dec := json.NewDecoder(bytes.NewReader(in))
			tok, err := dec.Token()
			if err != nil {
				return "", nil, nil, fmt.Errorf("%w: parsing quote string literal: %v", ErrSyntax, err)
			}
			str, ok := tok.(string)
			if !ok {
				return "", nil, nil, fmt.Errorf("%w: unexpected %v", ErrSyntax, tok)
			}
			dotident = append(dotident, []byte(strconv.Quote(str))...)
			in = skipblank(in[dec.InputOffset():])
			continue
		}
		ident, in, err = lexIdent(in)
		if err != nil {
			return "", nil, nil, fmt.Errorf("%w: %v", ErrSyntax, err)
		}
		if len(in) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		dotident = append(dotident, []byte(ident)...)
	}
	in = skipblank(in)
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] != '=' {
		return "", nil, nil, fmt.Errorf("%w: expected '='", ErrSyntax)
	}
	in = skipblank(in[1:])
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] == '.' {
		if err := lexdotdotdot(in); err != nil {
			return "", nil, nil, err
		}
		in = skipblank(in[3:])
		if len(in) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		if in[0] != '[' {
			return "", nil, nil, fmt.Errorf("%w: dotdotdot (...) requires a subsequent array: %s", ErrSyntax, in)
		}
		var val any
		in, err = parseJSON(in, &val)
		if err != nil {
			return "", nil, nil, fmt.Errorf("%w: failed to parse array value: %v", ErrSyntax, err)
		}
		opval, err := appendval(val)
		if err != nil {
			return "", nil, nil, err
		}
		return string(dotident), opval, in, nil
	}
	isarray := in[0] == '['
	var val any
	in, err = parseJSON(in, &val)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: failed to parse value as JSON: %v", ErrSyntax, err)
	}
	if isarray {
		in = skipblank(in)
		if len(in) > 0 && in[0] == '.' {
			if err := lexdotdotdot(in); err != nil {
				return "", nil, nil, err
			}
			in = in[3:]
			opval, err := prependval(val)
			if err != nil {
				return "", nil, nil, err
			}
			return string(dotident), opval, in, nil
		}
	}
	return string(dotident), val, in, nil
}

func parseWhere(in []byte) (Where, []byte, error) {
	if in[0] == '{' {
		var (
			where Where
			err   error
		)
		in, err = parseJSON(in, &where)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: failed to parse value as JSON Object: %v", ErrSyntax, err)
		}
		if len(where) == 0 {
			return nil, nil, fmt.Errorf("%w: WHERE object require key-value entries", ErrSyntax)
		}
		for _, k := range slices.Sorted(maps.Keys(where)) {
			if !isIdent(k) {
				return nil, nil, fmt.Errorf("%w: WHERE object keys need to be valid identifier but found %q", ErrSyntax, k)
			}
		}
		return where, in, nil
	}
	var err error
	var ident string
	ident, in, err = lexIdent(in)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrSyntax, err)
	}
	in = skipblank(in)
	if len(in) == 0 {
		return nil, nil, errUnexpectedEOF()
	}
	if in[0] != '=' {
		return nil, nil, fmt.Errorf("%w: invalid where: unexpected char %c", ErrSyntax, in[0])
	}
	in = in[1:]
	var val any
	in, err = parseJSON(in, &val)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: parsing value as JSON: %v", ErrSyntax, err)
	}
	return Where{
		ident: val,
	}, in, nil
}

func parseJSON[T any](in []byte, val *T) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(in))
	err := dec.Decode(val)
	if err != nil {
		return nil, fmt.Errorf("parsing JSON: %v", err)
	}
	return in[dec.InputOffset():], nil
}

type kind int

const (
	tany kind = iota
	tstr
	tfloat
	tbool
)

type arrayvalues struct {
	kind  kind
	avals []any
	bvals []bool
	fvals []float64
	svals []string
}

func arrayvals(val any) (arrayvalues, error) {
	anyvals, ok := val.([]any)
	if !ok {
		// parser must ensure: dotdotdot LBracket
		panic("unreachable")
	}
	var array arrayvalues
	var length int
	kinds := map[string]struct{}{}
	for _, v := range anyvals {
		if v != nil {
			// nulls are ignored because there's no use case for that and it complicates
			// implementation in the target storage system.
			length++
			array.avals = append(array.avals, v)
		}
		switch vv := v.(type) {
		case string:
			kinds["string"] = struct{}{}
			array.svals = append(array.svals, vv)
		case float64:
			kinds["float"] = struct{}{}
			array.fvals = append(array.fvals, vv)
		case bool:
			kinds["bool"] = struct{}{}
			array.bvals = append(array.bvals, vv)
		case nil:
			// skip
		default:
			kinds["any"] = struct{}{}
		}
	}
	if length == 0 {
		return arrayvalues{}, ErrMissingArrayValues
	}

	// kinds is supposed to have a single key if all entries are of same type.
	// In case there are multiple keys, there are mixed types in the array and then
	// we map into an `any` type.

	var kind string
	for k := range kinds {
		kind = k
		break
	}
	if len(kinds) > 1 || kind == "any" {
		array.kind = tany
		return array, nil
	}
	switch kind {
	case "bool":
		array.kind = tbool
	case "string":
		array.kind = tstr
	case "float":
		array.kind = tfloat
	default:
		panic("unreachable")
	}
	return array, nil
}

func appendval(val any) (any, error) {
	array, err := arrayvals(val)
	if err != nil {
		return nil, err
	}
	switch array.kind {
	case tany:
		return Append[any]{Values: array.avals}, nil
	case tbool:
		return Append[bool]{Values: array.bvals}, nil
	case tstr:
		return Append[string]{Values: array.svals}, nil
	case tfloat:
		return Append[float64]{Values: array.fvals}, nil
	}
	panic("unreachable")
}

func prependval(val any) (any, error) {
	array, err := arrayvals(val)
	if err != nil {
		return nil, err
	}
	switch array.kind {
	case tany:
		return Prepend[any]{Values: array.avals}, nil
	case tbool:
		return Prepend[bool]{Values: array.bvals}, nil
	case tstr:
		return Prepend[string]{Values: array.svals}, nil
	case tfloat:
		return Prepend[float64]{Values: array.fvals}, nil
	}
	panic("unreachable")
}

func errUnexpectedEOF() error {
	return fmt.Errorf("%w: unexpected eof", ErrSyntax)
}

func lexIdent(in []byte) (string, []byte, error) {
	r, size := utf8.DecodeRune(in)
	if r == utf8.RuneError || size == 0 || !unicode.IsLetter(r) {
		return "", nil, ErrNotIdent
	}

	ident := []rune{r}
	pos := size
	for pos < len(in) {
		r, size := utf8.DecodeRune(in[pos:])
		if r == utf8.RuneError || size == 0 {
			return "", nil, fmt.Errorf("invalid rune: %c", r)
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			break
		}
		ident = append(ident, r)
		pos += size
	}
	if ident[len(ident)-1] == '-' {
		ident = ident[:len(ident)-1]
		pos--
	}
	return string(ident), in[pos:], nil
}

func lexdotdotdot(in []byte) error {
	if len(in) < 3 || !bytes.HasPrefix(in, []byte(dotdotdot)) {
		return fmt.Errorf("%w: unexpected bytes: %s", ErrSyntax, in)
	}
	return nil
}

func skipblank(in []byte) []byte {
	for len(in) > 0 {
		r, size := utf8.DecodeRune(in)
		if r != utf8.RuneError && unicode.IsSpace(r) {
			in = in[size:]
			continue
		}
		break
	}
	return in
}

var errEOF = errors.New("eof")
