// Package prometheus provides halpers to communicate with Prometheus
package prometheus

import (
	"net/http"

	"github.com/mioxin/kbempgo/pkg/otel"

	promapi "github.com/prometheus/client_golang/api"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

func makeAPIConfig(dburl string) promapi.Config {
	return promapi.Config{
		Address: dburl,
		RoundTripper: otelhttp.NewTransport(http.DefaultTransport,
			otelhttp.WithSpanOptions(
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(otel.DBSystemPrometheus),
			),
		),
	}
}

// NewClient makes promapi client
func NewClient(cfg ClientConfig) (promapi.Client, error) {
	return promapi.NewClient(makeAPIConfig(cfg.DBURL))
}

// NewMetadataClient makes promapi client to query metadata
func NewMetadataClient(cfg ClientConfig) (promapi.Client, error) {
	dburl := cfg.MetadataDBURL
	if dburl == "" {
		dburl = cfg.DBURL
	}

	return promapi.NewClient(makeAPIConfig(dburl))
}
