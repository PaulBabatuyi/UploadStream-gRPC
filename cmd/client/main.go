// cmd/client/main.go
package main

import (
	"context"
	"fmt"
	"log"

	v1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Connect to server
	conn, _ := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()

	// 2. Create client
	client := v1.NewFileServiceClient(conn)

	// 3. Call RPC
	resp, err := client.GetFileMetadata(context.Background(),
		&v1.GetFileMetadataRequest{
			FileId: "some-uuid",
		})

	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File: %s\n", resp.Filename)
}
