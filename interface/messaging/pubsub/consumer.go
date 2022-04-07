package pubsub

import (
	"context"
	"fmt"
	"os"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	gcppubsub "cloud.google.com/go/pubsub/apiv1"
	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes/timestamp"
)

type Client struct {
	ps                        *gcppubsub.SubscriberClient
	m                         *monitoring.MetricClient
	projectID, subscriptionID string
	processOpts               processOptions
}

type consumerOptions struct {
	ps *gcppubsub.SubscriberClient
	m  *monitoring.MetricClient
}

type ConsumerOption func(o *consumerOptions)

// Pull implements Messaging.Consumer
func (c *Client) Pull(ctx context.Context, cb messaging.Callback) error {
	return c.Process(ctx, cb)
}

func DefaultSubscriberClient(ctx context.Context) (*gcppubsub.SubscriberClient, error) {
	var o []option.ClientOption
	if addr := os.Getenv("PUBSUB_EMULATOR_HOST"); addr != "" {
		// Environment variables for gcloud emulator:
		// https://cloud.google.com/sdk/gcloud/reference/beta/emulators/pubsub/
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("grpc.Dial: %v", err)
		}
		o = []option.ClientOption{option.WithGRPCConn(conn)}
	}
	return gcppubsub.NewSubscriberClient(ctx, o...)
}

func WithSubscriberClient(ps *gcppubsub.SubscriberClient) ConsumerOption {
	return func(o *consumerOptions) {
		o.ps = ps
	}
}
func WithMonitoringClient(m *monitoring.MetricClient) ConsumerOption {
	return func(o *consumerOptions) {
		o.m = m
	}
}

// NewConsumer returns a pubsub task queue client.
func NewConsumer(projectID, subscriptionID string, opts ...ConsumerOption) (*Client, error) {
	cl := &Client{
		projectID:      projectID,
		subscriptionID: subscriptionID,
	}
	clOpts := consumerOptions{}
	for _, o := range opts {
		o(&clOpts)
	}
	cl.ps = clOpts.ps
	cl.m = clOpts.m
	cl.processOpts = processOptions{
		ExtensionPeriod:   8 * time.Minute,
		OnErrorRetryDelay: -1,
	}

	return cl, nil
}

type processOptions struct {
	ExtensionPeriod      time.Duration
	ReturnImmediately    bool
	OnErrorRetryDelay    time.Duration
	ExitOnExtensionError bool
}

type ProcessOption func(o *processOptions)

// ExtensionPeriod is the duration by which to extend the ack
// deadline at a time. The ack deadline will continue to be extended by up
// to this duration until MaxExtension is reached. Setting ExtensionPeriod
// bounds the maximum amount of time before a message redelivery in the
// event the subscriber fails to extend the deadline.
//
// ExtensionPeriod configuration can be disabled by specifying a
// duration less than (or equal to) 0.
func ExtensionPeriod(t time.Duration) ProcessOption {
	if t > 10*time.Minute {
		panic("ExtensionPeriod must be <= 10 minutes")
	}
	return func(o *processOptions) {
		o.ExtensionPeriod = t
	}
}

func OnErrorRetryDelay(t time.Duration) ProcessOption {
	return func(o *processOptions) {
		o.OnErrorRetryDelay = t
	}
}

//ReturnImmediately will return nil immediately if there are no messages to process. If
//not set, Process will block until a message becomes available
func ReturnImmediately() ProcessOption {
	return func(o *processOptions) {
		o.ReturnImmediately = true
	}
}

//ExitOnExtensionError will make Process return an error immediately if the pubsub extension/
//acknowledgement process failed, without waiting for the callback to return its success/failure
//(i.e. the callback is still runnning in a goroutine until it checks that it's context has been
//cancelled.
//This option can/should be used when Process() will be calling long-running cgo calls where we cannot
//ensure that context cancellation can be checked or acted upon (it is not possible to interrupt a cgo call).
//When using this option, and to avoid ressource leaks, it is highly recommended that
//the caller of Process() ensures that the whole program exits as soon as possible after it receives a non nil error
//return value.
func ExitOnExtensionError() ProcessOption {
	return func(o *processOptions) {
		o.ExitOnExtensionError = true
	}
}

func (c *Client) SetProcessOption(opts ...ProcessOption) {
	for _, o := range opts {
		o(&c.processOpts)
	}
}

