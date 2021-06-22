package qbas

import (
	"context"
	"fmt"
	"math"
)

type Queue interface {
	Backlog(ctx context.Context) (int64, error)
}

type InstanceManager interface {
	Size(ctx context.Context) (int64, error)
	Resize(ctx context.Context, newSize int64) error
	ScaleDown(ctx context.Context, newSize int64) error
}

type QueueBasedInstanceManager interface {
	Queue
	InstanceManager
}

type Config struct {
	Ratio        float64 `json:"ratio"`
	MinRatio     float64 `json:"minRatio"`
	MaxStep      int64   `json:"maxStep"`
	MaxInstances int64   `json:"maxInstances"`
	MinInstances int64   `json:"minInstances"`
}

type Operation struct {
	Backlog   int64 `json:"backlog"`
	Instances int64 `json:"instances"`
	Delta     int64 `json:"delta"`
}

func Autoscale(ctx context.Context, cfg Config, qbim QueueBasedInstanceManager) (Operation, error) {
	op := Operation{}
	if cfg.Ratio < 1.0 {
		return op, fmt.Errorf("cfg.Ratio must be >= 1.0")
	}
	if cfg.MinRatio < 0.0 {
		return op, fmt.Errorf("cfg.MinRatio must be >= 0.0")
	}
	if cfg.MinRatio > cfg.Ratio {
		return op, fmt.Errorf("cfg.MinRatio must be <= cfg.Ratio")
	}
	if cfg.MaxStep < 1 {
		return op, fmt.Errorf("cfg.MaxStep must be >= 1")
	}
	if cfg.MaxInstances < 1 {
		return op, fmt.Errorf("cfg.MaxInstances must be >= 1")
	}
	if cfg.MinInstances < 0 {
		return op, fmt.Errorf("cfg.MaxInstances must be >= 0")
	}
	if cfg.MaxInstances < cfg.MinInstances {
		return op, fmt.Errorf("maxInstances must be >= minInstances")
	}

	subch := make(chan error, 1)
	go func() {
		var err error
		op.Backlog, err = qbim.Backlog(ctx)
		subch <- err
	}()
	instch := make(chan error, 1)
	go func() {
		var err error
		op.Instances, err = qbim.Size(ctx)
		instch <- err
	}()

	suberr := <-subch
	insterr := <-instch
	if suberr != nil {
		return op, fmt.Errorf("get subscription count: %w", suberr)
	}
	if insterr != nil {
		return op, fmt.Errorf("get instance count: %w", insterr)

	}

	neededsize := op.Instances
	if op.Backlog == 0 {
		//if we have no jobs to process, then we need no workers
		neededsize = 0
	} else {
		//we have at least one job
		if op.Instances > 0 {
			//we have at least one worker
			ratio := float64(op.Backlog) / float64(op.Instances)
			if ratio > cfg.Ratio {
				//need to add instance(s)
				neededsize = int64(math.Ceil(float64(op.Backlog) / cfg.Ratio))
			} else if op.Instances > 1 && cfg.MinRatio > 0 && ratio < cfg.MinRatio {
				//we can delete instance(s) even though they are occupied
				neededsize = int64(math.Ceil(float64(op.Backlog) / cfg.MinRatio))
			} else if ratio < 1.0 {
				//we have more instances than jobs to process
				neededsize = op.Backlog
			}
		} else {
			//we have no worker, set requested size to be the minimal configured ratio
			neededsize = int64(math.Ceil(float64(op.Backlog) / cfg.Ratio))
		}
	}
	//make sure we don't extend past the configured maximum
	if neededsize > cfg.MaxInstances {
		if op.Instances > cfg.MaxInstances {
			//if we already have more instances than the max (via an external manual setting, or previous config), don't delete working instances
			if op.Instances < op.Backlog {
				neededsize = op.Instances
			} else {
				neededsize = op.Backlog
			}
		} else {
			neededsize = cfg.MaxInstances
		}
	}
	if neededsize < cfg.MinInstances {
		neededsize = cfg.MinInstances
	} else {
		if neededsize-op.Instances > cfg.MaxStep {
			//don't add more than MaxStep instances
			neededsize = op.Instances + cfg.MaxStep
		} else if op.Instances-neededsize > cfg.MaxStep && op.Backlog > 0 {
			//don't remove more than MaxStep instances, unless queue is fully empty
			neededsize = op.Instances - cfg.MaxStep
		}
	}
	var err error
	op.Delta = neededsize - op.Instances
	if op.Delta != 0 {
		if op.Delta > 0 || op.Backlog == 0 {
			if err = qbim.Resize(ctx, neededsize); err != nil {
				err = fmt.Errorf("resize instances: %w", err)
			}
		} else {
			if err = qbim.ScaleDown(ctx, neededsize); err != nil {
				err = fmt.Errorf("scale down instances: %w", err)
			}
		}
	}
	return op, err
}
