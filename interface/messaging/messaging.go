package messaging

import (
	"context"
	"net/http"
	"time"
)

// Publisher is an interface to publish messages
type Publisher interface {
	Publish(ctx context.Context, data ...[]byte) error
}

type Message struct {
	ID          string
	Data        []byte
	Attributes  map[string]string
	PublishTime time.Time
	TryCount    int
}

// Callback is a function that processes a Message.
type Callback func(ctx context.Context, m *Message) error

// Consumer is an interface to consume messages
type Consumer interface {
	// Pull the next message, call callback and return
	Pull(ctx context.Context, cb Callback) error

	// Consume the request, call callback and return http code (pushing mode)
	Consume(req http.Request, cb Callback) (int, error)
}
