package dql

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

type (
	EncoderOption func(e *encoder)

	encoder struct {
		skipValidation bool
	}
)

// encoder errors
var (
	ErrMissingEntity      = errors.New(`missing entity`)
	ErrUnexpectedExprType = errors.New(`expression type not expected here`)
)

func SkipValidation() EncoderOption {
	return func(e *encoder) {
		e.skipValidation = true
	}
}

func Encode(w io.Writer, program Program, opts ...EncoderOption) error {
	enc := &encoder{}
	for _, opt := range opts {
		opt(enc)
	}
	for _, stmt := range program.Stmts {
		if !enc.skipValidation {
			err := validateStmt(stmt)
			if err != nil {
				return err
			}
			err = encodeStmt(w, stmt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateStmt(s Stmt) error {
	var errs []error
	if s.Entity == "" {
		errs = append(errs, ErrMissingEntity)
	}
	return errors.Join(errs...)
}

func encodeStmt(w io.Writer, s Stmt) error {
	err := encodePreamble(w, s)
	if err != nil {
		return err
	}
	if len(s.Fields) > 0 {
		err = encodeFields(w, s.Fields)
		if err != nil {
			return err
		}
	}
	if s.Where != nil {
		err = encodeWhere(w, s.Where)
		if err != nil {
			return err
		}
	}
	err = encodeLimit(w, s.Limit)
	if err != nil {
		return err
	}
	return write(w, ";")
}

func encodeFields(w io.Writer, fields []Expr) error {
	err := write(w, " ")
	if err != nil {
		return err
	}
	for i, expr := range fields {
		if i > 0 {
			err := write(w, ",")
			if err != nil {
				return err
			}
		}
		err := encodeExpr(w, expr)
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeWhere(w io.Writer, q *QueryExpr) error {
	err := write(w, " WHERE ")
	if err != nil {
		return err
	}
	return encodeQueryExpr(w, q)
}

func encodeExpr(w io.Writer, expr Expr) error {
	switch v := expr.(type) {
	default:
		panic("unreachable")
	case ObjectExpr:
		return encodeObjectExpr(w, v, false)
	case ListExpr:
		return encodeListExpr(w, v, false)
	case NumberExpr:
		return encodeNumberExpr(w, v)
	case StringExpr:
		return encodeStringExpr(w, v)
	case BoolExpr:
		return encodeBoolExpr(w, v)
	case VarExpr:
		return encodeVarExpr(w, v)
	case FncallExpr:
		return encodeFncallExpr(w, v)
	case *QueryExpr:
		return encodeQueryExpr(w, v)
	}
}

func encodeQueryExpr(w io.Writer, q *QueryExpr) error {
	switch q.Type {
	default:
		return encodePredicateExpr(w, q)
	case AND, OR, NOT:
		return encodeLogicalExpr(w, q)
	}
}

func encodePredicateExpr(w io.Writer, q *QueryExpr) error {
	err := write(w, "{")
	if err != nil {
		return err
	}
	err = encodeValue(w, strings.Join(q.LHS, "."))
	if err != nil {
		return err
	}
	err = write(w, ":")
	if err != nil {
		return err
	}
	err = encodeLiteralExpr(w, q.RHS)
	if err != nil {
		return err
	}
	return write(w, "}")
}

func encodeLogicalExpr(w io.Writer, q *QueryExpr) error {
	var op string
	if q.Type == AND {
		op = "$and"
	} else if q.Type == OR {
		op = "$or"
	} else {
		op = "$not"
	}
	err := write(w, "{")
	if err != nil {
		return err
	}
	err = encodeValue(w, op)
	if err != nil {
		return err
	}
	err = write(w, ":[")
	if err != nil {
		return err
	}
	for i, child := range q.Children {
		if i > 0 {
			err := write(w, ",")
			if err != nil {
				return err
			}
		}
		err := encodeQueryExpr(w, child)
		if err != nil {
			return err
		}
	}
	return write(w, "]}")
}

func encodeLiteralExpr(w io.Writer, expr Expr) error {
	switch v := expr.(type) {
	default:
		return fmt.Errorf("%w: type %T", ErrUnexpectedExprType, v)
	case StringExpr, NumberExpr, BoolExpr:
		return encodeExpr(w, v)
	case ListExpr:
		return encodeListExpr(w, v, true)
	case ObjectExpr:
		return encodeObjectExpr(w, v, true)
	}
}

func encodeVarExpr(w io.Writer, v VarExpr) error       { return write(w, v.Value) }
func encodeBoolExpr(w io.Writer, v BoolExpr) error     { return encodeValue(w, v.Value) }
func encodeStringExpr(w io.Writer, v StringExpr) error { return encodeValue(w, v.Value) }
func encodeNumberExpr(w io.Writer, v NumberExpr) error { return encodeValue(w, v.Value) }

func encodeFncallExpr(w io.Writer, v FncallExpr) error {
	err := write(w, v.Name)
	if err != nil {
		return err
	}
	err = write(w, "(")
	if err != nil {
		return err
	}
	for i, arg := range v.Args {
		if i > 0 {
			err := write(w, ",")
			if err != nil {
				return err
			}
		}
		err := encodeExpr(w, arg)
		if err != nil {
			return err
		}
	}
	return write(w, "}")
}
func encodeListExpr(w io.Writer, list ListExpr, literals bool) error {
	err := write(w, "[")
	if err != nil {
		return err
	}
	for i, expr := range list.Items {
		if i > 0 {
			err := write(w, ",")
			if err != nil {
				return err
			}
		}
		if literals {
			err = encodeLiteralExpr(w, expr)
		} else {
			err = encodeExpr(w, expr)
		}
		if err != nil {
			return err
		}
	}
	return write(w, "]")
}

func encodeObjectExpr(w io.Writer, obj ObjectExpr, literals bool) error {
	err := write(w, "{")
	if err != nil {
		return err
	}
	// encoder must be deterministic!
	for i, k := range slices.Sorted(maps.Keys(obj.Keyvals)) {
		if i > 0 {
			err = write(w, ",")
			if err != nil {
				return err
			}
		}
		v := obj.Keyvals[k]
		err := encodeValue(w, k)
		if err != nil {
			return err
		}
		err = write(w, ":")
		if err != nil {
			return err
		}
		if literals {
			err = encodeLiteralExpr(w, v)
		} else {
			err = encodeExpr(w, v)
		}
	}
	return write(w, "}")
}

func encodeLimit(w io.Writer, limit int) error {
	err := write(w, " LIMIT ")
	if err != nil {
		return err
	}
	return encodeValue(w, limit)
}

func encodeValue(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return write(w, string(data))
}

func encodePreamble(w io.Writer, s Stmt) error {
	err := write(w, "SEARCH ")
	if err != nil {
		return err
	}
	return write(w, s.Entity)
}

func write(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))
	return err
}
