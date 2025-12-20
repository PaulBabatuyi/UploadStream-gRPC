package database

import (
	"context"
	"database/sql"
	"time"

	pbv1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
	_ "github.com/lib/pq"
)

type PostgresDB struct {
	db *sql.DB
}

func NewPostgresDB(connectionString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) SaveFile(ctx context.Context, fileID string, metadata *pbv1.FileMetadata, size int64) error {
	fileType := DeriveFileType(metadata.ContentType)

	query := `
        INSERT INTO files (id, user_id, filename, content_type, size, storage_path, uploaded_at, file_type, deleted_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
	_, err := p.db.ExecContext(ctx, query,
		fileID,
		metadata.UserId,
		metadata.Filename,
		metadata.ContentType,
		size,
		fileID,
		time.Now(),
		string(fileType),
		nil,
	)
	return err
}

func (p *PostgresDB) GetFile(ctx context.Context, fileID string) (*FileRecord, error) {
	query := `
        SELECT id, user_id, filename, content_type, size, storage_path, uploaded_at, deleted_at
        FROM files
        WHERE id = $1 AND deleted_at IS NULL
    `

	var file FileRecord
	err := p.db.QueryRowContext(ctx, query, fileID).Scan(
		&file.ID,
		&file.UserID,
		&file.Name,
		&file.ContentType,
		&file.Size,
		&file.StoragePath,
		&file.UploadedAt,
		&file.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, err
	}

	return &file, err
}

func (p *PostgresDB) ListFiles(ctx context.Context, userID string, limit, offset int) ([]*FileRecord, error) {
	query := `
        SELECT id, user_id, filename, content_type, size, storage_path, uploaded_at
        FROM files
        WHERE user_id = $1 AND deleted_at IS NULL
        ORDER BY uploaded_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := p.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*FileRecord
	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(&f.ID, &f.UserID, &f.Name, &f.ContentType, &f.Size, &f.StoragePath, &f.UploadedAt); err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	return files, rows.Err()
}

func (p *PostgresDB) DeleteFile(ctx context.Context, fileID, userID string) error {
	query := `
        UPDATE files 
        SET deleted_at = NOW() 
        WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
    `
	result, err := p.db.ExecContext(ctx, query, fileID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (p *PostgresDB) CreateProcessingJob(ctx context.Context, fileID string) (int64, error) {
	var jobID int64
	query := `
        INSERT INTO processing_jobs (file_id, status, retry_count, max_retries)
        VALUES ($1, 'pending', 0, 3)
        RETURNING id
    `
	err := p.db.QueryRowContext(ctx, query, fileID).Scan(&jobID)
	return jobID, err
}

func (p *PostgresDB) GetNextPendingJob(ctx context.Context) (*ProcessingJob, error) {
	query := `
        SELECT id, file_id, status, retry_count, max_retries, error_message
        FROM processing_jobs
        WHERE status = 'pending' AND retry_count < max_retries
        ORDER BY created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    `
	var job ProcessingJob
	err := p.db.QueryRowContext(ctx, query).Scan(
		&job.ID, &job.FileID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.ErrorMessage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &job, err
}

func (p *PostgresDB) UpdateJobStatus(ctx context.Context, jobID int64, status, errorMsg string) error {
	query := `
        UPDATE processing_jobs
        SET status = $1, error_message = $2, retry_count = retry_count + 1, updated_at = NOW()
        WHERE id = $3
    `
	_, err := p.db.ExecContext(ctx, query, status, errorMsg, jobID)
	return err
}

func (p *PostgresDB) CompleteJob(ctx context.Context, jobID int64, thumbSmall, thumbMed, thumbLarge string, width, height int) error {
	query := `
        UPDATE processing_jobs
        SET status = 'completed', thumbnail_small = $1, thumbnail_medium = $2, thumbnail_large = $3,
            original_width = $4, original_height = $5, completed_at = NOW(), updated_at = NOW()
        WHERE id = $6
    `
	_, err := p.db.ExecContext(ctx, query, thumbSmall, thumbMed, thumbLarge, width, height, jobID)
	return err
}

func (p *PostgresDB) GetJobByFileID(ctx context.Context, fileID string) (*ProcessingJob, error) {
	query := `
        SELECT id, file_id, status, error_message, thumbnail_small, thumbnail_medium, thumbnail_large,
               original_width, original_height
        FROM processing_jobs
        WHERE file_id = $1
    `
	var job ProcessingJob
	err := p.db.QueryRowContext(ctx, query, fileID).Scan(
		&job.ID, &job.FileID, &job.Status, &job.ErrorMessage, &job.ThumbnailSmall, &job.ThumbnailMedium,
		&job.ThumbnailLarge, &job.OriginalWidth, &job.OriginalHeight,
	)
	return &job, err
}
