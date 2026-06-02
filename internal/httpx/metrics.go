package httpx

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "tech-challenge/httpx"

func Metrics() func(http.Handler) http.Handler {
	meter := otel.GetMeterProvider().Meter(meterName)

	requestCount, _ := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)

	requestDuration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request latency"),
		metric.WithUnit("ms"),

		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000),
	)

	errorCount, _ := meter.Int64Counter(
		"http.server.error.count",
		metric.WithDescription("Total number of HTTP requests that resulted in a 5xx response"),
		metric.WithUnit("{request}"),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			saw := &statusAwareResponseWriter{ResponseWriter: w}
			saw.status = http.StatusOK

			next.ServeHTTP(saw, r)

			elapsedMs := float64(time.Since(start).Milliseconds())
			statusStr := strconv.Itoa(saw.status)

			attrs := metric.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", r.URL.Path),
				attribute.String("http.status_code", statusStr),
			)

			ctx := r.Context()
			requestCount.Add(ctx, 1, attrs)
			requestDuration.Record(ctx, elapsedMs, attrs)

			if saw.status/100 == 5 {
				errorCount.Add(ctx, 1, metric.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.route", r.URL.Path),
					attribute.String("http.status_code", statusStr),
				))
			}
		})
	}
}
