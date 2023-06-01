// Package tracing provides functions to help integrate logging with tracing.
package tracing

import (
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