func (c *Client) Process(ctx context.Context, cb messaging.Callback) error {
	if c.ps == nil {
		var err error
		if c.ps, err = DefaultSubscriberClient(context.Background()); err != nil {
			return fmt.Errorf("create subscriber client: %w", err)
		}
	}

	sub := fmt.Sprintf("projects/%s/subscriptions/%s", c.projectID, c.subscriptionID)
	// Be sure to tune the MaxMessages parameter per your project's needs, and accordingly
	// adjust the ack behavior below to batch acknowledgements.
	req := pubsubpb.PullRequest{
		Subscription:      sub,
		MaxMessages:       1,
		ReturnImmediately: c.processOpts.ReturnImmediately,
	}

	var res *pubsubpb.PullResponse
	var err error
	for {
		res, err = c.ps.Pull(ctx, &req)
		if err != nil {
			return fmt.Errorf("ps.pull: %w", err)
		}
		// client.Pull returns an empty list if there are no messages available in the
		// backlog. We should skip processing steps when that happens.
		if len(res.ReceivedMessages) == 0 {
			if c.processOpts.ReturnImmediately {
				return nil
			}
			continue
		}
		if len(res.ReceivedMessages) > 1 {
			return fmt.Errorf("pull returned %d!=1 messages", len(res.ReceivedMessages))
		}
		break
	}

	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	//needs to be buffered so that the error can go through even if the extender
	//has exited (on done context, or on failed extension)
	errChan := make(chan error, 1)
	keepAliveError := make(chan error)

	go func() {
		deadline := time.Now().Add(c.processOpts.ExtensionPeriod) //when the task will expire

		next := time.Duration(0) //immediately extend the first time
		for {
			select {
			case <-ctx.Done():
				keepAliveError <- ctx.Err()
				return
			case err := <-errChan:
				if err == nil {
					// Acknowledgement
					req := pubsubpb.AcknowledgeRequest{
						Subscription: sub,
						AckIds:       []string{res.ReceivedMessages[0].AckId},
					}
					keepAliveError <- c.ps.Acknowledge(ctx, &req)
				} else if !utils.Temporary(err) {
					// Fatal error : acknowledgement
					log.Logger(ctx).Error(err.Error())
					req := pubsubpb.AcknowledgeRequest{
						Subscription: sub,
						AckIds:       []string{res.ReceivedMessages[0].AckId},
					}
					keepAliveError <- c.ps.Acknowledge(ctx, &req)
				} else {
					// Temporary error : retry
					log.Logger(ctx).Warn("Temporary error:" + err.Error())
					if c.processOpts.OnErrorRetryDelay >= 0 {
						keepAliveError <- c.ps.ModifyAckDeadline(ctx, &pubsubpb.ModifyAckDeadlineRequest{
							Subscription:       sub,
							AckIds:             []string{res.ReceivedMessages[0].AckId},
							AckDeadlineSeconds: int32(c.processOpts.OnErrorRetryDelay.Seconds()),
						})
					} else {
						keepAliveError <- nil
					}
				}
				return
			case <-time.After(next):
				if time.Now().After(deadline) {
					cncl() //stop cb
					keepAliveError <- fmt.Errorf("failed to extend past deadline")
					return
				}
				err := c.ps.ModifyAckDeadline(ctx, &pubsubpb.ModifyAckDeadlineRequest{
					Subscription:       sub,
					AckIds:             []string{res.ReceivedMessages[0].AckId},
					AckDeadlineSeconds: int32(c.processOpts.ExtensionPeriod.Seconds()),
				})
				if err != nil {
					next = next / 2
					if next < time.Second {
						next = time.Second
					}
					log.Logger(ctx).With(zap.Error(err)).Sugar().Warnf(
						"error extending, will retry in %v", next)
				} else {
					deadline = time.Now().Add(c.processOpts.ExtensionPeriod) //when the task will expire
					next = c.processOpts.ExtensionPeriod / 2
				}
			}
		}
	}()

	//wait for callback to terminate
	msg := messaging.Message{
		ID:         res.ReceivedMessages[0].Message.MessageId,
		Attributes: res.ReceivedMessages[0].Message.Attributes,
		Data:       res.ReceivedMessages[0].Message.Data,
		TryCount:   int(res.ReceivedMessages[0].DeliveryAttempt),
	}

	if !c.processOpts.ExitOnExtensionError {
		//block until callback has returned
		errChan <- cb(ctx, &msg)
	} else {
		go func() {
			errChan <- cb(ctx, &msg)
		}()
	}

	//wait for keepalive/ack to terminate
	//if opt.ExitOnExtensionError is true, then cb will be left running until it catches <-ctx.Done()
	//which might take some time if e.g. in a cgo call.
	return <-keepAliveError
}

// Backlog implements interface.autoscaler.qbas.Queue
func (c *Client) Backlog(ctx context.Context) (int64, error) {
	if c.m == nil {
		var err error
		if c.m, err = monitoring.NewMetricClient(context.Background()); err != nil {
			return 0, fmt.Errorf("create metric client: %w", err)
		}
	}
	req := &monitoringpb.ListTimeSeriesRequest{
		Name: fmt.Sprintf("projects/%s", c.projectID),
		Filter: fmt.Sprintf("metric.type = \"pubsub.googleapis.com/subscription/num_undelivered_messages\" AND resource.label.subscription_id = \"%s\"",
			c.subscriptionID),
		Interval: &monitoringpb.TimeInterval{
			StartTime: &timestamp.Timestamp{Seconds: time.Now().Add(-2 * time.Minute).Unix()},
			EndTime:   &timestamp.Timestamp{Seconds: time.Now().Unix()},
		},
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}
	it := c.m.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("it.next: %w", err)
		}
		pnts := resp.GetPoints()
		if len(pnts) == 0 {
			continue
		}
		return pnts[len(pnts)-1].Value.GetInt64Value(), nil
	}
	log.Logger(ctx).Warn("no monitoring metrics found. Does the subscription exist?")
	return 0, nil
}
