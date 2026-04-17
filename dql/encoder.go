package dql

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
)

type (
	EncoderOption func(e *Encoder)

	Encoder struct {
		in             Program
		buf            io.Writer
		skipValidation bool
		onlyShape      bool
		values         []Expr
	}
)

// encoder errors
var (
	ErrMissingEntity      = errors.New(`missing entity`)
	ErrUnexpectedExprType = errors.New(`expression type not expected here`)
	ErrInvalidLogicalExpr = errors.New(`invalid logical expression`)
)

func SkipValidation() EncoderOption {
	return func(e *Encoder) {
		e.skipValidation = true
	}
}

func OnlyShape() EncoderOption {
	return func(e *Encoder) {
		e.onlyShape = true
	}
}

func NewEncoder(w io.Writer, program Program, opts ...EncoderOption) *Encoder {
	enc := &Encoder{
		in:  program,
		buf: w,
	}
	for _, opt := range opts {
		opt(enc)
	}
	return enc
}

func (e *Encoder) Encode() error {
	for _, stmt := range e.in.Stmts {
		if !e.skipValidation {
			err := validateStmt(stmt)
			if err != nil {
				return err
			}
			err = e.stmt(stmt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Values collected if [OnlyShape] option was provided.
func (e *Encoder) Values() []Expr {
	return e.values
}

func validateStmt(s Stmt) error {
	var errs []error
	if s.Entity == "" {
		errs = append(errs, ErrMissingEntity)
	}
	return errors.Join(errs...)
}

func (e *Encoder) stmt(s Stmt) error {
	err := e.preamble(s)
	if err != nil {
		return err
	}
	if len(s.Fields) > 0 {
		err = e.fields(s.Fields)
		if err != nil {
			return err
		}
	}
	if s.Where != nil {
		err = e.where(s.Where)
		if err != nil {
			return err
		}
	}
	err = e.limit(s.Limit)
	if err != nil {
		return err
	}
	return e.emit(";")
}

func (e *Encoder) fields(fields []Expr) error {
	err := e.emit(" ")
	if err != nil {
		return err
	}
	for i, expr := range fields {
		if i > 0 {
			err := e.emit(",")
			if err != nil {
				return err
			}
		}
		err := e.expr(expr, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) where(q *QueryExpr) error {
	err := e.emit(" WHERE ")
	if err != nil {
		return err
	}
	return e.queryExpr(q, false)
}

func (e *Encoder) expr(expr Expr, retfield bool) error {
	switch v := expr.(type) {
	default:
		panic("unreachable")
	case ObjectExpr:
		return e.objExpr(v, retfield)
	case ListExpr:
		return e.listExpr(v, retfield)
	case NumberExpr:
		return e.numberExpr(v, retfield)
	case StringExpr:
		return e.strExpr(v, retfield)
	case BoolExpr:
		return e.boolExpr(v, retfield)
	case VarExpr:
		return e.varExpr(v, retfield)
	case FncallExpr:
		return e.fncallExpr(v, retfield)
	case *QueryExpr:
		return e.queryExpr(v, retfield)
	}
}

func (e *Encoder) queryExpr(q *QueryExpr, retfield bool) error {
	switch q.Type {
	default:
		return e.predicateExpr(q, retfield)
	case AND, OR, NOT:
		return e.logicalExpr(q, retfield)
	}
}

func (e *Encoder) predicateExpr(q *QueryExpr, retfield bool) error {
	err := e.emit("{")
	if err != nil {
		return err
	}
	err = e.json(strings.Join(q.LHS, "."))
	if err != nil {
		return err
	}
	err = e.emit(":")
	if err != nil {
		return err
	}
	err = e.expr(q.RHS, retfield)
	if err != nil {
		return err
	}
	return e.emit("}")
}

func (e *Encoder) logicalExpr(q *QueryExpr, retfield bool) error {
	var op string
	switch q.Type {
	default:
		return fmt.Errorf("%w: %+v", ErrInvalidLogicalExpr, q.Type)
	case AND:
		op = "$and"
	case OR:
		op = "$or"
	case NOT:
		op = "$not"
	}
	err := e.emit("{")
	if err != nil {
		return err
	}
	err = e.json(op)
	if err != nil {
		return err
	}
	err = e.emit(":[")
	if err != nil {
		return err
	}
	for i, child := range q.Children {
		if i > 0 {
			err := e.emit(",")
			if err != nil {
				return err
			}
		}
		err := e.queryExpr(child, retfield)
		if err != nil {
			return err
		}
	}
	return e.emit("]}")
}

func (e *Encoder) varExpr(v VarExpr, retfield bool) error {
	if e.onlyShape && !retfield {
		return e.slot(v)
	}
	return e.emit(v.Value)
}

func (e *Encoder) boolExpr(v BoolExpr, retfield bool) error {
	if e.onlyShape && !retfield {
		return e.slot(v)
	}
	return e.json(v.Value)
}
func (e *Encoder) strExpr(v StringExpr, retfield bool) error {
	if e.onlyShape && !retfield {
		return e.slot(v)
	}
	return e.json(v.Value)
}
func (e *Encoder) numberExpr(v NumberExpr, retfield bool) error {
	if e.onlyShape && !retfield {
		return e.slot(v)
	}
	return e.json(v.Value)
}

func (e *Encoder) fncallExpr(v FncallExpr, retfield bool) error {
	if !retfield && e.onlyShape {
		return e.slot(v)
	}
	err := e.emit(v.Name)
	if err != nil {
		return err
	}
	err = e.emit("(")
	if err != nil {
		return err
	}
	for i, arg := range v.Args {
		if i > 0 {
			err := e.emit(",")
			if err != nil {
				return err
			}
		}
		err := e.expr(arg, retfield)
		if err != nil {
			return err
		}
	}
	return e.emit("}")
}
func (e *Encoder) listExpr(list ListExpr, retfield bool) error {
	err := e.emit("[")
	if err != nil {
		return err
	}
	for i, expr := range list.Items {
		if i > 0 {
			err := e.emit(",")
			if err != nil {
				return err
			}
		}
		err = e.expr(expr, retfield)
		if err != nil {
			return err
		}
	}
	return e.emit("]")
}

func (e *Encoder) objExpr(obj ObjectExpr, retfield bool) error {
	err := e.emit("{")
	if err != nil {
		return err
	}
	// encoder must be deterministic!
	for i, k := range slices.Sorted(maps.Keys(obj.Keyvals)) {
		if i > 0 {
			err = e.emit(",")
			if err != nil {
				return err
			}
		}
		v := obj.Keyvals[k]
		err := e.json(k)
		if err != nil {
			return err
		}
		err = e.emit(":")
		if err != nil {
			return err
		}
		err = e.expr(v, retfield)
		if err != nil {
			return err
		}
	}
	return e.emit("}")
}

func (e *Encoder) limit(limit int) error {
	err := e.emit(" LIMIT ")
	if err != nil {
		return err
	}
	return e.json(limit)
}

func (e *Encoder) json(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return e.emit(string(data))
}

func (e *Encoder) preamble(s Stmt) error {
	err := e.emit("SEARCH ")
	if err != nil {
		return err
	}
	return e.emit(s.Entity)
}

func (e *Encoder) slot(v Expr) error {
	e.values = append(e.values, v)
	err := e.emit("$")
	if err != nil {
		return err
	}
	return e.emit(strconv.Itoa(len(e.values)))
}

func (e *Encoder) emit(s string) error {
	_, err := e.buf.Write([]byte(s))
	return err
}
