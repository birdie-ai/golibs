package event_test

import (
	"context"

	"github.com/birdie-ai/golibs/event"
)

// orderedPublisher is used to test that our ordered publisher implement the same interface.
type orderedPublisher[T any] interface {
	Publish(ctx context.Context, event T, orderingKey string) error
}

// Contains all our ordered publishers, ensure they implement the same interface.
var (
	_ orderedPublisher[any] = &event.OrderedGooglePublisher[any]{}
)
