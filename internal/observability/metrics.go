package observability

import (
	"net/http"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MetricsCollector wraps Prometheus metrics for gRPC
type MetricsCollector struct {
	serverMetrics *grpcprom.ServerMetrics
	handler       http.Handler
}

// InitMetrics initializes Prometheus metrics for gRPC server
func InitMetrics() (*MetricsCollector, error) {
	// Create server metrics with default buckets
	serverMetrics := grpcprom.NewServerMetrics(
		grpcprom.WithServerHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.5, 1, 2.5, 5, 10}),
		),
	)

	// Register with Prometheus
	if err := prometheus.Register(serverMetrics); err != nil {
		// If already registered, that's okay (useful for testing)
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return nil, err
		}
	}

	// Create HTTP handler for /metrics endpoint
	handler := promhttp.Handler()

	return &MetricsCollector{
		serverMetrics: serverMetrics,
		handler:       handler,
	}, nil
}

// GetServerMetrics returns the gRPC server metrics
func (mc *MetricsCollector) GetServerMetrics() *grpcprom.ServerMetrics {
	return mc.serverMetrics
}

// GetHandler returns the HTTP handler for /metrics endpoint
func (mc *MetricsCollector) GetHandler() http.Handler {
	return mc.handler
}

// StartMetricsServer starts an HTTP server on port 9090 for metrics
func StartMetricsServer(port string, logger *zap.Logger) {
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		http.Handle("/metrics", promhttp.Handler())

		logger.Info("starting metrics server", zap.String("port", port))
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			logger.Error("metrics server failed", zap.Error(err))
		}
	}()
}
