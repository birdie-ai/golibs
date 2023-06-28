// Package event provides functionality for publish/suscribe of events.
package event

import (
	"context"
	"encoding/json"

	"gocloud.dev/pubsub"
)

// Publisher represents a publisher of events of type T.
// The publisher guarantees that the events conform to our basic schema for events.
type Publisher[T any] struct {
	name  string
	topic *pubsub.Topic
}

// Body represents the general structure of the body of events.
type Body[T any] struct {
	TraceID string `json:"trace_id"`
	OrgID   string `json:"organization_id"`
	Name    string `json:"name"`
	Event   T      `json:"event"`
}

// NewPublisher creates a new event publisher for the given event name and topic.
func NewPublisher[T any](name string, t *pubsub.Topic) *Publisher[T] {
	return &Publisher[T]{
		name:  name,
		topic: t,
	}
}

// Publish will publish the given event.
func (p *Publisher[T]) Publish(ctx context.Context, event T) error {
	e := Body[T]{
		Name:  p.name,
		Event: event,
	}

	messageBody, err := json.Marshal(e)
	if err != nil {
		return err
	}

	return p.topic.Send(ctx, &pubsub.Message{
		Body: messageBody,
	})
}
