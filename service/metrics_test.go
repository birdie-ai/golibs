package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/birdie-ai/golibs/service"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricInstrumentation(t *testing.T) {
	// Just guarantee that it is not broken/panicking (like wrong labels/etc).
	// We test everything together/integrated by design, it would be possible for one set of
	// metrics to collide with some other, theoretically services should be able to use
	// all the metrics exported on the pkg together, we should have no collisions.
	metricsRegistry := prometheus.NewRegistry()
	service.MustRegisterMetrics(metricsRegistry)
	service.MustRegisterHTTPMetrics(metricsRegistry)
	service.SampleBuildInfo()

	called := false
	handler := service.InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		called = true
	}))
	handler = service.InstrumentHTTPByPath(handler, "/test")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if !called {
		t.Fatal("want instrumented handler to be called")
	}
}
