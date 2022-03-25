package messaging

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	// If callback returns a temporary error, the message must be rescheduled, increasing its trycount
	Pull(ctx context.Context, cb Callback) error
}

// Consume the request, call callback and return http code (pushing mode)
func Consume(req http.Request, cb Callback) (int, error) {
	ctx := req.Context()
	if req.Method != "POST" {
		return http.StatusMethodNotAllowed, nil
	}

	psm := struct {
		Message struct {
			Data string `json:"data"`
		} `json:"message"`
	}{}

	err := json.NewDecoder(req.Body).Decode(&psm)
	if err != nil {
		return 400, err
	}

	data, err := base64.StdEncoding.DecodeString(psm.Message.Data)
	if err != nil {
		return 400, err
	}

	if err := cb(ctx, &Message{Data: data, PublishTime: time.Now()}); err != nil {
		return 503, err
	}
	return 200, nil
}
