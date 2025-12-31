package observability

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// InitTracerProvider initializes OpenTelemetry tracing with stdout exporter
func InitTracerProvider(ctx context.Context, logger *zap.Logger) (*trace.TracerProvider, error) {
	// Create stdout exporter for development (swap to Jaeger for production)
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		logger.Error("failed to create trace exporter", zap.Error(err))
		return nil, err
	}

	// Create trace provider with exporter
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
	)

	if err := tp.ForceFlush(ctx); err != nil {
		logger.Error("failed to flush traces", zap.Error(err))
	}

	return tp, nil
}

// ShutdownTracerProvider gracefully shuts down the tracer provider
func ShutdownTracerProvider(ctx context.Context, tp *trace.TracerProvider, logger *zap.Logger) {
	if err := tp.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown tracer provider", zap.Error(err))
	}
}

// GetOTelGRPCOption returns OpenTelemetry gRPC interceptor options
func GetOTelGRPCOption(tp *trace.TracerProvider) []any {
	return []any{
		otelgrpc.WithTracerProvider(tp),
	}
}
