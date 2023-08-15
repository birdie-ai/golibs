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
	const (
		wantTraceID = "test-trace-id"
		wantOrgID   = "test-org-id"
		wantStatus  = 201 // should be a non-default status, to actually test things.
	)
	var (
		gotLogger  *slog.Logger
		gotTraceID string
		gotOrgID   string
	)
	handler := tracing.InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotLogger = slog.FromCtx(req.Context())
		gotTraceID = tracing.CtxGetTraceID(req.Context())
		gotOrgID = tracing.CtxGetOrgID(req.Context())
		w.WriteHeader(wantStatus)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", wantTraceID)
	req.Header.Set("Birdie-Organization-ID", wantOrgID)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if gotLogger == nil {
		t.Fatal("got nil logger")
	}

	if gotTraceID != wantTraceID {
		t.Fatalf("got %q != want %q", gotTraceID, wantTraceID)
	}

	if gotOrgID != wantOrgID {
		t.Fatalf("got %q != want %q", gotOrgID, wantOrgID)
	}

	gotStatus := res.Result().StatusCode
	if gotStatus != wantStatus {
		t.Fatalf("got %v; want %v", gotStatus, wantStatus)
	}
}

func TestCtxWithTraceAndOrgID(t *testing.T) {
	const (
		wantTraceID = "trace-id-value"
		wantOrgID   = "org-id-value"
	)
	ctx := context.Background()

	got := tracing.CtxGetTraceID(ctx)
	if got != "" {
		t.Fatalf("unexpected trace id: %q", got)
	}
	got = tracing.CtxGetOrgID(ctx)
	if got != "" {
		t.Fatalf("unexpected trace id: %q", got)
	}

	ctx = tracing.CtxWithTraceID(ctx, wantTraceID)
	ctx = tracing.CtxWithOrgID(ctx, wantOrgID)

	got = tracing.CtxGetTraceID(ctx)
	if got != wantTraceID {
		t.Fatalf("got %q != want %q", got, wantTraceID)
	}

	got = tracing.CtxGetOrgID(ctx)
	if got != wantOrgID {
		t.Fatalf("got %q != want %q", got, wantTraceID)
	}
}
