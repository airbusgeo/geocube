package mocks

import (
	"context"
	"net/http"

	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/stretchr/testify/mock"
)

type Publisher struct {
	mock.Mock
}

func (_m *Publisher) Publish(ctx context.Context, data ...[]byte) error {
	ret := _m.Called(ctx, data)
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, [][]byte) error); ok {
		r0 = rf(ctx, data)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

type Consumer struct {
	mock.Mock
}

func (_m *Consumer) Pull(ctx context.Context, cb messaging.Callback) error {
	ret := _m.Called(ctx, cb)
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, messaging.Callback) error); ok {
		r0 = rf(ctx, cb)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *Consumer) Consume(req http.Request, cb messaging.Callback) (int, error) {
	ret := _m.Called(req, cb)

	var r0 int
	if rf, ok := ret.Get(0).(func(http.Request, messaging.Callback) int); ok {
		r0 = rf(req, cb)
	} else {
		r0 = ret.Int(0)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(http.Request, messaging.Callback) error); ok {
		r1 = rf(req, cb)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
