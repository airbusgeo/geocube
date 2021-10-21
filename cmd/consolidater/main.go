package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/messaging/pubsub"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/godal"
	"github.com/airbusgeo/osio"
	osioGcs "github.com/airbusgeo/osio/gcs"
	"go.uber.org/zap"
)

var (
	eventPublisher messaging.Publisher
	taskConsumer   messaging.Consumer
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	ctx := context.Background()
	err := run(ctx)
	if err != nil {
		log.Logger(ctx).Error("exit on error", zap.Error(err))
	} else {
		log.Logger(ctx).Info("exiting")
	}
}

func run(ctx context.Context) error {
	consolidaterConfig, err := newConsolidationAppConfig()
	if err != nil {
		return err
	}

	jobStarted := time.Time{}
	go func() {
		http.HandleFunc("/termination_cost", func(w http.ResponseWriter, r *http.Request) {
			terminationCost := 0
			if jobStarted != (time.Time{}) {
				terminationCost = int(time.Since(jobStarted).Seconds() * 1000) //milliseconds since task was leased
			}
			fmt.Fprintf(w, "%d", terminationCost)
		})
		http.ListenAndServe(":9000", nil)
	}()

	godal.RegisterAll()

	osioGCSHandle, err := osioGcs.Handle(ctx)
	if err != nil {
		return fmt.Errorf("gcshandler: %w", err)
	}
	gcsa, err := osio.NewAdapter(osioGCSHandle,
		osio.BlockSize("1Mb"),
		osio.NumCachedBlocks(500))
	if err != nil {
		return fmt.Errorf("adapter: %w", err)
	}
	if err := godal.RegisterVSIAdapter("gs://", gcsa); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Create Messaging Service
	var logMessaging string
	{
		// Connection to pubsub
		if consolidaterConfig.PsSubscriptionName != "" {
			logMessaging += fmt.Sprintf(" pulling on %s/%s", consolidaterConfig.Project, consolidaterConfig.PsSubscriptionName)
			consumer, err := pubsub.NewConsumer(consolidaterConfig.Project, consolidaterConfig.PsSubscriptionName)
			if err != nil {
				return fmt.Errorf("pubsub.new: %w", err)
			}
			consumer.SetProcessOption(pubsub.MaxTries(1))
			taskConsumer = consumer
		}

		if consolidaterConfig.PsEventsTopic != "" {
			logMessaging += fmt.Sprintf(" pushing on %s/%s", consolidaterConfig.Project, consolidaterConfig.PsEventsTopic)
			p, err := pubsub.NewPublisher(ctx, consolidaterConfig.Project, consolidaterConfig.PsEventsTopic)
			if err != nil {
				return fmt.Errorf("pubsub.newpublisher: %w", err)
			}
			defer p.Stop()
			eventPublisher = p
		}
	}
	if taskConsumer == nil {
		return fmt.Errorf("missing configuration for taskConsumer")
	}
	if eventPublisher == nil {
		return fmt.Errorf("missing configuration for eventPublisher")
	}

	handlerConsolidation := image.NewHandleConsolidation(image.NewCogGenerator(), image.NewMucogGenerator(), consolidaterConfig.CancelledJobsStorage)
	log.Logger(ctx).Debug("consolidater starts " + logMessaging)
	for {
		err := taskConsumer.Pull(ctx, func(ctx context.Context, msg *messaging.Message) error {
			jobStarted = time.Now()
			defer func() {
				jobStarted = time.Time{}
			}()

			// Retrieve consolidation event
			evt, err := geocube.UnmarshalConsolidationEvent(bytes.NewReader(msg.Data))
			if err != nil {
				return fmt.Errorf("got message id %s in workdir %s : unreadable (%d bytes): %w", msg.ID, consolidaterConfig.WorkDir, len(msg.Data), err)
			}

			var taskStatus geocube.TaskStatus
			var taskErr error

			if msg.TryCount > consolidaterConfig.RetryCount {
				log.Logger(ctx).Sugar().Errorf("message have already been consumed")
				taskErr = errors.New("message have already been consumed")
				if err := notify(ctx, evt, geocube.TaskFailed, taskErr); err != nil {
					return fmt.Errorf("failed to notify consolidation event: %w", err)
				}
				return nil
			}

			// Start consolidation
			log.Logger(ctx).Sugar().Infof("got message id %s in workdir %s : start consolidation of %d records into the container: %s", msg.ID, consolidaterConfig.WorkDir, len(evt.Records), evt.Container.URI)
			taskErr = handlerConsolidation.Consolidate(ctx, evt, consolidaterConfig.WorkDir)
			switch {
			case taskErr == nil:
				taskStatus = geocube.TaskSuccessful
			case taskErr == image.TaskCancelledConsolidationError:
				taskStatus = geocube.TaskCancelled
			case taskErr == image.NotImplementedError:
				taskStatus = geocube.TaskIgnored
			default:
				log.Logger(ctx).Sugar().Errorf("failed to consolidate: %s", taskErr.Error())
				taskStatus = geocube.TaskFailed
			}

			if err = notify(ctx, evt, taskStatus, taskErr); err != nil {
				return fmt.Errorf("failed to notify consolidation event: %w", err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("cl.process: %w", err)
		}
	}
}

func newConsolidationAppConfig() (*consolidaterConfig, error) {
	project := flag.String("project", "", "subscription project (gcp pubSub only)")
	psSubscriptionName := flag.String("psSubscription", "", "pubsub subscription name")
	psEventTopic := flag.String("psEventTopic", "", "pubsub events topic name")
	workdir := flag.String("workdir", "", "scratch work directory")
	cancelledJobs := flag.String("cancelledJobs", "", "storage where cancelled jobs are referenced")
	retryCount := flag.Int("retryCount", 1, "number of retries when consolidation job failed (default: 1)")

	flag.Parse()

	if *workdir == "" {
		return nil, fmt.Errorf("missing --workdir config flag")
	}
	if *cancelledJobs == "" {
		return nil, fmt.Errorf("missing --cancelledJobs storage flag")
	}

	return &consolidaterConfig{
		PsSubscriptionName:   *psSubscriptionName,
		WorkDir:              *workdir,
		Project:              *project,
		PsEventsTopic:        *psEventTopic,
		CancelledJobsStorage: *cancelledJobs,
		RetryCount:           *retryCount,
	}, nil
}

type consolidaterConfig struct {
	Project               string
	PsEventsTopic         string
	PsConsolidationsTopic string
	WorkDir               string
	PsSubscriptionName    string
	CancelledJobsStorage  string
	RetryCount            int
}

func notify(ctx context.Context, evt *geocube.ConsolidationEvent, taskStatus geocube.TaskStatus, taskError error) error {
	taskEvt := geocube.NewTaskEvent(evt.JobID, evt.TaskID, taskStatus, taskError)
	data, err := geocube.MarshalEvent(*taskEvt)
	if err != nil {
		return utils.MakeTemporary(fmt.Errorf("MarshalTaskEvent: %w", err))
	}

	if err = eventPublisher.Publish(ctx, data); err != nil {
		return utils.MakeTemporary(fmt.Errorf("PublishTaskEvent: %w", err))
	}

	return nil
}
