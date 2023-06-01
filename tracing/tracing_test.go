package tracing_test

import (
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
}
