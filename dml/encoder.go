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

// encoder errors
var (
	ErrInvalidOperation   = errors.New("invalid operation")
	ErrMissingEntity      = errors.New(`entity is not provided`)
	ErrMissingAssign      = errors.New(`"SET" requires an assign`)
	ErrMissingArrayValues = errors.New(`...: missing array values`)
	ErrInvalidAssignKey   = errors.New(`invalid assign key`)
	ErrMissingWhereClause = errors.New(`WHERE clause is not given`)
	ErrNotIdent           = errors.New(`not an identifier`)
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
	for _, k := range slices.Sorted(maps.Keys(stmt.Assign)) {
		v := stmt.Assign[k]
		if vv, ok := v.(array); ok && vv.len() == 0 {
			errs = append(errs, ErrMissingArrayValues)
		}
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
	err = encodeAssign(w, stmt.Assign)
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

func encodeAssign(w io.Writer, assign Assign) error {
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

func write(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))
	return err
}
