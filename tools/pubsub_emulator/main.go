package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var consolidationsTopic = "consolidations"
var consolidationsWorkerTopic = "consolidations-worker"
var eventTopic = "events"
var consolidationsSubscription = consolidationsTopic
var consolidationsWorkerSubscription = consolidationsWorkerTopic
var eventSubscription = eventTopic

func main() {
	ctx := context.Background()

	endPoint := flag.String("geocube-server", "http://127.0.0.1:8080", "geocube server uri")
	projectID := flag.String("project", "geocube-emulator", "emulator project id")
	flag.Parse()

	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085")
	*endPoint += "/push"

	log.Print("New client for project-id " + *projectID)
	client, err := pubsub.NewClient(ctx, *projectID)
	if err != nil {
		log.Fatalf("pubsub.NewClient: %v", err)
	}

	log.Print("Create Topic : " + consolidationsTopic)
	if _, err = client.CreateTopic(ctx, consolidationsTopic); err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("pubsub.CreateTopic: %v", err)
	}

	log.Print("Create Topic : " + consolidationsWorkerTopic)
	if _, err = client.CreateTopic(ctx, consolidationsWorkerTopic); err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("pubsub.CreateTopic: %v", err)
	}

	log.Print("Create Topic : " + eventTopic)
	if _, err = client.CreateTopic(ctx, eventTopic); err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("pubsub.CreateTopic: %v", err)
	}

	log.Print("Create Subscription : " + consolidationsSubscription)
	if _, err = client.CreateSubscription(ctx, consolidationsSubscription, pubsub.SubscriptionConfig{
		Topic:       client.Topic(consolidationsTopic),
		AckDeadline: 10 * time.Second,
	}); err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("CreateSubscription: %v", err)
	}

	log.Print("Create Subscription : " + consolidationsWorkerSubscription)
	if _, err = client.CreateSubscription(ctx, consolidationsWorkerSubscription, pubsub.SubscriptionConfig{
		Topic:       client.Topic(consolidationsWorkerTopic),
		AckDeadline: 10 * time.Second,
	}); err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("CreateSubscription: %v", err)
	}

	log.Print("Create Subscription : " + eventSubscription + " pushing to " + *endPoint)
	if _, err = client.CreateSubscription(ctx, eventSubscription, pubsub.SubscriptionConfig{
		Topic:       client.Topic(eventTopic),
		AckDeadline: 10 * time.Second,
		PushConfig:  pubsub.PushConfig{Endpoint: *endPoint},
	}); err != nil && status.Code(err) != codes.AlreadyExists {
		log.Fatalf("CreateSubscription: %v", err)
	}
	log.Print("Done!")
}
