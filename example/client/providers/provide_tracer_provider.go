package providers

import (
	"context"
	"strings"

	"github.com/gbdevw/gowsclient/example/client/configuration"
	"go.opentelemetry.io/otel"
	jaeger "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

func ProvideTracerProvider(ctx context.Context, config configuration.Configuration) (trace.TracerProvider, error) {
	if strings.ToLower(config.TracingEnabled) == "true" || config.TracingEnabled == "1" {
		// Configure Jaeger exporter
		exp, err := jaeger.New(ctx, jaeger.WithEndpoint(config.JaegerTracingBackendEndpoint), jaeger.WithInsecure())
		if err != nil {
			return nil, err
		}
		/// COnfigure tracer provider
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("wsclient.example.client"),
				semconv.DeploymentEnvironmentKey.String("production"),
			)),
		)
		// Register tracer provider as global tracer provider
		otel.SetTracerProvider(tp)
		// Return tracer provider
		return tp, nil
	} else {
		// Do not configure tracer provider.
		// Global tracer provider will return a NopTracerProvider
		return nil, nil
	}
}
