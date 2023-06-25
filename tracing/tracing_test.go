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
	var got *slog.Logger
	handler := tracing.InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got = slog.FromCtx(req.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if got == nil {
		t.Fatal("got nil logger")
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
