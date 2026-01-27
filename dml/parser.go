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
		stmt.Assign, in, err = parseSetAssigns(in)
	} else {
		stmt.Assign, in, err = parseDelAssigns(in)
	}
	if err != nil {
		return Stmt{}, nil, err
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

func parseSetAssigns(in []byte) (Assign, []byte, error) {
	assign := Assign{}
	for len(in) > 0 {
		var (
			key string
			val any
			err error
		)
		key, val, in, err = parseAssign(in)
		if err != nil {
			return Assign{}, nil, err
		}

		assign[key] = val
		in = skipblank(in)
		if len(in) == 0 {
			return Assign{}, nil, errUnexpectedEOF()
		}
		if in[0] == ',' {
			// only one "." assign
			if _, ok := assign["."]; ok {
				return Assign{}, nil, fmt.Errorf("%w: only one '.' assignment is permitted. Unexpected ','", ErrSyntax)
			}
			in = skipblank(in[1:])
			if len(in) == 0 {
				return Assign{}, nil, errUnexpectedEOF()
			}
			continue
		}
		break
	}
	return assign, in, nil
}

func parseDelAssigns(in []byte) (Assign, []byte, error) {
	assign := Assign{}
	for len(in) > 0 {
		var (
			key string
			val any
			err error
		)
		key, val, in, err = parseDelFilters(in)
		if err != nil {
			return Assign{}, nil, err
		}

		assign[key] = val
		in = skipblank(in)
		if len(in) == 0 {
			return Assign{}, nil, errUnexpectedEOF()
		}
		if in[0] == ',' {
			in = skipblank(in[1:])
			if len(in) == 0 {
				return Assign{}, nil, errUnexpectedEOF()
			}
			continue
		}
		break
	}
	return assign, in, nil
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

	dotident, in, err := parsePathTraversal(in)
	if err != nil {
		return "", nil, nil, err
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

func parseDelFilters(in []byte) (string, any, []byte, error) {
	in = skipblank(in)
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] == '.' {
		return ".", DeleteKey{}, in[1:], nil
	}
	dotident, in, err := parsePathTraversal(in)
	if err != nil {
		return "", nil, nil, err
	}
	in = skipblank(in)
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] != '[' {
		return dotident, DeleteKey{}, in, nil
	}
	in = in[1:]
	var (
		vark string
		varv string
	)
	if len(in) > 0 && in[0] == '_' {
		vark = "_"
	} else {
		vark, in, err = lexIdent(in)
		if err != nil {
			return "", nil, nil, err
		}
	}
	in = skipblank(in)
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] != ']' {
		return "", nil, nil, fmt.Errorf("%w: expected ']' but got %q", ErrSyntax, in)
	}
	in = skipblank(in[1:])
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] == '=' {
		if len(in[1:]) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		if in[1] != '>' {
			return "", nil, nil, fmt.Errorf("%w: expected '>' but got %q", ErrSyntax, in[1:])
		}
		in = skipblank(in[2:])
		if len(in) == 0 {
			return "", nil, nil, errUnexpectedEOF()
		}
		varv, in, err = lexIdent(in)
		if err != nil {
			return "", nil, nil, err
		}
	}
	in = skipblank(in)
	if len(in) == 0 {
		return "", nil, nil, errUnexpectedEOF()
	}
	if in[0] != ':' {
		return "", nil, nil, fmt.Errorf("%w: expected ':' but got %q", ErrSyntax, in)
	}
	in = skipblank(in[1:])
	cond, in, err := parseWhere(in)
	if err != nil {
		return "", nil, nil, err
	}
	var kf KeyFilter
	if vark != "_" {
		condk, ok := cond[vark]
		if !ok {
			return "", nil, nil, fmt.Errorf("%w: variable %q not found in DELETE condition", ErrSyntax, vark)
		}
		kf, err = kfilter(condk)
		if err != nil {
			return "", nil, nil, err
		}
		delete(cond, vark)
		if varv == "" || varv == "_" {
			if len(cond) > 0 {
				return "", nil, nil, fmt.Errorf("%w: more clauses than declared variables", ErrSyntax)
			}
			return dotident, kf, in, nil
		}
	}
	if len(kf.Keys) > 1 {
		return "", nil, nil, fmt.Errorf("%w: key-value filter requires filtering by single keys but %d key clauses given: %v", ErrSyntax, len(kf.Keys), kf.Keys)
	}
	condv, ok := cond[varv]
	if !ok {
		return "", nil, nil, fmt.Errorf("%w: variable %s not found in DELETE condition", ErrSyntax, varv)
	}
	kvf, err := kvfilter(kf.Keys[0], condv)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: handling condition variable %s", err, varv)
	}
	delete(cond, varv)
	if len(cond) > 0 {
		return "", nil, nil, fmt.Errorf("%w: %d surplus conditions in DELETE filter: %v", ErrSyntax, len(cond), cond)
	}
	return dotident, kvf, in, nil
}

