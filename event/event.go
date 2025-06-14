// Package event provides functionality for publish/subscribe of events.
package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"github.com/birdie-ai/golibs/slog"
	"github.com/birdie-ai/golibs/tracing"
	"github.com/google/uuid"
	"gocloud.dev/pubsub"
)

type (
	// Publisher represents a publisher of events of type [T].
	// The publisher guarantees that the events conform to our basic schema for events.
	Publisher[T any] struct {
		name  string
		topic *pubsub.Topic
	}

	// Event represents the structure of all data that wraps all events, like the [Envelope], but
	// but with Ack/Nack. After the [Event] is handled [Event.Ack] or [Event.Nack] must be called.
	// This type is used when receiving individual events with [Subscription.Receive].
	Event[T any] struct {
		Envelope[T]
		msg *message
	}

	// Envelope represents the structure of all data that wraps all events.
	Envelope[T any] struct {
		TraceID string `json:"trace_id,omitempty"`
		OrgID   string `json:"organization_id"`
		Name    string `json:"name"`
		Event   T      `json:"event"`
	}

	// Subscription is a subscription that received only specific types of events
	// defined by [T].
	Subscription[T any] struct {
		name   string
		rawsub *MessageSubscription
	}

	// Handler is responsible for handling events from a [Subscription].
	// The context passed to the handler will have all metadata relevant to that
	// event like org and trace IDs. It will also contain a logger that can be retrieved
	// by using [slog.FromCtx].
	Handler[T any] func(context.Context, T) error

	// HandlerWithMetadata is responsible for handling events from a [Subscription] with its associated [Metadata].
	// The context passed to the handler will have the same general metadata as the ones passed to [Handler], like the trace ID,
	// and extra metadata that is more event specific as defined by [Metadata].
	HandlerWithMetadata[T any] func(context.Context, T, Metadata) error

	// Message represents a raw message received on a subscription.
	Message struct {
		Body     []byte
		Metadata Metadata
	}

	// Metadata has information that is defined by the event broker
	// and attributes that may be defined by the publisher.
	Metadata struct {
		// ID is the event/message ID as defined by the event broker (if any).
		ID string
		// PublishedTime represents when an event was published as defined by the event broker (if any).
		PublishedTime time.Time
		// Attributes are defined by the publisher, like Google Cloud Pub Sub attributes (or similar concepts in other brokers).
		Attributes map[string]string
	}

	// MessageSubscription represents a subscription that delivers messages as is.
	// No assumptions are made about the message contents. This should rarely be used in favor of [Subscription].
	MessageSubscription struct {
		sub            *pubsub.Subscription
		maxConcurrency int
	}

	// MessageHandler is responsible for handling messages from a [MessageSubscription].
	MessageHandler func(Message) error
)

// NewPublisher creates a new event publisher for the given event name and topic.
func NewPublisher[T any](name string, t *pubsub.Topic) *Publisher[T] {
	return &Publisher[T]{
		name:  name,
		topic: t,
	}
}

// Name returns the name of the event.
func (p *Publisher[T]) Name() string {
	return p.name
}

// Publish will publish the given event.
func (p *Publisher[T]) Publish(ctx context.Context, event T) error {
	return p.PublishWithAttrs(ctx, event, nil)
}

