-- 008_file_expired_at.sql
-- Adds expired_at column so cleanup can mark file records when originals are purged.

ALTER TABLE files ADD COLUMN expired_at TEXT;
CREATE INDEX IF NOT EXISTS idx_files_expired_at ON files(expired_at);
