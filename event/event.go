// Package event provides functionality for publish/subscribe of events.
package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
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

	// AckerNacker supports ack/nack.
	AckerNacker interface {
		Ack()
		Nack()
	}

	// Event represents the structure of all data that wraps all events, like the [Envelope], but
	// but with Ack/Nack. After the [Event] is handled [Event.Ack] or [Event.Nack] must be called.
	// This type is used when receiving individual events with [Subscription.Receive] or [Subscription.ReceiveN].
	Event[T any] struct {
		Envelope[T]
		AckerNacker
		Metadata Metadata
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

	// BatchHandler is responsible for handling N events at once.
	// Since it is usual for partial results to happen it is the responsibility of the handler
	// to ack/unack individual events.
	// The context passed to the handler will not have any metadata (opposed to [Handler]) since
	// each event has its own metadata (different org ID, different trace ID, etc).
	BatchHandler[T any] func(context.Context, []*Event[T])

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
	// If an error is returned it sends a nack.
	// If error is nil ack is sent.
	MessageHandler func(Message) error

	// AckerNackerMsg is a [Message] that implements [AckerNacker]. It is used when handling batches.
	AckerNackerMsg struct {
		Message
		AckerNacker
	}

	// MessageBatchHandler is responsible for handling a batch of N messages from a [MessageSubscription] at once.
	// No [Message] from the batch is acked or nacked, the handler is responsible for sending ack/nack for each message (allows partial processing).
	MessageBatchHandler func(context.Context, []*AckerNackerMsg)
)

// ErrUnrecoverable represents unrecoverable errors, used to deal with ordered publishing errors.
var ErrUnrecoverable = errors.New("unrecoverable")

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
	encBody, err := serializeEvent(ctx, p.name, event)
	if err != nil {
		return err
	}

	sample := publishSampler()
	err = p.topic.Send(ctx, &pubsub.Message{
		Body:     encBody,
		Metadata: attributes,
	})
	sample(p.name, len(encBody), err)

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
	_, envelope, err := createEnvelope[T](ctx, s.name, m.Body)
	if err != nil {
		return nil, err
	}
	var res Event[T]
	res.Envelope = envelope
	res.AckerNacker = m
	res.Metadata = m.Metadata
	return &res, nil
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
		ctx, event, err := createEnvelope[T](context.Background(), s.name, msg.Body)
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
		ctx, event, err := createEnvelope[T](context.Background(), s.name, msg.Body)
		if err != nil {
			return err
		}
		return handler(ctx, event.Event, msg.Metadata)
	}))
}

// ServeBatch will start serving all events from the subscription calling handler for each batch of events.
// It will run until the given [context.Context] is cancelled.
// If a received event is not a valid JSON it will be discarded as malformed and a Nack will be sent automatically.
// If a received event has the wrong name it will be discarded as malformed and a Nack will be sent automatically.
// ServeBatch may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" go-routines (configured on [Subscription] creation).
//
// The batch handler is called with N events where 0 < N <= batchSize.
// The batch time window controls for how long it will wait for a batch to fill.
// The [context.Context] might not have any deadline enforced on it (it is the overall batch run one),
// it is responsibility of handlers to create a new child context with an explicit/clean deadline for the event handling.
//
// If the handler panics it is assumed that the effect of the panic was isolated to the failed batch handling,
// Since when dealing with batches partial results are possible nothing is done with the events, like nack'ing them.
// It recovers the panic, logs a stack trace with ERROR log level and keeps running (unacked events will eventually
// expire and be re-delivered, depending on the broker configuration).
func (s *Subscription[T]) ServeBatch(
	ctx context.Context,
	batchSize int,
	batchWindow time.Duration,
	bh BatchHandler[T],
) error {
	return s.rawsub.ServeBatch(ctx, batchSize, batchWindow, func(ctx context.Context, rawbatch []*AckerNackerMsg) {
		var batch []*Event[T]
		for _, v := range rawbatch {
			_, event, err := createEnvelope[T](ctx, s.name, v.Body)
			if err != nil {
				slog.FromCtx(ctx).Error("golibs: invalid event received, sending nack", "event", v, "error", err)
				v.Nack()
				continue
			}
			batch = append(batch, &Event[T]{
				Envelope: event,
				AckerNacker: &sampledAckerNacker{
					name:    s.name,
					bodyLen: len(v.Body),
					// We measure process time as time spent on handler/processing.
					// It does not include idle wait time from batch time window.
					// We might need a different metric/information for that.
					start:       time.Now(),
					ackerNacker: v,
				},
				Metadata: v.Metadata,
			})
		}
		if len(batch) == 0 {
			return
		}
		sampleBatchSize(s.name, len(batch))
		bh(ctx, batch)
	})
}

type sampledAckerNacker struct {
	name        string
	bodyLen     int
	start       time.Time
	ackerNacker AckerNacker
}

func (s *sampledAckerNacker) Ack() {
	s.ackerNacker.Ack()
	sampleProcessStatus(s.name, time.Since(s.start), float64(s.bodyLen), "ok")
}

func (s *sampledAckerNacker) Nack() {
	s.ackerNacker.Nack()
	sampleProcessStatus(s.name, time.Since(s.start), float64(s.bodyLen), "error")
}

// Shutdown will shutdown the subscriber, stopping any calls to [Subscription.Serve].
// The subscription should not be used after this method is called.
func (s *Subscription[T]) Shutdown(ctx context.Context) error {
	return s.rawsub.Shutdown(ctx)
}

