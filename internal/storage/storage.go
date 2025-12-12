package storage

import (
	"io"
	"os"
	"path/filepath"
)

// StorageInterface defines how we store files
type StorageInterface interface {
	CreateFile(fileID string) (io.WriteCloser, error)
	ReadFile(path string) (io.ReadCloser, error)
	DeleteFile(path string) error
}

// FilesystemStorage stores files on local disk
type FilesystemStorage struct {
	basePath string // e.g., "./data/files"
}

func NewFilesystemStorage(basePath string) *FilesystemStorage {
	// Create directory if it doesn't exist
	os.MkdirAll(basePath, 0755)
	return &FilesystemStorage{basePath: basePath}
}

func (fs *FilesystemStorage) CreateFile(fileID string) (io.WriteCloser, error) {
	filePath := filepath.Join(fs.basePath, fileID)
	return os.Create(filePath)
}

func (fs *FilesystemStorage) ReadFile(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (fs *FilesystemStorage) DeleteFile(path string) error {
	return os.Remove(path)
}
