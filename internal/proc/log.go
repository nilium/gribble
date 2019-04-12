package proc

import (
	"context"

	"go.uber.org/zap"
)

func Named(ctx context.Context, name string) context.Context {
	return WithLogger(ctx, Logger(ctx).Named(name))
}

func With(ctx context.Context, fields ...zap.Field) context.Context {
	return WithLogger(ctx, Logger(ctx).With(fields...))
}

func WithOptions(ctx context.Context, options ...zap.Option) context.Context {
	return WithLogger(ctx, Logger(ctx).WithOptions(options...))
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if ck := skipLogger(ctx).Check(zap.DebugLevel, msg); ck != nil {
		ck.Write(fields...)
	}
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	if ck := skipLogger(ctx).Check(zap.InfoLevel, msg); ck != nil {
		ck.Write(fields...)
	}
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if ck := skipLogger(ctx).Check(zap.WarnLevel, msg); ck != nil {
		ck.Write(fields...)
	}
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	if ck := skipLogger(ctx).Check(zap.ErrorLevel, msg); ck != nil {
		ck.Write(fields...)
	}
}

func Panic(ctx context.Context, msg string, fields ...zap.Field) {
	if ck := skipLogger(ctx).Check(zap.PanicLevel, msg); ck != nil {
		ck.Write(fields...)
	}
}

func DPanic(ctx context.Context, msg string, fields ...zap.Field) {
	if ck := skipLogger(ctx).Check(zap.DPanicLevel, msg); ck != nil {
		ck.Write(fields...)
	}
}