func kfilter(val any) (KeyFilter, error) {
	switch v := val.(type) {
	default:
		// the key condition must be a string or []string
		// TODO(i4k): use proper error for type check errors.
		return KeyFilter{}, fmt.Errorf("%w: the variable has string type but condition uses %T", ErrSyntax, val)
	case string:
		return KeyFilter{Keys: []string{v}}, nil
	case []any:
		strs, err := tarr[string](v)
		if err != nil {
			return KeyFilter{}, err
		}
		return KeyFilter{Keys: strs}, nil
	}
}

func kvfilter(key string, val any) (any, error) {
	// there's an invariance that `val` can *ONLY* be a list **iff* if syntax `<name> IN [...]`
	// or `{"<name>": [...]}` is used and both are ensured by `parseWhere()` to have len(val)>0.

	switch vv := val.(type) {
	case []any:
		switch vv[0].(type) {
		case string:
			vals, err := tarr[string](vv)
			if err != nil {
				return nil, fmt.Errorf("%w: list with mixed types: %v", ErrSyntax, err)
			}
			return KeyValueFilter[string]{Key: key, Values: vals}, nil
		case float64:
			vals, err := tarr[float64](vv)
			if err != nil {
				return nil, fmt.Errorf("%w: list with mixed types: %v", ErrSyntax, err)
			}
			return KeyValueFilter[float64]{Key: key, Values: vals}, nil
		case bool:
			vals, err := tarr[bool](vv)
			if err != nil {
				return nil, fmt.Errorf("%w: list with mixed types: %v", ErrSyntax, err)
			}
			return KeyValueFilter[bool]{Key: key, Values: vals}, nil
		default:
			return nil, fmt.Errorf("%w: unexpected list with type %T", ErrSyntax, vv)
		}
	case string:
		return KeyValueFilter[string]{Key: key, Values: []string{vv}}, nil
	default:
		return nil, fmt.Errorf("%w: filters need to operate on primitive types (string, float64, bool) or lists of them but type %T is used", ErrSyntax, vv)
	}
}

func tarr[T any](arr []any) ([]T, error) {
	ret := make([]T, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(T)
		if !ok {
			return nil, fmt.Errorf("unexpected value %[1]v with type %[1]T in list items", v)
		}
		ret = append(ret, s)
	}
	return ret, nil
}

func parsePathTraversal(in []byte) (string, []byte, error) {
	var (
		dotident []byte
		ident    string
		err      error
	)

	ident, in, err = lexIdent(in)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", ErrSyntax, err)
	}

	dotident = append(dotident, []byte(ident)...)
	if len(in) == 0 {
		return "", nil, errUnexpectedEOF()
	}

	for len(in) > 0 && in[0] == '.' {
		dotident = append(dotident, '.')
		in = skipblank(in[1:])
		if len(in) == 0 {
			return "", nil, errUnexpectedEOF()
		}
		if in[0] == '"' {
			// parse the string as a JSON string.
			// This means we support all of its escape sequences!
			dec := json.NewDecoder(bytes.NewReader(in))
			tok, err := dec.Token()
			if err != nil {
				return "", nil, fmt.Errorf("%w: parsing quote string literal: %v", ErrSyntax, err)
			}
			str, ok := tok.(string)
			if !ok {
				return "", nil, fmt.Errorf("%w: unexpected %v", ErrSyntax, tok)
			}
			dotident = append(dotident, []byte(strconv.Quote(str))...)
			in = skipblank(in[dec.InputOffset():])
			continue
		}
		ident, in, err = lexIdent(in)
		if err != nil {
			return "", nil, fmt.Errorf("%w: %v", ErrSyntax, err)
		}
		if len(in) == 0 {
			return "", nil, errUnexpectedEOF()
		}
		dotident = append(dotident, []byte(ident)...)
	}
	return string(dotident), in, nil
}

