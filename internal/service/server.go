package service

import (
	"context"
	"io"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
)

type fileServer struct {
	pbv1.UnimplementedFileServiceServer

	storage  StorageInterface
	database DatabaseInterface
}

type StorageInterface interface {
	CreateFile(fileID string) (io.WriteCloser, error)
	ReadFile(fileID string) (io.ReadCloser, error)
	DeleteFile(fileID string) error
}

type DatabaseInterface interface {
	SaveFile(ctx context.Context, fileID string, metadata *pbv1.FileMetadata, size int64) error
	GetFile(ctx context.Context, fileID string) (*database.FileRecord, error)
	ListFiles(ctx context.Context, userID string, limit int, offset int) ([]*database.FileRecord, error)
	DeleteFile(ctx context.Context, fileID, userID string) error
}
