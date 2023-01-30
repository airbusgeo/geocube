package log

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _logger *zap.Logger
var defaultlogger *zap.Logger

type contextKey int

const (
	contextKeyFields contextKey = iota
)

func onK8S() bool {
	_, err := os.Stat("/var/run/secrets/kubernetes.io")
	return !os.IsNotExist(err)
}

func init() {
	Structured()
}

func setLogger(l *zap.Logger) {
	defaultlogger = l
}
func resetLogger() {
	defaultlogger = _logger
}

// Structured sets output to be JSON encoded
func Structured() {
	// a root logger
	cfg := zap.NewProductionConfig()
	enc := zap.NewProductionEncoderConfig()
	enc.LevelKey = "severity"
	enc.TimeKey = "timestamp"
	if onK8S() {
		//log collection in k8s handles timestamps
		enc.TimeKey = ""
	} else {
		enc.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	enc.StacktraceKey = ""
	enc.MessageKey = "message"
	cfg.EncoderConfig = enc
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	if lvl := os.Getenv("LOGLEVEL"); lvl != "" {
		err := cfg.Level.UnmarshalText([]byte(lvl))
		if err != nil {
			cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		}
	} else {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	var err error
	_logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
	defaultlogger = _logger
}

func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02T15:04:05.000"))
}

// Console sets output to be human-readable
func Console() {
	// a root logger
	cfg := zap.NewDevelopmentConfig()
	enc := zap.NewDevelopmentEncoderConfig()
	enc.LevelKey = "severity"
	enc.TimeKey = "timestamp"
	enc.EncodeTime = timeEncoder
	enc.StacktraceKey = ""
	enc.MessageKey = "message"
	enc.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig = enc
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	if lvl := os.Getenv("LOGLEVEL"); lvl != "" {
		err := cfg.Level.UnmarshalText([]byte(lvl))
		if err != nil {
			cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		}
	} else {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	var err error
	_logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
	defaultlogger = _logger
}

// Logger returns a logger that will print fields previously added to the context
func Logger(ctx context.Context) *zap.Logger {
	flds := ctx.Value(contextKeyFields)
	if flds != nil {
		fflds := flds.([]zap.Field)
		return defaultlogger.With(fflds...)
	}
	return defaultlogger
}

// With adds a key=value field to the returned context
func With(ctx context.Context, key string, value interface{}) context.Context {
	fld := zap.Any(key, value)
	return WithFields(ctx, fld)
}

// CopyContext returns a context derived from dst that contains the eventual logging
// keys that are contained in ctx
func CopyContext(ctx context.Context, dst context.Context) context.Context {
	if cflds := ctx.Value(contextKeyFields); cflds != nil {
		flds := cflds.([]zapcore.Field)
		if cdflds := dst.Value(contextKeyFields); cdflds != nil {
			flds = append(flds, cdflds.([]zapcore.Field)...)
		}
		return context.WithValue(dst, contextKeyFields, flds)
	} else {
		return dst
	}
}

// WithFields adds fields to the returned context
func WithFields(ctx context.Context, fields ...zapcore.Field) context.Context {
	flds := ctx.Value(contextKeyFields)
	var fflds []zap.Field
	if flds != nil {
		fflds = flds.([]zap.Field)
	}
	fflds = append(fflds, fields...)
	return context.WithValue(ctx, contextKeyFields, fflds)
}

// Print logs at Info level
func Print(v ...interface{}) {
	defaultlogger.Info(fmt.Sprint(v...))
}

// Printf logs at Info level
func Printf(format string, v ...interface{}) {
	defaultlogger.Sugar().Infof(format, v...)
}

func Println(v ...interface{}) {
	defaultlogger.Info(fmt.Sprintln(v...))
}

func Fatal(v ...interface{}) {
	defaultlogger.Fatal(fmt.Sprint(v...))
}
func Fatalf(format string, v ...interface{}) {
	defaultlogger.Sugar().Fatalf(format, v...)
}

func Panic(v ...interface{}) {
	defaultlogger.Panic(fmt.Sprint(v...))
}

func Panicf(msg string, v ...interface{}) {
	defaultlogger.Sugar().Panicf(msg, v...)
}
