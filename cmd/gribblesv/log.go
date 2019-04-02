package main

import (
	"log"
	"net/http"
	"time"
)

type AccessLogger struct {
	next http.Handler
}

func AccessLog(next http.Handler) *AccessLogger {
	return &AccessLogger{next}
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
	rec := &responseRecorder{ResponseWriter: w}
	defer func(t time.Time) {
		log.Printf("%d %s %q %q %d %v",
			rec.code,
			req.Method,
			req.RequestURI,
			req.RemoteAddr,
			rec.bytes,
			time.Since(t),
		)
	}(time.Now())
	a.next.ServeHTTP(rec, req)
}
