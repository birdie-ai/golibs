package event

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MustRegisterMetrics will register all event related metrics on the given registry.
// If metrics with the same name already exist no the register this function will panic.
func MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(publishMsgBodySize, publishDuration, publishCounter,
		processMsgBodySize, processCounter, processDuration, processBatchSize)
}

// SampledMessageHandler will instrument the given MessageHandler returning a new one
// that samples metrics. These will be `event_process_*` metrics using as `name` the
// given eventName.
func SampledMessageHandler(eventName string, handler MessageHandler) MessageHandler {
	return func(msg Message) error {
		start := time.Now()
		err := handler(msg)
		elapsed := time.Since(start)
		sampleMsgProcess(msg, eventName, elapsed, err)
		return err
	}
}

func publishSampler() func(string, int, error) {
	start := time.Now()
	return func(name string, bodySize int, err error) {
		elapsed := time.Since(start)
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
}

func sampleMsgProcess(msg Message, name string, elapsed time.Duration, err error) {
	sampleProcess(name, elapsed, float64(len(msg.Body)), err)
}

func sampleProcess(name string, elapsed time.Duration, bodyLen float64, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	labels := prometheus.Labels{
		"status": status,
		"name":   name,
	}
	processMsgBodySize.With(labels).Observe(bodyLen)
	processDuration.With(labels).Observe(elapsed.Seconds())
	processCounter.With(labels).Inc()
}

func sampleBatchSize(name string, size int) {
	labels := prometheus.Labels{"name": name}
	processBatchSize.With(labels).Observe(float64(size))
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
			// event handling usually takes longer and we dont need it be as fine grained as HTTP requests.
			Buckets: append([]float64{
				.1, .2, .3, .4, .5, .6, .7, .8, .9,
			},
				prometheus.LinearBuckets(1, 5, 120)..., // Linearly go until 10min with 5 sec granularity
			),
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
	processBatchSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "event_process_batch_size",
			Help: "Size of each batch when using batching (ServeBatch)",
			Buckets: []float64{
				1, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 60, 70, 80, 90,
				100, 150, 200, 250, 300, 350, 400, 450, 500, 550, 600, 650, 700, 750, 800, 850, 900, 950,
				1000, 1100, 1200, 1300, 1400, 1500, 1600, 1700, 1800, 1900,
				2000, 2500, 3000, 3500, 4000, 4500, 5000, 5500, 6000, 6500, 7000, 7500, 8000, 8500, 9000, 9500,
				10_000, 11_000, 12_000, 13_000, 15_000, 16_000, 17_000, 18_000, 19_000,
				20_000, 30_000, 40_000, 50_000, 60_000, 70_000, 80_000, 90_000,
				100_000, 200_000, 300_000, 400_000, 500_000, 600_000, 700_000, 800_000, 900_000,
				1_000_000,
			},
		},
		[]string{"name"},
	)
	processCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_process_total",
			Help: "Total of processed events",
		},
		[]string{"status", "name"},
	)
)
