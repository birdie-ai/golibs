package service_test

import (
	"testing"

	"github.com/birdie-ai/golibs/service"
	"github.com/prometheus/client_golang/prometheus"
)

func TestBuildInfoSample(*testing.T) {
	// Just guarantee that it is not broken/panicking (like wrong labels/etc).
	metricsRegistry := prometheus.NewRegistry()
	service.MustRegisterMetrics(metricsRegistry)
	service.SampleBuildInfo()
}
