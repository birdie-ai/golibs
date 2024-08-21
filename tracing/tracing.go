// Package tracing provides functions to help integrate logging with tracing.
package tracing

import (
	"context"
	"net/http"
	"time"

	"github.com/birdie-ai/golibs/slog"
	"github.com/google/uuid"
)

// RequestStats contains stats for a completed request.
type RequestStats struct {
	Method       string `json:"method,omitempty"`
	URL          string `json:"url,omitempty"`
	RequestSize  int64  `json:"request_size,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	Protocol     string `json:"protocol,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	ResponseSize int    `json:"response_size,omitempty"`
	Elapsed      string `json:"elapsed,omitempty"`
}

// StatsHandler handles completed requests stats (like logging).
type StatsHandler func(context.Context, RequestStats)

// InstrumentHTTP will instrument the given [http.handler] by adding a slog.Logger on the request context.
// The logger will have `trace_id`, `request_id` and `organization_id` added to it.
// Use slog.FromCtx(ctx) to retrieve the logger.
// It will log each completed request on the INFO level (may be too much for some services, for more fine grained control see [InstrumentHTTPWithStats]).
func InstrumentHTTP(h http.Handler) http.Handler {
	return InstrumentHTTPWithStats(h, func(ctx context.Context, req RequestStats) {
		slog.FromCtx(ctx).Info("handled request", "http_request", req)
	})
}

// InstrumentHTTPWithStats will instrument the given [http.handler] by adding a slog.Logger on the request context.
// The logger will have `trace_id`, `request_id` and `organization_id` added to it.
// Use slog.FromCtx(ctx) to retrieve the logger.
// For each completed request the provided [StatsHandler] will be called.
func InstrumentHTTPWithStats(h http.Handler, statsHandler StatsHandler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// We don't parse/generate trace IDs exactly as in the spec, for now
		// just using the specified header name.
		traceID := req.Header.Get(traceIDHeader)
		if traceID == "" {
			traceID = uuid.NewString()
		}
		orgID := req.Header.Get(orgIDHeader)

		ctx := req.Context()
		ctx = CtxWithTraceID(ctx, traceID)
		if orgID != "" {
			ctx = CtxWithOrgID(ctx, orgID)
		}

		log := slog.FromCtx(ctx)
		log = log.With("trace_id", traceID)
		log = log.With("request_id", uuid.NewString())
		if orgID != "" {
			log = log.With("organization_id", orgID)
		}
		ctx = slog.NewContext(ctx, log)

		httpReq := RequestStats{
			Method:      req.Method,
			URL:         req.URL.String(),
			RequestSize: req.ContentLength,
			UserAgent:   req.UserAgent(),
			Protocol:    req.Proto,
		}

		resWriter := newResponseWriter(res)
		start := time.Now()
		defer func() {
			elapsed := time.Since(start)
			status := resWriter.Status()
			if status == 0 {
				// Handler did not write a status code. This means 200 OK.
				status = http.StatusOK
			}
			httpReq.StatusCode = status
			httpReq.ResponseSize = resWriter.ContentLength()
			httpReq.Elapsed = elapsed.String()
			statsHandler(ctx, httpReq)
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

// SetRequestHeaders adds headers to the given [Request] using information
// extracted from the given [context.Context].
//
// It is intended for outgoing client request creation, making it easier to propagate trace IDs
// (and other request scoped information).
func SetRequestHeaders(ctx context.Context, req *http.Request) {
	if traceID := CtxGetTraceID(ctx); traceID != "" {
		req.Header.Set(traceIDHeader, traceID)
	}
	if orgID := CtxGetOrgID(ctx); orgID != "" {
		req.Header.Set(orgIDHeader, orgID)
	}
}

type (
	responseWriterObserver interface {
		http.ResponseWriter
		Status() int
		ContentLength() int
	}
	responseWriter struct {
		http.ResponseWriter
		status        int
		contentLength int
	}
	responseWriterFlusher struct {
		*responseWriter
		http.Flusher
	}

	// key is the type used to store data on contexts.
	key int
)

const (
	traceIDHeader     = "traceparent"
	orgIDHeader       = "Birdie-Organization-ID"
	traceIDKey    key = iota
	orgIDKey
)

func newResponseWriter(r http.ResponseWriter) responseWriterObserver {
	rw := &responseWriter{ResponseWriter: r}
	flusher, ok := r.(http.Flusher)
	if ok {
		return &responseWriterFlusher{
			responseWriter: rw,
			Flusher:        flusher,
		}
	}
	return rw
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseWriter) Status() int {
	return r.status
}

func (r *responseWriter) ContentLength() int {
	return r.contentLength
}

func (r *responseWriter) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.contentLength += n
	return n, err
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
