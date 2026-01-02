package service

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log"
	"strconv"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	maxFileSize  = 512 * 1024 * 1024 // 512MB (matches proto)
	maxChunkSize = 4 * 1024 * 1024   // 4MB per gRPC message limit
)

func NewFileServer(storage StorageInterface, db DatabaseInterface) *fileServer {
	return &fileServer{
		storage:   storage,
		database:  db,
		uploadSem: semaphore.NewWeighted(100),
	}
}

func (s *fileServer) UploadFile(stream pbv1.FileService_UploadFileServer) error {
	//  Receive metadata
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "no metadata received: %v", err)
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// Validate metadata
	if err := metadata.Validate(); err != nil {
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  Enforce size limit
	if metadata.Size > maxFileSize {
		return status.Errorf(codes.InvalidArgument,
			"file too large: %d bytes (max %d)", metadata.Size, maxFileSize)
	}

	// Create file in storage
	fileID := uuid.New().String()
	writer, err := s.storage.CreateFile(fileID)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create file: %v", err)
	}
	defer writer.Close()

	// Buffer for magic byte validation
	var firstChunk []byte
	validateMagicBytes := true

	ctx := stream.Context()
	//  Stream chunks with enforced limits
	totalSize := int64(0)
	for {
		// Check if context is canceled before receiving
		select {
		case <-ctx.Done():
			return status.Errorf(codes.Canceled, "upload canceled: %v", ctx.Err())
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Clean up on failure
			s.storage.DeleteFile(fileID)
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		chunk := msg.GetChunk()
		// Validate magic bytes on first chunk
		if validateMagicBytes && len(chunk) > 0 {
			firstChunk = chunk
			if len(firstChunk) >= 512 || totalSize+int64(len(chunk)) == metadata.Size {
				reader := bytes.NewReader(firstChunk)
				if err := ValidateContentType(reader, metadata.ContentType); err != nil {
					s.storage.DeleteFile(fileID)
					return status.Errorf(codes.InvalidArgument, "invalid file: %v", err)
				}
			}
			validateMagicBytes = false
		}

		chunkLen := int64(len(chunk))

		// Check chunk size
		if chunkLen > maxChunkSize {
			s.storage.DeleteFile(fileID)
			return status.Errorf(codes.InvalidArgument,
				"chunk too large: %d bytes (max %d)", chunkLen, maxChunkSize)
		}

		// Check total size doesn't exceed declared size
		if totalSize+chunkLen > metadata.Size {
			s.storage.DeleteFile(fileID)
			return status.Errorf(codes.InvalidArgument,
				"received %d bytes, expected %d", totalSize+chunkLen, metadata.Size)
		}

		// Write chunk
		n, err := writer.Write(chunk)
		if err != nil {
			s.storage.DeleteFile(fileID)
			return status.Errorf(codes.Internal, "failed to write chunk: %v", err)
		}
		totalSize += int64(n)
	}

	// Verify final size matches declared size
	if totalSize != metadata.Size {
		s.storage.DeleteFile(fileID)
		return status.Errorf(codes.InvalidArgument,
			"size mismatch: received %d bytes, expected %d", totalSize, metadata.Size)
	}

	//  Save metadata to database
	if err := s.database.SaveFile(stream.Context(), fileID, metadata, totalSize); err != nil {
		s.storage.DeleteFile(fileID)
		return status.Errorf(codes.Internal, "failed to save metadata: %v", err)
	}

	// Create processing job
	if _, err := s.database.CreateProcessingJob(stream.Context(), fileID); err != nil {
		// Non-fatal: log warning
		log.Printf("Warning: failed to create processing job: %v\n", err)
	}

	//  Send response
	return stream.SendAndClose(&pbv1.UploadFileResponse{
		FileId:           fileID,
		Filename:         metadata.Filename,
		Size:             totalSize,
		ProcessingStatus: pbv1.ProcessingStatus_PROCESSING_STATUS_PENDING,
	})
}

