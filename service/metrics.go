package service

import (
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
)

// MustRegister will register all metrics on the given registry.
func MustRegister(registry *prometheus.Registry) {
	registry.MustRegister(buildInfo)
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
)
