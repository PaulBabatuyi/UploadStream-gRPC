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
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/service"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/storage"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/worker"
	"google.golang.org/grpc"
)

func main() {
	//  Initialize storage
	storageLayer := storage.NewFilesystemStorage("./data/files")
	log.Println("✓ Filesystem storage initialized")

	// Initialize database
	dbURL := os.Getenv("UPLOADSTREAM")
	if dbURL == "" {
		panic("UPLOADSTREAM env var is required")
	}

	db, err := database.NewPostgresDB(dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Println("✓ Database connected")

	// : Start background worker
	workerConfig := &worker.WorkerConfig{
		DB:           db,
		StoragePath:  "./data/files",
		PollInterval: 2 * time.Second,
	}
	processingWorker := worker.NewProcessingWorker(workerConfig)
	processingWorker.Start(context.Background())

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryAuthInterceptor),
		grpc.StreamInterceptor(middleware.StreamAuthInterceptor),
	)
	//  register  service
	fileServer := service.NewFileServer(storageLayer, db)
	pbv1.RegisterFileServiceServer(grpcServer, fileServer)
	log.Println("✓ FileService registered")

	//  Listen and serve
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}
	log.Println(" Server listening on :50051")

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n Shutting down...")
		processingWorker.Stop()
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve:", err)
	}

}