// NewRawSubscription creates a new raw subscription. It provides messages in a
// service like manner (serve) and manages concurrent execution, each message
// is processed in its own go-routines respecting the given maxConcurrency.
func NewRawSubscription(url string, maxConcurrency int) (*MessageSubscription, error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("max concurrency must be > 0: %d", maxConcurrency)
	}
	// FIXME(katcipis): we should accept a context here (so we can cancel when a signal is received/process shutdown).
	sub, err := pubsub.OpenSubscription(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &MessageSubscription{
		sub:            sub,
		maxConcurrency: maxConcurrency,
	}, nil
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

// ServeBatch will start serving all events from the subscription calling handler for each batch of events.
// It will run until the given [context.Context] is cancelled. When the context is cancelled
// it will wait for all handlers to finish before returning. Handlers receive the same context
// as a parameter and must respect its cancellation.
// The [context.Context] might not have any deadline enforced on it (it is the overall batch run one),
// it is responsibility of handlers to create a new child context with an explicit/clean deadline for the event handling.
//
// ServeBatch may be called multiple times, each time will start a new serving service that will
// run up to "maxConcurrency" go-routines (configured on [MessageSubscription] creation).
//
// The batch handler is called with N events where 0 < N <= batchSize.
// The batch time window controls for how long it will wait for a batch to fill.
//
// If the handler panics it is assumed that the effect of the panic was isolated to the failed batch handling,
// Since when dealing with batches partial results are possible nothing is done with the events, like nack'ing them.
// It recovers the panic, logs a stack trace with ERROR log level and keeps running (unacked events will eventually
// expire and be re-delivered, depending on the broker configuration).
func (r *MessageSubscription) ServeBatch(
	ctx context.Context,
	batchSize int,
	batchWindow time.Duration,
	bh MessageBatchHandler,
) error {
	if batchSize <= 0 {
		return fmt.Errorf("batch size %d must be > 0", batchSize)
	}
	if batchWindow <= 0 {
		return fmt.Errorf("batch window %v must be > 0", batchWindow)
	}

	semaphore := make(chan struct{}, r.maxConcurrency)
	fatalErr := make(chan error)
	var wg sync.WaitGroup

	for ctx.Err() == nil {
		select {
		case semaphore <- struct{}{}:
		case err := <-fatalErr:
			wg.Wait()
			return err
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		}
		wg.Go(func() {
			defer func() {
				<-semaphore
			}()

			rcvCtx, cancel := context.WithTimeout(ctx, batchWindow)
			rmsgs, err := r.receiveN(rcvCtx, batchSize)
			cancel()
			if err != nil {
				// From: https://pkg.go.dev/gocloud.dev@v0.30.0/pubsub#example-Subscription.Receive-Concurrent
				// Errors from Receive indicate that Receive will no longer succeed.
				fatalErr <- err
				return
			}
			if len(rmsgs) == 0 {
				return
			}
			msgs := make([]*AckerNackerMsg, len(rmsgs))
			for i, v := range rmsgs {
				msgs[i] = &AckerNackerMsg{
					Message:     v.Message,
					AckerNacker: v,
				}
			}
			// We only handle panics from the handler, the core logic shouldn't panic and
			// should abort if a panic happens.
			defer func() {
				if err := recover(); err != nil {
					// 64KB, if it is good enough for Go's standard lib it is good enough for us :-)
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					// We might have partial results, we cant ack/nack any message, just log.
					slog.Error("panic: message subscription: handling message",
						"error", err,
						"messages_total", len(rmsgs),
						"batch_size", batchSize,
						"batch_window", batchWindow,
						"stack_trace", string(buf))
				}
			}()

			bh(ctx, msgs)
		})
	}
	wg.Wait()
	return ctx.Err()
}

// Shutdown will shutdown the subscriber, stopping any calls to [MessageSubscription.Serve].
// The subscription should not be used after this method is called.
func (r *MessageSubscription) Shutdown(ctx context.Context) error {
	return r.sub.Shutdown(ctx)
}

type message struct {
	Message
	msg *pubsub.Message
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

func (r *MessageSubscription) receiveN(ctx context.Context, n int) ([]*message, error) {
	if n < 0 {
		panic(fmt.Errorf("n must be >= 0"))
	}
	var events []*message
	for len(events) < n {
		event, err := r.receive(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) ||
				errors.Is(err, context.Canceled) {
				// Time window reached/canceled, returning current batch/N
				return events, nil
			}
			// Some other error happened, normal failure then
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
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

func serializeEvent[T any](ctx context.Context, eventName string, event T) ([]byte, error) {
	return json.Marshal(Envelope[T]{
		TraceID: tracing.CtxGetTraceID(ctx),
		OrgID:   tracing.CtxGetOrgID(ctx),
		Name:    eventName,
		Event:   event,
	})
}

func createEnvelope[T any](ctx context.Context, eventName string, data []byte) (context.Context, Envelope[T], error) {
	var event Envelope[T]

	log := slog.Default()

	if err := json.Unmarshal(data, &event); err != nil {
		return nil, event, fmt.Errorf("parsing event %q as JSON, event: %q, error: %v", eventName, string(data), err)
	}

	if event.Name != eventName {
		return nil, event, fmt.Errorf("event name %q doesn't match event %q", eventName, string(data))
	}

	if event.TraceID == "" {
		event.TraceID = uuid.NewString()
	}

	log = log.With("request_id", uuid.NewString())
	log = log.With("trace_id", event.TraceID)
	ctx = tracing.CtxWithTraceID(ctx, event.TraceID)

	if event.OrgID != "" {
		log = log.With("organization_id", event.OrgID)
		ctx = tracing.CtxWithOrgID(ctx, event.OrgID)
	}

	ctx = slog.NewContext(ctx, log)
	return ctx, event, nil
}
