package service

import (
	"context"
	"io"

	fileservicev1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
)

type fileServer struct {
	fileservicev1.UnimplementedFileServiceServer

	storage  StorageInterface
	database DatabaseInterface
}

type StorageInterface interface {
	CreateFile(fileID string) (io.WriteCloser, error)
	ReadFile(path string) (io.ReadCloser, error)
	DeleteFile(path string) error
}

type DatabaseInterface interface {
	SaveFile(ctx context.Context, fileID string, metadata *fileservicev1.FileMetadata, size int64) error
	GetFile(ctx context.Context, fileID string) (*database.FileRecord, error)
	ListFiles(ctx context.Context, userID string, limit int, offset int) ([]*database.FileRecord, error)
	DeleteFile(ctx context.Context, fileID, userID string) error
}
