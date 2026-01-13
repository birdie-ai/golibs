// Package xjson extends Go's [json] in order to make it easier to handle
// dynamic JSON building/traversal and some other niceties.
package xjson

import (
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"os"

	"github.com/birdie-ai/golibs/obj"
)

type (
	// Obj represents a dynamic JSON object.
	// Use [DynGet] to manipulate it more easily.
	Obj = obj.O

	// Decoder specializes the [json.Decoder] for streams of objects of the same type
	// (although you can create a [Decoder] with type [Obj], then it is dynamic),
	// leveraging parametric types and iterators to make things easier.
	// It won't cover all possible scenarios by design, for that you can use Go's [json.Decoder].
	Decoder[T any] struct {
		d   *json.Decoder
		err error
	}

	// UnmarshalError is returned by [Unmarshal] when an unmarshalling error happens.
	UnmarshalError struct {
		// Err is the unmarshalling error (returned by [json.Unmarshal].
		Err error
		// Data is the data that caused the unmarshalling error, useful for debugging.
		Data string
	}
)

var (
	// ErrNotFound indicates that an object was not found while traversing a [Obj].
	ErrNotFound = obj.ErrNotFound

	// ErrInvalidPath indicates that a traversal path is invalid.
	ErrInvalidPath = obj.ErrInvalidPath
)

// UnmarshalFile calls [Unmarshal] with the opened file (closing it afterwards) and returns the unmarshalled value.
// If you need more details, like the data that was read when an unmarshalling error happened, you can:
//
//	var errDetails UnmarshalError
//	if errors.As(err, &errDetails) {
//	    fmt.Println(errDetails.Data)
//	}
func UnmarshalFile[T any](path string) (T, error) {
	var z T
	f, err := os.Open(path)
	if err != nil {
		return z, fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	return Unmarshal[T](f)
}

// Unmarshal calls [json.Unmarshal] after reading the given reader into memory
// and returns the unmarshalled value.
// If you need more details, like the data that was read when an unmarshalling error happened, you can:
//
//	var errDetails UnmarshalError
//	if errors.As(err, &errDetails) {
//	    fmt.Println(errDetails.Data)
//	}
func Unmarshal[T any](v io.Reader) (T, error) {
	var r T
	d, err := io.ReadAll(v)
	if err != nil {
		return r, fmt.Errorf("reading stream: %w", err)
	}
	if err := json.Unmarshal(d, &r); err != nil {
		return r, UnmarshalError{err, string(d)}
	}
	return r, nil
}

// NewDecoder creates a new decoder for type T.
func NewDecoder[T any](r io.Reader) *Decoder[T] {
	return &Decoder[T]{json.NewDecoder(r), nil}
}

// All returns a single-use iterator for the stream.
func (d *Decoder[T]) All() iter.Seq[T] {
	return func(yield func(v T) bool) {
		for d.d.More() {
			var v T
			if err := d.d.Decode(&v); err != nil {
				d.err = err
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}

// Error returns the error that interrupted iteration or nil if no error happened.
func (d *Decoder[T]) Error() error {
	return d.err
}

func (e UnmarshalError) Error() string {
	return e.Err.Error()
}

// DynGet traverses the given obj using the given path and returns the value (if any)
// of type T. If traversal fails, like part of the path is not an object, an error is returned.
// If traversal succeeds but the value type is different an error is returned.
//
// Path is defined using '.' as delimiter like: "key.nested1.nested2.nested3".
// For the aforementioned path "key", "nested1" and "nested2" MUST be objects, or else traversal will fail
// and an error is returned. If the entire path is valid but the last key is not found an [ErrNotFound] is returned.
// Once traversal is finished, then "nested3" must match the given type [T].
//
// Key names with "." can be traversed by using double quotes like:
//   - "key."nested.dot".value"
//
// It will traverse key -> nested.dot -> value.
func DynGet[T any](o Obj, path string) (T, error) {
	return obj.Get[T](o, path)
}

// DynSet traverses the given [Obj] using the given path and sets it to the given value.
// It will create any necessary intermediate objects as it traverses the path.
// Any keys on the traversal path that already exist and are not an object will be overwritten with an object.
//
// Path is defined using '.' as delimiter like: "key.nested1.nested2.nested3".
//
// Key names with "." can be traversed by using double quotes like:
//   - "key."nested.dot".key2"
//
// It will traverse key -> nested.dot -> key2 and set "key2" to be the given value.
// If the given path is invalid, like "" or "." or the [Obj] is nil an error is returned.
func DynSet(o Obj, path string, value any) error {
	return obj.Set(o, path, value)
}

// DynDel traverses the given [Obj] using the given path and deletes the target key.
// Path is defined in the same way as [DynGet] and [DynSet].
// DynDel is tolerant if obj is nil, empty or if the path does not exist, and in such
// cases it does nothing.
func DynDel(o Obj, path string) error {
	return obj.Del(o, path)
}

// IsValidDynPath returns true if the given path is valid for [DynGet] and [DynSet] operations.
func IsValidDynPath(path string) bool {
	return obj.IsValidPath(path)
}
