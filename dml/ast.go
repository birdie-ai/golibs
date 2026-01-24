package dml

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unique"
)

type (
	// Stmt is a single dml statement.
	// A statement manipulates fields of a single entity row.
	// The [Stmt.Assign] assigns data manipulation operations to individual fields.
	Stmt struct {
		Entity unique.Handle[string]
		Op     OpKind
		Assign Assign
		Where  Where
	}

	// Stmts is a list of statements.
	Stmts []Stmt

	// Assign assigns field manipulations.
	// The meaning of the assignment depends on the stmt operation kind.
	//
	// If the key is a dot (".") then it MUST be the only assignment.
	//
	// When stmt.Op == "SET", the assignment value can be any of:
	// - [Primtype]
	// - [Colltype]
	// - [Append[Primtype]]
	// - [Prepend[Primtype]]
	// and the provided value is used to set the target field (the assign key).
	//
	// When stmt.Op == "DELETE", the assignment value can be any of:
	// - [KeyFilter]
	// - [ValueFilter[Primtype]]
	// - [KeyValueFilter[Primtype]]
	// and the provided value is used to select the records to be deleted.
	//
	// Note: we are aware that "assign" name is a bit misleading and it was an
	// oversight. Now it holds the data manipulations intended for each field,
	// be it a SET or a DELETE.
	Assign map[string]any

	// OpKind is the intended operation kind: SET | DELETE
	OpKind string

	// Where clause of the update.
	Where map[string]any

	// Primtype is a constraint for the primitive types supported in dml.
	Primtype interface {
		~float64 | ~string | ~bool
	}

	// Colltype is a constraint for the collection types supported in dml.
	Colltype interface {
		~[]any | ~map[string]any
	}

	// Append is an assign operation to append values.
	Append[T Primtype | Colltype] struct {
		Values []T
	}

	// Prepend is an assign operation to prepend values.
	Prepend[T Primtype | Colltype] struct {
		Values []T
	}

	// KeyFilter query a hash-map collection by keys.
	KeyFilter struct {
		Keys []string
	}

	// ValueFilter query a collection by value.
	ValueFilter[T Primtype] struct {
		Values []T
	}

	// KeyValueFilter query a collection by key and value.
	KeyValueFilter[T Primtype] struct {
		Key    string
		Values []T
	}
)

// As we will process large bulks of statements, this ensures we don't waste memory in redundant information.
var (
	SET    = OpKind("SET")
	DELETE = OpKind("DELETE")
)

// dml errors.
var (
	ErrInvalidOperation      = errors.New("invalid operation")
	ErrMissingEntity         = errors.New(`entity is not provided`)
	ErrMissingAssign         = errors.New(`"SET" requires an assign`)
	ErrMissingArrayValues    = errors.New(`...: missing array values`)
	ErrUnsupportedArrayValue = errors.New(`unsupported array values`)
	ErrArrayWithMixedTypes   = errors.New(`array items with mixed types`)
	ErrInvalidAssignKey      = errors.New(`invalid assign key`)
	ErrMissingWhereClause    = errors.New(`WHERE clause is not given`)
	ErrNotIdent              = errors.New(`not an identifier`)

	// delete specific errors
	ErrDelEmptyFilterKeys        = errors.New(`missing keys in filter`)
	ErrDelEmptyFilterValues      = errors.New(`missing values in filter`)
	ErrDelInvalidFilterKeyValues = errors.New(`invalid keyvalue filter`)
	ErrDelInvalidAssign          = errors.New(`invalid DELETE assign`)
)

type arrayOp int

const (
	invalid arrayOp = iota
	appendOp
	prependOp
)

// array interface is only needed to bypass a Go type system limitation.
// If you have an `a any` variable at hand, you cannot type assert/check for an specific struct
// shape.
// At the moment this is used by array-like operations: append/prepend.
type array interface {
	len() int
	op() arrayOp
	vals() any
}

func (a Append[T]) len() int      { return len(a.Values) }
func (a Prepend[T]) len() int     { return len(a.Values) }
func (a KeyFilter) len() int      { return len(a.Keys) }
func (a ValueFilter[T]) len() int { return len(a.Values) }

func (a Append[T]) op() arrayOp  { return appendOp }
func (a Prepend[T]) op() arrayOp { return prependOp }

func (a Append[T]) vals() any      { return a.Values }
func (a Prepend[T]) vals() any     { return a.Values }
func (a KeyFilter) vals() any      { return a.Keys }
func (a ValueFilter[T]) vals() any { return a.Values }

type validator interface {
	validate() error
}

func (a Append[T]) validate() error {
	if a.len() == 0 {
		return fmt.Errorf("append: %w", ErrMissingArrayValues)
	}
	return nil
}

func (a Prepend[T]) validate() error {
	if a.len() == 0 {
		return fmt.Errorf("prepend: %w", ErrMissingArrayValues)
	}
	return nil
}

func (a KeyFilter) validate() error {
	if len(a.Keys) == 0 {
		return fmt.Errorf("delete by key node: %w", ErrMissingArrayValues)
	}
	return nil
}

func (a ValueFilter[T]) validate() error {
	if len(a.Values) == 0 {
		return fmt.Errorf("delete by value node: %w", ErrMissingArrayValues)
	}
	return nil
}

func (a KeyValueFilter[T]) validate() error {
	if a.Key == "" {
		return fmt.Errorf("%w: empty key", ErrDelInvalidFilterKeyValues)
	}
	if len(a.Values) == 0 {
		return fmt.Errorf("%w: empty values list", ErrDelInvalidFilterKeyValues)
	}
	return nil
}

type assignEncoder interface {
	encode(w io.Writer, key string) error
}

func (a KeyFilter) encode(w io.Writer, target string) error {
	if len(a.Keys) == 1 {
		return write(w, target+"."+a.Keys[0])
	}
	d, err := json.Marshal(a.Keys)
	if err != nil {
		return err
	}
	return write(w, "k IN "+target+" WHERE k IN "+string(d))
}

func (a ValueFilter[T]) encode(w io.Writer, target string) error {
	if len(a.Values) == 1 {
		d, err := json.Marshal(a.Values[0])
		if err != nil {
			return err
		}
		return write(w, "_,v IN "+target+" WHERE v="+string(d))
	}
	d, err := json.Marshal(a.Values[0])
	if err != nil {
		return err
	}
	return write(w, "_, v IN "+target+" WHERE v IN "+string(d))
}

func (a KeyValueFilter[T]) encode(w io.Writer, target string) error {
	err := write(w, "k,v IN "+target+" WHERE k="+strconv.Quote(a.Key)+" AND v")
	if err != nil {
		return err
	}
	if len(a.Values) == 1 {
		d, err := json.Marshal(a.Values[0])
		if err != nil {
			return err
		}
		return write(w, "="+string(d))
	}
	d, err := json.Marshal(a.Values)
	if err != nil {
		return err
	}
	return write(w, " IN "+string(d))
}

// ensure Append/Prepend implements array.
var (
	_ array = Append[float64]{}
	_ array = Prepend[float64]{}

	_ validator = Append[float64]{}
	_ validator = Prepend[float64]{}
	_ validator = KeyFilter{}
	_ validator = ValueFilter[float64]{}
	_ validator = KeyValueFilter[float64]{}

	_ assignEncoder = KeyFilter{}
	_ assignEncoder = ValueFilter[float64]{}
	_ assignEncoder = KeyValueFilter[float64]{}
)
