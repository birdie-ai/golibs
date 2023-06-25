// Package tracing provides functions to help integrate logging with tracing.
package tracing

import (
	"context"
	"net/http"

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
		log := slog.FromCtx(ctx)
		log = log.With("trace_id", traceid)
		ctx = slog.NewContext(ctx, log)

		h.ServeHTTP(res, req.WithContext(ctx))
	})
}

// CtxWithTraceID creates a new [context.Context] with the given trace ID associated with it.
// Call [CtxGetTraceID] to retrieve the trace ID.
func CtxWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// CtxGetTraceID gets the trace ID associated with this context.
// Return the trace ID and true if there is a trace ID, empty and false otherwise.
func CtxGetTraceID(ctx context.Context) (string, bool) {
	val := ctx.Value(traceIDKey)
	if val == nil {
		return "", false
	}
	traceID, ok := val.(string)
	if !ok {
		return "", false
	}
	return traceID, true
}

// key is the type used to store data on contexts.
type key int

const (
	traceIDKey key = iota
	orgIDKey
)
