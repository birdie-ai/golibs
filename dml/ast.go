package dml

import "unique"

type (
	// Stmt is a single dml statement.
	// A statement manipulates fields of a single entity row.
	// The [Stmt.Assign] assigns manipulation
	Stmt struct {
		Entity unique.Handle[string]
		Op     OpKind
		Assign Assign
		Where  Where
	}

	// Stmts is a list of statements.
	Stmts []Stmt

	// Assign is the field assignments of the operation.
	// The meaning of the assignment depends on the stmt operation.
	// If the key is a dot (".") then it MUST be the only assignment.
	Assign map[string]any

	// OpKind is the intended operation kind: SET | DELETE
	OpKind string

	// Where clause of the update.
	Where map[string]any

	// Append is an assign operation to append values.
	Append[T any] struct {
		Values []T
	}

	// Prepend is an assign operation to prepend values.
	Prepend[T any] struct {
		Values []T
	}
)

// As we will process large bulks of statements, this ensures we don't waste memory in redundant information.
var (
	SET    = OpKind("SET")
	DELETE = OpKind("DELETE")
)
