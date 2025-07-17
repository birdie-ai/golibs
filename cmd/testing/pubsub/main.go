// Pubsub is a simple manual test for ordered pubsub.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/birdie-ai/golibs/event"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Event is the test event data.
type Event struct {
	PartitionID string
	Count       int
}

func main() {
	usage := func() {
		log.Fatalf("usage: %q [subscriber|publisher]", os.Args[0])
	}
	if len(os.Args) <= 1 {
		usage()
	}
	projectID := os.Getenv("GOOGLE_PROJECT")
	if projectID == "" {
		log.Fatal("missing env var: GOOGLE_PROJECT")
	}
	topicName := os.Getenv("TOPIC_NAME")
	if topicName == "" {
		log.Fatal("missing env var: TOPIC_NAME")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	switch os.Args[1] {
	case "subscriber":
		subscriber(ctx, projectID, topicName)
	case "publisher":
		publisher(ctx, projectID, topicName)
	default:
		usage()
	}
}

const (
	eventName       = "golibs-test"
	totalPartitions = 10
	totalEvents     = 100
)

func subscriber(ctx context.Context, projectID, topicName string) {
	client, err := pubsub.NewClient(ctx, projectID)
	panicerr(err)
	defer func() {
		_ = client.Close()
	}()

	_, err = client.CreateSubscription(ctx, topicName, pubsub.SubscriptionConfig{
		Topic:                 client.Topic(topicName),
		ExpirationPolicy:      24 * time.Hour,
		EnableMessageOrdering: true,
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("creating subscription: %v", err)
	}

	log.Printf("starting handler with concurrent=%d", totalPartitions)
	sub, err := event.NewOrderedGoogleSub[Event](ctx, projectID, topicName, eventName, totalPartitions)
	panicerr(err)

	err = sub.Serve(ctx, func(_ context.Context, event Event) error {
		fmt.Printf("partition %q: count %d\n", event.PartitionID, event.Count)
		time.Sleep(5 * time.Second)
		return nil
	})

	log.Printf("subscription stopped: %v", err)
}

func publisher(ctx context.Context, projectID, topicName string) {
	const region = "us-central1"
	client, err := pubsub.NewClient(ctx, projectID)
	panicerr(err)
	defer func() {
		_ = client.Close()
	}()

	createTopic(ctx, client, topicName)

	publisher, err := event.NewOrderedGooglePublisher[Event](ctx, projectID, region, topicName, eventName)
	panicerr(err)

	log.Printf("starting publisher with %d concurrent partitions", totalPartitions)

	g := &errgroup.Group{}

	for i := range totalPartitions {
		partitionID := fmt.Sprintf("partition-%d", i)
		g.Go(func() error {
			for i := range totalEvents {
				err := publisher.Publish(ctx, Event{
					PartitionID: partitionID,
					Count:       i,
				}, partitionID)
				panicerr(err)
			}
			return nil
		})
	}

	err = g.Wait()
	log.Printf("publishers stopped: %v", err)
}

func createTopic(ctx context.Context, client *pubsub.Client, topicName string) {
	_, err := client.CreateTopic(ctx, topicName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("creating topic: %v", err)
	}
	log.Print("created topic")
}

func panicerr(err error) {
	if err != nil {
		panic(err)
	}
}
