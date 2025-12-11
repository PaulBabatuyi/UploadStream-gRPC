// cmd/server/main.go
package main

import (
    "net"
    "google.golang.org/grpc"
    pb "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
    "github.com/PaulBabatuyi/UploadStream-gRPC/internal/server"
)

func main() {
    // 1. Create dependencies
    db := database.New(os.Getenv("DATABASE_URL"))
    storage := storage.NewFilesystem("./data/files")
    
    // 2. Create your service implementation
    fileServer := server.NewFileServer(storage, db)
    
    // 3. Create gRPC server
    grpcServer := grpc.NewServer()
    
    // 4. Register your service
    pb.RegisterFileServiceServer(grpcServer, fileServer)
    
    // 5. Listen and serve
    lis, _ := net.Listen("tcp", ":50051")
    grpcServer.Serve(lis)
}