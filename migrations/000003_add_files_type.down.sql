DROP INDEX IF EXISTS idx_files_file_type;
ALTER TABLE files DROP COLUMN IF EXISTS file_type;