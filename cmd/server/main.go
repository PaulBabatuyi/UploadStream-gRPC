package main

import (
	"log"
	"net"
	"os"

	fileservicev1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"

	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/service"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/storage"
	"google.golang.org/grpc"
)

func main() {
	// 1. Initialize storage
	storageLayer := storage.NewFilesystemStorage("./data/files")
	log.Println("✓ Filesystem storage initialized")

	// 2. Initialize database
	dbURL := os.Getenv("UPLOADSTREAM")
	if dbURL == "" {
		panic("UPLOADSTREAM env var is required")
	}

	db, err := database.NewPostgresDB(dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Println("✓ Database connected")

	// 3. Create gRPC server
	grpcServer := grpc.NewServer()

	// 4. Create and register your service
	fileServer := service.NewFileServer(storageLayer, db)
	fileservicev1.RegisterFileServiceServer(grpcServer, fileServer)
	log.Println("✓ FileService registered")

	// 5. Listen and serve
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	log.Println(" Server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve:", err)
	}
}
