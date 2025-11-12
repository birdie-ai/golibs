package dml

import "unique"

type (
	// Stmt is a single bdml statement.
	Stmt struct {
		Entity unique.Handle[string]
		Op     OpKind
		Assign Assign
		Where  Clauses
	}

	// Stmts is a list of statements.
	Stmts []Stmt

	// Assign is the field assignments of the operation.
	// If the key is a dot (".") then it MUST be the only assignment.
	Assign map[string]any

	// OpKind is the intended operation kind: SET | DELETE
	OpKind string

	// Clauses is a AND-based list of clause.
	Clauses []Clause

	// Clause is a filter predicate.
	Clause struct {
		Field string
		Op    LogicalOperator
		Value any
	}

	// LogicalOperator is a logical operator. Eg.: "=", "!="
	LogicalOperator string
)

// As we will process large bulks of statements, this ensures we don't waste memory in redundant information.
var (
	SET    = OpKind("SET")
	DELETE = OpKind("DELETE")
	Eq     = LogicalOperator("=")
	Neq    = LogicalOperator("!=")
)
