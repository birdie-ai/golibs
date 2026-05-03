package dql

import (
	"maps"
	"slices"
)

// expr checks
var (
	_ Expr = ObjectExpr{}
	_ Expr = ListExpr{}
	_ Expr = NumberExpr{}
	_ Expr = BoolExpr{}
	_ Expr = StringExpr{}
	_ Expr = FncallExpr{}
	_ Expr = VarExpr{}
	_ Expr = PathExpr{}
	_ Expr = QueryExpr{}
)

// NewVarExpr creates an expression for a variable.
func NewVarExpr(name string) VarExpr { return VarExpr{Value: name} }

// NewFncallExpr creates an expression for a function call.
func NewFncallExpr(fn string, args ...Expr) FncallExpr {
	return FncallExpr{
		Name: fn,
		Args: args,
	}
}

// NewNumberExpr creates a new number expression for a literal number.
func NewNumberExpr(v float64) NumberExpr { return NumberExpr{Value: v} }

// NewStringExpr creates a new string expression for a literal string.
func NewStringExpr(s string) StringExpr { return StringExpr{Value: s} }

// NewBoolExpr creates a bool expression for a literal boolean.
func NewBoolExpr(b bool) BoolExpr { return BoolExpr{Value: b} }

// NewObjectExpr creates an object expression.
func NewObjectExpr(keyvals map[string]Expr) ObjectExpr { return ObjectExpr{Keyvals: keyvals} }

// NewListExpr creates a list of expressions.
func NewListExpr(vals []Expr) ListExpr { return ListExpr{Items: vals} }

// NewPathExpr creates a new path expression.
func NewPathExpr(base Expr, steps ...PathStep) PathExpr {
	return PathExpr{
		Base:  base,
		Steps: steps,
	}
}

// NewFieldStep creates a new step for an object field addressing.
func NewFieldStep(field string) PathStep {
	return PathStep{
		Type:  FieldStep,
		Field: field,
	}
}

// NewIndexStep creates a new step for array/list indexing.
func NewIndexStep(expr Expr) PathStep {
	return PathStep{
		Type:  IndexStep,
		Index: expr,
	}
}

// Variables of the expression.
func (e ObjectExpr) Variables() (vars []VarExpr) {
	for _, k := range slices.Sorted(maps.Keys(e.Keyvals)) {
		vars = append(vars, e.Keyvals[k].Variables()...)
	}
	return vars
}

// Variables of the expression.
func (e ListExpr) Variables() (vars []VarExpr) {
	for _, v := range e.Items {
		vars = append(vars, v.Variables()...)
	}
	return vars
}

// Variables of the expression.
func (e FncallExpr) Variables() (vars []VarExpr) {
	for _, arg := range e.Args {
		vars = append(vars, arg.Variables()...)
	}
	return vars
}

// Variables of the expression.
func (e QueryExpr) Variables() (vars []VarExpr) {
	if e.Type == predicate {
		switch e.OP {
		case Eq, Match:
			return e.RHS.Variables()
		case Range:
			if e.Lower.Set {
				vars = append(vars, e.Lower.Val.Variables()...)
			}
			if e.Upper.Set {
				vars = append(vars, e.Upper.Val.Variables()...)
			}
			return vars
		default:
			panic(e.OP)
		}
	}
	for _, children := range e.Children {
		vars = append(vars, children.Variables()...)
	}
	return vars
}

// Variables of the expression.
func (e VarExpr) Variables() []VarExpr { return []VarExpr{e} }

// Variables of the expression.
func (e PathExpr) Variables() (vars []VarExpr) {
	vars = append(vars, e.Base.Variables()...)
	for _, step := range e.Steps {
		switch step.Type {
		case IndexStep:
			vars = append(vars, step.Index.Variables()...)
		}
	}
	return vars
}

// literals

// Variables of the expression.
func (e BoolExpr) Variables() []VarExpr { return nil }

// Variables of the expression.
func (e NumberExpr) Variables() []VarExpr { return nil }

// Variables of the expression.
func (e StringExpr) Variables() []VarExpr { return nil }

// Path is an static/evaluated path expression.
func Path(steps ...string) StaticPath { return StaticPath(steps) }
