package event_test

import (
	"context"

	"github.com/birdie-ai/golibs/event"
)

type (
	// orderedPublisher is used to test that our ordered publisher implement the same interface.
	orderedPublisher[T any] interface {
		Publish(context.Context, T, string) error
		Resume(context.Context, string) error
		Shutdown(context.Context) error
	}

	// orderedSub is used to test that our ordered subscriptions implement the same interface.
	orderedSub[T any] interface {
		Serve(context.Context, event.Handler[T]) error
		Shutdown(context.Context) error
	}
)

// Contains all our ordered publishers, ensure they implement the same interface.
var (
	_ orderedPublisher[any] = &event.OrderedGooglePublisher[any]{}
	_ orderedSub[any]       = &event.OrderedGoogleSub[any]{}
)
