package service

import (
	"context"
	"io"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
	"golang.org/x/sync/semaphore"
)

type fileServer struct {
	pbv1.UnimplementedFileServiceServer

	storage   StorageInterface
	database  DatabaseInterface
	uploadSem *semaphore.Weighted
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
	CreateProcessingJob(ctx context.Context, fileID string) (int64, error)
	GetNextPendingJob(ctx context.Context) (*database.ProcessingJob, error)
	UpdateJobStatus(ctx context.Context, jobID int64, status, errorMsg string) error
	CompleteJob(ctx context.Context, jobID int64, thumbSmall, thumbMed, thumbLarge string, width, height int) error
	GetJobByFileID(ctx context.Context, fileID string) (*database.ProcessingJob, error)
}
