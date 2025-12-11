package server

import (
	"context"
	"io"

	v1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/proto/fileservice/v1/fileService_grpc.v1.go"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fileServer struct {
	v1.UnimplementedFileServiceServer // Embeds for forward compatibility

	storage  StorageInterface  // Where files are stored
	database DatabaseInterface // Where metadata is stored
}

func NewFileServer(storage StorageInterface, db DatabaseInterface) *fileServer {
	return &fileServer{
		storage:  storage,
		database: db,
	}
}

func (s *fileServer) UploadFile(stream v1.FileService_UploadFileServer) error {

	// 1. Receive first message (metadata)
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "no metadata received")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
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
	return stream.SendAndClose(&v1.UploadFileResponse{
		FileId:   fileID,
		Filename: metadata.Filename,
		Size:     totalSize,
	})
}

func (s *fileServer) DownloadFile(req *v1.DownloadFileRequest, stream v1.FileService_DownloadFileServer) error {

	// 1. Get file metadata
	file, err := s.database.GetFile(stream.Context(), req.FileId)
	if err != nil {
		return status.Error(codes.NotFound, "file not found")
	}

	// 2. Send file info first
	err = stream.Send(&v1.DownloadFileResponse{
		Data: &v1.DownloadFileResponse_Info{
			Info: &v1.FileInfo{
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
	reader, err := s.storage.ReadFile(file.StoragePath)
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

		err = stream.Send(&v1.DownloadFileResponse{
			Data: &v1.DownloadFileResponse_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *fileServer) GetFileMetadata(ctx context.Context, req *v1.GetFileMetadataRequest) (*v1.GetFileMetadataResponse, error) {

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
	return &v1.GetFileMetadataResponse{
		FileId:      file.ID,
		Filename:    file.Name,
		ContentType: file.ContentType,
		Size:        file.Size,
		UploadedAt:  timestamppb.New(file.UploadedAt),
	}, nil
}

func (fs fileServer) ListFiles(ctx context.Context, req *v1.ListFilesRequest) (*v1.ListFilesResponse, error) {
	return nil, nil
}
func (fs fileServer) DeleteFile(ctx context.Context, req *v1.DeleteFileRequest) (*v1.DeleteFileResponse, error) {
	return nil, nil
}
