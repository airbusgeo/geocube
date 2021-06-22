package log

import (
	"context"
	"reflect"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func installLogger() *observer.ObservedLogs {
	c, o := observer.New(zapcore.DebugLevel)
	l := zap.New(c)
	setLogger(l)
	return o
}

func testEntry(t *testing.T, e observer.LoggedEntry, flds []zapcore.Field) {
	t.Helper()
	for _, fld := range flds {
		ok := false
		for _, f := range e.Context {
			if reflect.DeepEqual(f, fld) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("missing field %v", fld)
		}
	}
	for _, fld := range e.Context {
		ok := false
		for _, f := range flds {
			if reflect.DeepEqual(f, fld) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("extraneaous field %v", fld)
		}
	}
}
func TestLogger(t *testing.T) {
	o := installLogger()
	defer resetLogger()
	ctx := context.Background()
	lg := Logger(ctx)
	lg.Info("")
	lg.Sync()
	e := o.TakeAll()[0]
	if len(e.Context) > 0 {
		t.Errorf("got %d fields, expected 0", len(e.Context))
	}

	//simple field test
	ctx1 := With(ctx, "foo", "bar")
	lg = Logger(ctx1)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("foo", "bar")})

	//check new field was added
	ctx1 = With(ctx1, "bar", "baz")
	lg = Logger(ctx1)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("foo", "bar"), zap.Any("bar", "baz")})

	//check ctx1 fields did not bleed into ctx2
	ctx2 := With(ctx, "foo", "baz")
	lg = Logger(ctx2)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("foo", "baz")})

	//check child fields did not bleed into root
	lg = Logger(ctx)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	if len(e.Context) > 0 {
		t.Errorf("got %d fields, expected 0", len(e.Context))
	}
}

func TestCopy(t *testing.T) {
	o := installLogger()
	defer resetLogger()
	ctx := context.Background()
	ctx2 := context.Background()
	ctx = With(ctx, "ctx", "root")
	ctx2 = CopyContext(ctx, ctx2)

	lg := Logger(ctx2)
	lg.Info("")
	lg.Sync()
	e := o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("ctx", "root")})

	ctx = With(ctx, "ctx", "root2")

	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("ctx", "root")})

	ctx2 = With(ctx2, "ctx2", "root2")
	lg = Logger(ctx)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("ctx", "root"), zap.Any("ctx", "root2")})

	ctx = context.Background()
	ctx = With(ctx, "c1", "c1")
	ctx2 = context.Background()
	ctx2 = With(ctx2, "c2", "c2")
	ctx2 = CopyContext(ctx, ctx2)
	lg = Logger(ctx2)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("c1", "c1"), zap.Any("c2", "c2")})

	ctx = context.Background()
	ctx2 = context.Background()
	ctx2 = With(ctx2, "c2", "c2")
	ctx2 = CopyContext(ctx, ctx2)
	lg = Logger(ctx2)
	lg.Info("")
	lg.Sync()
	e = o.TakeAll()[0]
	testEntry(t, e, []zapcore.Field{zap.Any("c2", "c2")})
}
