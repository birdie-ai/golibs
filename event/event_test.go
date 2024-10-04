package event_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/birdie-ai/golibs/event"
	"github.com/birdie-ai/golibs/tracing"
	"github.com/google/go-cmp/cmp"
	"gocloud.dev/pubsub"

	_ "gocloud.dev/pubsub/mempubsub"
)

func TestPublishEvent(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)

	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = topic.Shutdown(ctx) }()

	type Event struct {
		Field string `json:"field"`
	}

	subscription, err := pubsub.OpenSubscription(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, subscription)

	const (
		eventName = "test"
		fieldData = "some data"
		traceID   = "trace-id"
		orgID     = "org-id"
	)

	publisher := event.NewPublisher[Event](eventName, topic)
	wantEvt := Event{
		Field: fieldData,
	}

	go func() {
		// Tracing info stored on the context is propagated to the events.
		ctx := tracing.CtxWithTraceID(ctx, traceID)
		ctx = tracing.CtxWithOrgID(ctx, orgID)

		err := publisher.Publish(ctx, wantEvt)
		t.Logf("publish error: %v", err)
	}()

	gotMsg, err := subscription.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotMsg.Ack()

	want := event.Envelope[Event]{
		TraceID: traceID,
		OrgID:   orgID,
		Name:    eventName,
		Event:   wantEvt,
	}
	got := event.Envelope[Event]{}
	if err := json.Unmarshal(gotMsg.Body, &got); err != nil {
		t.Fatal(err)
	}

	if want != got {
		t.Fatalf("got %+v != want %+v", got, want)
	}
}

func TestPublishEventWithoutTracingInfo(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	type Event struct{}

	subscription, err := pubsub.OpenSubscription(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, subscription)

	const (
		eventName = "test"
	)

	publisher := event.NewPublisher[Event](eventName, topic)
	go func() {
		err := publisher.Publish(ctx, Event{})
		t.Logf("publish error: %v", err)
	}()

	gotMsg, err := subscription.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotMsg.Ack()

	want := event.Envelope[Event]{
		Name: eventName,
	}
	got := event.Envelope[Event]{}
	if err := json.Unmarshal(gotMsg.Body, &got); err != nil {
		t.Fatal(err)
	}

	if want != got {
		t.Fatalf("got %+v != want %+v", got, want)
	}
}

