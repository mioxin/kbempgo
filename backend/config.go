package backend

import (
	"fmt"

	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
	"github.com/mioxin/kbempgo/pkg/otel"
	"github.com/mioxin/kbempgo/pkg/prometheus"
)

// Config of slicd
type Config struct {
	Grpc       gsrv.ServerConfig       `embed:"" json:"grpc" prefix:"grpc-"`
	GrpcProxy  gsrv.ProxyConfig        `embed:"" json:"grpc-proxy" prefix:"grpc-proxy-"`
	Prometheus prometheus.ClientConfig `embed:"" json:"prometheus" prefix:"prometheus-"`
	// Log        slog.Logger             `embed:"" yaml:",inline"`
	Otel otel.OtelConfig `embed:"" json:"otel" prefix:"otel-" help:"OpenTelemetry config"`
}

func (config *Config) AfterApply() error {

	apply := map[string]func() error{
		//		"log":            config.Log.AfterApply,
		"grpc tls":       config.Grpc.AfterApply,
		"grpc proxy tls": config.GrpcProxy.AfterApply,
	}

	for key, fn := range apply {
		err := fn()
		if err != nil {
			return fmt.Errorf("%s load error: %w", key, err)
		}
	}

	return nil
}
