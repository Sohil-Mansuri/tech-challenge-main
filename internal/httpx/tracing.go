package httpx

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "tech-challenge/httpx"

func Tracing() func(http.Handler) http.Handler {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	propagator := otel.GetTextMapPropagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPath(r.URL.Path),
					attribute.String("http.host", r.Host),
					attribute.String("user_agent.original", r.UserAgent()),
				),
			)
			defer span.End()

			saw := &statusAwareResponseWriter{ResponseWriter: w, status: http.StatusOK}

			propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

			next.ServeHTTP(saw, r.WithContext(ctx))

			span.SetAttributes(semconv.HTTPResponseStatusCode(saw.status))

			if saw.status/100 == 5 {
				span.SetStatus(codes.Error, http.StatusText(saw.status))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}
