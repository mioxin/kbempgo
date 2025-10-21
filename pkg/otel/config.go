// Package otelutil provides common tracing setup
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mcuadros/go-defaults"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

const InitTimeout = 5 * time.Second

// OtelConfig settings for OpenTelemetry exporter
type OtelConfig struct {
	Enabled        bool   `name:"enabled" json:"enabled" default:"false" help:"OpenTelemetry tracing enabled"`
	AgentAddress   string `name:"agent-address" json:"agent_address" default:"localhost:4317" help:"OpenTelemetry collection agent address"`
	ServiceNameKey string `name:"service-name-key" json:"service_name_key" help:"OpenTelemetry service name"`
}

// SetDefaults apply defaults
func (cfg *OtelConfig) SetDefaults() {
	defaults.SetDefaults(cfg)
}

// Init configure tracer
func (cfg *OtelConfig) Init() (context.CancelFunc, error) {
	lg := slog.Default()
	nilCf := func() {}

	if !cfg.Enabled {
		lg.Debug("Tracer disabled")
		return nilCf, nil
	}

	if cfg.AgentAddress == "" {
		lg.Debug("Use stdout tracer")

		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			lg.Error("Failed to create exporter", "error", err)
			return nilCf, fmt.Errorf("failed to create exporter: %w", err)
		}

		bsp := sdktrace.NewBatchSpanProcessor(exporter)
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithSpanProcessor(bsp),
		)

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

		return nilCf, nil
	}

	lg.Info("Setting up agent connection")
	ctx := context.Background()

	dCtx, dCancel := context.WithTimeout(ctx, InitTimeout)
	defer dCancel()

	exporter, err := otlptracegrpc.New(dCtx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.AgentAddress),
		// otlptracegrpc.WithDialOption(grpc.WithBlock()),
	)
	if err != nil {
		lg.Error("Failed to create exporter", "error", err)
		return nilCf, fmt.Errorf("failed to create exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String(cfg.ServiceNameKey),
			semconv.ProcessCommand(Must(os.Executable())),
		),
	)
	if err != nil {
		lg.Error("Failed to create resource", "error", err)
		return nilCf, fmt.Errorf("failed to create resource: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		lg.Debug("Stopping tracer")
		ctx := context.Background()

		err = tp.Shutdown(ctx)
		if err != nil {
			lg.Error("Failed to Shutdown provider", "error", err)
		}
	}, nil
}
