package dql

import (
	"maps"
	"slices"
)

// expr checks
var (
	_ Expr = ObjectExpr{}
	_ Expr = ListExpr{}
	_ Expr = FloatExpr{}
	_ Expr = BoolExpr{}
	_ Expr = StringExpr{}
	_ Expr = FncallExpr{}
	_ Expr = VarExpr{}
)

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

// literals
func (e FloatExpr) Variables() []VarExpr  { return nil }
func (e BoolExpr) Variables() []VarExpr   { return nil }
func (e StringExpr) Variables() []VarExpr { return nil }
