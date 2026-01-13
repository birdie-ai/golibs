// Package obj provides an easy way to handle dynamic "objects" in Go.
// An object being basically a map[string]any.
package obj

import (
	"errors"
	"fmt"
	"strings"
)

// O represents a dynamic object.
// It is just an alias to avoid typing map[string]any until your fingers bleed.
type O = map[string]any

var (
	// ErrNotFound indicates that an object was not found while traversing a [O].
	ErrNotFound = errors.New("traversing object: key not found")

	// ErrInvalidPath indicates that a traversal path is invalid.
	ErrInvalidPath = errors.New("object traversal path is invalid")
)

// Get traverses the given obj using the given path and returns the value (if any)
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
func Get[T any](o O, path string) (T, error) {
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

// Set traverses the given [O] using the given path and sets it to the given value.
// It will create any necessary intermediate objects as it traverses the path.
// Any keys on the traversal path that already exist and are not an object will be overwritten with an object.
//
// Path is defined using '.' as delimiter like: "key.nested1.nested2.nested3".
//
// Key names with "." can be traversed by using double quotes like:
//   - "key."nested.dot".key2"
//
// It will traverse key -> nested.dot -> key2 and set "key2" to be the given value.
// If the given path is invalid, like "" or "." or the [O] is nil an error is returned.
func Set(o O, path string, value any) error {
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

// Del traverses the given [O] using the given path and deletes the target key.
// Path is defined in the same way as [Get] and [Set].
// Del is tolerant if obj is nil, empty or if the path does not exist, and in such
// cases it does nothing and returns a nil error.
func Del(o O, path string) error {
	if o == nil {
		// no-op if obj does not exist.
		return nil
	}
	key, leaf, err := traverse(o, path)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// no-op if target does not exist.
			return nil
		}
		return err
	}
	delete(leaf, key)
	return nil
}

// IsValidPath returns true if the given path is valid for [DynGet] and [DynSet] operations.
func IsValidPath(path string) bool {
	return len(parseSegments(path)) > 0
}

func createPath(o O, segments []string) O {
	node := o
	for _, segment := range segments {
		anyV := node[segment]
		v, ok := anyV.(O)
		if !ok {
			v := O{}
			node[segment] = v
			node = v
			continue
		}
		node = v
	}
	return node
}

func traverse(o O, path string) (string, O, error) {
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
		v, ok := anyV.(O)
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
