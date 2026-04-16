-- Add suspension and session revocation controls for admin operations.

ALTER TABLE users ADD COLUMN is_suspended INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN suspended_reason TEXT;
ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 1;
