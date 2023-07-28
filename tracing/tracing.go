// Package tracing provides functions to help integrate logging with tracing.
package tracing

import (
	"context"
	"net/http"
	"time"

	"github.com/birdie-ai/golibs/slog"
	"github.com/google/uuid"
)

// InstrumentHTTP will instrument the given [http.handler] by adding a slog.Logger on the request context.
// The logger will have `trace_id` added to it.
// Use slog.FromCtx(ctx) to retrieve the logger.
func InstrumentHTTP(h http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// We don't parse/generate trace IDs exactly as in the spec, for now
		// just using the specified header name.
		traceid := req.Header.Get("traceparent")

		slog.Debug("traceparent header", "trace_id", traceid)

		if traceid == "" {
			traceid = uuid.NewString()
			slog.Debug("header absent, generated UUID", "trace_id", traceid)
		}

		ctx := req.Context()
		ctx = CtxWithTraceID(ctx, traceid)

		log := slog.FromCtx(ctx)
		log = log.With("trace_id", traceid)
		log = log.With("request_id", uuid.NewString())
		ctx = slog.NewContext(ctx, log)

		log.Debug("handling request", "url", req.URL, "method", req.Method)
		resWriter := &responseWriter{
			ResponseWriter: res,
		}
		start := time.Now()
		defer func() {
			elapsed := time.Since(start)
			log.Info("handled request", "url", req.URL, "method", req.Method, "status", resWriter.status, "elapsed", elapsed)
		}()

		h.ServeHTTP(resWriter, req.WithContext(ctx))
	})
}

// CtxWithTraceID creates a new [context.Context] with the given trace ID associated with it.
// Call [CtxGetTraceID] to retrieve the trace ID.
func CtxWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// CtxGetTraceID gets the trace ID associated with this context.
// Return the trace ID and true if there is a trace ID, empty and false otherwise.
func CtxGetTraceID(ctx context.Context) string {
	return ctxget(ctx, traceIDKey)
}

// CtxWithOrgID creates a new [context.Context] with the given organization ID associated with it.
// Call [CtxGetOrgID] to retrieve the organization ID.
func CtxWithOrgID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, orgIDKey, orgID)
}

// CtxGetOrgID gets the trace ID associated with this context.
func CtxGetOrgID(ctx context.Context) string {
	return ctxget(ctx, orgIDKey)
}

type (
	responseWriter struct {
		http.ResponseWriter
		status int
	}

	// key is the type used to store data on contexts.
	key int
)

const (
	traceIDKey key = iota
	orgIDKey
)

func (r *responseWriter) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func ctxget(ctx context.Context, k key) string {
	val := ctx.Value(k)
	if val == nil {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}
