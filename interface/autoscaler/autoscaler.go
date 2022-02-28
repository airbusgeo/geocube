package autoscaler

import (
	"context"
	"time"

	rc "github.com/airbusgeo/geocube/interface/autoscaler/k8s"
	"github.com/airbusgeo/geocube/interface/autoscaler/qbas"

	"go.uber.org/zap"
)

func (as Autoscaler) Backlog(ctx context.Context) (int64, error) {
	return as.queue.Backlog(ctx)
}
func (as Autoscaler) Size(ctx context.Context) (int64, error) {
	s, err := as.rc.Size(ctx)
	return s, err
}
func (as Autoscaler) Resize(ctx context.Context, newSize int64) error {
	return as.rc.Resize(ctx, newSize)
}
func (as Autoscaler) ScaleDown(ctx context.Context, newSize int64) error {
	return as.rc.ScaleDown(ctx, newSize)
}

type Autoscaler struct {
	logger *zap.Logger
	rc     *rc.ReplicationController
	queue  qbas.Queue
	cfg    qbas.Config
}

func (as Autoscaler) Run(ctx context.Context, refresh time.Duration) {
	as.autoscale(ctx)
	ticker := time.NewTicker(refresh)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			as.autoscale(ctx)
		}
	}
}

func (as Autoscaler) autoscale(ctx context.Context) {
	res, err := qbas.Autoscale(ctx, as.cfg, as)
	if as.logger != nil {
		if err != nil {
			as.logger.Error("failed autoscale", zap.Error(err), zap.Any("qbas", res))
		} else if res.Delta != 0 {
			as.logger.Info("autoscaled", zap.Any("qbas", res))
		}
	}
}

func New(queue qbas.Queue, rc *rc.ReplicationController, cfg qbas.Config, logger *zap.Logger) Autoscaler {
	return Autoscaler{
		logger: logger,
		rc:     rc,
		queue:  queue,
		cfg:    cfg,
	}
}
