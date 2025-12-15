# UploadStream-gRPC

A high-performance file upload/download service built with gRPC streaming in Go. This project demonstrates production-ready patterns for handling large file transfers using bidirectional streaming, PostgreSQL for metadata storage, and filesystem-based file storage.

## Features

- **Client-Streaming Upload**: Efficiently upload large files in chunks without loading entire files into memory
- **Server-Streaming Download**: Stream files back to clients with configurable chunk sizes
- **Metadata Management**: Store and retrieve file metadata with PostgreSQL
- **User File Listings**: Paginated file listings per user
- **Soft Delete**: Files are soft-deleted for potential recovery
- **Input Validation**: Comprehensive validation using protovalidate
- **Protocol Buffers**: Type-safe API definitions with buf.build tooling

## Tech Stack

- **Go 1.25+**: Modern Go with generics support
- **gRPC**: High-performance RPC framework
- **Protocol Buffers v3**: API schema and code generation
- **PostgreSQL**: Metadata storage with ACID guarantees
- **Buf**: Modern protobuf toolchain for linting and code generation

## Project Structure

```
.
├── fileservice/v1/          # Proto definitions
│   └── file_service.proto
├── gen/fileservice/v1/      # Generated Go code
├── cmd/
│   ├── server/             # gRPC server entrypoint
│   └── client/             # Example client
├── internal/
│   ├── database/           # PostgreSQL integration
│   ├── service/            # Business logic
│   └── storage/            # File storage interface
├── migrations/             # SQL migration files
├── buf.yaml                # Buf workspace config
└── buf.gen.yaml            # Code generation config
```

## Getting Started

### Prerequisites

- Go 1.25 or higher
- PostgreSQL 12+
- Buf CLI (for proto generation)

```bash
# Install buf
brew install bufbuild/buf/buf
# Or
go install github.com/bufbuild/buf/cmd/buf@latest
```

### Installation

1. Clone the repository:
```bash
git clone https://github.com/PaulBabatuyi/UploadStream-gRPC.git
cd UploadStream-gRPC
```

2. Install dependencies:
```bash
go mod download
```

3. Set up PostgreSQL database:
```bash
createdb uploadstream
```

4. Run migrations:
```bash
psql -d uploadstream -f migrations/000001_create_files_table.up.sql
```

5. Configure environment:
```bash
export UPLOADSTREAM="postgresql://user:password@localhost:5432/uploadstream?sslmode=disable"
```

### Running the Server

```bash
go run cmd/server/main.go
```

The server will start on `localhost:50051`.

### Running the Example Client

```bash
go run cmd/client/main.go
```

## API Reference

### UploadFile (Client Streaming)

Upload a file by streaming chunks to the server.

**Request Flow:**
1. First message: `FileMetadata` (filename, content_type, size, user_id)
2. Subsequent messages: `bytes chunk` (file data)

**Response:**
```protobuf
message UploadFileResponse {
  string file_id = 1;
  string filename = 2;
  int64 size = 3;
  string content_type = 4;
  google.protobuf.Timestamp uploaded_at = 5;
}
```

### DownloadFile (Server Streaming)

Download a file by receiving chunks from the server.

**Request:**
```protobuf
message DownloadFileRequest {
  string file_id = 1;
}
```

**Response Flow:**
1. First message: `FileInfo` (metadata)
2. Subsequent messages: `bytes chunk` (file data)

### GetFileMetadata (Unary)

Retrieve metadata for a specific file.

**Request:**
```protobuf
message GetFileMetadataRequest {
  string file_id = 1;
}
```

### ListFiles (Unary)

List files for a user with pagination.

**Request:**
```protobuf
message ListFilesRequest {
  string user_id = 1;
  int32 page_size = 2;  // 1-100, default 20
  string page_token = 3;
}
```

### DeleteFile (Unary)

Soft-delete a file (ownership check enforced).

**Request:**
```protobuf
message DeleteFileRequest {
  string file_id = 1;
  string user_id = 2;
}
```

## Development

### Regenerating Code from Proto

After modifying `.proto` files:

```bash
buf generate
```

This will regenerate:
- `gen/fileservice/v1/file_service.pb.go` (message types)
- `gen/fileservice/v1/file_service_grpc.pb.go` (service stubs)
- `gen/fileservice/v1/file_service.pb.validate.go` (validators)

### Linting Proto Files

```bash
buf lint
```

### Checking Breaking Changes

```bash
buf breaking --against '.git#branch=main'
```

## Configuration

### Environment Variables

- `UPLOADSTREAM`: PostgreSQL connection string (required)

### Storage Configuration

Files are stored in `./data/files` by default. Modify in `cmd/server/main.go`:

```go
storageLayer := storage.NewFilesystemStorage("./data/files")
```

## Validation Rules

The service enforces the following validations:

- **Filename**: 1-255 chars, alphanumeric with spaces, dots, hyphens, underscores
- **Content Type**: Must match pattern `type/subtype`
- **File Size**: 1 byte to 512 MB (configurable)
- **User ID**: Must be valid UUID
- **Page Size**: 1-100 files per page

## Architecture Decisions

### Why Client Streaming for Upload?

Client streaming allows the client to send large files in chunks without loading the entire file into memory on either side. The server can process chunks as they arrive, making it memory-efficient and suitable for large file transfers.

### Why Server Streaming for Download?

Server streaming prevents the server from loading entire files into memory. Files are read in chunks and streamed directly to the client, enabling efficient bandwidth utilization and support for partial downloads.

### Why Soft Delete?

Soft deletes (setting `deleted_at` timestamp) allow for file recovery and audit trails while keeping the data queryable. Physical deletion can be handled by a separate garbage collection process.

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...
```

## Performance Considerations

- **Chunk Size**: Default 64KB for downloads, configurable up to 4MB per gRPC message limits
- **Connection Pooling**: PostgreSQL connection pool managed by `database/sql`
- **Streaming**: Both upload and download use streaming to avoid memory spikes
- **Indexing**: Database indexes on `user_id` and `uploaded_at` for fast queries

## Future Enhancements (on it )

- [ ] S3-compatible storage backend
- [ ] Resume interrupted uploads
- [ ] File compression
- [ ] Image thumbnail generation
- [ ] Virus scanning integration
- [ ] Rate limiting per user
- [ ] Metrics and observability
- [ ] gRPC middleware (auth, logging, tracing)
- [ ] Base64 cursor-based pagination

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [gRPC](https://grpc.io/) for the excellent RPC framework
- [Buf](https://buf.build/) for modern protobuf tooling
- [Protocol Buffers](https://protobuf.dev/) for efficient serialization

## Contact

Paul Babatuyi - [@PaulBabatuyi](https://github.com/PaulBabatuyi)

Project Link: [https://github.com/PaulBabatuyi/UploadStream-gRPC](https://github.com/PaulBabatuyi/UploadStream-gRPC)
