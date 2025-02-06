// Package xerrgroup extends [errgroup] by providing a way to collect results from subtasks.
// It has a very similar API and as much as possible the exact same behavior as [errgroup].
package xerrgroup

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Group behaves like a [errgroup.Group] but collects results from subtasks
// Use [New] or [WithContext] to create a [Group] (zero value is invalid).
type Group[T any] struct {
	mu    *sync.Mutex
	group *errgroup.Group
	vals  []T
}

// New creates a new [Group].
func New[T any]() *Group[T] {
	return &Group[T]{
		mu:    &sync.Mutex{},
		group: &errgroup.Group{},
	}
}

// WithContext behaves like [errgroup.WithContext].
func WithContext[T any](ctx context.Context) (*Group[T], context.Context) {
	g, ctx := errgroup.WithContext(ctx)
	return &Group[T]{
		mu:    &sync.Mutex{},
		group: g,
	}, ctx
}

// Wait blocks until all function calls from the Go method have returned, then returns the first non-nil error (if any) from them.
// It will collect the results of each function call and return it as a slice.
// In case an error happened in one of the subtasks partial results are possible and the slice may not be empty.
// It is the caller responsibility to decide if a partial result is acceptable or just fail the entire task because some subtask failed.
func (g *Group[T]) Wait() ([]T, error) {
	err := g.group.Wait()
	v := g.vals
	g.vals = nil
	return v, err
}

// SetLimit limits the number of active goroutines in this group to at most n.
// A negative value indicates no limit.
//
// Any subsequent call to the Go method will block until it can add an active
// goroutine without exceeding the configured limit.
//
// The limit must not be modified while any goroutines in the group are active.
func (g *Group[T]) SetLimit(n int) {
	g.group.SetLimit(n)
}

// Go calls the given function in a new goroutine. It blocks until the new
// goroutine can be added without the number of active goroutines in the group
// exceeding the configured limit.
//
// If the function returns a nil error the returned value will be collected and returned on the [Group.Wait] call.
// If the function returns an error the returned value won't be collected.
func (g *Group[T]) Go(f func() (T, error)) {
	g.group.Go(func() error {
		v, err := f()
		if err != nil {
			return err
		}
		g.mu.Lock()
		g.vals = append(g.vals, v)
		g.mu.Unlock()
		return err
	})
}
