package tracing_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/birdie-ai/golibs/slog"
	"github.com/birdie-ai/golibs/tracing"
)

func TestSetRequestHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		wantTraceID = "traceid"
		wantOrgID   = "orgid"
	)

	ctx = tracing.CtxWithOrgID(ctx, wantOrgID)
	ctx = tracing.CtxWithTraceID(ctx, wantTraceID)

	tracing.SetRequestHeaders(ctx, req)
	gotTraceID := req.Header.Get("traceparent")
	gotOrgID := req.Header.Get("Birdie-Organization-ID")

	if gotTraceID != wantTraceID {
		t.Fatalf("got traceID %q; want %q", gotTraceID, wantTraceID)
	}
	if gotOrgID != wantOrgID {
		t.Fatalf("got orgID %q; want %q", gotOrgID, wantOrgID)
	}
}

func TestSetRequestHeadersEmptyCtx(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}

	tracing.SetRequestHeaders(context.Background(), req)

	if len(req.Header) != 0 {
		t.Fatalf("unexpected headers: %v", req.Header)
	}
}

func TestIntrumentedHTTPHandler(t *testing.T) {
	const (
		wantTraceID = "test-trace-id"
		wantOrgID   = "test-org-id"
		wantStatus  = 201 // should be a non-default status, to actually test things.
		wantBody    = "Worked!"
	)
	var (
		gotLogger         *slog.Logger
		gotTraceID        string
		gotOrgID          string
		gotResponseWriter http.ResponseWriter
	)
	handler := tracing.InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotLogger = slog.FromCtx(req.Context())
		gotTraceID = tracing.CtxGetTraceID(req.Context())
		gotOrgID = tracing.CtxGetOrgID(req.Context())
		w.WriteHeader(wantStatus)
		_, _ = fmt.Fprint(w, wantBody)
		gotResponseWriter = w
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", wantTraceID)
	req.Header.Set("Birdie-Organization-ID", wantOrgID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if gotLogger == nil {
		t.Fatal("got nil logger")
	}
	if gotTraceID != wantTraceID {
		t.Fatalf("got %q != want %q", gotTraceID, wantTraceID)
	}
	if gotOrgID != wantOrgID {
		t.Fatalf("got %q != want %q", gotOrgID, wantOrgID)
	}
	res := w.Result()
	if got := res.StatusCode; got != wantStatus {
		t.Fatalf("got status %v; want %v", got, wantStatus)
	}
	if got := w.Body.String(); got != wantBody {
		t.Fatalf("got body %v; want %v", got, wantBody)
	}
	// HTTP1/2 http.ResponseWriter always implement http.Flush
	// Lets guarantee that our wrapping doesn't break this
	if _, ok := gotResponseWriter.(http.Flusher); !ok {
		t.Fatal("wrapped response writter doesn't implement http.Flusher")
	}
}

func TestIntrumentedHTTPHandlerNoFlusher(t *testing.T) {
	const (
		wantTraceID = "test-trace-id"
		wantOrgID   = "test-org-id"
		wantStatus  = 201 // should be a non-default status, to actually test things.
		wantBody    = "Worked!"
	)
	var (
		gotLogger         *slog.Logger
		gotTraceID        string
		gotOrgID          string
		gotResponseWriter http.ResponseWriter
	)
	handler := tracing.InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotLogger = slog.FromCtx(req.Context())
		gotTraceID = tracing.CtxGetTraceID(req.Context())
		gotOrgID = tracing.CtxGetOrgID(req.Context())
		w.WriteHeader(wantStatus)
		_, _ = fmt.Fprint(w, wantBody)
		gotResponseWriter = w
	}))
	// Lets force the http.ResponseWriter to be non-flusheable
	type nonFlusheable struct {
		http.ResponseWriter
	}
	nonFlushHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w = nonFlusheable{w}
		handler.ServeHTTP(w, req)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", wantTraceID)
	req.Header.Set("Birdie-Organization-ID", wantOrgID)
	w := httptest.NewRecorder()

	nonFlushHandler.ServeHTTP(w, req)

	if gotLogger == nil {
		t.Fatal("got nil logger")
	}
	if gotTraceID != wantTraceID {
		t.Fatalf("got %q != want %q", gotTraceID, wantTraceID)
	}
	if gotOrgID != wantOrgID {
		t.Fatalf("got %q != want %q", gotOrgID, wantOrgID)
	}
	res := w.Result()
	if got := res.StatusCode; got != wantStatus {
		t.Fatalf("got status %v; want %v", got, wantStatus)
	}
	if got := w.Body.String(); got != wantBody {
		t.Fatalf("got body %v; want %v", got, wantBody)
	}
	if _, ok := gotResponseWriter.(http.Flusher); ok {
		t.Fatal("wrapped response writter implement http.Flusher but shouldn't")
	}
}

func TestCtxWithTraceOrgRequestID(t *testing.T) {
	const (
		wantTraceID   = "trace-id-value"
		wantRequestID = "request-id"
		wantOrgID     = "org-id-value"
	)
	ctx := context.Background()

	got := tracing.CtxGetRequestID(ctx)
	if got != "" {
		t.Fatalf("unexpected request id: %q", got)
	}
	got = tracing.CtxGetTraceID(ctx)
	if got != "" {
		t.Fatalf("unexpected trace id: %q", got)
	}
	got = tracing.CtxGetOrgID(ctx)
	if got != "" {
		t.Fatalf("unexpected trace id: %q", got)
	}

	ctx = tracing.CtxWithRequestID(ctx, wantRequestID)
	ctx = tracing.CtxWithTraceID(ctx, wantTraceID)
	ctx = tracing.CtxWithOrgID(ctx, wantOrgID)

	got = tracing.CtxGetRequestID(ctx)
	if got != wantRequestID {
		t.Fatalf("got %q != want %q", got, wantRequestID)
	}

	got = tracing.CtxGetTraceID(ctx)
	if got != wantTraceID {
		t.Fatalf("got %q != want %q", got, wantTraceID)
	}

	got = tracing.CtxGetOrgID(ctx)
	if got != wantOrgID {
		t.Fatalf("got %q != want %q", got, wantTraceID)
	}
}
