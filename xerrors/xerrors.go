// Package xerrors extends Go's stdlib errors pkg.
package xerrors

import "errors"

// Tag tags the given error with the given error tag.
// This is very similar to wrapping with one crucial difference, the tag error message
// won't be present on the original err, but calling errors.Is(err, tag) will return true.
//
// This is useful when you want to tag an error as an specific kind of error but
// you don't want to change the error message. This happens when the tag/sentinel error
// information is redundant (like a "not found" generic sentinel), or to avoid duplicated information on error messages
// when multiple layers may tag an error with the same "kind/type".
//
// Calling [errors.As] to retrieve the error tag will also work.
// Calls to [errors.As] and [errors.Is] will be dispatched to the tag first
// and then fallback to the original error if they don't match the tag.
func Tag(err, tag error) error {
	return tagged{err, tag}
}

type tagged struct {
	err error
	tag error
}

func (t tagged) Is(target error) bool {
	if errors.Is(t.tag, target) {
		return true
	}
	return errors.Is(t.err, target)
}

func (t tagged) As(target any) bool {
	if errors.As(t.tag, target) {
		return true
	}
	return errors.As(t.err, target)
}

func (t tagged) Error() string {
	return t.err.Error()
}
