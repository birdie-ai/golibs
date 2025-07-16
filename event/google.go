package event

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
)

// GooglePublisher is an ordered google publisher.
type GooglePublisher[T any] struct {
	eventName string
	client    *pubsub.Client
	topic     *pubsub.Topic
}

// NewGooglePublisher creates a new ordered Google Cloud event publisher for the given project/topic/event name.
// We need a specific Google publisher because ordering doesn't generalize well.
// All ordered publishers should implement [OrderedPublisher].
func NewGooglePublisher[T any](ctx context.Context, project, topicName, eventName string) (*GooglePublisher[T], error) {
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating pubsub client: %w", err)
	}
	topic := client.Topic(topicName)
	topic.EnableMessageOrdering = true
	return &GooglePublisher[T]{eventName: eventName, client: client, topic: topic}, nil
}

// Publish will publish the given event with the given ordering key.
func (p *GooglePublisher[T]) Publish(ctx context.Context, event T, orderingKey string) error {
	encBody, err := serializeEvent(ctx, p.eventName, event)
	if err != nil {
		return err
	}

	sample := publishSampler()
	res := p.topic.Publish(ctx, &pubsub.Message{
		OrderingKey: orderingKey,
		Data:        encBody,
	})
	_, err = res.Get(ctx)
	sample(p.eventName, len(encBody), err)

	return err
}
