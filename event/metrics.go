package event

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MustRegisterMetrics will register all event related metrics on the given registry.
// If metrics with the same name already exist no the register this function will panic.
func MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(publishMsgBodySize, publishDuration, publishCounter,
		processMsgBodySize, processCounter, processDuration)
}

// SampledMessageHandler will instrument the given MessageHandler returning a new one
// that samples metrics. These will be `event_process_*` metrics using as `name` the
// given eventName.
func SampledMessageHandler(eventName string, handler MessageHandler) MessageHandler {
	return func(msg Message) error {
		start := time.Now()
		err := handler(msg)
		elapsed := time.Since(start)
		sampleProcess(msg, eventName, elapsed, err)
		return err
	}
}

func samplePublish(name string, elapsed time.Duration, bodySize int, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	labels := prometheus.Labels{
		"status": status,
		"name":   name,
	}
	publishMsgBodySize.With(labels).Observe(float64(bodySize))
	publishDuration.With(labels).Observe(elapsed.Seconds())
	publishCounter.With(labels).Inc()
}

func sampleProcess(msg Message, name string, elapsed time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	labels := prometheus.Labels{
		"status": status,
		"name":   name,
	}
	processMsgBodySize.With(labels).Observe(float64(len(msg.Body)))
	processDuration.With(labels).Observe(elapsed.Seconds())
	processCounter.With(labels).Inc()
}

var (
	// GCP max message size is 10mb
	bodySizeBuckets    = prometheus.ExponentialBucketsRange(256, 1024*1024*10, 30)
	publishMsgBodySize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "event_publish_msg_body_size_bytes",
			Help:    "Size in bytes of published event message body",
			Buckets: bodySizeBuckets,
		},
		[]string{"status", "name"},
	)
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
	processMsgBodySize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "event_process_msg_body_size_bytes",
			Help:    "Size in bytes of processed event message body",
			Buckets: bodySizeBuckets,
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
