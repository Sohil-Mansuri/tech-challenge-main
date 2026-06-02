// Package otelx configures OpenTelemetry metric and trace providers.
// It uses stdout exporters so that metrics and spans are visible in the
// process log without requiring an external collector.  Swap the exporters
// for OTLP when you want to send data to a real backend (Prometheus,
// Tempo, Jaeger, …).
package otelx

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const serviceName = "tech-challenge"

func Setup(ctx context.Context) (shutdown func(context.Context) error, err error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otelx: create resource: %w", err)
	}

	metricExp, err := stdoutmetric.New()
	if err != nil {
		return nil, fmt.Errorf("otelx: create metric exporter: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		// Export a snapshot of all metrics every 15 seconds.
		metric.WithReader(metric.NewPeriodicReader(metricExp,
			metric.WithInterval(15*time.Second),
		)),
	)
	otel.SetMeterProvider(mp)

	traceExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("otelx: create trace exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(traceExp),
		// Sample every request; tune with trace.WithSampler() in production.
		trace.WithSampler(trace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	shutdown = func(ctx context.Context) error {
		var merr error
		if err := mp.Shutdown(ctx); err != nil {
			merr = fmt.Errorf("metric provider shutdown: %w", err)
		}
		if err := tp.Shutdown(ctx); err != nil {
			merr = fmt.Errorf("trace provider shutdown: %w", err)
		}
		return merr
	}
	return shutdown, nil
}