func TestSubscriptionServing(t *testing.T) {
	t.Parallel()

	type Event struct {
		ID  int `json:"id"`
		ctx context.Context
	}

	url := newTopicURL(t)
	ctx := context.Background()

	const (
		eventName = "test-subscription"
	)

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	publisher := event.NewPublisher[Event](eventName, topic)
	getOrgID := func(e Event) string {
		return fmt.Sprintf("org-id-%d", e.ID)
	}
	getTraceID := func(e Event) string {
		return fmt.Sprintf("trace-id-%d", e.ID)
	}

	publish := func(event Event) {
		t.Helper()
		// We test that trace and org IDs are transported correctly on the envelope.
		ctx := tracing.CtxWithOrgID(ctx, getOrgID(event))
		ctx = tracing.CtxWithTraceID(ctx, getTraceID(event))

		if err := publisher.Publish(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	const maxConcurrency = 5

	subscription, err := event.NewSubscription[Event](eventName, url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	gotEvents := make(chan Event)
	handlersDone := make(chan struct{})
	servingDone := make(chan struct{})

	go func() {
		err := subscription.Serve(func(ctx context.Context, event Event) error {
			t.Logf("handler called, event: %v", event)
			event.ctx = ctx
			gotEvents <- event
			// We block the handlers to ensure concurrency is being respected
			<-handlersDone
			return nil
		})
		t.Logf("subscription.Service error: %v", err)
		close(servingDone)
	}()

	want := []Event{}
	got := []Event{}

	// Lets check that all go-routines were created and handled each message
	for i := 0; i < maxConcurrency; i++ {
		event := Event{
			ID: i,
		}
		want = append(want, event)

		t.Log("publishing message")

		publish(event)

		t.Log("waiting for message received from subscription")
		got = append(got, <-gotEvents)
		t.Log("message received from subscription")
	}

	sort.SliceStable(got, func(i, j int) bool {
		return got[i].ID < got[j].ID
	})

	if len(got) != len(want) {
		t.Logf("got: %v", got)
		t.Logf("want: %v", want)
		t.Fatal("got != want")
	}

	assertCtxData := func(e Event) {
		t.Helper()

		wantTraceID := getTraceID(e)
		wantOrgID := getOrgID(e)
		gotTraceID := tracing.CtxGetTraceID(e.ctx)
		gotOrgID := tracing.CtxGetOrgID(e.ctx)

		assertEqual(t, gotTraceID, wantTraceID)
		assertEqual(t, gotOrgID, wantOrgID)
	}

	for i, g := range got {
		w := want[i]
		if g.ID != w.ID {
			t.Errorf("got[%d] != want[%d]", g, w)
		}
		assertCtxData(g)
	}

	// Now lets ensure we didn't create any extra go-routines
	// Ensure is a strong word, this is time sensitive, but we don't have false positives
	// here, only false negatives, so good enough? (no random/wrong failures, only false successes maybe).
	finalEvent := Event{
		ID: 666,
	}
	publish(finalEvent)

	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	select {
	case event := <-gotEvents:
		t.Fatalf("got unexpected event %q, an extra goroutine was created on the subscription", event)
	case <-timeout.Done():
		break
	}

	// Now lets free all blocked handlers
	close(handlersDone)

	gotFinalEvent := <-gotEvents
	if gotFinalEvent.ID != finalEvent.ID {
		t.Fatalf("final event got %v != want %v", gotFinalEvent, finalEvent)
	}

	if err := subscription.Shutdown(ctx); err != nil {
		t.Fatalf("shutting down subscription: %v", err)
	}

	// Wait for subscription to shutdown
	<-servingDone
}

func TestSubscriptionReceiveN(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	type Event struct {
		Value int
	}
	const (
		eventName   = "test"
		batchSize   = 5
		totalEvents = 9
	)
	subscription, err := event.NewSubscription[Event](eventName, url, 1)
	if err != nil {
		t.Fatalf("creating subscription: %v", err)
	}
	defer shutdown(t, subscription)

	publisher := event.NewPublisher[Event](eventName, topic)
	wantEvents := make([]Event, totalEvents)
	for i := 0; i < 9; i++ {
		wantEvents[i] = Event{i}
		if err := publisher.Publish(ctx, Event{i}); err != nil {
			t.Fatalf("publishing test event: %v", err)
		}
	}

	// The first receive N should have a full batch, we can use the background context
	firstBatch, err := subscription.ReceiveN(ctx, batchSize)
	if err != nil {
		t.Fatalf("receiving first batch: %v", err)
	}
	if len(firstBatch) != batchSize {
		t.Fatalf("first batch has size %d; want %d", len(firstBatch), batchSize)
	}

	// The second receive N should have the rest of the available messages, we need a context with proper timeout.
	// This tests that the time window for getting a batch of N also works
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	secondBatch, err := subscription.ReceiveN(ctx, batchSize)
	if err != nil {
		t.Fatalf("receiving second batch: %v", err)
	}
	const secondBatchSize = 4
	if len(secondBatch) != secondBatchSize {
		t.Fatalf("second batch has size %d; want %d", len(secondBatch), secondBatchSize)
	}

	// There are no order guarantees, lets order the results
	got := append(firstBatch, secondBatch...)
	gotEvents := make([]Event, len(got))

	for i, v := range got {
		gotEvents[i] = v.Event
		v.Ack()
	}

	sort.SliceStable(gotEvents, func(i, j int) bool {
		return gotEvents[i].Value < gotEvents[j].Value
	})
	assertEqual(t, gotEvents, wantEvents)

	// When there are no messages left we will get empty/no error
	ctx2, cancel2 := context.WithTimeout(ctx, time.Millisecond)
	defer cancel2()
	empty, err := subscription.ReceiveN(ctx2, batchSize)
	if len(empty) != 0 {
		t.Fatalf("expected no messages, got: %+v", empty)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionReceiveNError(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	subscription, err := event.NewSubscription[any]("test", url, 1)
	if err != nil {
		t.Fatalf("creating subscription: %v", err)
	}
	shutdown(t, subscription)

	got, err := subscription.ReceiveN(ctx, 10)
	if err == nil {
		t.Fatalf("got no error and %v; want error", got)
	}
}

func TestSubscriptionRecoversFromPanic(t *testing.T) {
	t.Parallel()

	type Event struct {
		ID int `json:"id"`
	}

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	const eventName = "eventname"
	publisher := event.NewPublisher[Event](eventName, topic)

	const maxConcurrency = 1

	subscription, err := event.NewSubscription[Event](eventName, url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	gotEventsChan := make(chan Event)
	servingDone := make(chan struct{})

	go func() {
		err := subscription.Serve(func(_ context.Context, event Event) error {
			gotEventsChan <- event
			panic("oh no, something went terribly wrong")
		})
		t.Logf("subscription.Service error: %v", err)
		close(servingDone)
	}()

	wantEvent := Event{ID: 555}
	if err := publisher.Publish(ctx, wantEvent); err != nil {
		t.Fatal(err)
	}

	timeout, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	select {
	case event := <-gotEventsChan:
		assertEqual(t, event, wantEvent)
	case <-timeout.Done():
		t.Fatal("timeout waiting for event")
		break
	}

	if err := subscription.Shutdown(ctx); err != nil {
		t.Fatalf("shutting down subscription: %v", err)
	}
	<-servingDone
}

func TestSubscriptionDiscardsEventsWithWrongName(t *testing.T) {
	t.Parallel()

	type Event struct {
		ID int `json:"id"`
	}

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	publisher := event.NewPublisher[Event]("publish_name", topic)

	const maxConcurrency = 1

	subscription, err := event.NewSubscription[Event]("wrong_name", url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	gotEvents := make(chan Event)
	servingDone := make(chan struct{})

	go func() {
		err := subscription.Serve(func(_ context.Context, event Event) error {
			gotEvents <- event
			return nil
		})
		t.Logf("subscription.Service error: %v", err)
		close(servingDone)
	}()

	if err := publisher.Publish(ctx, Event{ID: 666}); err != nil {
		t.Fatal(err)
	}

	timeout, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	select {
	case event := <-gotEvents:
		t.Fatalf("got unexpected event %+v, should not receive event with wrong name", event)
	case <-timeout.Done():
		break
	}

	if err := subscription.Shutdown(ctx); err != nil {
		t.Fatalf("shutting down subscription: %v", err)
	}

	// Wait for subscription to shutdown
	<-servingDone
}

func TestRawSubscriptionServing(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = topic.Shutdown(ctx) }()

	sendMsg := func(msg []byte) {
		t.Helper()
		if err := topic.Send(ctx, &pubsub.Message{Body: []byte(msg)}); err != nil {
			t.Fatalf("publishing message: %v", err)
		}
	}

	const maxConcurrency = 5

	subscription, err := event.NewRawSubscription(url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	gotMsgs := make(chan []byte)
	handlersDone := make(chan struct{})
	servingDone := make(chan struct{})

	go func() {
		err := subscription.Serve(func(msg event.Message) error {
			t.Logf("handler called, msg: %v", string(msg.Body))
			gotMsgs <- msg.Body
			// We block the handlers to ensure concurrency is being respected
			<-handlersDone
			return nil
		})
		t.Logf("subscription.Service error: %v", err)
		close(servingDone)
	}()

	want := []string{}
	got := []string{}

	// Lets check that all go-routines were created and handled each message
	for i := 0; i < maxConcurrency; i++ {
		msg := fmt.Sprintf("message %d", i)
		want = append(want, msg)

		t.Log("publishing message")

		sendMsg([]byte(msg))

		t.Log("waiting for message received from subscription")
		got = append(got, string(<-gotMsgs))
		t.Log("message received from subscription")
	}

	sort.Strings(got)
	assertEqual(t, got, want)

	// Now lets ensure we didn't create any extra go-routines
	// Ensure is a strong word, this is time sensitive, but we don't have false positives
	// here, only false negatives, so good enough? (no random/wrong failures, only false successes maybe).
	const finalMsg = "final message"

	sendMsg([]byte(finalMsg))

	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	select {
	case msg := <-gotMsgs:
		t.Fatalf("got unexpected msg %q, an extra goroutine was created on the subscription", msg)
	case <-timeout.Done():
		break
	}

	// Now lets free all blocked handlers
	close(handlersDone)

	gotFinalMsg := string(<-gotMsgs)
	assertEqual(t, gotFinalMsg, finalMsg)

	if err := subscription.Shutdown(ctx); err != nil {
		t.Fatalf("shutting down subscription: %v", err)
	}

	// Wait for subscription to shutdown
	<-servingDone
}

func TestRawSubscriptionRecoversFromPanic(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, topic)

	const maxConcurrency = 1

	subscription, err := event.NewRawSubscription(url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	gotMsgs := make(chan event.Message)
	servingDone := make(chan struct{})

	go func() {
		err := subscription.Serve(func(msg event.Message) error {
			gotMsgs <- msg
			panic("oh no, something went terribly wrong")
		})
		t.Logf("rawsubscription.Serve error: %v", err)
		close(servingDone)
	}()

	const wantMsgBody = "test"
	if err := topic.Send(ctx, &pubsub.Message{Body: []byte(wantMsgBody)}); err != nil {
		t.Fatal(err)
	}

	timeout, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	select {
	case msg := <-gotMsgs:
		assertEqual(t, wantMsgBody, string(msg.Body))
	case <-timeout.Done():
		t.Fatal("timeout waiting for event")
		break
	}

	if err := subscription.Shutdown(ctx); err != nil {
		t.Fatalf("shutting down subscription: %v", err)
	}
	<-servingDone
}

func TestSubscriptionServingWithMetadata(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = topic.Shutdown(ctx) }()

	const maxConcurrency = 1
	eventName := t.Name()

	subscription, err := event.NewSubscription[struct{}](eventName, url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	receivedMetadata := make(chan event.Metadata)
	go func() {
		err := subscription.ServeWithMetadata(func(_ context.Context, _ struct{}, metadata event.Metadata) error {
			receivedMetadata <- metadata
			return nil
		})
		if err != nil {
			t.Errorf("subscription serve failed: %v", err)
		}
		close(receivedMetadata)
	}()

	wantAttributes := map[string]string{"key": t.Name()}
	publisher := event.NewPublisher[struct{}](eventName, topic)

	if err = publisher.PublishWithAttrs(ctx, struct{}{}, wantAttributes); err != nil {
		t.Fatalf("PublishWithAttrs failed: %v", err)
	}

	gotMetadata := <-receivedMetadata

	assertEqual(t, gotMetadata.Attributes, wantAttributes)
	// No easy way to test actual metadata, would need google cloud pubsub emulation or messing around with the pubsub driver
	var zeroTime time.Time
	assertEqual(t, gotMetadata.PublishedTime, zeroTime)
	assertEqual(t, gotMetadata.ID, "")
}

func TestRawSubscriptionServingWithMetadata(t *testing.T) {
	t.Parallel()

	url := newTopicURL(t)
	ctx := context.Background()

	topic, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = topic.Shutdown(ctx) }()

	const maxConcurrency = 1
	subscription, err := event.NewRawSubscription(url, maxConcurrency)
	if err != nil {
		t.Fatal(err)
	}

	receivedMsgs := make(chan event.Message)
	go func() {
		err := subscription.Serve(func(msg event.Message) error {
			receivedMsgs <- msg
			return nil
		})
		if err != nil {
			t.Errorf("subscription serve failed: %v", err)
		}
		close(receivedMsgs)
	}()

	wantBody := t.Name()
	wantAttributes := map[string]string{"key": t.Name()}

	if err := topic.Send(ctx, &pubsub.Message{Body: []byte(wantBody), Metadata: wantAttributes}); err != nil {
		t.Fatalf("publishing message: %v", err)
	}

	gotMsg := <-receivedMsgs

	assertEqual(t, string(gotMsg.Body), wantBody)
	assertEqual(t, gotMsg.Metadata.Attributes, wantAttributes)
	// No easy way to test actual metadata, would need google cloud pubsub emulation or messing around with the pubsub driver
	var zeroTime time.Time
	assertEqual(t, gotMsg.Metadata.PublishedTime, zeroTime)
	assertEqual(t, gotMsg.Metadata.ID, "")
}

type shutdowner interface {
	Shutdown(context.Context) error
}

func shutdown(t *testing.T, s shutdowner) {
	t.Helper()

	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func newTopicURL(t *testing.T) string {
	return "mem://" + t.Name()
}

func assertEqual[T any](t *testing.T, got T, want T) {
	t.Helper()
	// Parametric helps to ensure we don't compare things of different types (which doesn't make sense)
	// so we want 2 of any that are of the same type.
	// maybe this could be generalized in a small assert lib :-).

	if diff := cmp.Diff(got, want); diff != "" {
		t.Logf("got: %v", got)
		t.Logf("want: %v", want)
		t.Fatalf("diff: %v", diff)
	}
}
