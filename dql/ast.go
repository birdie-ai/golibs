package dql

// basic statement shape
type (
	Stmt struct {
		Name    string
		Entity  string
		Fields  map[string]any
		Where   map[string]any
		Limit   int
		OrderBy OrderBy
		Aggs    map[string]Agg

		// TODO(i4k): add Span
	}

	Return struct {
		Format string
		Expr   Expr
	}

	Expr interface {
		Variables() []VarExpr
	}

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

	Stmts []Stmt
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

	FloatExpr struct {
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
)

// Sort values
const (
	ASC  Sort = "ASC"
	DESC Sort = "DESC"
)
