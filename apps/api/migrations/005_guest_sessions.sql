ALTER TABLE files ADD COLUMN guest_session_id TEXT;

CREATE INDEX IF NOT EXISTS idx_files_guest_session_id ON files(guest_session_id);
