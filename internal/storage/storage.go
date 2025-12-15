package storage

import (
	"io"
	"os"
	"path/filepath"
)

// FilesystemStorage stores files on local disk
type FilesystemStorage struct {
	basePath string //  "./data/files"
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

func (fs *FilesystemStorage) ReadFile(fileID string) (io.ReadCloser, error) {
	filePath := filepath.Join(fs.basePath, fileID)
	return os.Open(filePath)
}

func (fs *FilesystemStorage) DeleteFile(fileID string) error {
	filePath := filepath.Join(fs.basePath, fileID)
	return os.Remove(filePath)
}
