package event

import "github.com/prometheus/client_golang/prometheus"

// MustRegisterMetrics will register all event related metrics on the given registry.
// If metrics with the same name already exist no the register this function will panic.
func MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(publishDuration, publishCounter, processCounter, processDuration)
}

var (
	publishDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "event_publish_duration_seconds",
			Help: "Duration of event publish",
			Buckets: []float64{
				.1, .2, .3, .4, .5, .6, .7, .8, .9, 1,
				2, 3, 4, 5, 10, 15, 20, 30,
			},
		},
		[]string{"status", "name"},
	)
	publishCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_publish_total",
			Help: "Total of published events",
		},
		[]string{"status", "name"},
	)
	processDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "event_process_duration_seconds",
			Help: "Duration of event processing",
			Buckets: []float64{
				.1, .2, .3, .4, .5, .6, .7, .8, .9, 1,
				2, 3, 4, 5, 10, 15, 20, 30,
			},
		},
		[]string{"status", "name"},
	)
	processCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_process_total",
			Help: "Total of processed events",
		},
		[]string{"status", "name"},
	)
)
