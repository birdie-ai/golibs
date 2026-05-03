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
	// EncoderOption is a type used for configuring the encoder.
	// See [SkipValidation] as an example.
	EncoderOption func(e *Encoder)

	// Encoder is a dql encoder.
	Encoder struct {
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

// SkipValidation skips the validation step. This should be used in the case you are encoding
// previously parsed dql programs or if you can guarantee that the AST is valid.
func SkipValidation() EncoderOption {
	return func(e *Encoder) {
		e.skipValidation = true
	}
}

// OnlyShape encodes only the shape of the program, which means all values are extracted and
// replaced by ordinals. The extracted values are available by the [Encoder.Values] method.
func OnlyShape() EncoderOption {
	return func(e *Encoder) {
		e.onlyShape = true
	}
}

// NewEncoder creates a new dql encoder.
func NewEncoder(w io.Writer, opts ...EncoderOption) *Encoder {
	enc := &Encoder{
		buf: w,
	}
	for _, opt := range opts {
		opt(enc)
	}
	return enc
}

// Encode the in program into its text form, writing the output to the [io.Writer] provided
// when creating the encoder with [NewEncoder].
func (e *Encoder) Encode(in Program) error {
	for _, stmt := range in.Stmts {
		err := e.EncodeStmt(stmt)
		if err != nil {
			return err
		}
	}
	return nil
}

// EncodeStmt encodes the provided statement.
func (e *Encoder) EncodeStmt(in Stmt) error {
	if !e.skipValidation {
		err := validateStmt(in)
		if err != nil {
			return err
		}
	}
	return e.stmt(in)
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
		panic(expr)
	case PathExpr:
		return e.pathExpr(v, retfield)
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
	err = e.emit(":{")
	if err != nil {
		return err
	}
	switch q.OP {
	case Eq:
		fallthrough
	case Match:
		err = e.json(q.OP.String())
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
	case Range:
		bounds := make([]Bound, 0, 2)
		if q.Lower.Set {
			bounds = append(bounds, q.Lower)
		}
		if q.Upper.Set {
			bounds = append(bounds, q.Upper)
		}
		for i, bound := range bounds {
			if i > 0 {
				err = e.emit(",")
				if err != nil {
					return err
				}
			}
			err = e.json(bound.OP.String())
			if err != nil {
				return err
			}
			err = e.emit(":")
			if err != nil {
				return err
			}
			err = e.expr(bound.Val, retfield)
			if err != nil {
				return err
			}
		}
	}
	return e.emit("}}")
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

func (e *Encoder) pathExpr(v PathExpr, retfield bool) error {
	if e.onlyShape && !retfield {
		return e.slot(v)
	}
	err := e.expr(v.Base, retfield)
	if err != nil {
		return err
	}
	if len(v.Steps) == 0 {
		return fmt.Errorf("path expr %v has no steps", v)
	}
	for _, step := range v.Steps {
		switch step.Type {
		default:
			return fmt.Errorf("unexpected step type %v", step)
		case FieldStep:
			err := e.emit("." + step.Field)
			if err != nil {
				return err
			}
		case IndexStep:
			err := e.emit("[")
			if err != nil {
				return err
			}
			err = e.expr(step.Index, retfield)
			if err != nil {
				return err
			}
			err = e.emit("]")
			if err != nil {
				return err
			}
		}
	}
	return nil
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
