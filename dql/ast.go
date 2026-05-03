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

// Expr nodes
// NOTE(i4k): all expr nodes are struct mostly because we would like to add a
// `SourceRange` or `Span` kind of field containing the range of Pos.
type (
	ObjectExpr struct {
		Keyvals map[string]Expr
	}

	ListExpr struct {
		Items []Expr
	}

	NumberExpr struct {
		Value float64
	}

	BoolExpr struct {
		Value bool
	}

	StringExpr struct {
		Value string
	}

	VarExpr struct {
		Value string
	}

	FncallExpr struct {
		Name string
		Args []Expr
	}

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

	PathExpr struct {
		Base  Expr
		Steps PathSteps
	}

	// tagged union for performance reasons

	PathStep struct {
		Type  StepType
		Field string // .<field_name>
		Index Expr   // [<expr>]
	}

	StepType int

	PathSteps []PathStep
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
