package service

import (
	"context"
	"database/sql"
	"io"
	"log"
	"strconv"

	fileservicev1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewFileServer(storage StorageInterface, db DatabaseInterface) *fileServer {
	return &fileServer{
		storage:  storage,
		database: db,
	}
}

func (s *fileServer) UploadFile(stream fileservicev1.FileService_UploadFileServer) error {

	//  . Receive first message (metadata)
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "no metadata received")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	//validate metadata
	if err := metadata.Validate(); err != nil {
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  . Create file in storage
	fileID := uuid.New().String()
	writer, err := s.storage.CreateFile(fileID)
	if err != nil {
		return status.Error(codes.Internal, "failed to create file")
	}
	defer writer.Close()

	//  . Stream chunks from client
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

	//  . Save metadata to database
	err = s.database.SaveFile(ctx, fileID, metadata, totalSize)
	if err != nil {
		return status.Error(codes.Internal, "failed to save metadata")
	}

	//  . Send response once
	return stream.SendAndClose(&fileservicev1.UploadFileResponse{
		FileId:   fileID,
		Filename: metadata.Filename,
		Size:     totalSize,
	})
}

func (s *fileServer) DownloadFile(req *fileservicev1.DownloadFileRequest, stream fileservicev1.FileService_DownloadFileServer) error {
	// Validate request
	if err := req.Validate(); err != nil {
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  Get file metadata
	file, err := s.database.GetFile(stream.Context(), req.FileId)
	if err != nil {
		return status.Error(codes.NotFound, "file not found")
	}

	//  . Send file info first
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

	//  . Open file from storage
	reader, err := s.storage.ReadFile(file.ID)
	if err != nil {
		return status.Error(codes.Internal, "failed to open file")
	}
	defer reader.Close()

	//  . Stream chunks to client
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

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  . Query database
	file, err := s.database.GetFile(ctx, req.FileId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "file not found")
	}

	//  . Return response
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
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  . Set reasonable defaults
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

	//  . Fetch from DB ( +1 to check if there's more)
	records, err := fs.database.ListFiles(ctx, req.UserId, limit+1, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list files")
	}

	//  . Build response
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

	//  . Next page token if needed
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
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  . Delete from storage first (fail-fast if file missing)
	file, err := fs.database.GetFile(ctx, req.FileId)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "database error")
	}

	//  . Ownership check
	if file.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "not owner")
	}

	//  . Delete from storage
	if err := fs.storage.DeleteFile(req.FileId); err != nil {
		// Log but don't fail — orphaned storage is better than orphaned DB
		log.Printf("Warning: failed to delete file from storage: %v", err)
	}

	//  . Soft-delete in DB
	if err := fs.database.DeleteFile(ctx, req.FileId, req.UserId); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete file metadata")
	}

	return &fileservicev1.DeleteFileResponse{
		Success: true,
		Message: "file deleted",
	}, nil
}
