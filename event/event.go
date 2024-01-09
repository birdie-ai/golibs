// Package event provides functionality for publish/suscribe of events.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/birdie-ai/golibs/slog"
	"github.com/birdie-ai/golibs/tracing"
	"github.com/google/uuid"
	"gocloud.dev/pubsub"
)

type (
	// Publisher represents a publisher of events of type T.
	// The publisher guarantees that the events conform to our basic schema for events.
	Publisher[T any] struct {
		name  string
		topic *pubsub.Topic
	}

	// Envelope represents the structure of all data that wraps all events.
	Envelope[T any] struct {
		// Metadata is publisher specific metadata. Including metadata that may be added by specific
		// pubsub brokers like Google Cloud PubSub.
		Metadata map[string]string
		TraceID  string `json:"trace_id"`
		OrgID    string `json:"organization_id"`
		Name     string `json:"name"`
		Event    T      `json:"event"`
	}

	// Subscription is a subscription that received only specific types of events
	// defined by T.
	Subscription[T any] struct {
		name   string
		rawsub *MessageSubscription
	}

	// Handler is responsible for handling events from a [Subscription].
	// The context passed to the handler will have all metadata relevant to that
	// event like org and trace IDs. It will also contain a logger that can be retrieved
	// by using [slog.FromCtx].
	Handler[T any] func(context.Context, T) error

	// Message represents a raw message received on a subscription.
	Message struct {
		body     []byte
		metadata map[string]string
	}

	// MessageSubscription represents a subscription that delivers messages as is.
	// No assumptions are made about the message contents. This should rarely be used in favor of [Subscription].
	MessageSubscription struct {
		sub            *pubsub.Subscription
		maxConcurrency int
	}

	// MessageHandler is responsible for handling messages from a [MsgSubscription].
	MessageHandler func(Message) error
)

// NewPublisher creates a new event publisher for the given event name and topic.
func NewPublisher[T any](name string, t *pubsub.Topic) *Publisher[T] {
	return &Publisher[T]{
		name:  name,
		topic: t,
	}
}

// Publish will publish the given event.
func (p *Publisher[T]) Publish(ctx context.Context, event T) error {
	start := time.Now()

	body := Envelope[T]{
		TraceID: tracing.CtxGetTraceID(ctx),
		OrgID:   tracing.CtxGetOrgID(ctx),
		Name:    p.name,
		Event:   event,
	}

	encBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	err = p.topic.Send(ctx, &pubsub.Message{
		Body: encBody,
	})

	elapsed := time.Since(start)

	samplePublish(p.name, elapsed, err)

	return err
}

// NewSubscription creates a subscription that will accept on events of the given type and name.
func NewSubscription[T any](name, url string, maxConcurrency int) (*Subscription[T], error) {
	rawsub, err := NewRawSubscription(url, maxConcurrency)
	if err != nil {
		return nil, err
	}
	return &Subscription[T]{
		name:   name,
		rawsub: rawsub,
	}, nil
}

// NewRawSubscription creates a new raw subscription. It provides messages in a
// service like manner (serve) and manages concurrent execution, each message
// is processed in its own goroutines respecting the given maxConcurrency.
func NewRawSubscription(url string, maxConcurrency int) (*MessageSubscription, error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("max concurrency must be > 0: %d", maxConcurrency)
	}
	// We dont want the subscription to expire, so we use the background context.
	sub, err := pubsub.OpenSubscription(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &MessageSubscription{
		sub:            sub,
		maxConcurrency: maxConcurrency,
	}, nil
}

// Serve will start serving all events from the subscription calling handler for each
// event. It will run until [Subscription.Shutdown] is called.
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler Unack will be sent.
// If a received event is not a valid JSON it will be discarded as malformed and a Nack will be sent automatically.
// If a received event has the wrong name it will be discarded as malformed and a Nack will be sent automatically.
// Serve may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" goroutines.
func (s *Subscription[T]) Serve(handler Handler[T]) error {
	return s.rawsub.Serve(SampledMessageHandler(func(msg Message) error {
		var event Envelope[T]

		ctx := context.Background()
		log := slog.FromCtx(ctx)

		if err := json.Unmarshal(msg.Body(), &event); err != nil {
			log.Error("unable to parse event as JSON", "error", err, "event", msg)
			return fmt.Errorf("parsing event as JSON, event: %v, error: %v", msg, err)
		}

		if event.Name != s.name {
			log.Error("event name doesn't match handler", "expected", s.name, "received", event.Name)
			return fmt.Errorf("event name doesn't match %q: event: %v", s.name, msg)
		}

		event.Metadata = msg.Metadata()

		ctx = tracing.CtxWithTraceID(ctx, event.TraceID)
		ctx = tracing.CtxWithOrgID(ctx, event.OrgID)

		log = log.With("request_id", uuid.NewString())
		log = log.With("trace_id", event.TraceID)
		log = log.With("organization_id", event.OrgID)
		ctx = slog.NewContext(ctx, log)

		return handler(ctx, event.Event)
	}, s.name))
}

// Shutdown will shutdown the subscriber, stopping any calls to [Subscription.Serve].
// The subscription should not be used after this method is called.
func (s *Subscription[T]) Shutdown(ctx context.Context) error {
	return s.rawsub.Shutdown(ctx)
}

// Serve will start serving all messages from the subscription calling handler for each
// message. It will run until [MessageSubscription.Shutdown] is called.
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler Unack will be sent.
// Serve may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" goroutines.
func (r *MessageSubscription) Serve(handler MessageHandler) error {
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

			err := handler(NewMessage(msg.Body, msg.Metadata))
			if err != nil {
				if msg.Nackable() {
					msg.Nack()
				}
				return
			}
			msg.Ack()
		}()
	}
}

// Shutdown will shutdown the subscriber, stopping any calls to [MessageSubscription.Serve].
// The subscription should not be used after this method is called.
func (r *MessageSubscription) Shutdown(ctx context.Context) error {
	return r.sub.Shutdown(ctx)
}

// NewMessage creates a new [Message] with the given body
func NewMessage(body []byte, metadata map[string]string) Message {
	return Message{body, metadata}
}

// Body of the message.
func (m Message) Body() []byte {
	return m.body
}

// Metadata of the message.
func (m Message) Metadata() map[string]string {
	return m.metadata
}

// String representation of the message.
func (m Message) String() string {
	return string(m.body)
}
