package main

import (
	"context"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
)

var projectID = "projectID"
var consolidationsTopic = "consolidations"
var eventTopic = "events"
var consolidationsSubscription = consolidationsTopic
var eventSubscription = eventTopic
var endPoint = "http://127.0.0.1:8080/push"

func main() {
	ctx := context.Background()

	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085")

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("pubsub.NewClient: %v", err)
	}

	log.Print("Create Topic : Consolidation")
	if _, err = client.CreateTopic(ctx, consolidationsTopic); err != nil {
		log.Fatalf("pubsub.CreateTopic: %v", err)
	}

	log.Print("Create Topic : Event")
	if _, err = client.CreateTopic(ctx, eventTopic); err != nil {
		log.Fatalf("pubsub.CreateTopic: %v", err)
	}

	log.Print("Create Subscription : Consolidation")
	if _, err = client.CreateSubscription(ctx, consolidationsSubscription, pubsub.SubscriptionConfig{
		Topic:       client.Topic(consolidationsTopic),
		AckDeadline: 10 * time.Second,
	}); err != nil {
		log.Fatalf("CreateSubscription: %v", err)
	}

	log.Print("Create Subscription : Consolidation")
	if _, err = client.CreateSubscription(ctx, consolidationsSubscription, pubsub.SubscriptionConfig{
		Topic:       client.Topic(consolidationsTopic),
		AckDeadline: 10 * time.Second,
	}); err != nil {
		log.Fatalf("CreateSubscription: %v", err)
	}

	log.Print("Create Subscription : Event")
	if _, err = client.CreateSubscription(ctx, eventSubscription, pubsub.SubscriptionConfig{
		Topic:       client.Topic(eventTopic),
		AckDeadline: 10 * time.Second,
		PushConfig:  pubsub.PushConfig{Endpoint: endPoint},
	}); err != nil {
		log.Fatalf("CreateSubscription: %v", err)
	}
}
