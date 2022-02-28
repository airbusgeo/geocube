package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/airbusgeo/geocube/cmd"
	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/messaging/pubsub"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
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

	if err := cmd.InitGDAL(ctx, consolidaterConfig.GDALConfig); err != nil {
		return fmt.Errorf("init gdal: %w", err)
	}

	// Create Messaging Service
	var logMessaging string
	{
		// Connection to pubsub
		if consolidaterConfig.PsConsolidationsSubscription != "" {
			logMessaging += fmt.Sprintf(" pulling on %s/%s", consolidaterConfig.Project, consolidaterConfig.PsConsolidationsSubscription)
			consumer, err := pubsub.NewConsumer(consolidaterConfig.Project, consolidaterConfig.PsConsolidationsSubscription)
			if err != nil {
				return fmt.Errorf("pubsub.new: %w", err)
			}
			consumer.SetProcessOption(pubsub.OnErrorRetryDelay(60 * time.Second))
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

	handlerConsolidation := image.NewHandleConsolidation(image.NewCogGenerator(), image.NewMucogGenerator(), consolidaterConfig.CancelledJobsStorage, consolidaterConfig.Workers)
	log.Logger(ctx).Sugar().Debugf("consolidater starts "+logMessaging+" with %d worker(s)", consolidaterConfig.Workers)
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

			if msg.TryCount > consolidaterConfig.RetryCount {
				log.Logger(ctx).Sugar().Errorf("too many tries")
				if err := notify(ctx, evt, geocube.TaskFailed, fmt.Errorf("too many tries")); err != nil {
					return fmt.Errorf("failed to notify consolidation event: %w", err)
				}
				return nil
			}

			// Start consolidation
			var taskStatus geocube.TaskStatus
			log.Logger(ctx).Sugar().Infof("got message id %s in workdir %s : start consolidation of %d records into the container: %s", msg.ID, consolidaterConfig.WorkDir, len(evt.Records), evt.Container.URI)
			taskErr := handlerConsolidation.Consolidate(ctx, evt, consolidaterConfig.WorkDir)

			if taskErr != nil && utils.Temporary(taskErr) && msg.TryCount < consolidaterConfig.RetryCount {
				log.Logger(ctx).Sugar().Errorf("temporary error: %s", taskErr.Error())
				return taskErr
			}

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
	consolidaterConfig := consolidaterConfig{}

	flag.StringVar(&consolidaterConfig.Project, "psProject", "", "subscription project (gcp pubSub only)")
	flag.StringVar(&consolidaterConfig.PsConsolidationsSubscription, "psConsolidationsSubscription", "", "pubsub consolidation subscription name")
	flag.StringVar(&consolidaterConfig.PsEventsTopic, "psEventsTopic", "", "pubsub events topic name")
	flag.StringVar(&consolidaterConfig.WorkDir, "workdir", "", "scratch work directory")
	flag.StringVar(&consolidaterConfig.CancelledJobsStorage, "cancelledJobs", "", "storage where cancelled jobs are referenced")
	flag.IntVar(&consolidaterConfig.RetryCount, "retryCount", 1, "number of retries when consolidation job failed with a temporary error")
	flag.IntVar(&consolidaterConfig.Workers, "workers", 1, "number of workers for parallel tasks")
	consolidaterConfig.GDALConfig = cmd.GDALConfigFlags()

	flag.Parse()

	if consolidaterConfig.WorkDir == "" {
		return nil, fmt.Errorf("missing --workdir config flag")
	}
	if consolidaterConfig.CancelledJobsStorage == "" {
		return nil, fmt.Errorf("missing --cancelledJobs storage flag")
	}

	return &consolidaterConfig, nil
}

type consolidaterConfig struct {
	Project                      string
	PsEventsTopic                string
	WorkDir                      string
	PsConsolidationsSubscription string
	CancelledJobsStorage         string
	RetryCount                   int
	Workers                      int
	GDALConfig                   *cmd.GDALConfig
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
