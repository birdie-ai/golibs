package event

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/birdie-ai/golibs/slog"
	"github.com/birdie-ai/golibs/xerrors"
)

type (
	// OrderedGooglePublisher is an ordered google publisher.
	OrderedGooglePublisher[T any] struct {
		eventName string
		client    *pubsub.Client
		topic     *pubsub.Topic
	}
	// OrderedGoogleSub is an ordered google subscription.
	OrderedGoogleSub[T any] struct {
		eventName string
		client    *pubsub.Client
		sub       *pubsub.Subscription
	}
)

// NewOrderedGooglePublisher creates a new ordered Google Cloud event publisher for the given project/topic/event name.
// We need a specific Google publisher because ordering doesn't generalize well.
// All ordered publishers should implement the same interface.
// Call [OrderedGooglePublisher.Shutdown] to stop all goroutines/clean up all resources.
func NewOrderedGooglePublisher[T any](ctx context.Context, project, topicName, eventName string) (*OrderedGooglePublisher[T], error) {
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating pubsub client: %w", err)
	}
	topic := client.Topic(topicName)
	topic.EnableMessageOrdering = true
	return &OrderedGooglePublisher[T]{eventName: eventName, client: client, topic: topic}, nil
}

// Publish will publish the given event with the given ordering key.
// If an unrecoverable error happens an [ErrUnrecoverable] will be returned, when that happens if
// [OrderedGooglePublisher.Resume] is not called with the same orderingKey all subsequent
// [Publish] calls with that orderingKey will fail.
// This allows clients to control the ordering behavior when something went wrong, resuming will
// discard the failed publish and will result in an out of order stream.
func (p *OrderedGooglePublisher[T]) Publish(ctx context.Context, event T, orderingKey string) error {
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

	return xerrors.Tag(err, ErrUnrecoverable)
}

// Resume must be called for the given orderingKey after a Publish call with the same
// orderingKey failed and the error is [ErrUnrecoverable].
func (p *OrderedGooglePublisher[T]) Resume(_ context.Context, orderingKey string) error {
	p.topic.ResumePublish(orderingKey)
	return nil
}

// Shutdown will send all pending publish messages and stop all goroutines.
func (p *OrderedGooglePublisher[T]) Shutdown(context.Context) error {
	p.topic.Stop()
	return p.client.Close()
}

// NewOrderedGoogleSub creates an ordered subscription on Google Cloud Pubsub that will accept on events of the given type and name,
// similar to [NewSubscription]. Ordering affects how concurrency is handled. Concurrency is done by handling
// different ordering keys/partitions, every ordered key will be handled sequentially only different ordering keys will be
// handled concurrently. This requires a client to be created per go routine, so beware of setting concurrency to a high value (every go routine
// will create a different client/connection to pubsub).
// Call [OrderedGoogleSub.Shutdown] to stop all goroutines/clean up all resources.
func NewOrderedGoogleSub[T any](ctx context.Context, project, subName, eventName string, maxConcurrency int) (*OrderedGoogleSub[T], error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("max concurrency must be > 0: %d", maxConcurrency)
	}
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	sub := client.Subscription(subName)
	sub.ReceiveSettings.NumGoroutines = maxConcurrency
	return &OrderedGoogleSub[T]{eventName: eventName, client: client, sub: sub}, nil
}

// Serve will start serving all events from the subscription calling handler for each
// event. It will run until [OrderedGoogleSub.Shutdown] is called.
// The handler might be called concurrently if max concurrency > 1 but guarantees that
// events with the same ordering key are handled sequentially. You can handle different ordering keys
// concurrently but there is no way to handler N events of the same ordering key at once or out of order.
//
// If the error is nil Ack is sent.
// If a non-nil error is returned by the handler Nack will be sent.
// If a received event is not a valid JSON it will be discarded as malformed and a Nack will be sent automatically.
// If a received event has the wrong name it will be discarded as malformed and a Nack will be sent automatically.
// It is a programming error to call Serve more than once (breaks ordering invariant).
//
// If the handler function panics, the [OrderedGoogleSub] assumes
// that the effect of the panic was isolated to that single event handling.
// It recovers the panic, logs a stack trace and sends a Nack (failing the event handling gracefully,
// which in most event systems will trigger some form of retry).
//
// If the handler returns an error the error will be logged on the "ERROR" level,
// only the event name and error will be logged, any other details myst be logged by
// the handler function if necessary.
func (s *OrderedGoogleSub[T]) Serve(ctx context.Context, handler Handler[T]) error {
	return s.sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		ctx, event, err := createEvent[T](ctx, s.eventName, msg.Data)
		if err != nil {
			slog.FromCtx(ctx).Error("unacking invalid event (handler not called)", "event_name", s.eventName, "error", err)
			msg.Nack()
			return
		}
		if err := handler(ctx, event.Event); err != nil {
			slog.FromCtx(ctx).Error("event handling failed", "event_name", s.eventName, "error", err)
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

// Shutdown will send all pending publish messages and stop all goroutines.
func (s *OrderedGoogleSub[T]) Shutdown(context.Context) error {
	return s.client.Close()
}
