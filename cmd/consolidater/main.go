package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/airbusgeo/geocube/cmd"
	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/messaging/pgqueue"
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
		log.Logger(ctx).Fatal("exit on error", zap.Error(err))
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
		if consolidaterConfig.PgqDbConnection != "" {
			db, w, err := pgqueue.SqlConnect(ctx, consolidaterConfig.PgqDbConnection)
			if err != nil {
				return fmt.Errorf("pgqMessaging: %w", err)
			}
			if consolidaterConfig.ConsolidationsQueue != "" {
				logMessaging += fmt.Sprintf(" pulling on pgqueue:%s", consolidaterConfig.ConsolidationsQueue)
				consumer := pgqueue.NewConsumer(db, consolidaterConfig.ConsolidationsQueue)
				defer consumer.Stop()
				//consumer.SetProcessOption(pubsub.OnErrorRetryDelay(60 * time.Second))
				taskConsumer = consumer
			}

			if consolidaterConfig.EventsQueue != "" {
				logMessaging += fmt.Sprintf(" publishing on pgqueue:%s", consolidaterConfig.EventsQueue)
				eventPublisher = pgqueue.NewPublisher(w, consolidaterConfig.EventsQueue)
			}
		} else {
			// Connection to pubsub
			if consolidaterConfig.ConsolidationsQueue != "" {
				logMessaging += fmt.Sprintf(" pulling on pubsub:%s/%s", consolidaterConfig.Project, consolidaterConfig.ConsolidationsQueue)
				consumer, err := pubsub.NewConsumer(consolidaterConfig.Project, consolidaterConfig.ConsolidationsQueue)
				if err != nil {
					return fmt.Errorf("pubsub.new: %w", err)
				}
				consumer.SetProcessOption(pubsub.OnErrorRetryDelay(60 * time.Second))
				taskConsumer = consumer
			}

			if consolidaterConfig.EventsQueue != "" {
				logMessaging += fmt.Sprintf(" publishing on pubsub:%s/%s", consolidaterConfig.Project, consolidaterConfig.EventsQueue)
				p, err := pubsub.NewPublisher(ctx, consolidaterConfig.Project, consolidaterConfig.EventsQueue)
				if err != nil {
					return fmt.Errorf("pubsub.newpublisher: %w", err)
				}
				defer p.Stop()
				eventPublisher = p
			}
		}
	}
	if taskConsumer == nil {
		return fmt.Errorf("missing configuration for taskConsumer")
	}
	if eventPublisher == nil {
		return fmt.Errorf("missing configuration for eventPublisher")
	}
	if consolidaterConfig.LocalDownload && consolidaterConfig.LocalDownloadMaxMb == 0 {
		consolidaterConfig.LocalDownloadMaxMb = math.MaxInt64
	}

	handlerConsolidation := image.NewHandleConsolidation(image.NewCogGenerator(), image.NewMucogGenerator(), consolidaterConfig.CancelledJobsStorage, consolidaterConfig.Workers, consolidaterConfig.LocalDownloadMaxMb)
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
			ctx = log.WithFields(ctx, zap.String("JobID", evt.JobID), zap.String("TaskID", evt.TaskID))

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

	// Configuration
	flag.StringVar(&consolidaterConfig.WorkDir, "workdir", "", "scratch work directory")
	flag.StringVar(&consolidaterConfig.CancelledJobsStorage, "cancelledJobs", "", "storage where cancelled jobs are referenced")
	flag.IntVar(&consolidaterConfig.RetryCount, "retryCount", 1, "number of retries when consolidation job failed with a temporary error")
	flag.IntVar(&consolidaterConfig.Workers, "workers", 1, "number of workers to parallelize the processing of the slices of a cube (see also GdalMultithreading)")
	flag.BoolVar(&consolidaterConfig.LocalDownload, "local-download", true, "DEPRECTATED: use --local-download-max-mb instead. locally download the datasets before starting the consolidation (generally faster than letting GDAL to download them tile by tile)")
	flag.IntVar(&consolidaterConfig.LocalDownloadMaxMb, "local-download-max-mb", 0, "maximum storage (in Mb) usable to download the datasets before starting the consolidation (generally faster than letting GDAL to download them tile by tile). 0 to disable local download.")

	// Messaging
	flag.StringVar(&consolidaterConfig.PgqDbConnection, "pgqConnection", "", "url of the postgres database to enable pgqueue messaging system (pgqueue only)")
	flag.StringVar(&consolidaterConfig.Project, "psProject", "", "subscription project (gcp pubSub only)")
	flag.StringVar(&consolidaterConfig.ConsolidationsQueue, "consolidationsQueue", "", "name of the messaging queue for consolidation jobs (pgqueue or pubsub subscription)")
	flag.StringVar(&consolidaterConfig.EventsQueue, "eventsQueue", "", "name of the messaging queue for job events (pgquue or pubsub topic)")

	// GDAL
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
	Project              string
	EventsQueue          string
	PgqDbConnection      string
	WorkDir              string
	ConsolidationsQueue  string
	CancelledJobsStorage string
	RetryCount           int
	Workers              int
	LocalDownload        bool
	LocalDownloadMaxMb   int
	GDALConfig           *cmd.GDALConfig
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
