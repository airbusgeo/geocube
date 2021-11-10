package mocks

import (
	"context"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/stretchr/testify/mock"
)

type Generator struct {
	mock.Mock
}

func (_m *Generator) Consolidate(ctx context.Context, cEvent *geocube.ConsolidationEvent, workspace string) error {
	ret := _m.Called(ctx, cEvent, workspace)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *geocube.ConsolidationEvent, string) error); ok {
		r0 = rf(ctx, cEvent, workspace)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
