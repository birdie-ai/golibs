// Package event provides functionality for publish/suscribe of events.
package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/birdie-ai/golibs/tracing"
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
	body := Body[T]{
		TraceID: tracing.CtxGetTraceID(ctx),
		OrgID:   tracing.CtxGetOrgID(ctx),
		Name:    p.name,
		Event:   event,
	}

	encBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	return p.topic.Send(ctx, &pubsub.Message{
		Body: encBody,
	})
}

// RawSubscription represents a subscription that delivers messages as is.
// No assumptions are made about the message contents. This should rarely be used in favor of [Subscription].
type RawSubscription struct {
	sub            *pubsub.Subscription
	maxConcurrency int
}

// RawMessageHandler is responsible for handling raw messages from a subscription.
type RawMessageHandler func([]byte) error

// NewRawSubscription creates a new raw subscription. It provides messages in a
// service like manner (serve) and manages concurrent execution, each message
// is processed in its own goroutines respecting the given maxConcurrency.
func NewRawSubscription(url string, maxConcurrency int) (*RawSubscription, error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("max concurrency must be > 0: %d", maxConcurrency)
	}
	// We dont want the subscription to expire, so we use the background context.
	sub, err := pubsub.OpenSubscription(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &RawSubscription{
		sub:            sub,
		maxConcurrency: maxConcurrency,
	}, nil
}

// Serve will start serving all messages from the subscription calling handler for each
// message. It will run until [RawSubscription.Shutdown] is called.
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler Unack will be sent.
// Serve may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" goroutines.
func (r *RawSubscription) Serve(handler RawMessageHandler) error {
	semaphore := make(chan struct{}, r.maxConcurrency)
	for {
		semaphore <- struct{}{}
		msg, err := r.sub.Receive(context.Background())
		if err != nil {
			// From: https://pkg.go.dev/gocloud.dev@v0.30.0/pubsub#example-Subscription.Receive-Concurrent
			// Errors from Receive indicate that Receive will no longer succeed.
			return fmt.Errorf("receive from subscription failed, stopping serving: %v", err)
		}
		go func() {
			defer func() {
				<-semaphore
			}()
			err := handler(msg.Body)
			if err == nil {
				msg.Ack()
			} else {
				msg.Nack()
			}
		}()
	}
}

// Shutdown will shutdown the subscriber, stopping any calls to [RawSubscription.Serve].
// The subscription should not be used after this method is called.
func (r *RawSubscription) Shutdown(ctx context.Context) error {
	return r.sub.Shutdown(ctx)
}
