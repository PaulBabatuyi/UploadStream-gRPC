package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	serverAddr = "localhost:50051"
	chunkSize  = 64 * 1024 // 64KB chunks
)

type FileClient struct {
	client pbv1.FileServiceClient
}

func NewFileClient(addr string) (*FileClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(clientAuthInterceptor),
		grpc.WithStreamInterceptor(clientStreamAuthInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &FileClient{
		client: pbv1.NewFileServiceClient(conn),
	}, nil
}

func clientAuthInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "api-key", "dev-key-123")
	return invoker(ctx, method, req, reply, cc, opts...)
}

func clientStreamAuthInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "api-key", "dev-key-123")
	return streamer(ctx, desc, cc, method, opts...)
}

// UploadFile streams a file to the server
func (fc *FileClient) UploadFile(ctx context.Context, filePath, userID string) (*pbv1.UploadFileResponse, error) {
	//  Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	//  Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Create upload stream
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second) // Longer for files
	defer cancel()
	stream, err := fc.client.UploadFile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	//  Send metadata first
	metadata := &pbv1.FileMetadata{
		Filename:    fileInfo.Name(),
		ContentType: detectContentType(filePath),
		Size:        fileInfo.Size(),
		UserId:      userID,
	}

	err = stream.Send(&pbv1.UploadFileRequest{
		Data: &pbv1.UploadFileRequest_Metadata{
			Metadata: metadata,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send metadata: %w", err)
	}

	//  Stream file chunks
	buffer := make([]byte, chunkSize)
	totalSent := int64(0)

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		err = stream.Send(&pbv1.UploadFileRequest{
			Data: &pbv1.UploadFileRequest_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to send chunk: %w", err)
		}

		totalSent += int64(n)
		progress := float64(totalSent) / float64(fileInfo.Size()) * 100
		fmt.Printf("\rUploading: %.2f%%", progress)
	}
	fmt.Println()

	// Close stream and get response
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	return resp, nil
}

// DownloadFile streams a file from the server
func (fc *FileClient) DownloadFile(ctx context.Context, fileID, outputPath string) error {
	// Create download stream
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Longer for files
	defer cancel()
	stream, err := fc.client.DownloadFile(ctx, &pbv1.DownloadFileRequest{
		FileId: fileID,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	//  Receive first message (file info)
	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive file info: %w", err)
	}

	fileInfo := firstMsg.GetInfo()
	if fileInfo == nil {
		return fmt.Errorf("expected file info in first message")
	}

	fmt.Printf("Downloading: %s (%d bytes)\n", fileInfo.Filename, fileInfo.Size)

	//  Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	//  Receive chunks and write to file
	totalReceived := int64(0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive chunk: %w", err)
		}

		chunk := msg.GetChunk()
		n, err := outFile.Write(chunk)
		if err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}

		totalReceived += int64(n)
		progress := float64(totalReceived) / float64(fileInfo.Size) * 100
		fmt.Printf("\rDownloading: %.2f%%", progress)
	}
	fmt.Println() // New line after progress

	return nil
}

// GetFileMetadata retrieves metadata for a file
func (fc *FileClient) GetFileMetadata(ctx context.Context, fileID string) (*pbv1.GetFileMetadataResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := fc.client.GetFileMetadata(ctx, &pbv1.GetFileMetadataRequest{
		FileId: fileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	return resp, nil
}

// ListFiles lists files for a user with pagination
func (fc *FileClient) ListFiles(ctx context.Context, userID string, pageSize int32, pageToken string) (*pbv1.ListFilesResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := fc.client.ListFiles(ctx, &pbv1.ListFilesRequest{
		UserId:    userID,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	return resp, nil
}

// DeleteFile deletes a file
func (fc *FileClient) DeleteFile(ctx context.Context, fileID, userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := fc.client.DeleteFile(ctx, &pbv1.DeleteFileRequest{
		FileId: fileID,
		UserId: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("delete failed: %s", resp.Message)
	}
	return nil
}

// detectContentType attempts to detect MIME type, falls back to extension
func detectContentType(filePath string) string {
	// Try magic bytes first
	file, err := os.Open(filePath)
	if err == nil {
		defer file.Close()
		buffer := make([]byte, 512)
		n, err := file.Read(buffer)
		if err == nil && n > 0 {
			detected := http.DetectContentType(buffer[:n])
			// Don't return "application/octet-stream" if we can infer from extension
			if detected != "application/octet-stream" {
				return detected
			}
		}
	}

	// Fallback to extension-based detection
	ext := filepath.Ext(filePath)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK from client "))
	})
	// Create client
	client, err := NewFileClient(serverAddr)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440000" //

	// Example 1: Upload a file
	fmt.Println("=== Uploading File ===")
	uploadResp, err := client.UploadFile(ctx, "test-file.txt", userID)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			log.Printf("%s failed: %s", st.Code(), st.Message())
		} else {
			log.Printf("Upload failed: %v", err)
		}
	} else {
		fmt.Printf("✓ Uploaded: %s (ID: %s, Size: %d bytes)\n",
			uploadResp.Filename, uploadResp.FileId, uploadResp.Size)
	}

	// Use uploaded file ID for subsequent operations
	fileID := uploadResp.FileId

	// Example 2: Get file metadata
	fmt.Println("\n=== Getting File Metadata ===")
	metadata, err := client.GetFileMetadata(ctx, fileID)
	if err != nil {
		log.Printf("Get metadata failed: %v", err)
	} else {
		fmt.Printf("✓ File: %s\n", metadata.Filename)
		fmt.Printf("  Type: %s\n", metadata.ContentType)
		fmt.Printf("  Size: %d bytes\n", metadata.Size)
		fmt.Printf("  Uploaded: %s\n", metadata.UploadedAt.AsTime().Format(time.RFC3339))
	}

	// Example 3: List files
	fmt.Println("\n=== Listing Files ===")
	listResp, err := client.ListFiles(ctx, userID, 10, "")
	if err != nil {
		log.Printf("List files failed: %v", err)
	} else {
		fmt.Printf("✓ Found %d files:\n", len(listResp.Files))
		for i, file := range listResp.Files {
			fmt.Printf("  %d. %s (ID: %s, %d bytes)\n",
				i+1, file.Filename, file.FileId, file.Size)
		}
		if listResp.NextPageToken != "" {
			fmt.Printf("  Next page token: %s\n", listResp.NextPageToken)
		}
	}

	// Example 4: Download file
	fmt.Println("\n=== Downloading File ===")
	err = client.DownloadFile(ctx, fileID, "downloaded-file.txt")
	if err != nil {
		log.Printf("Download failed: %v", err)
	} else {
		fmt.Println("✓ File downloaded successfully")
	}

	// Example 5: Delete file
	fmt.Println("\n=== Deleting File ===")
	err = client.DeleteFile(ctx, fileID, userID)
	if err != nil {
		log.Printf("Delete failed: %v", err)
	} else {
		fmt.Println("✓ File deleted successfully")
	}
}
