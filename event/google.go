package event

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub/v2"
)

// GooglePublisher is an ordered google publisher.
type GooglePublisher[T any] struct {
	eventName string
	client    *pubsub.Client
	publisher *pubsub.Publisher
}

// NewGooglePublisher creates a new ordered Google Cloud event publisher for the given project/topic/event name.
// We need a specific Google publisher because ordering doesn't generalize well.
// All ordered publishers should implement [OrderedPublisher].
func NewGooglePublisher[T any](ctx context.Context, project, topic, eventName string) (*GooglePublisher[T], error) {
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating pubsub client: %w", err)
	}
	publisher := client.Publisher(topic)
	return &GooglePublisher[T]{eventName: eventName, client: client, publisher: publisher}, nil
}

// Publish will publish the given event with the given ordering key.
func (p *GooglePublisher[T]) Publish(ctx context.Context, orderingKey string, event T) error {
	return nil
}
