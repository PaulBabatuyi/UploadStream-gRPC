package server

import (
	"context"
	"database/sql"
	"io"
	"log"
	"strconv"

	fileservicev1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/models"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StorageInterface interface {
	CreateFile(fileID string) (io.WriteCloser, error)
	ReadFile(fileID string) (io.ReadCloser, error)
	DeleteFile(fileID string) error
}

type DatabaseInterface interface {
	SaveFile(ctx context.Context, fileID string, metadata *fileservicev1.FileMetadata, size int64) error
	GetFile(ctx context.Context, fileID string) (*models.FileRecord, error)
	ListFiles(ctx context.Context, userID string, limit int, offset int) ([]*models.FileRecord, error)
	DeleteFile(ctx context.Context, fileID, userID string) error
}

type fileServer struct {
	fileservicev1.UnimplementedFileServiceServer

	storage  StorageInterface
	database DatabaseInterface
}

func NewFileServer(storage StorageInterface, db DatabaseInterface) *fileServer {
	return &fileServer{
		storage:  storage,
		database: db,
	}
}

func (s *fileServer) UploadFile(stream fileservicev1.FileService_UploadFileServer) error {

	// 1. Receive first message (metadata)
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "no metadata received")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	//validate
	if err := metadata.Validate(); err != nil {
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// 2. Create file in storage
	fileID := uuid.New().String()
	writer, err := s.storage.CreateFile(fileID)
	if err != nil {
		return status.Error(codes.Internal, "failed to create file")
	}
	defer writer.Close()

	// 3. Stream chunks from client
	totalSize := int64(0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break // Client finished sending
		}
		if err != nil {
			return status.Error(codes.Internal, "failed to receive chunk")
		}

		chunk := msg.GetChunk()
		n, err := writer.Write(chunk)
		if err != nil {
			return status.Error(codes.Internal, "failed to write chunk")
		}
		totalSize += int64(n)
	}
	ctx := context.Background()

	// 4. Save metadata to database
	err = s.database.SaveFile(ctx, fileID, metadata, totalSize)
	if err != nil {
		return status.Error(codes.Internal, "failed to save metadata")
	}

	// 5. Send response once
	return stream.SendAndClose(&fileservicev1.UploadFileResponse{
		FileId:   fileID,
		Filename: metadata.Filename,
		Size:     totalSize,
	})
}

func (s *fileServer) DownloadFile(req *fileservicev1.DownloadFileRequest, stream fileservicev1.FileService_DownloadFileServer) error {

	// 1. Get file metadata
	file, err := s.database.GetFile(stream.Context(), req.FileId)
	if err != nil {
		return status.Error(codes.NotFound, "file not found")
	}

	// 2. Send file info first
	err = stream.Send(&fileservicev1.DownloadFileResponse{
		Data: &fileservicev1.DownloadFileResponse_Info{
			Info: &fileservicev1.FileInfo{
				FileId:      file.ID,
				Filename:    file.Name,
				ContentType: file.ContentType,
				Size:        file.Size,
			},
		},
	})
	if err != nil {
		return err
	}

	// 3. Open file from storage
	reader, err := s.storage.ReadFile(file.ID)
	if err != nil {
		return status.Error(codes.Internal, "failed to open file")
	}
	defer reader.Close()

	// 4. Stream chunks to client
	buffer := make([]byte, 64*1024) // 64KB chunks
	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Error(codes.Internal, "failed to read file")
		}

		err = stream.Send(&fileservicev1.DownloadFileResponse{
			Data: &fileservicev1.DownloadFileResponse_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *fileServer) GetFileMetadata(ctx context.Context, req *fileservicev1.GetFileMetadataRequest) (*fileservicev1.GetFileMetadataResponse, error) {

	// 1. Validate input (protovalidate already did basic checks)
	if req.FileId == "" {
		return nil, status.Error(codes.InvalidArgument, "file_id required")
	}

	// 2. Query database
	file, err := s.database.GetFile(ctx, req.FileId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "file not found")
	}

	// 3. Return response
	return &fileservicev1.GetFileMetadataResponse{
		FileId:      file.ID,
		Filename:    file.Name,
		ContentType: file.ContentType,
		Size:        file.Size,
		UploadedAt:  timestamppb.New(file.UploadedAt),
	}, nil
}

// message ListFilesRequest {
func (fs *fileServer) ListFiles(ctx context.Context, req *fileservicev1.ListFilesRequest) (*fileservicev1.ListFilesResponse, error) {
	// 1. Basic validation
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// 2. Set reasonable defaults
	limit := int(req.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := 0
	if req.PageToken != "" {
		// Simple integer offset encoded as string (i will upgrade to base64 cursor later
		parsed, _ := strconv.Atoi(req.PageToken)
		offset = parsed
	}

	// 3. Fetch from DB
	// +1 to check if there's more
	records, err := fs.database.ListFiles(ctx, req.UserId, limit+1, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list files")
	}

	// 4. Build response
	var entries []*fileservicev1.FileEntry
	for i, rec := range records {
		if i == limit {
			// this is the "has more" marker
			break
		}
		entries = append(entries, &fileservicev1.FileEntry{
			FileId:      rec.ID,
			Filename:    rec.Name,
			ContentType: rec.ContentType,
			Size:        rec.Size,
			UploadedAt:  timestamppb.New(rec.UploadedAt),
			// placeholder for now
			ProcessingStatus: fileservicev1.ProcessingStatus_PROCESSING_STATUS_COMPLETED,
		})
	}

	// 5. Next page token if needed
	nextToken := ""
	if len(records) > limit {
		nextToken = strconv.Itoa(offset + limit)
	}

	return &fileservicev1.ListFilesResponse{
		Files:         entries,
		NextPageToken: nextToken,
	}, nil
}

func (fs *fileServer) DeleteFile(ctx context.Context, req *fileservicev1.DeleteFileRequest) (*fileservicev1.DeleteFileResponse, error) {
	// 1. Basic validation
	if req.FileId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "file_id and user_id required")
	}

	// 2. Delete from storage first (fail-fast if file missing)
	file, err := fs.database.GetFile(ctx, req.FileId)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "database error")
	}

	// 3. Ownership check
	if file.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "not owner")
	}

	// 4. Delete from storage
	if err := fs.storage.DeleteFile(req.FileId); err != nil {
		// Log but don't fail — orphaned storage is better than orphaned DB
		log.Printf("Warning: failed to delete file from storage: %v", err)
	}

	// 5. Soft-delete in DB
	if err := fs.database.DeleteFile(ctx, req.FileId, req.UserId); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete file metadata")
	}

	return &fileservicev1.DeleteFileResponse{
		Success: true,
		Message: "file deleted",
	}, nil
}
