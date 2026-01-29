package event

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/birdie-ai/golibs/slog"
	"github.com/birdie-ai/golibs/xerrors"
	"google.golang.org/api/option"
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
	// GoogleExperimentalBatchSubscription helps build batches of N events even for ordered subscriptions.
	// N events will be received for the same ordering key, but in order.
	// In order to do this we need to do unconventional stuff since the conventional docs just don't allow this at all:
	//
	//	- https://pkg.go.dev/cloud.google.com/go/pubsub#hdr-Receiving
	//
	// The side effects/possible issues are something we are willing to live with when using this.
	// If in doubt, don't use this, it is somewhat experimental (even though we really need this to work well in production).
	GoogleExperimentalBatchSubscription[T any] struct {
		eventName string
		client    *pubsub.Client
		sub       *pubsub.Subscription
		events    chan *Event[T]
		batchSize int
	}
)

// NewOrderedGooglePublisher creates a new ordered Google Cloud event publisher for the given project/topic/event name.
// We need a specific Google publisher because ordering doesn't generalize well.
// All ordered publishers should implement the same interface.
// Call [OrderedGooglePublisher.Shutdown] to stop all goroutines/clean up all resources.
//
// Region is required since it is a best practice to publish all messages within the same region:
//   - https://cloud.google.com/pubsub/docs/publish-best-practices#ordering
//   - https://cloud.google.com/pubsub/docs/reference/service_apis_overview#pubsub_endpoints
//
// It must be a valid Google cloud region, it is used to defined the publish endpoint.
func NewOrderedGooglePublisher[T any](ctx context.Context, project, region, topicName, eventName string) (*OrderedGooglePublisher[T], error) {
	client, err := pubsub.NewClient(ctx, project, option.WithEndpoint(region+"-pubsub.googleapis.com:443"))
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

	if err != nil {
		return xerrors.Tag(err, ErrUnrecoverable)
	}
	return nil
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
// different ordering keys/partitions, every ordered key will be handled sequentially only different ordering keys will be handled concurrently.
// Call [OrderedGoogleSub.Shutdown] to stop all goroutines/clean up all resources.
func NewOrderedGoogleSub[T any](ctx context.Context, project, subName, eventName string, maxConcurrentEvents int) (*OrderedGoogleSub[T], error) {
	if maxConcurrentEvents <= 0 {
		return nil, fmt.Errorf("max concurrency must be > 0: %d", maxConcurrentEvents)
	}
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	sub := client.Subscription(subName)
	sub.ReceiveSettings.MaxOutstandingMessages = maxConcurrentEvents
	return &OrderedGoogleSub[T]{eventName: eventName, client: client, sub: sub}, nil
}

// Serve behaves exactly like [ServeWithMetadata] but omits the metadata.
func (s *OrderedGoogleSub[T]) Serve(ctx context.Context, handler Handler[T]) error {
	return s.ServeWithMetadata(ctx, func(ctx context.Context, event T, _ Metadata) error {
		return handler(ctx, event)
	})
}

// ServeWithMetadata will start serving all events from the subscription calling handler for each
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
func (s *OrderedGoogleSub[T]) ServeWithMetadata(ctx context.Context, handler HandlerWithMetadata[T]) error {
	return s.sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		defer func() {
			if err := recover(); err != nil {
				// 64KB, if it is good enough for Go's standard lib it is good enough for us :-)
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				slog.Error("panic: ordered google subscription: handling message",
					"error", err,
					"id", msg.ID,
					"attributes", msg.Attributes,
					"stack_trace", string(buf))
				msg.Nack()
			}
		}()
		ctx, event, err := createEvent[T](ctx, s.eventName, msg.Data)
		if err != nil {
			slog.FromCtx(ctx).Error("unacking invalid event (handler not called)", "event_name", s.eventName, "error", err)
			msg.Nack()
			return
		}
		metadata := Metadata{
			ID:            msg.ID,
			PublishedTime: msg.PublishTime,
			Attributes:    msg.Attributes,
		}

		start := time.Now()
		err = handler(ctx, event.Event, metadata)
		elapsed := time.Since(start)
		sampleProcess(s.eventName, elapsed, float64(len(msg.Data)), err)

		if err != nil {
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

// NewGoogleExperimentalBatchSubscription creates a new google batch subscriber that can read N events at once (building a batch).
func NewGoogleExperimentalBatchSubscription[T any](ctx context.Context, project, subName, eventName string, batchSize int) (*GoogleExperimentalBatchSubscription[T], error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size %q must be > 0", batchSize)
	}
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	s := &GoogleExperimentalBatchSubscription[T]{
		eventName: eventName,
		client:    client,
		sub:       client.Subscription(subName),
		events:    make(chan *Event[T]),
		batchSize: batchSize,
	}
	s.runReceiver(ctx)
	return s, nil
}

// ReceiveN will receive at most N events where N is defined by the batch size informed when creating the subscription.
// It may return less events if the provided context is canceled/deadline exceeded (and will return as much events as it could, or none).
// If a batch size can never be reached and the given context has no deadline this method will wait forever.
// Always pass a context.Context with a max period you are willing to wait for a batch to be built.
// Events returned here must be Ack-ed after the caller is done with them.
// Some events may have been sitting idle for quite some time, since this waits
// for the context to expire or for the batch to be built.
// So if a deadline is too long (the context.Context) all events may already been redelivered.
// This is a fairly advanced/risky API and shouldn't be used lightly (or maybe not used at all ?).
// Each call to this method creates a new receiver go-routine that finishes only
// when all received events are acked/nacked/expired. So callers should always ack/nack events
// as fast as possible or else resources will start to pile up. Since events always expire, it is
// not a proper leak, but it might use increasing amounts of memory depending on how poorly the API is used
// and the frequency. You have been warned.
// This method should NOT be called concurrently, we can make only a single receive call per subscription.
func (s *GoogleExperimentalBatchSubscription[T]) ReceiveN(ctx context.Context) ([]*Event[T], error) {
	events := []*Event[T]{}
	for {
		select {
		case <-ctx.Done():
			return events, nil
		case e := <-s.events:
			events = append(events, e)
			if len(events) == s.batchSize {
				return events, nil
			}

		}
	}
}

func (s *GoogleExperimentalBatchSubscription[T]) runReceiver(ctx context.Context) {
	// Yeah this is not a great idea according to the docs:
	//  - https://pkg.go.dev/cloud.google.com/go/pubsub#hdr-Receiving
	// But seems to still be doable and we really want to collect N pending events, in order, but all in memory at once.
	// Maybe this is something that shouldn't be done in pubsub, lets find out !!!
	// If we wait to ack msgs inside the callback, as documented, we will never get N events for the same ordering key.
	// There might be better ways to do this, in a hurry right now.
	go func() {
		// Batch behavior favors long ack times, enforce this as high as possible, which is 600s currently.
		// MaxExtension was copied from the current default (which seems to be the pubsub max limit ? Maybe ?).
		// The other ones are the documented max values.
		const maxExtension = 60 * time.Minute
		s.sub.ReceiveSettings.NumGoroutines = 1
		s.sub.ReceiveSettings.MaxExtension = maxExtension
		s.sub.ReceiveSettings.MinExtensionPeriod = 10 * time.Minute
		s.sub.ReceiveSettings.MaxExtensionPeriod = 10 * time.Minute
		s.sub.ReceiveSettings.MaxOutstandingMessages = s.batchSize

		err := s.sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			ctx, event, err := createEvent[T](ctx, s.eventName, msg.Data)
			if err != nil {
				slog.FromCtx(ctx).Error("unacking invalid event (handler not called)", "event_name", s.eventName, "error", err)
				msg.Nack()
				return
			}
			// Avoid stale events being sent if caller took too long to call ReceiveN
			// Relying on receive deadline extension to keep the message valid as much as possible.
			ctx, cancel := context.WithTimeout(ctx, maxExtension)
			defer cancel()

			select {
			case s.events <- &Event[T]{Envelope: event, msg: msg}:
				// Just send event and let the client ack the event, unblocking this goroutine to get the next message from the same partition.
				// Not recommended by the docs, but lets see if it works well enough.
			case <-ctx.Done():
				msg.Nack()
			}
		})
		if err != nil {
			slog.FromCtx(ctx).Error("google batch subscription receiver stopped (future calls to ReceiveN will fail)", "error", err)
		}
	}()
}

// Shutdown will send all pending publish messages and stop all goroutines.
func (s *GoogleExperimentalBatchSubscription[T]) Shutdown(context.Context) error {
	return s.client.Close()
}
