package main

import (
	"log"
	"net"
	"os"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"

	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/middleware"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/service"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/storage"
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

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor),
		grpc.StreamInterceptor(middleware.StreamAuthInterceptor),
	)
	//  Create and register your service
	fileServer := service.NewFileServer(storageLayer, db)
	pbv1.RegisterFileServiceServer(grpcServer, fileServer)
	log.Println("✓ FileService registered")

	//  Listen and serve
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	log.Println(" Server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve:", err)
	}
}
