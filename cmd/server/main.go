package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"

	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/middleware"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/observability"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/service"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/storage"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/worker"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Initialize logger
	isDev := os.Getenv("ENV") != "production"
	logger, err := observability.InitLogger(isDev)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting UploadStream server",
		zap.String("environment", os.Getenv("ENV")),
		zap.String("version", "0.1.0"),
	)

	// Initialize tracing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	tp, err := observability.InitTracerProvider(ctx, logger)
	if err != nil {
		logger.Fatal("failed to initialize tracer provider", zap.Error(err))
	}
	cancel()
	defer observability.ShutdownTracerProvider(context.Background(), tp, logger)

	// Initialize metrics
	metrics, err := observability.InitMetrics()
	if err != nil {
		logger.Error("failed to initialize metrics", zap.Error(err))
	} else {
		// Start metrics HTTP server on port 9090
		observability.StartMetricsServer("9090", logger)
		logger.Info("metrics endpoint available at http://localhost:9090/metrics")
	}

	// Initialize storage
	storageLayer := storage.NewFilesystemStorage("./data/files")
	logger.Info("filesystem storage initialized")

	// Initialize database
	dbURL := os.Getenv("UPLOADSTREAM")
	if dbURL == "" {
		logger.Fatal("UPLOADSTREAM env var is required")
	}

	db, err := database.NewPostgresDB(dbURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	logger.Info("database connected")

	// Start background worker
	workerConfig := &worker.WorkerConfig{
		DB:           db,
		StoragePath:  "./data/files",
		PollInterval: 2 * time.Second,
	}
	processingWorker := worker.NewProcessingWorker(workerConfig)
	processingWorker.Start(context.Background())

	// Build gRPC server with observability interceptors
	grpcServerOpts := []grpc.ServerOption{
		// Auth interceptors
		grpc.UnaryInterceptor(
			middleware.ChainUnaryInterceptors(
				middleware.UnaryAuthInterceptor,
				middleware.UnaryLoggingInterceptor(logger),
			),
		),
		grpc.StreamInterceptor(
			middleware.ChainStreamInterceptors(
				middleware.StreamAuthInterceptor,
				middleware.StreamLoggingInterceptor(logger),
			),
		),
		// OpenTelemetry tracing
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}

	// Add Prometheus metrics if available
	if metrics != nil {
		grpcServerOpts = append(grpcServerOpts,
			grpc.ChainUnaryInterceptor(metrics.GetServerMetrics().UnaryServerInterceptor()),
			grpc.ChainStreamInterceptor(metrics.GetServerMetrics().StreamServerInterceptor()),
		)
	}

	grpcServer := grpc.NewServer(grpcServerOpts...)

	// Register service
	fileServer := service.NewFileServer(storageLayer, db)
	pbv1.RegisterFileServiceServer(grpcServer, fileServer)
	logger.Info("FileService registered")

	// Listen and serve
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("failed to listen", zap.Error(err))
	}
	logger.Info("gRPC server listening", zap.String("addr", ":50051"))

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))

		processingWorker.Stop()
		grpcServer.GracefulStop()
		logger.Info("server shutdown complete")
	}()

	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("server failed", zap.Error(err))
	}
}
