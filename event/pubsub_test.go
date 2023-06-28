package event_test

import (
	"context"
	"encoding/json"
	"testing"

	// load in memory driver
	"github.com/birdie-ai/golibs/event"
	"github.com/birdie-ai/golibs/tracing"
	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/mempubsub"
)

func TestPublishEvent(t *testing.T) {
	const url = "mem://test"

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
		// tracing info stored no the context is propagated to the events.
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
