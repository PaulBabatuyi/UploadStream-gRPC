ALTER TABLE files ADD COLUMN file_type TEXT DEFAULT 'other';
CREATE INDEX idx_files_file_type ON files(file_type);