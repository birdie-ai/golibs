// Package xjson extends Go's [json] in order to make it easier to handle
// dynamic JSON building/traversal and some other niceties.
package xjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"strings"
)

type (
	// Obj represents a dynamic JSON object.
	// Use [DynGet] to manipulate it more easily.
	Obj = map[string]any

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
	ErrNotFound = errors.New("traversing JSON: key not found")

	// ErrInvalidPath indicates that a traversal path is invalid.
	ErrInvalidPath = errors.New("JSON traversal path is invalid")
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
	var z T
	key, leaf, err := traverse(o, path)
	if err != nil {
		return z, err
	}

	anyV, ok := leaf[key]
	if !ok {
		return z, fmt.Errorf("%w: %q", ErrNotFound, key)
	}

	v, ok := anyV.(T)
	if !ok {
		return z, fmt.Errorf("value at path %q: expected to have type %T but has %T", path, z, anyV)
	}

	return v, nil
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
	if o == nil {
		return fmt.Errorf("can't set %q on nil object", path)
	}
	segments := parseSegments(path)
	if len(segments) == 0 {
		return fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}

	traversalSegments := segments[:len(segments)-1]
	leafSegment := segments[len(segments)-1]

	leafNode := createPath(o, traversalSegments)
	leafNode[leafSegment] = value
	return nil
}

func createPath(o Obj, segments []string) Obj {
	node := o
	for _, segment := range segments {
		anyV := node[segment]
		v, ok := anyV.(Obj)
		if !ok {
			v := Obj{}
			node[segment] = v
			node = v
			continue
		}
		node = v
	}
	return node
}

func traverse(o Obj, path string) (string, Obj, error) {
	segments := parseSegments(path)
	if len(segments) == 0 {
		return "", nil, fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	traverseSegments := segments[0 : len(segments)-1]
	node := o

	for i, key := range traverseSegments {
		anyV, ok := node[key]
		if !ok {
			traversed := strings.Join(segments[:i+1], ".")
			return "", nil, fmt.Errorf("%w: %q", ErrNotFound, traversed)
		}
		v, ok := anyV.(Obj)
		if !ok {
			traversed := strings.Join(segments[:i+1], ".")
			return "", nil, fmt.Errorf("traversing path %q: at: %q: want object got %T", path, traversed, anyV)
		}
		node = v
	}

	key := segments[len(segments)-1]
	return key, node, nil
}

func parseSegments(path string) []string {
	var (
		segments       []string
		currentSegment []rune
		quoted         bool
		previous       rune
	)

	for _, r := range path {
		switch {
		case r == '.' && !quoted:
			if len(currentSegment) == 0 {
				// dot must be preceded by something, this is invalid.
				return nil
			}
			segments = append(segments, string(currentSegment))
			currentSegment = nil
		case r == '"' && previous != '\\':
			// TODO(katcipis): handle things like """.
			quoted = !quoted
		case r == '"' && previous == '\\':
			// When we escape " we need to remove the escape from the key
			currentSegment = currentSegment[:len(currentSegment)-1]
			currentSegment = append(currentSegment, r)
		default:
			currentSegment = append(currentSegment, r)
		}
		previous = r
	}
	if len(currentSegment) == 0 {
		// This means empty or something like "name." which is invalid.
		return nil
	}
	segments = append(segments, string(currentSegment))
	return segments
}
