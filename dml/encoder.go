package dml

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unique"
)

// encoder errors.
var (
	ErrInvalidOperation      = errors.New("invalid operation")
	ErrMissingEntity         = errors.New(`entity is not provided`)
	ErrMissingAssign         = errors.New(`missing an assign`)
	ErrMissingArrayValues    = errors.New(`...: missing array values`)
	ErrUnsupportedArrayValue = errors.New(`unsupported array values`)
	ErrArrayWithMixedTypes   = errors.New(`array items with mixed types`)
	ErrInvalidAssignKey      = errors.New(`invalid assign key`)
	ErrMissingWhereClause    = errors.New(`WHERE clause is not given`)
	ErrNotIdent              = errors.New(`not an identifier`)
	ErrInvalidDotAssign      = errors.New(`. (dot) requires to be the only assignment`)

	// delete specific errors
	ErrEmptyFilterKeys        = errors.New(`missing keys in filter`)
	ErrEmptyFilterValues      = errors.New(`missing values in filter`)
	ErrInvalidFilterKeyValues = errors.New(`invalid keyvalue filter`)
	ErrInvalidAssign          = errors.New(`invalid DELETE assign`)
)

// Encode validates and encode the statements in its text format.
// TODO(i4k): support prettify output.
func Encode(w io.Writer, stmts Stmts) error {
	for _, stmt := range stmts {
		err := validate(stmt)
		if err != nil {
			return err
		}
		err = encode(w, stmt)
		if err != nil {
			return err
		}
	}
	return nil
}

func validate(stmt Stmt) error {
	var errs []error
	switch stmt.Op {
	default:
		errs = append(errs, fmt.Errorf("%w: %q", ErrInvalidOperation, stmt.Op))
	case SET, DELETE:
	}
	var empty unique.Handle[string]
	if stmt.Entity == empty || stmt.Entity.Value() == "" {
		errs = append(errs, ErrMissingEntity)
	}
	if stmt.Entity != empty && !isIdent(stmt.Entity.Value()) {
		errs = append(errs, fmt.Errorf("invalid entity %s: %w", stmt.Entity.Value(), ErrNotIdent))
	}
	if len(stmt.Assign) == 0 && stmt.Op != DELETE {
		errs = append(errs, ErrMissingAssign)
	}
	keys := slices.Sorted(maps.Keys(stmt.Assign))
	hasdot := false
	for _, k := range keys {
		if !hasdot {
			hasdot = k == "."
		}
		v, ok := stmt.Assign[k].(validator)
		if ok {
			errs = append(errs, v.validate())
		}
	}
	if hasdot && len(stmt.Assign) > 1 {
		errs = append(errs, ErrInvalidDotAssign)
	}
	if len(stmt.Where) == 0 {
		errs = append(errs, ErrMissingWhereClause)
	}
	for k := range stmt.Where {
		if !isIdent(k) {
			errs = append(errs, fmt.Errorf("clause with invalid field %s: %w", k, ErrNotIdent))
		}
	}

	// other validations happens at encoding phase.
	return errors.Join(errs...)
}

func encode(w io.Writer, stmt Stmt) error {
	err := encodePreamble(w, stmt)
	if err != nil {
		return err
	}
	err = encodeAssign(w, stmt.Op, stmt.Assign)
	if err != nil {
		return err
	}
	err = write(w, " WHERE ")
	if err != nil {
		return err
	}
	err = encodeClauses(w, stmt.Where)
	if err != nil {
		return err
	}
	return write(w, ";")
}

func encodePreamble(w io.Writer, stmt Stmt) error {
	return write(w, string(stmt.Op)+" "+string(OpKind(stmt.Entity.Value()))+" ")
}

func encodeAssign(w io.Writer, op OpKind, assign Assign) error {
	if op == SET {
		return encodeSetAssign(w, assign)
	}
	return encodeDelAssign(w, assign)
}

func encodeSetAssign(w io.Writer, assign Assign) error {
	for i, key := range slices.Sorted(maps.Keys(assign)) {
		if key != "." {
			for i, s := range strings.Split(key, ".") {
				if _, err := strconv.Unquote(s); i > 0 && len(s) > 2 && s[0] == '"' && err == nil {
					continue
				}
				if !isIdent(s) {
					return fmt.Errorf("%w: expected an ident or a quoted string but found %s", ErrInvalidAssignKey, s)
				}
			}
		}
		err := write(w, key+"=")
		if err != nil {
			return err
		}
		val := assign[key]
		if varr, ok := val.(array); ok {
			val = varr.vals()
			d, err := json.Marshal(val)
			if err != nil {
				return err
			}
			if varr.op() == appendOp {
				err = write(w, dotdotdot)
				if err != nil {
					return err
				}
			}
			err = write(w, string(d))
			if err != nil {
				return err
			}
			if varr.op() == prependOp {
				err = write(w, dotdotdot)
				if err != nil {
					return err
				}
			}
		} else {
			d, err := json.Marshal(val)
			if err != nil {
				return err
			}
			err = write(w, string(d))
			if err != nil {
				return err
			}
		}
		if i+1 < len(assign) {
			err = write(w, ",")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func encodeDelAssign(w io.Writer, assign Assign) error {
	var hasdot bool
	keys := slices.Sorted(maps.Keys(assign))
	for i, key := range keys {
		v, ok := assign[key].(assignEncoder)
		if !ok {
			return ErrInvalidAssign
		}
		if key == "." {
			if hasdot {
				return ErrInvalidDotAssign
			}
			hasdot = true
		}
		err := v.encode(w, key)
		if err != nil {
			return err
		}
		if i+1 < len(keys) {
			err = write(w, ",")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func encodeClauses(w io.Writer, clauses Where) error {
	if len(clauses) == 1 {
		for k, v := range clauses {
			d, err := json.Marshal(v)
			if err != nil {
				return err
			}
			err = write(w, k+"="+string(d))
			if err != nil {
				return err
			}
		}
		return nil
	}
	d, err := json.Marshal(clauses)
	if err != nil {
		return err
	}
	return write(w, string(d))
}

type assignEncoder interface {
	encode(w io.Writer, key string) error
}

func (a KeyFilter) encode(w io.Writer, target string) error {
	if len(a.Keys) == 1 {
		return write(w, target+"[k] : k="+strconv.Quote(a.Keys[0]))
	}
	d, err := json.Marshal(a.Keys)
	if err != nil {
		return err
	}
	return write(w, target+"[k] : k IN "+string(d))
}

func (a ValueFilter[T]) encode(w io.Writer, target string) error {
	if len(a.Values) == 1 {
		d, err := json.Marshal(a.Values[0])
		if err != nil {
			return err
		}
		return write(w, target+"[_] => v : v="+string(d))
	}
	d, err := json.Marshal(a.Values)
	if err != nil {
		return err
	}
	return write(w, target+"[_] => v : v IN "+string(d))
}

func (a KeyValueFilter[T]) encode(w io.Writer, target string) error {
	err := write(w, target+"[k] => v : k="+strconv.Quote(a.Key)+" AND v")
	if err != nil {
		return err
	}
	if len(a.Values) == 1 {
		d, err := json.Marshal(a.Values[0])
		if err != nil {
			return err
		}
		return write(w, "="+string(d))
	}
	d, err := json.Marshal(a.Values)
	if err != nil {
		return err
	}
	return write(w, " IN "+string(d))
}

func (a DeleteKey) encode(w io.Writer, target string) error {
	return write(w, target)
}

func write(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))
	return err
}
