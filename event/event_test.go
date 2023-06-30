package event_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	// load in memory driver
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
	defer func() { _ = subscription.Shutdown(ctx) }()

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
		// tracing info stored on the context is propagated to the events.
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

	want := event.Body[Event]{
		TraceID: traceID,
		OrgID:   orgID,
		Name:    eventName,
		Event:   wantEvt,
	}
	got := event.Body[Event]{}
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
	defer func() { _ = topic.Shutdown(ctx) }()

	type Event struct{}

	subscription, err := pubsub.OpenSubscription(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Shutdown(ctx) }()

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

	want := event.Body[Event]{
		Name: eventName,
	}
	got := event.Body[Event]{}
	if err := json.Unmarshal(gotMsg.Body, &got); err != nil {
		t.Fatal(err)
	}

	if want != got {
		t.Fatalf("got %+v != want %+v", got, want)
	}
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
		err := subscription.Serve(func(msg []byte) error {
			t.Logf("handler called, msg: %v", string(msg))
			gotMsgs <- msg
			// we block the handlers to ensure concurrency is being respected
			<-handlersDone
			return nil
		})
		t.Logf("subscription.Service error: %v", err)
		close(servingDone)
	}()

	want := []string{}
	got := []string{}

	// Lets check that all goroutines were created and handled each message
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

	// Now lets ensure we didn't create any extra goroutines
	// Ensure is a strong word, this is time sensitive, but we dont have false positives
	// here, only false negatives, so good enough ? (no random/wrong failures, only false successes maybe).
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

	// now lets free all blocked handlers
	close(handlersDone)

	gotFinalMsg := string(<-gotMsgs)
	assertEqual(t, gotFinalMsg, finalMsg)

	if err := subscription.Shutdown(ctx); err != nil {
		t.Fatalf("shutting down subscription: %v", err)
	}

	// wait for subscription to shutdown
	<-servingDone
}

func newTopicURL(t *testing.T) string {
	return "mem://" + t.Name()
}

func assertEqual[T any](t *testing.T, got T, want T) {
	t.Helper()
	// parametric helps to ensure we don't compare things of different types (which doesn't make sense)
	// so we want 2 of any that are of the same type.
	// maybe this could be generalized in an small assert lib :-).

	if diff := cmp.Diff(got, want); diff != "" {
		t.Logf("got: %v", got)
		t.Logf("want: %v", want)
		t.Fatalf("diff: %v", diff)
	}
}