// PublishWithAttrs will publish the given event with the provided attributes.
// The attributes will be available when receiving the events as [Metadata.Attributes].
func (p *Publisher[T]) PublishWithAttrs(ctx context.Context, event T, attributes map[string]string) error {
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

	start := time.Now()
	err = p.topic.Send(ctx, &pubsub.Message{
		Body:     encBody,
		Metadata: attributes,
	})
	elapsed := time.Since(start)

	samplePublish(p.name, elapsed, len(encBody), err)

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
// is processed in its own go-routines respecting the given maxConcurrency.
func NewRawSubscription(url string, maxConcurrency int) (*MessageSubscription, error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("max concurrency must be > 0: %d", maxConcurrency)
	}
	// We don't want the subscription to expire, so we use the background context.
	sub, err := pubsub.OpenSubscription(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &MessageSubscription{
		sub:            sub,
		maxConcurrency: maxConcurrency,
	}, nil
}

// Name returns the name of the event.
func (s *Subscription[T]) Name() string {
	return s.name
}

// ReceiveN will receive at most N events.
// It may return less events if the provided context is canceled/deadline exceeded.
// If called concurrently with [Subscription.Serve] it will compete for events.
// Events returned here must be Ack-ed after the caller is done with them.
// For simple event handling [Subscription.Serve] will be better. This method is useful
// when you need more control, like batching N events together.
func (s *Subscription[T]) ReceiveN(ctx context.Context, n int) ([]*Event[T], error) {
	if n < 0 {
		panic(fmt.Errorf("n must be >= 0"))
	}
	events := []*Event[T]{}
	for len(events) < n {
		event, err := s.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) ||
				errors.Is(err, context.Canceled) {
				// Time window reached, returning current batch/N
				return events, nil
			}
			// Some other error happened, normal failure then
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// Receive will receive a single event.
// If called concurrently with [Subscription.Serve] it will compete for events.
// Events returned here must be Ack-ed after the caller is done with them.
// For simple event handling [Subscription.Serve] will be better. This method is useful
// when you need more control, like batching N events together.
func (s *Subscription[T]) Receive(ctx context.Context) (*Event[T], error) {
	m, err := s.rawsub.receive(ctx)
	if err != nil {
		return nil, err
	}
	_, envelope, err := s.createEvent(m.Message)
	if err != nil {
		return nil, err
	}
	var res Event[T]
	res.Envelope = envelope
	res.msg = m
	return &res, nil
}

// Ack this event.
func (e *Event[T]) Ack() {
	e.msg.Ack()
}

// Nack this event.
func (e *Event[T]) Nack() {
	e.msg.Nack()
}

// Serve will start serving all events from the subscription calling handler for each
// event. It will run until [Subscription.Shutdown] is called.
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler Nack will be sent.
// If a received event is not a valid JSON it will be discarded as malformed and a Nack will be sent automatically.
// If a received event has the wrong name it will be discarded as malformed and a Nack will be sent automatically.
// Serve may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" go-routines.
//
// If the handler panics, the [Subscription] (the caller of the handler) assumes
// that the effect of the panic was isolated to the active event handling.
// It recovers the panic, logs a stack trace and returns an error (failing the event handling gracefully,
// which in most event systems will trigger some form of retry).
func (s *Subscription[T]) Serve(handler Handler[T]) error {
	return s.rawsub.Serve(SampledMessageHandler(s.name, func(msg Message) error {
		ctx, event, err := s.createEvent(msg)
		if err != nil {
			return err
		}
		return handler(ctx, event.Event)
	}))
}

// ServeWithMetadata will start serving all events from the subscription calling handler for each
// event, providing both the event and any metadata associated with it.
// It will run until [Subscription.Shutdown] is called.
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler Nack will be sent.
// If a received event is not a valid JSON it will be discarded as malformed and a Nack will be sent automatically.
// If a received event has the wrong name it will be discarded as malformed and a Nack will be sent automatically.
// ServeWithMetadata may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" go-routines.
func (s *Subscription[T]) ServeWithMetadata(handler HandlerWithMetadata[T]) error {
	return s.rawsub.Serve(SampledMessageHandler(s.name, func(msg Message) error {
		ctx, event, err := s.createEvent(msg)
		if err != nil {
			return err
		}
		return handler(ctx, event.Event, msg.Metadata)
	}))
}

func (s *Subscription[T]) createEvent(msg Message) (context.Context, Envelope[T], error) {
	var event Envelope[T]

	log := slog.Default()

	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Error("parsing event body", "name", s.name, "error", err, "body", string(msg.Body))
		return nil, event, fmt.Errorf("parsing event as JSON, event: %v, error: %v", msg, err)
	}

	if event.Name != s.name {
		log.Error("event name doesn't match handler", "expected", s.name, "received", event.Name)
		return nil, event, fmt.Errorf("event name doesn't match %q: event: %v", s.name, msg)
	}

	if event.TraceID == "" {
		event.TraceID = uuid.NewString()
	}

	log = log.With("request_id", uuid.NewString())
	log = log.With("trace_id", event.TraceID)
	log = log.With("organization_id", event.OrgID)

	ctx := context.Background()
	ctx = tracing.CtxWithTraceID(ctx, event.TraceID)
	ctx = tracing.CtxWithOrgID(ctx, event.OrgID)
	ctx = slog.NewContext(ctx, log)
	return ctx, event, nil
}

// Shutdown will shutdown the subscriber, stopping any calls to [Subscription.Serve].
// The subscription should not be used after this method is called.
func (s *Subscription[T]) Shutdown(ctx context.Context) error {
	return s.rawsub.Shutdown(ctx)
}

// Serve will start serving all messages from the subscription calling handler for each
// message. It will run until [MessageSubscription.Shutdown] is called.
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler then a Nack will be sent.
// Serve may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" go-routines.
//
// If the handler panics, the [MessageSubscription] (the caller of the handler) assumes
// that the effect of the panic was isolated to the active event handling.
// It recovers the panic, logs a stack trace and returns an error (failing the event handling gracefully,
// which in most event systems will trigger some form of retry).
func (r *MessageSubscription) Serve(handler MessageHandler) error {
	semaphore := make(chan struct{}, r.maxConcurrency)
	for {
		semaphore <- struct{}{}
		rmsg, err := r.receive(context.Background())
		if err != nil {
			// From: https://pkg.go.dev/gocloud.dev@v0.30.0/pubsub#example-Subscription.Receive-Concurrent
			// Errors from Receive indicate that Receive will no longer succeed.
			return fmt.Errorf("receive from subscription failed, stopping serving: %v", err)
		}
		go func() {
			defer func() {
				<-semaphore
			}()

			defer func() {
				if err := recover(); err != nil {
					// 64KB, if it is good enough for Go's standard lib it is good enough for us :-)
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					slog.Error("panic: message subscription: handling message",
						"error", err,
						"message_body", rmsg.Body,
						"metadata", rmsg.Metadata,
						"stack_trace", string(buf))
					rmsg.Nack()
				}
			}()

			err := handler(rmsg.Message)
			if err != nil {
				rmsg.Nack()
				return
			}
			rmsg.Ack()
		}()
	}
}

// Shutdown will shutdown the subscriber, stopping any calls to [MessageSubscription.Serve].
// The subscription should not be used after this method is called.
func (r *MessageSubscription) Shutdown(ctx context.Context) error {
	return r.sub.Shutdown(ctx)
}

func (r *MessageSubscription) receive(ctx context.Context) (*message, error) {
	gocloudMsg, err := r.sub.Receive(ctx)
	if err != nil {
		return nil, err
	}
	id, publishedTime := getMetadata(gocloudMsg)
	return &message{
		Message: Message{
			Body: gocloudMsg.Body,
			Metadata: Metadata{
				ID:            id,
				PublishedTime: publishedTime,
				Attributes:    gocloudMsg.Metadata,
			},
		},
		msg: gocloudMsg,
	}, nil
}

type message struct {
	Message
	msg *pubsub.Message
}

// Nack this msg (if possible).
func (r *message) Nack() {
	if r.msg.Nackable() {
		r.msg.Nack()
	}
}

// Ack this msg.
func (r *message) Ack() {
	r.msg.Ack()
}

func getMetadata(msg *pubsub.Message) (string, time.Time) {
	// This is the only way to get broker specific metadata
	// For now we only support Google Cloud.
	var pbmsg *pubsubpb.PubsubMessage
	if msg.As(&pbmsg) {
		return pbmsg.MessageId, pbmsg.PublishTime.AsTime()
	}
	return "", time.Time{}
}
