package observability

import (
	"context"

	"go.opentelemetry.io/exporters/jaeger/jaegergrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

func InitTracerProvider(ctx context.Context, logger *zap.Logger) (*trace.TracerProvider, error) {
	exporter, err := jaegergrpc.New(
		ctx,
		jaegergrpc.WithEndpoint("localhost:14250"),
	)
	if err != nil {
		logger.Error("failed to create jaeger exporter", zap.Error(err))
		return nil, err
	}

	tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
	return tp, nil
}
