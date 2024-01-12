package event

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MustRegisterMetrics will register all event related metrics on the given registry.
// If metrics with the same name already exist no the register this function will panic.
func MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(publishDuration, publishCounter, processCounter, processDuration)
}

// SampledMessageHandler will instrument the given MessageHandler returning a new one
// that samples metrics. These will be `event_process_*` metrics using as `name` the
// given eventName.
func SampledMessageHandler(eventName string, handler MessageHandler) MessageHandler {
	return func(msg Message) error {
		start := time.Now()
		err := handler(msg)
		elapsed := time.Since(start)
		sampleProcess(eventName, elapsed, err)
		return err
	}
}

func samplePublish(name string, elapsed time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	labels := prometheus.Labels{
		"status": status,
		"name":   name,
	}
	publishDuration.With(labels).Observe(elapsed.Seconds())
	publishCounter.With(labels).Inc()
}

func sampleProcess(name string, elapsed time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	labels := prometheus.Labels{
		"status": status,
		"name":   name,
	}
	processDuration.With(labels).Observe(elapsed.Seconds())
	processCounter.With(labels).Inc()
}

var (
	publishDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "event_publish_duration_seconds",
			Help: "Duration of event publish",
			// publish times are much smaller since they measure only communication with broker.
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
			// processing takes longer and GCP max processing time is 10 minutes
			Buckets: []float64{
				.1, .2, .3, .4, .5, .6, .7, .8, .9, 1,
				2, 3, 4, 5, 10, 15, 20, 30, 60, 90, 120,
				180, 240, 300, 360, 420, 480, 540, 600,
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
