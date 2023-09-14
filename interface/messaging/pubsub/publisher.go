package pubsub

import (
	"context"
	"fmt"
	"math"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/airbusgeo/geocube/internal/utils"
)

type publisherOptions struct {
	maxRetries int
}

type PublisherOption func(o *publisherOptions)

func WithMaxRetries(maxRetries int) PublisherOption {
	return func(o *publisherOptions) {
		o.maxRetries = maxRetries
	}
}

// Publisher implements messaging.Publisher
type Publisher struct {
	topic      *pubsub.Topic
	maxRetries int
}

// NewPublisher creates a pubsub publisher
func NewPublisher(ctx context.Context, projectID, topic string, opts ...PublisherOption) (*Publisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("NewPublisher.NewClient: %w", err)
	}

	clOpts := publisherOptions{}
	for _, o := range opts {
		o(&clOpts)
	}

	return &Publisher{topic: client.Topic(topic), maxRetries: clOpts.maxRetries}, nil
}

// Publish implements messaging.Publisher
func (p *Publisher) Publish(ctx context.Context, data ...[]byte) error {
	return p.publish(ctx, 0, data...)
}

// Publish implements messaging.Publisher
func (p *Publisher) publish(ctx context.Context, retry int, data ...[]byte) error {
	retryIds := []int{}

	for i := 0; i < len(data); {
		var results []*pubsub.PublishResult
		for limit := pubsub.DefaultPublishSettings.BufferedByteLimit / 2; i < len(data) && limit > 0; i++ {
			// len(data[i]) is assumed to be smaller than half the BufferedByteLimit.
			// It's dirty, but it's a way to control the size of the buffered messages without knowing their exact size ((which is bigger than len(data[i]))
			limit -= len(data[i])
			results = append(results, p.topic.Publish(ctx, &pubsub.Message{
				Data: data[i],
			}))
		}

		for i, r := range results {
			// Block until the result is returned and a server-generated ID is returned for the published message.
			if _, err := r.Get(ctx); err != nil {
				if utils.Retriable(err) && retry < p.maxRetries {
					retryIds = append(retryIds, i)
				} else {
					return fmt.Errorf("Publish: %w", err)
				}
			}
		}
	}

	if len(retryIds) > 0 {
		ndata := [][]byte{}
		for _, i := range retryIds {
			ndata = append(ndata, data[i])
		}
		time.Sleep(time.Second * time.Duration(math.Exp2(float64(retry))))
		return p.publish(ctx, retry+1, ndata...)
	}

	return nil
}

func (p *Publisher) Stop() {
	p.topic.Stop()
}
