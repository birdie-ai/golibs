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

		httpReq := map[string]any{
			"method":       req.Method,
			"url":          req.URL.String(),
			"request_size": req.ContentLength,
			"user_agent":   req.UserAgent(),
			"protocol":     req.Proto,
		}

		log.Debug("handling request", "http_request", httpReq)
		resWriter := newResponseWriter(res)
		start := time.Now()
		defer func() {
			elapsed := time.Since(start)
			httpReq["status_code"] = resWriter.Status()
			httpReq["response_size"] = resWriter.ContentLength()
			httpReq["elapsed"] = elapsed.String()
			log.Info("handled request", "http_request", httpReq)
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
