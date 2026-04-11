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
)

func NewVarExpr(name string) VarExpr { return VarExpr{Value: name} }

func NewFncallExpr(fn string, args ...Expr) FncallExpr {
	return FncallExpr{
		Name: fn,
		Args: args,
	}
}

func NewNumberExpr(v float64) NumberExpr { return NumberExpr{Value: v} }

func NewStringExpr(s string) StringExpr { return StringExpr{Value: s} }

func NewPathExpr(base Expr, steps ...PathStep) PathExpr {
	return PathExpr{
		Base:  base,
		Steps: steps,
	}
}

func NewFieldStep(field string) PathStep {
	return PathStep{
		Type:  FieldStep,
		Field: field,
	}
}

func NewIndexStep(expr Expr) PathStep {
	return PathStep{
		Type:  IndexStep,
		Index: expr,
	}
}

func (e ObjectExpr) Variables() (vars []VarExpr) {
	for _, k := range slices.Sorted(maps.Keys(e.Keyvals)) {
		vars = append(vars, e.Keyvals[k].Variables()...)
	}
	return vars
}

func (e ListExpr) Variables() (vars []VarExpr) {
	for _, v := range e.Items {
		vars = append(vars, v.Variables()...)
	}
	return vars
}

func (e FncallExpr) Variables() (vars []VarExpr) {
	for _, arg := range e.Args {
		vars = append(vars, arg.Variables()...)
	}
	return vars
}

func (e VarExpr) Variables() []VarExpr { return []VarExpr{e} }

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
func (e BoolExpr) Variables() []VarExpr   { return nil }
func (e NumberExpr) Variables() []VarExpr { return nil }
func (e StringExpr) Variables() []VarExpr { return nil }
