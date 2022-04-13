package pgqueue

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/btubbs/pgq"
	"github.com/sirupsen/logrus"
)

type PublisherOption func(o *Publisher)
type ConsumerOption func(o *Consumer)

func WithMaxRetries(maxRetries int) PublisherOption {
	return func(p *Publisher) {
		p.maxRetries = maxRetries
	}
}

// Publisher implements messaging.Publisher
type Publisher struct {
	worker     *pgq.Worker
	queueName  string
	maxRetries int
}

// Consumer implements messaging.Consumer and interface.autoscaler.qbas.Queue
type Consumer struct {
	db        *sql.DB
	worker    *pgq.Worker
	queueName string
	run       bool
}

func SetDefaultLogger() pgq.WorkerOption {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.WarnLevel)
	logger.SetOutput(os.Stdout)
	return pgq.SetLogger(logger)
}

func SqlConnect(ctx context.Context, dbConnection string) (*sql.DB, *pgq.Worker, error) {
	db, err := sql.Open("postgres", dbConnection)
	if err != nil {
		return nil, nil, fmt.Errorf("pgqueue.Connect: %w", err)
	}
	db.SetMaxOpenConns(5)
	if err := db.PingContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("pgqueue.Connect: failed to ping database: %w", err)
	}

	return db, pgq.NewWorker(db, SetDefaultLogger()), nil
}

// NewPublisher returns a pg queue publisher.
// A publisher can share its worker with another instance
func NewPublisher(w *pgq.Worker, queueName string, opts ...PublisherOption) *Publisher {
	p := &Publisher{
		queueName:  queueName,
		worker:     w,
		maxRetries: 0,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Publish implements Publisher
func (p *Publisher) Publish(ctx context.Context, data ...[]byte) error {
	return p.publish(ctx, 0, data...)
}

// publish with retry
func (p *Publisher) publish(ctx context.Context, retry int, data ...[]byte) error {
	retryIds := []int{}
	for _, d := range data {
		jobID, err := p.worker.EnqueueJob(p.queueName, d)
		if utils.Temporary(err) && retry < p.maxRetries {
			retryIds = append(retryIds, jobID)
		} else if err != nil {
			return fmt.Errorf("pgQueue.Publish: %w", err)
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

// NewConsumer returns a pg queue consumer.
// A consumer cannot share the worker with another instance
func NewConsumer(db *sql.DB, queueName string, opts ...ConsumerOption) *Consumer {
	c := &Consumer{
		db:        db,
		queueName: queueName,
		worker:    pgq.NewWorker(db, SetDefaultLogger()),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Pull implements Consumer
func (c *Consumer) Pull(ctx context.Context, cb messaging.Callback) error {
	cbwc := CallbackWithContext{
		ctx: ctx,
		cb:  cb,
	}
	if err := c.worker.RegisterQueue(c.queueName, cbwc.handler); err != nil {
		return fmt.Errorf("pgQueue.Pull.RegisterQueue: %w", err)
	}
	c.run = true
	return c.worker.Run()
}

func (c *Consumer) Stop() {
	if c.run {
		c.worker.StopChan <- true
	}
}

type CallbackWithContext struct {
	ctx context.Context
	cb  messaging.Callback
}

func (c *CallbackWithContext) handler(data []byte) error {
	if err := c.cb(c.ctx, &messaging.Message{
		ID:          "",
		Data:        data,
		Attributes:  map[string]string{},
		PublishTime: time.Time{},
		TryCount:    -1,
	}); err != nil {
		if utils.Temporary(err) {
			// Temporary error : retry
			log.Logger(c.ctx).Warn("Temporary error: " + err.Error())
			return err
		}
		// Fatal error : acknowledgement
		log.Logger(c.ctx).Error("Fatal error: " + err.Error())
		return nil
	}
	return nil
}

// Backlog implements interface.autoscaler.qbas.Queue
func (c *Consumer) Backlog(ctx context.Context) (int64, error) {
	//return c.worker.Count(c.queueName)
	var count int64
	if err := c.db.QueryRow(`
		SELECT count(*) FROM pgq_jobs
		WHERE
			queue_name = $1
			AND run_after < $2
			AND ran_at IS NULL;`, c.queueName, time.Now()).Scan(&count); err != nil {
		return -1, fmt.Errorf("Backlog: could not count jobs: %w", err)
	}
	return count, nil
}
