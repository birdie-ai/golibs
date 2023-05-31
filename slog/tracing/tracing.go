// Package tracing provides functions to help integrate logging with tracing.
package tracing

import (
	"context"
	"net/http"

	"github.com/birdie-ai/golibs/slog"
	"github.com/google/uuid"
)

// Instrument will instrument the given handler by adding tracing context on the
// request context.
func Instrument(h http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// We don't parse/generate trace IDs exactly as in the spec, for now
		// just using the specified header name.
		traceid := req.Header.Get("traceparent")

		slog.Debug("traceparent header", "trace_id", traceid)

		if traceid == "" {
			traceid = uuid.NewString()
			slog.Debug("header absent, generated UUID", "trace_id", traceid)
		}

		req = req.WithContext(context.WithValue(req.Context(), slog.TraceID, traceid))
		h.ServeHTTP(res, req)
	})
}
