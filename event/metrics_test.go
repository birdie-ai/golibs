package event_test

import (
	"testing"

	"github.com/birdie-ai/golibs/event"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterMetrics(t *testing.T) {
	// For now we only test that the metrics definitions are valid.
	registry := prometheus.NewRegistry()
	event.MustRegisterMetrics(registry)
}
