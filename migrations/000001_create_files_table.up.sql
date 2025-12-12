-- migrations/000001_create_files_table.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE files (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       TEXT NOT NULL,
    filename      TEXT NOT NULL,
    content_type  TEXT NOT NULL,
    size          BIGINT NOT NULL CHECK (size > 0),
    storage_path  TEXT NOT NULL,
    uploaded_at   TIMESTAMPTZ DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX idx_files_user_id ON files(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_files_uploaded_at ON files(uploaded_at DESC);