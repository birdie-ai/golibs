package dql

import (
	"fmt"
	"strings"
)

type (
	// Program is the parsed script.
	Program struct {
		Stmts  Stmts
		Return Return
	}

	// Stmt is a dql statement node.
	Stmt struct {
		Name       string
		Entity     string
		Fields     []Expr
		Where      *QueryExpr
		Limit      int
		WithCursor bool
		After      Expr
		OrderBy    []OrderBy
		Aggs       Aggs

		// TODO(i4k): add Span
	}

	// Stmts is a list of stmt.
	Stmts []Stmt

	// Aggs is a map of aggregations.
	Aggs map[string]Agg

	// Return is a dql RETURN node.
	// There should be only one RETURN node per script.
	Return struct {
		Format string
		Expr   Expr
	}

	// Expr is an expression tree.
	Expr interface {
		Variables() []VarExpr
	}

	// StaticPath is an evaluated path expression.
	StaticPath []string

	// Bound represents a boundary of a range interval.
	Bound struct {
		// OP can be any number/date Predicate **except** Range.
		OP  Predicate
		Val Expr
		Set bool // indicates whether this bound exists (optional)
	}

	// QueryType is the type of the query node.
	QueryType uint8

	// Predicate operation.
	Predicate uint8

	// Agg is an aggregation node.
	Agg struct {
		Name    string
		Func    FncallExpr
		Limit   *int
		After   string // base64 cursor
		OrderBy []OrderBy

		Children Aggs
	}

	// OrderBy clause.
	OrderBy struct {
		Field StaticPath
		Sort  Sort
	}

	// Sort of the order by clause.
	// See [ASC] and [DESC].
	Sort int
)

// In the below `type` group are the Expr nodes.
//
// NOTE(i4k): all expr nodes are struct mostly because we would like to add a
// `SourceRange` or `Span` kind of field containing the range of Pos. By using structs
// we can add the source info later with back-compatibility.

type (
	// ObjectExpr is an object expression. It's a map of string to other expressions, optionally
	// nested expressions. It can represent fully an JSON object but additionally its leaves can
	// contain variables, funcalls, etc, any other expression.
	// Example:
	//   {
	//     "result": {
	//       "average_rating": avg(var1, var2, var3),
	//       "docs": all(res1.docs, res2.docs, res3.docs)
	//     }
	//   }
	// An ObjectExpr can only be evaluated once all of its expressions are evaluated.
	// The [Variables] method can be used to return all variable dependencies.
	ObjectExpr struct {
		Keyvals map[string]Expr
	}

	// ListExpr is a list expression. Similarly to [ObjectExpr], the ListExpr is a list of other
	// expressions and can represent a JSON array but its elements can be variables, funcalls, etc.
	ListExpr struct {
		Items []Expr
	}

	// NumberExpr is a number expression that represents a literal number and always evaluates to a
	// float64.
	NumberExpr struct {
		Value float64
	}

	// BoolExpr is a boolean expression that represents a literal boolean and always evaluates to
	// either `true` or `false`.
	BoolExpr struct {
		Value bool
	}

	// StringExpr is a string expression that represents a literal string.
	StringExpr struct {
		Value string
	}

	// VarExpr is a variable expression.
	VarExpr struct {
		Value string
	}

	// FncallExpr is a function expression.
	FncallExpr struct {
		Name string
		Args []Expr
	}

	// PathExpr is a path traversal expression. It represents the process of accessing/addressing
	// inner parts of other expressions.
	// Examples:
	//   myvar.obj.field.test
	//   funcall().test
	//   myvar[0]
	//   myvar[otherval]
	//   myvar[otherval].test
	PathExpr struct {
		Base  Expr
		Steps PathSteps
	}

	// PathStep is a step in a PathExpr. A step can be either of [FieldStep] or [IndexStep] type.
	// NOTE(i4k): a tagged union is used for performance reasons.
	PathStep struct {
		Type  StepType
		Field string // .<field_name>
		Index Expr   // [<expr>]
	}

	// StepType is the type of the path expression step.
	StepType int

	// PathSteps is a list of path expression steps.
	PathSteps []PathStep

	// QueryExpr is an expression that represents a query/filter. It's not widely supported
	// inside other expressions but in specific places like `WHERE` clause and AGGS block.
	QueryExpr struct {
		Type QueryType

		// NOTE(i4k): uses a pointer because query rewriting heavily depends on appends
		// and then otherwise it copies too much the Query struct.
		Children []*QueryExpr

		// Fields below are only set if QueryType == predicate

		LHS StaticPath
		RHS Expr
		OP  Predicate

		// For Range predicate, use bounds (only one of these two is required)
		Lower Bound
		Upper Bound
	}
)

// Sort values
const (
	ASC Sort = iota
	DESC
)

// Query nodes
const (
	predicate QueryType = iota
	OR
	AND
	NOT
)

// predicates
const (
	Eq Predicate = iota
	Match
	In
	Exists
	Missing
	Range
	Gte
	Gt
	Lte
	Lt
)

// path steps
const (
	invalidStep StepType = iota
	FieldStep
	IndexStep
)

// IsRange tells if the predicate is a range.
func (op Predicate) IsRange() bool {
	switch op {
	case Gte, Gt, Lte, Lt:
		return true
	default:
		return false
	}
}

func (op Predicate) String() string {
	switch op {
	default:
		return fmt.Sprintf("<unknown predicate %d>", op)
	case Eq:
		return "$eq"
	case In:
		return "$in"
	case Exists:
		return "$exists"
	case Missing:
		return "$missing"
	case Match:
		return "$match"
	case Range:
		return "$range"
	case Gte:
		return "$gte"
	case Gt:
		return "$gt"
	case Lte:
		return "$lte"
	case Lt:
		return "$lt"
	}
}

func (o OrderBy) String() string {
	return strings.Join(o.Field, ".") + " " + o.Sort.String()
}

func (s Sort) String() string {
	switch s {
	case ASC:
		return "ASC"
	case DESC:
		return "DESC"
	default:
		return "<INVALID>"
	}
}
