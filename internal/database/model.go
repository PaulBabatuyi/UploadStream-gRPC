package database

import (
	"strings"
	"time"
)

type FileRecord struct {
	ID          string
	UserID      string
	Name        string
	ContentType string
	Size        int64
	StoragePath string
	UploadedAt  time.Time
	DeletedAt   *time.Time
	FileType    FileType
}

type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeDocument FileType = "document"
	FileTypeOther    FileType = "other"
)

type ProcessingJob struct {
	ID              int64
	FileID          string
	Status          string
	RetryCount      int
	MaxRetries      int
	ErrorMessage    string
	ThumbnailSmall  string
	ThumbnailMedium string
	ThumbnailLarge  string
	OriginalWidth   int
	OriginalHeight  int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

func DeriveFileType(contentType string) FileType {
	if strings.HasPrefix(contentType, "image/") {
		return FileTypeImage
	}
	if strings.HasPrefix(contentType, "video/") {
		return FileTypeVideo
	}
	if strings.HasPrefix(contentType, "audio/") {
		return FileTypeAudio
	}
	if strings.Contains(contentType, "pdf") {
		return FileTypeDocument
	}
	return FileTypeOther
}
