package dql

// basic statement shape
type (
	// Program is the parsed script.
	Program struct {
		Stmts  Stmts
		Return Return
	}

	// Stmt is a dql statement node.
	Stmt struct {
		Name    string
		Entity  string
		Fields  []Expr
		Where   *Query
		Limit   int
		OrderBy OrderBy
		Aggs    map[string]Agg

		// TODO(i4k): add Span
	}

	Stmts []Stmt

	// Return is a dql RETURN node.
	// The should be only one RETURN node per script.
	Return struct {
		Format string
		Expr   Expr
	}

	// Expr is an expression tree.
	Expr interface {
		Variables() []VarExpr
	}

	Query struct {
		Type QueryNode

		// NOTE(i4k): uses a pointer because query rewriting heavily depends on appends
		// and then otherwise it copies too much the Query struct.
		Children []*Query
		LHS      StaticPath
		RHS      Expr
		OP       Predicate
	}

	StaticPath []string

	QueryNode int

	Predicate int

	Agg struct {
		By       any
		Size     int
		Children map[string]Agg
	}

	OrderBy struct {
		Field string
		Sort  Sort
	}

	Sort string
)

// aggs nodes
type (
	// DistinctAgg maps to either `SELECT DISTINCT(...)` or `ES terms(...)`.
	DistinctAgg struct {
		Field string
	}

	FilterAgg struct {
		Where map[string]any
	}

	HistogramAgg struct {
		Field    string
		Interval int
	}

	// TODO(i4k): add other aggs.
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
	ASC  Sort = "ASC"
	DESC Sort = "DESC"
)

// Query nodes
const (
	predicate QueryNode = iota
	OR
	AND
)

// predicates
const (
	Eq Predicate = iota
	Match
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
