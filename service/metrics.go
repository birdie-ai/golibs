package service

import (
	"net/http"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MustRegisterMetrics will register all metrics on the given registry.
func MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(buildInfo)
}

// MustRegisterHTTPMetrics will register all HTTP related metrics on the given registry.
func MustRegisterHTTPMetrics(registry *prometheus.Registry) {
	registry.MustRegister(httpInFlightCounter, httpReqDuration, httpReqCounter)
}

// InstrumentHTTP will instrument the given HTTP handler returning an instrumented
// http handler for HTTP metrics that are global (don't depend on specific handler path).
func InstrumentHTTP(handler http.Handler) http.Handler {
	return promhttp.InstrumentHandlerInFlight(httpInFlightCounter, handler)
}

// InstrumentHTTPByPath will instrument the given HTTP handler returning an instrumented
// http handler for basic HTTP metrics currying all the metrics with the given "path" as the "handler" label.
func InstrumentHTTPByPath(handler http.Handler, path string) http.Handler {
	handlerLabel := prometheus.Labels{
		"handler": path,
	}
	reqDuration := httpReqDuration.MustCurryWith(handlerLabel)
	reqCounter := httpReqCounter.MustCurryWith(handlerLabel)

	handler = promhttp.InstrumentHandlerDuration(reqDuration, handler)
	return promhttp.InstrumentHandlerCounter(reqCounter, handler)
}

// SampleBuildInfo creates a sample of the service_build_info metric.
// Since it is a gauge it needs to be set only once on the service startup.
func SampleBuildInfo() {
	goVersion := "undefined"
	revision := "undefined"

	goBuildInfo, ok := debug.ReadBuildInfo()
	if ok {
		goVersion = goBuildInfo.GoVersion
		for _, buildSetting := range goBuildInfo.Settings {
			if buildSetting.Key == "vcs.revision" {
				revision = buildSetting.Value
				if len(revision) > 7 {
					// Useful for git revisions (short hash)
					revision = revision[0:7]
				}
			}
		}
	}

	labels := prometheus.Labels{
		"goversion": goVersion,
		"revision":  revision,
	}
	buildInfo.With(labels).Set(1.0)
}

var (
	buildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_build_info",
			Help: "Build information of the service",
		},
		[]string{"revision", "goversion"},
	)

	httpInFlightCounter = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight_total",
			Help: "HTTP total in-flight request",
		},
	)

	httpReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration distribution",
			Buckets: []float64{
				.1, .25, .5, .75, 1, 1.25, 1.5, 1.75, 2, 3, 4, 5, 10, 15, 20, 25, 30,
			},
		},
		[]string{"code", "method", "handler"},
	)
	httpReqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP requests count",
		},
		[]string{"code", "method", "handler"},
	)
)
