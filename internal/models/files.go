package models

import "time"

type FileRecord struct {
	ID          string
	UserID      string
	Name        string
	ContentType string
	Size        int64
	StoragePath string
	UploadedAt  time.Time
	DeletedAt   time.Time
}

// type FileType string
// const (
//     FileTypeImage   FileType = "image"
//     FileTypeVideo   FileType = "video"s
//     FileTypeAudio   FileType = "audio"
//     FileTypeDocument FileType = "document"
//     FileTypeOther   FileType = "other"
// )

// type FileUploadedEvent struct {
//     FileID       string
//     UserID       string
//     OriginalName string
//     Size         int64
//     ContentType  string    // "image/jpeg", "video/mp4", etc.
//     FileType     FileType  // derived from ContentType or magic bytes
//     TempPath     string
//     UploadedAt   time.Time
// }
