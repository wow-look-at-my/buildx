package metricutil

import (
	"github.com/docker/buildx/version"
	"go.opentelemetry.io/otel/metric"
)

func Meter(mp metric.MeterProvider) metric.Meter {
	return mp.Meter(version.Package,
		metric.WithInstrumentationVersion(version.Version))
}
