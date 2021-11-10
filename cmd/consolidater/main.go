package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/airbusgeo/godal"
	"github.com/airbusgeo/osio"

	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/messaging/pubsub"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"go.uber.org/zap"
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

	gcs, err := osio.GCSHandle(ctx)
	if err != nil {
		return fmt.Errorf("gcshandler: %w", err)
	}
	gcsa, err := osio.NewAdapter(gcs,
		osio.BlockSize("1Mb"),
		osio.NumCachedBlocks(500))
	if err != nil {
		return fmt.Errorf("adapter: %w", err)
	}
	if err := godal.RegisterVSIAdapter("gs://", gcsa); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Create Messaging Service
	var taskConsumer messaging.Consumer
	var eventPublisher messaging.Publisher
	{
		// Connection to pubsub
		if consolidaterConfig.PsSubscriptionName != "" {
			consumer, err := pubsub.NewConsumer(consolidaterConfig.Project, consolidaterConfig.PsSubscriptionName)
			if err != nil {
				return fmt.Errorf("pubsub.new: %w", err)
			}
			consumer.SetProcessOption(pubsub.MaxTries(1))
			taskConsumer = consumer
		}

		if consolidaterConfig.PsEventsTopic != "" {
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

	handlerConsolidation := image.NewHandleConsolidation(image.NewCogGenerator(), image.NewMucogGenerator())
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

			log.Logger(ctx).Sugar().Infof("got message id %s in workdir %s : start consolidation of %d records into the container: %s", msg.ID, consolidaterConfig.WorkDir, len(evt.Records), evt.Container.URI)

			var taskStatus geocube.TaskStatus
			var taskErr error

			if taskErr = handlerConsolidation.Consolidate(ctx, evt, consolidaterConfig.WorkDir); taskErr != nil {
				log.Logger(ctx).Sugar().Errorf("failed to consolidate: %s", taskErr.Error())
				taskStatus = geocube.TaskFailed
			} else {
				taskStatus = geocube.TaskSuccessful
			}

			// Publish the result
			taskEvt := geocube.NewTaskEvent(evt.JobID, evt.TaskID, taskStatus, taskErr)
			data, err := geocube.MarshalEvent(*taskEvt)
			if err != nil {
				return utils.MakeTemporary(fmt.Errorf("MarshalTaskEvent: %w", err))
			}

			if err = eventPublisher.Publish(ctx, data); err != nil {
				return utils.MakeTemporary(fmt.Errorf("PublishTaskEvent: %w", err))
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
	workir := flag.String("workdir", "", "scratch work directory")

	flag.Parse()

	if *workir == "" {
		return nil, fmt.Errorf("missing workdir config flag")
	}
	return &consolidaterConfig{
		PsSubscriptionName: *psSubscriptionName,
		WorkDir:            *workir,
		Project:            *project,
		PsEventsTopic:      *psEventTopic,
	}, nil
}

type consolidaterConfig struct {
	Project               string
	PsEventsTopic         string
	PsConsolidationsTopic string
	WorkDir               string
	PsSubscriptionName    string
}
