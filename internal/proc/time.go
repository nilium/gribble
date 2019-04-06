package proc

import (
	"context"
	"time"
)

type contextKey uint

const (
	ctxTime contextKey = 1 + iota
)

func Now(ctx context.Context) time.Time {
	if t, ok := ctx.Value(ctxTime).(time.Time); ok && !t.IsZero() {
		return t
	}
	return time.Now()
}

func WithTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, ctxTime, t)
}
