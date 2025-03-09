// Package xjson extends Go's [json] in order to make it easier to handle
// dynamic JSON building/traversal and some other niceties.
package xjson

import (
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"strings"
)

type (
	// Obj represents a dynamic JSON object.
	// Use [DynGet] to manipulate it more easily.
	Obj = map[string]any

	// Decoder specializes the [json.Decoder] for streams of objects of the same type
	// (although you can create a [Decoder] with type [Object], then it is dynamic),
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

// Unmarshal calls [json.Unmarshal] after reading the given reader into memory
// and returns the unmarshalled value.
// If you need more details, like the data that was read when an unmarshalling error happened,
// you can:
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
// and an error is returned. If any of them are objects but traversal can't go on
// because a key is absent, no error is returned, just false indicating that the data is not there.
// Once traversal is finished, then "nested3" must match the given type [T].
func DynGet[T any](o Obj, path string) (T, bool, error) {
	var z T
	key, leaf, err := traverse(o, path)
	if err != nil {
		return z, false, err
	}

	anyV, ok := leaf[key]
	if !ok {
		return z, false, nil
	}

	v, ok := anyV.(T)
	if !ok {
		return z, false, fmt.Errorf("value at path %q: expected to have type %T but has %T", path, z, anyV)
	}

	return v, true, nil
}

func traverse(o Obj, path string) (string, Obj, error) {
	parsedPath := strings.Split(path, ".")
	traversePath := parsedPath[0 : len(parsedPath)-1]
	leaf := o

	for i, key := range traversePath {
		anyV, ok := leaf[key]
		if !ok {
			return "", nil, nil
		}
		v, ok := anyV.(Obj)
		if !ok {
			traversed := strings.Join(parsedPath[:i+1], ".")
			return "", nil, fmt.Errorf("traversing path %q: at: %q: want object got %T", path, traversed, anyV)
		}
		leaf = v
	}

	key := parsedPath[len(parsedPath)-1]
	return key, leaf, nil
}
