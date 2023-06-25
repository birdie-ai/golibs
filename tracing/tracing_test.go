package tracing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/birdie-ai/golibs/slog"
	"github.com/birdie-ai/golibs/tracing"
)

func TestIntrumentedHTTPHandler(t *testing.T) {
	const wantTraceID = "test-trace-id"
	var (
		gotLogger  *slog.Logger
		gotTraceID string
	)
	handler := tracing.InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotLogger = slog.FromCtx(req.Context())
		gotTraceID, _ = tracing.CtxGetTraceID(req.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", wantTraceID)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if gotLogger == nil {
		t.Fatal("got nil logger")
	}

	if gotTraceID != wantTraceID {
		t.Fatalf("got %q != want %q", gotTraceID, wantTraceID)
	}
}

func TestCtxWithTraceID(t *testing.T) {
	const want = "trace-id-value"
	ctx := context.Background()

	got, ok := tracing.CtxGetTraceID(ctx)
	if ok {
		t.Fatalf("unexpected trace id: %q", got)
	}

	ctx = tracing.CtxWithTraceID(ctx, want)

	got, ok = tracing.CtxGetTraceID(ctx)
	if !ok {
		t.Fatal("want trace ID")
	}
	if got != want {
		t.Fatalf("got %q != want %q", got, want)
	}
}
