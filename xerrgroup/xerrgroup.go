// Package xerrgroup extends [errgroup] by providing a way to collect results from subtasks.
// It has a very similar API and as much as possible the exact same behavior as [errgroup].
package xerrgroup

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

// Group behaves like a [errgroup.Group] but collects results from subtasks
// A [Group] must not be copied after first use.
type Group[T any] struct {
	mu   sync.Mutex
	vals []T
	g    errgroup.Group
}

// Wait blocks until all function calls from the Go method have returned, then returns the first non-nil error (if any) from them.
// It will collect the results of each function call and return it as a slice.
// In case an error happened in one of the subtasks partial results are possible and the slice may not be empty.
// It is the caller responsibility to decide if a partial result is acceptable or just fail the entire task because some subtask failed.
func (g *Group[T]) Wait() ([]T, error) {
	return nil, nil
}

// Go calls the given function in a new goroutine. It blocks until the new
// goroutine can be added without the number of active goroutines in the group
// exceeding the configured limit.
//
// If the function returns a nil error the returned value will be collected and returned on the [Group.Wait] call.
// If the function returns an error the returned value won't be collected.
//
// The first call to return a non-nil error cancels the group's context,
// if the group was created by calling WithContext. The error will be returned
// by Wait.
func (g *Group[T]) Go(func() (T, error)) {
}
