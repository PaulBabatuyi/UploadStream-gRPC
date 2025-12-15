package database

import (
	"context"
	"database/sql"
	"time"

	fileservicev1 "github.com/PaulBabatuyi/UploadStream-gRPC/gen/fileservice/v1"
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

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) SaveFile(ctx context.Context, fileID string, metadata *fileservicev1.FileMetadata, size int64) error {
	query := `
        INSERT INTO files (id, user_id, filename, content_type, size, storage_path, uploaded_at, deleted_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `
	_, err := p.db.ExecContext(ctx, query,
		fileID,
		metadata.UserId,
		metadata.Filename,
		metadata.ContentType,
		size,
		fileID,
		time.Now(),
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
		return nil, err // File not found
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
