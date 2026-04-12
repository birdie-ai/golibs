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
		Where   *QueryExpr
		Limit   int
		OrderBy OrderBy
		Aggs    Aggs

		// TODO(i4k): add Span
	}

	Stmts []Stmt
	Aggs  map[string]Agg

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

	StaticPath []string

	QueryNode int

	Predicate int

	// tagged union for performande reasons.
	Agg struct {
		Name    string
		Func    FncallExpr
		Limit   *int
		After   string // base64 cursor
		OrderBy []OrderBy

		Children Aggs
	}

	OrderBy struct {
		Field string
		Sort  Sort
	}

	Sort string
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
		Type QueryNode

		// NOTE(i4k): uses a pointer because query rewriting heavily depends on appends
		// and then otherwise it copies too much the Query struct.
		Children []*QueryExpr
		LHS      StaticPath
		RHS      Expr
		OP       Predicate
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
	NOT
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
