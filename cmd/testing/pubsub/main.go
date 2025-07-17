package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func subscriber(ctx context.Context, projectID, topicName string) {
	client, err := pubsub.NewClient(ctx, projectID)
	panicerr(err)
	defer client.Close()

	// Message ordering can only be set when creating a subscription.
	_, err = client.CreateSubscription(ctx, topicName, pubsub.SubscriptionConfig{
		Topic:                 client.Topic(topicName),
		ExpirationPolicy:      time.Hour,
		EnableMessageOrdering: true,
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("creating subscription: %v", err)
	}

	log.Print("created subscription")
}

func publisher(ctx context.Context, projectID, topicName string) {
	client, err := pubsub.NewClient(ctx, projectID)
	panicerr(err)
	defer client.Close()

	// Message ordering can only be set when creating a subscription.
	_, err = client.CreateTopic(ctx, topicName)
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
