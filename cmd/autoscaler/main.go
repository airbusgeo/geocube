package main

import (
	"context"
	"flag"
	"time"

	"github.com/airbusgeo/geocube/interface/autoscaler"
	rc "github.com/airbusgeo/geocube/interface/autoscaler/k8s"
	"github.com/airbusgeo/geocube/interface/autoscaler/qbas"
	"github.com/airbusgeo/geocube/interface/messaging/pubsub"
	"github.com/airbusgeo/geocube/internal/log"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	var (
		argupd      = flag.Duration("update", 30*time.Second, "time between updates")
		argproject  = flag.String("psProject", "", "pubsub subscription project")
		argsub      = flag.String("psSubscription", "", "pubsub subscription to configure the backlog for autoscaling (needs --psProject)")
		argrc       = flag.String("rc", "", "K8S replication controller")
		argns       = flag.String("ns", "default", "replication controller namespace")
		argratio    = flag.Float64("ratio", 10.0, "job/worker ratio over which instances will be added")
		argminratio = flag.Float64("minratio", 0.0, "job/worker under which instances will be deleted")
		argstep     = flag.Uint("step", 3, "max worker increment/decrement")
		argmax      = flag.Uint("max", 15, "max number of workers")
		argmin      = flag.Uint("min", 0, "min number of workers")
		podCostPath = flag.String("pod.cost.path", "", "pod termination cost url")
		podCostPort = flag.Uint("pod.cost.port", 0, "pod termination cost url")
	)

	flag.Parse()
	if *argrc == "" {
		panic("missing replication controller")
	}
	if *argstep == 0 {
		panic("step must be >0")
	}
	ctx = log.WithFields(ctx, zap.String("rc", *argrc), zap.String("queue", *argsub))

	controller, err := rc.New(*argrc, *argns)
	if err != nil {
		panic(err.Error())
	}
	controller.AllowEviction = false
	controller.CostPath = *podCostPath
	controller.CostPort = int(*podCostPort)

	var queue qbas.Queue

	if *argsub != "" {
		if queue, err = pubsub.NewConsumer(*argproject, *argsub); err != nil {
			panic(err)
		}
	}
	if queue == nil {
		panic("missing backlog configuration (e.g. psSubscription)")
	}
	cfg := qbas.Config{
		Ratio:        *argratio,
		MinRatio:     *argminratio,
		MaxInstances: int64(*argmax),
		MinInstances: int64(*argmin),
		MaxStep:      int64(*argstep),
	}
	as := autoscaler.New(queue, controller, cfg, log.Logger(ctx))
	log.Logger(ctx).Sugar().Infof("starting autoscaler with refresh %s", argupd.String())
	as.Run(ctx, *argupd)
}