func parseWhere(in []byte) (Where, []byte, error) {
	if in[0] == '{' {
		where, in, err := parseWhereObject(in)
		if err != nil {
			return Where{}, nil, err
		}
		in = skipblank(in)
		if len(in) > 3 && bytes.EqualFold(in[:3], []byte{'A', 'N', 'D'}) {
			in = skipblank(in[3:])
			var next Where
			next, in, err = parseWhere(in)
			if err != nil {
				return Where{}, nil, err
			}
			for k, v := range next {
				if _, ok := where[k]; ok {
					return Where{}, nil, fmt.Errorf("%w: invalid WHERE: duplicate AND field %q", ErrSyntax, k)
				}
				where[k] = v
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
	isin := len(in) >= 2 && bytes.EqualFold(in[:2], []byte{'I', 'N'})
	if in[0] != '=' && !isin {
		return nil, nil, fmt.Errorf("%w: invalid where: unexpected char %c", ErrSyntax, in[0])
	}
	if isin {
		in = in[2:]
	} else {
		in = in[1:]
	}
	in = skipblank(in)
	if len(in) == 0 {
		return nil, nil, errUnexpectedEOF()
	}
	if isin && in[0] != '[' {
		return nil, nil, fmt.Errorf("%w: IN requires a JSON list argument but got %q", ErrSyntax, in)
	}
	var val any
	in, err = parseJSON(in, &val)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: parsing value as JSON: %v", ErrSyntax, err)
	}
	where := Where{
		ident: val,
	}
	in = skipblank(in)
	if len(in) > 3 && bytes.EqualFold(in[:3], []byte{'A', 'N', 'D'}) {
		in = skipblank(in[3:])
		var next Where
		next, in, err = parseWhere(in)
		if err != nil {
			return Where{}, nil, err
		}
		for k, v := range next {
			if _, ok := where[k]; ok {
				return Where{}, nil, fmt.Errorf("%w: invalid WHERE: duplicate AND field %q", ErrSyntax, k)
			}
			where[k] = v
		}
	}
	return where, in, nil
}

func parseWhereObject(in []byte) (Where, []byte, error) {
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
	tinvalid = iota
	tstr
	tfloat
	tbool
	tarray
	tobj
)

type arrayvalues struct {
	kind  kind
	avals [][]any
	ovals []map[string]any
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
	if len(anyvals) == 0 {
		return arrayvalues{}, ErrMissingArrayValues
	}
	var err error
	var array arrayvalues
	switch anyvals[0].(type) {
	case string:
		array.kind = tstr
		array.svals, err = appendchk(array.svals, anyvals...)
	case float64:
		array.kind = tfloat
		array.fvals, err = appendchk(array.fvals, anyvals...)
	case bool:
		array.kind = tbool
		array.bvals, err = appendchk(array.bvals, anyvals...)
	case []any:
		array.kind = tarray
		array.avals, err = appendchk(array.avals, anyvals...)
	case map[string]any:
		array.kind = tobj
		array.ovals, err = appendchk(array.ovals, anyvals...)
	default:
		return arrayvalues{}, ErrUnsupportedArrayValue
	}
	if err != nil {
		return arrayvalues{}, err
	}
	return array, nil
}

func appendchk[T any](arr []T, values ...any) ([]T, error) {
	for _, v := range values {
		vv, ok := v.(T)
		if !ok {
			return nil, ErrArrayWithMixedTypes
		}
		arr = append(arr, vv)
	}
	return arr, nil
}

func appendval(val any) (any, error) {
	array, err := arrayvals(val)
	if err != nil {
		return nil, err
	}
	switch array.kind {
	case tbool:
		return Append[bool]{Values: array.bvals}, nil
	case tstr:
		return Append[string]{Values: array.svals}, nil
	case tfloat:
		return Append[float64]{Values: array.fvals}, nil
	case tarray:
		return Append[[]any]{Values: array.avals}, nil
	case tobj:
		return Append[map[string]any]{Values: array.ovals}, nil
	}
	panic("unreachable")
}

func prependval(val any) (any, error) {
	array, err := arrayvals(val)
	if err != nil {
		return nil, err
	}
	switch array.kind {
	case tbool:
		return Prepend[bool]{Values: array.bvals}, nil
	case tstr:
		return Prepend[string]{Values: array.svals}, nil
	case tfloat:
		return Prepend[float64]{Values: array.fvals}, nil
	case tarray:
		return Prepend[[]any]{Values: array.avals}, nil
	case tobj:
		return Prepend[map[string]any]{Values: array.ovals}, nil
	}
	panic("unreachable")
}

func errUnexpectedEOF() error {
	return fmt.Errorf("%w: unexpected eof", ErrSyntax)
}

func lexIdent(in []byte) (string, []byte, error) {
	r, size := utf8.DecodeRune(in)
	if r == utf8.RuneError || size == 0 || !unicode.IsLetter(r) {
		return "", nil, fmt.Errorf("%w: parsing %q", ErrNotIdent, in)
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
