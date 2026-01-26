// Pubsub is a simple manual test for ordered pubsub.
package main

import (
	"context"
	"flag"
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

	_ "gocloud.dev/pubsub/gcppubsub"
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

	createTopic(ctx, projectID, topicName)

	switch os.Args[1] {
	case "subscriber":
		createSubscription(ctx, projectID, topicName)
		args := os.Args[2:]
		subscriber(ctx, args, projectID, topicName)
	case "publisher":
		publisher(ctx, projectID, topicName)
	default:
		usage()
	}
}

const (
	totalPartitions = 10
	totalEvents     = 100
)

func subscriber(ctx context.Context, args []string, projectID, topicName string) {
	var batch bool
	fset := flag.NewFlagSet("subscriber", flag.ExitOnError)
	fset.BoolVar(&batch, "batch", false, "test batched ordering")
	panicerr(fset.Parse(args))

	if batch {
		subscriberBatch(ctx, projectID, topicName)
		return
	}

	log.Printf("starting handler with concurrency=%d", totalPartitions)
	sub, err := event.NewOrderedGoogleSub[Event](ctx, projectID, topicName, topicName, totalPartitions)
	panicerr(err)

	err = sub.Serve(ctx, func(_ context.Context, event Event) error {
		fmt.Printf("partition %q: count %d\n", event.PartitionID, event.Count)
		time.Sleep(5 * time.Second)
		return nil
	})

	log.Printf("subscription stopped: %v", err)
}

func createSubscription(ctx context.Context, projectID, name string) {
	client, err := pubsub.NewClient(ctx, projectID)
	panicerr(err)
	defer func() {
		_ = client.Close()
	}()

	_, err = client.CreateSubscription(ctx, name, pubsub.SubscriptionConfig{
		Topic:                 client.Topic(name),
		ExpirationPolicy:      24 * time.Hour,
		EnableMessageOrdering: true,
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("creating subscription: %v", err)
	}
}

func subscriberBatch(ctx context.Context, projectID, topicName string) {
	// We need to split the Receive/ReceiveN subscription from the "server like" subscription.
	// Right now they are together in the same object and the concurrency configuration makes no sense :-(.
	const unusedMaxConcurrency = 1

	url := fmt.Sprintf("gcppubsub://projects/%s/subscriptions/%s", projectID, topicName)
	sub, err := event.NewSubscription[Event](topicName, url, unusedMaxConcurrency)
	panicerr(err)

	const (
		batchSize       = 200
		batchTimeWindow = time.Minute
	)

	log.Println("starting batch handler")
	batches := map[string][]int{}

	for ctx.Err() == nil {
		ctx, cancel := context.WithTimeout(ctx, batchTimeWindow)
		events, err := sub.ReceiveN(ctx, batchSize)
		cancel()
		panicerr(err)

		fmt.Printf("=== start batch size %d ===\n", len(events))
		for i, e := range events {
			fmt.Printf("event %d: partition %q: count %d\n", i, e.Event.PartitionID, e.Event.Count)
			batches[e.Event.PartitionID] = append(batches[e.Event.PartitionID], e.Event.Count)
			e.Ack()
		}
		fmt.Printf("=== end batch size %d ===\n", len(events))
	}

	log.Printf("generating received batches report (values should be in order)\n")
	for partitionID, values := range batches {
		fmt.Printf("\tpartition %q: values: %v\n", partitionID, values)
	}
	log.Printf("done\n")
}

func publisher(ctx context.Context, projectID, topicName string) {
	const region = "us-central1"

	publisher, err := event.NewOrderedGooglePublisher[Event](ctx, projectID, region, topicName, topicName)
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

func createTopic(ctx context.Context, projectID, topicName string) {
	client, err := pubsub.NewClient(ctx, projectID)
	panicerr(err)
	defer func() {
		_ = client.Close()
	}()

	_, err = client.CreateTopic(ctx, topicName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("creating topic: %v", err)
	}
}

func panicerr(err error) {
	if err != nil {
		panic(err)
	}
}
