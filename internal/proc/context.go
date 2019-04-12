package proc

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type contextKey uint

const (
	ctxTime contextKey = 1 + iota
	ctxLogger
)

// Time (used in testing)

func Now(ctx context.Context) time.Time {
	if t, ok := ctx.Value(ctxTime).(time.Time); ok && !t.IsZero() {
		return t
	}
	return time.Now()
}

func WithTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, ctxTime, t)
}

// Context logger

var nopLogger = zap.NewNop()

type contextLogger struct {
	base *zap.Logger
	skip *zap.Logger
}

var skipCaller = []zap.Option{zap.AddCallerSkip(2)}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxLogger, contextLogger{
		base: logger,
		skip: logger.WithOptions(skipCaller...),
	})
}

func Logger(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxLogger).(contextLogger); ok {
		return l.base
	}
	return zap.L()
}

func skipLogger(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxLogger).(contextLogger); ok {
		return l.skip
	}
	return zap.L().WithOptions(skipCaller...)
}
