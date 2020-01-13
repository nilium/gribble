package main

import (
	"net/http"
	"time"

	"go.spiff.io/gribble/internal/proc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type AccessLogger struct {
	next   http.Handler
	level  zapcore.Level
	logger *zap.Logger
}

func AccessLog(next http.Handler, logger *zap.Logger, level zapcore.Level) *AccessLogger {
	if logger == nil {
		logger = zap.L()
	}
	return &AccessLogger{
		next:   next,
		level:  level,
		logger: logger.Named("access"),
	}
}

type responseRecorder struct {
	code  int
	bytes int64
	http.ResponseWriter
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

func (a *AccessLogger) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ck := a.logger.Check(a.level, "Request received")
	if ck == nil {
		a.next.ServeHTTP(w, req)
		return
	}

	ctx := req.Context()
	rec := &responseRecorder{ResponseWriter: w}
	defer func(t time.Time) {
		ck.Write(
			zap.Int("http_status", rec.code),
			zap.Time("http_start", t),
			zap.String("http_method", req.Method),
			zap.String("http_server", req.Host),
			zap.String("http_request", req.RequestURI),
			zap.String("http_remote_addr", req.RemoteAddr),
			zap.Int64("http_bytes_written", rec.bytes),
			zap.Duration("http_elapsed", proc.Now(ctx).Sub(t)),
		)
	}(proc.Now(ctx))
	a.next.ServeHTTP(rec, req)
}