func (s *fileServer) DownloadFile(req *pbv1.DownloadFileRequest, stream pbv1.FileService_DownloadFileServer) error {
	ctx := stream.Context()

	// Validate request
	if err := req.Validate(); err != nil {
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  Get file metadata
	file, err := s.database.GetFile(stream.Context(), req.FileId)
	if err != nil {
		return status.Errorf(codes.NotFound, "file not found: %v", err)
	}

	//  . Send file info first
	err = stream.Send(&pbv1.DownloadFileResponse{
		Data: &pbv1.DownloadFileResponse_Info{
			Info: &pbv1.FileInfo{
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
		return status.Errorf(codes.Internal, "failed to open file: %v", err)
	}
	defer reader.Close()

	//  . Stream chunks to client
	buffer := make([]byte, 64*1024) // 64KB chunks
	for {
		select {
		case <-ctx.Done():
			return status.Errorf(codes.Canceled, "download canceled: %v", ctx.Err())
		default:
		}

		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read file: %v", err)
		}

		err = stream.Send(&pbv1.DownloadFileResponse{
			Data: &pbv1.DownloadFileResponse_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *fileServer) GetFileMetadata(ctx context.Context, req *pbv1.GetFileMetadataRequest) (*pbv1.GetFileMetadataResponse, error) {

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  Query database
	file, err := s.database.GetFile(ctx, req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
	}

	//  Get processing job
	job, err := s.database.GetJobByFileID(ctx, req.FileId)
	processingStatus := pbv1.ProcessingStatus_PROCESSING_STATUS_PENDING
	var processingResult *pbv1.ProcessingResult

	if err == nil && job != nil {
		// Map job status to proto enum
		switch job.Status {
		case "completed":
			processingStatus = pbv1.ProcessingStatus_PROCESSING_STATUS_COMPLETED
			processingResult = &pbv1.ProcessingResult{
				ThumbnailSmall:  job.ThumbnailSmall,
				ThumbnailMedium: job.ThumbnailMedium,
				ThumbnailLarge:  job.ThumbnailLarge,
				OriginalWidth:   int32(job.OriginalWidth),
				OriginalHeight:  int32(job.OriginalHeight),
			}
		case "processing":
			processingStatus = pbv1.ProcessingStatus_PROCESSING_STATUS_PROCESSING
		case "failed":
			processingStatus = pbv1.ProcessingStatus_PROCESSING_STATUS_FAILED
			processingResult = &pbv1.ProcessingResult{
				ErrorMessage: job.ErrorMessage,
			}
		}
	}

	return &pbv1.GetFileMetadataResponse{
		FileId:           file.ID,
		Filename:         file.Name,
		ContentType:      file.ContentType,
		Size:             file.Size,
		UploadedAt:       timestamppb.New(file.UploadedAt),
		ProcessingStatus: processingStatus,
		ProcessingResult: processingResult,
	}, nil
}

func (fs *fileServer) ListFiles(ctx context.Context, req *pbv1.ListFilesRequest) (*pbv1.ListFilesResponse, error) {
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
		return nil, status.Errorf(codes.Internal, "failed to list files: %v", err)
	}

	//  . Build response
	var entries []*pbv1.FileEntry
	for i, rec := range records {
		if i == limit {
			// this is the "has more" marker
			break
		}
		entries = append(entries, &pbv1.FileEntry{
			FileId:      rec.ID,
			Filename:    rec.Name,
			ContentType: rec.ContentType,
			Size:        rec.Size,
			UploadedAt:  timestamppb.New(rec.UploadedAt),
			// placeholder for now
			ProcessingStatus: pbv1.ProcessingStatus_PROCESSING_STATUS_COMPLETED,
		})
	}

	//  . Next page token if needed
	nextToken := ""
	if len(records) > limit {
		nextToken = strconv.Itoa(offset + limit)
	}

	return &pbv1.ListFilesResponse{
		Files:         entries,
		NextPageToken: nextToken,
	}, nil
}

func (fs *fileServer) DeleteFile(ctx context.Context, req *pbv1.DeleteFileRequest) (*pbv1.DeleteFileResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	//  . Delete from storage first (fail-fast if file missing)
	file, err := fs.database.GetFile(ctx, req.FileId)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	//  . Ownership check
	if file.UserID != req.UserId {
		return nil, status.Errorf(codes.PermissionDenied, "not owner: %v", err)
	}

	//  . Delete from storage
	if err := fs.storage.DeleteFile(req.FileId); err != nil {
		// Log but don't fail — orphaned storage is better than orphaned DB
		log.Printf("Warning: failed to delete file from storage: %v", err)
	}

	//  . Soft-delete in DB
	if err := fs.database.DeleteFile(ctx, req.FileId, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete file metadata: %v", err)
	}

	return &pbv1.DeleteFileResponse{
		Success: true,
		Message: "file deleted",
	}, nil
}
