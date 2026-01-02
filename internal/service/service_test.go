package service_test

import (
	"context"
	"io"
	"os"
	"testing"

	"net"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/service"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	lis        *bufconn.Listener
	testUserID = "550e8400-e29b-41d4-a716-446655440000"
)

func setupTestServer(t *testing.T) (pbv1.FileServiceClient, func()) {
	// Setup test database
	dbURL := os.Getenv("UPLOADSTREAM")
	if dbURL == "" {
		t.Skip("UPLOADSTREAM env not set")
	}

	db, err := database.NewPostgresDB(dbURL)
	require.NoError(t, err)

	// Setup test storage
	tmpDir := t.TempDir()
	storageLayer := storage.NewFilesystemStorage(tmpDir)

	// Create gRPC server
	lis = bufconn.Listen(bufSize)
	server := grpc.NewServer()
	pbv1.RegisterFileServiceServer(server, service.NewFileServer(storageLayer, db))

	go func() {
		if err := server.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Create client
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := pbv1.NewFileServiceClient(conn)

	cleanup := func() {
		conn.Close()
		server.Stop()
	}

	return client, cleanup
}

func TestUploadDownloadFlow(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create test file
	testContent := []byte("Hello, gRPC streaming!")

	// 1. Upload file
	stream, err := client.UploadFile(ctx)
	require.NoError(t, err)

	// Send metadata
	err = stream.Send(&pbv1.UploadFileRequest{
		Data: &pbv1.UploadFileRequest_Metadata{
			Metadata: &pbv1.FileMetadata{
				Filename:    "test.txt",
				ContentType: "text/plain",
				Size:        int64(len(testContent)),
				UserId:      testUserID,
			},
		},
	})
	require.NoError(t, err)

	// Send chunk
	err = stream.Send(&pbv1.UploadFileRequest{
		Data: &pbv1.UploadFileRequest_Chunk{
			Chunk: testContent,
		},
	})
	require.NoError(t, err)

	// Get response
	uploadResp, err := stream.CloseAndRecv()
	require.NoError(t, err)
	assert.NotEmpty(t, uploadResp.FileId)
	assert.Equal(t, "test.txt", uploadResp.Filename)
	assert.Equal(t, int64(len(testContent)), uploadResp.Size)

	fileID := uploadResp.FileId

	// 2. Download file
	downloadStream, err := client.DownloadFile(ctx, &pbv1.DownloadFileRequest{
		FileId: fileID,
	})
	require.NoError(t, err)

	// Receive file info
	firstMsg, err := downloadStream.Recv()
	require.NoError(t, err)
	info := firstMsg.GetInfo()
	assert.NotNil(t, info)
	assert.Equal(t, "test.txt", info.Filename)

	// Receive chunks
	var downloaded []byte
	for {
		msg, err := downloadStream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		downloaded = append(downloaded, msg.GetChunk()...)
	}

	assert.Equal(t, testContent, downloaded)

	// 3. Get metadata
	metadata, err := client.GetFileMetadata(ctx, &pbv1.GetFileMetadataRequest{
		FileId: fileID,
	})
	require.NoError(t, err)
	assert.Equal(t, "test.txt", metadata.Filename)

	// 4. List files
	listResp, err := client.ListFiles(ctx, &pbv1.ListFilesRequest{
		UserId:   testUserID,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listResp.Files), 1)

	// 5. Delete file
	deleteResp, err := client.DeleteFile(ctx, &pbv1.DeleteFileRequest{
		FileId: fileID,
		UserId: testUserID,
	})
	require.NoError(t, err)
	assert.True(t, deleteResp.Success)
}

func TestUploadSizeLimit(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Try uploading oversized file
	stream, err := client.UploadFile(ctx)
	require.NoError(t, err)

	err = stream.Send(&pbv1.UploadFileRequest{
		Data: &pbv1.UploadFileRequest_Metadata{
			Metadata: &pbv1.FileMetadata{
				Filename:    "huge.bin",
				ContentType: "application/octet-stream",
				Size:        600 * 1024 * 1024, // 600MB > 512MB limit
				UserId:      testUserID,
			},
		},
	})
	require.NoError(t, err)

	_, err = stream.CloseAndRecv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")
}

func TestInvalidContentType(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create image file
	testContent := []byte("\x89PNG\r\n\x1a\n")

	stream, err := client.UploadFile(ctx)
	require.NoError(t, err)

	// Declare as text but send PNG
	err = stream.Send(&pbv1.UploadFileRequest{
		Data: &pbv1.UploadFileRequest_Metadata{
			Metadata: &pbv1.FileMetadata{
				Filename:    "fake.txt",
				ContentType: "text/plain",
				Size:        int64(len(testContent)),
				UserId:      testUserID,
			},
		},
	})
	require.NoError(t, err)

	err = stream.Send(&pbv1.UploadFileRequest{
		Data: &pbv1.UploadFileRequest_Chunk{
			Chunk: testContent,
		},
	})
	require.NoError(t, err)

	_, err = stream.CloseAndRecv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content type mismatch")
}

// Benchmark upload performance
func BenchmarkUpload(b *testing.B) {
	client, cleanup := setupTestServer(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	testContent := make([]byte, 1024*1024) // 1MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := client.UploadFile(ctx)
		stream.Send(&pbv1.UploadFileRequest{
			Data: &pbv1.UploadFileRequest_Metadata{
				Metadata: &pbv1.FileMetadata{
					Filename:    "bench.bin",
					ContentType: "application/octet-stream",
					Size:        int64(len(testContent)),
					UserId:      testUserID,
				},
			},
		})
		stream.Send(&pbv1.UploadFileRequest{
			Data: &pbv1.UploadFileRequest_Chunk{
				Chunk: testContent,
			},
		})
		stream.CloseAndRecv()
	}
}
