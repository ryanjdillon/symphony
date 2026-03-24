package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	serviceName    = "symphony"
	serviceVersion = "0.1.0"
)

// Provider wraps the OTEL meter provider and exposes a shutdown function.
type Provider struct {
	mp *sdkmetric.MeterProvider
}

// Init creates and registers a global meter provider with an OTLP gRPC exporter.
// Only initializes if OTEL_EXPORTER_OTLP_ENDPOINT is set; returns nil Provider
// (no-op) otherwise. The exporter respects standard OTEL env vars.
func Init(ctx context.Context) (*Provider, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return nil, nil
	}

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTEL resource: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second))),
	)

	otel.SetMeterProvider(mp)

	return &Provider{mp: mp}, nil
}

// Shutdown flushes pending metrics and shuts down the provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.mp == nil {
		return nil
	}
	return p.mp.Shutdown(ctx)
}

// Meter returns a named meter for creating instruments.
func Meter() metric.Meter {
	return otel.Meter(serviceName)
}
