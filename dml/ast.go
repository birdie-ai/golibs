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

type arrayOp int

const (
	invalid arrayOp = iota
	appendOp
	prependOp
)

// array interface is only needed to bypass a Go type system limitation.
// The builtin len(v) is a special kind of generic that works on generic collections seamless
// but us mere mortals lack such powerfulness. If you have an `a any` variable at hand, you cannot
// type assert/check for an specific struct shape and you cannot also have a generic function using
// pattern matching:
//
//	func oplen[T ~struct { Values []_ }](v T) int { return len(v.Values) }
//
// Note the _ above, the slice item type is irrelevant.
type array interface {
	len() int
	op() arrayOp
	vals() any
}

func (a Append[T]) len() int  { return len(a.Values) }
func (a Prepend[T]) len() int { return len(a.Values) }

func (a Append[T]) op() arrayOp  { return appendOp }
func (a Prepend[T]) op() arrayOp { return prependOp }

func (a Append[T]) vals() any  { return a.Values }
func (a Prepend[T]) vals() any { return a.Values }

var (
	_ array = Append[any]{}
	_ array = Prepend[any]{}
)
