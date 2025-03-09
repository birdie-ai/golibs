// Package xjson extends Go's [json] in order to make it easier to handle
// dynamic JSON building/traversal and some other niceties.
package xjson

import (
	"encoding/json"
	"fmt"
	"io"
	"iter"
)

type (
	// Object represents a dynamic JSON object.
	// Use [DynGet] and [DynSet] to manipulate it more easily.
	Object map[string]any

	// Decoder specializes the [json.Decoder] for streams of objects of the same type
	// (although you can create a [Decoder] with type [Object], then it is dynamic),
	// leveraging parametric types and iterators to make things easier.
	// It won't cover all possible scenarios by design, for that you can use
	// Go's [json.Decoder].
	Decoder[T any] struct {
		d   *json.Decoder
		err error
	}

	// UnmarshalError might be returned by [Unmarshal] when an unmarshalling error happens.
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
