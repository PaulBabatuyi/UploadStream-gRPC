CREATE TABLE processing_jobs (
    id               BIGSERIAL PRIMARY KEY,
    file_id          UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    status           TEXT NOT NULL DEFAULT 'pending',
    
    retry_count      INT DEFAULT 0,
    max_retries      INT DEFAULT 3,
    error_message    TEXT,
    
    thumbnail_small  TEXT,    
    thumbnail_medium TEXT,    
    thumbnail_large  TEXT,    
    original_width   INT,
    original_height  INT,
    
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX idx_jobs_status ON processing_jobs(status, created_at);
CREATE INDEX idx_jobs_file_id ON processing_jobs(file_id);